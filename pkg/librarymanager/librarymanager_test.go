// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// failingStoreRemovalFs wraps an afero.Fs and forces RemoveAll to fail for
// any path under storePrefix. Used to exercise the cleanup-failed branch of
// tryCleanupLibrary without relying on POSIX permission bits (which root
// silently bypasses on CI).
type failingStoreRemovalFs struct {
	afero.Fs
	storePrefix string
}

func (f *failingStoreRemovalFs) RemoveAll(path string) error {
	if strings.HasPrefix(path, f.storePrefix) {
		return fmt.Errorf("simulated store removal failure")
	}
	return f.Fs.RemoveAll(path)
}

type testImage struct {
	tarPath string
	name    string
	tag     string
}

type testVolume struct {
	name          string
	version       string
	pull          bool
	volumeID      string
	expectedFiles []string
}

func TestLibraryManager(t *testing.T) {
	tests := map[string]struct {
		// images is a list of images to load into the registry.
		images []*testImage
		// volumes is a list of volumes to create and remove as part of the test.
		volumes []*testVolume
		// expectedManagerFiles is the list of files expected after volumes are setup but before they're deleted.
		expectedManagerFiles []string
	}{
		"a single volume sets up a single library": {
			images: []*testImage{
				{
					tarPath: "testdata/image.tar",
					name:    "test-image",
					tag:     "latest",
				},
			},
			volumes: []*testVolume{
				{
					name:     "test-image",
					version:  "latest",
					pull:     false,
					volumeID: "test-volume-001",
					expectedFiles: []string{
						"datadog-init/package/library.txt",
					},
				},
			},
			expectedManagerFiles: []string{
				"db/datadog-csi-driver.db",
				"store/56275150d5d94778425fc2fd850ff88c28e1d478e3812fa1255aed86ab9c143e/datadog-init/package/library.txt",
			},
		},
		"multiple volumes for the same library maintains a single library in the store": {
			images: []*testImage{
				{
					tarPath: "testdata/image.tar",
					name:    "test-image",
					tag:     "latest",
				},
			},
			volumes: []*testVolume{
				{
					name:     "test-image",
					version:  "latest",
					pull:     false,
					volumeID: "test-volume-001",
					expectedFiles: []string{
						"datadog-init/package/library.txt",
					},
				},
				{
					name:     "test-image",
					version:  "latest",
					pull:     false,
					volumeID: "test-volume-002",
					expectedFiles: []string{
						"datadog-init/package/library.txt",
					},
				},
			},
			expectedManagerFiles: []string{
				"db/datadog-csi-driver.db",
				"store/56275150d5d94778425fc2fd850ff88c28e1d478e3812fa1255aed86ab9c143e/datadog-init/package/library.txt",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup local registry
			localRegistry := testutil.NewLocalRegistry(t)
			defer localRegistry.Stop()
			for _, img := range test.images {
				localRegistry.AddImage(t, img.tarPath, img.name, img.tag)
			}

			// Create downloader.
			d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))

			// Create scratch space.
			tsd := testutil.NewTempScratchDirectory(t)
			defer tsd.Cleanup(t)
			basePath := tsd.Path(t)

			// Setup library manager.
			ctx := context.Background()
			lm, err := librarymanager.NewLibraryManager(basePath,
				librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
				librarymanager.WithDownloader(d),
			)
			require.NoError(t, err)
			defer func() {
				err := lm.Stop()
				require.NoError(t, err)
			}()

			// Setup all volumes and ensure they have expected files.
			for _, volume := range test.volumes {
				// Get library for the volume.
				lib := createTestLibrary(t, volume, localRegistry.Registry(t))
				path, err := lm.GetLibraryForVolume(ctx, volume.volumeID, lib)
				require.NoError(t, err)

				// Ensure the volume path returned contains the expected files.
				actualFiles := testutil.ListFiles(t, path)
				for _, expected := range volume.expectedFiles {
					require.Contains(t, actualFiles, expected)
				}
			}

			// Ensure the manager file system contains the expected files.
			actualFiles := testutil.ListFiles(t, tsd.Path(t))
			for _, expected := range test.expectedManagerFiles {
				require.Contains(t, actualFiles, expected)
			}

			// Delete the volumes.
			for _, volume := range test.volumes {
				err = lm.RemoveVolume(ctx, volume.volumeID)
				require.NoError(t, err)
			}

			// Ensure the store is empty.
			actualFiles = testutil.ListFiles(t, filepath.Join(tsd.Path(t), librarymanager.StoreDirectory))
			require.Empty(t, actualFiles)
		})
	}
}

func createTestLibrary(t *testing.T, tl *testVolume, registry string) *librarymanager.Library {
	t.Helper()
	lib, err := librarymanager.NewLibrary(tl.name, registry, tl.version, tl.pull)
	require.NoError(t, err)
	return lib
}

// TestLibraryManagerStartupResync ensures the manager can be created on top of a
// database that already contains links (as happens after a driver restart) and
// that the persisted package names are preserved.
func TestLibraryManagerStartupResync(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)
	basePath := tsd.Path(t)

	// Pre-populate the database with two libraries and three linked volumes,
	// covering two packages.
	dbDir := filepath.Join(basePath, librarymanager.DatabaseDirectory)
	require.NoError(t, os.MkdirAll(dbDir, 0o755))
	db, err := librarymanager.NewDatabase(dbDir)
	require.NoError(t, err)
	require.NoError(t, db.AddLibrary("lib-java", "dd-lib-java-init", 0))
	require.NoError(t, db.AddLibrary("lib-php", "dd-lib-php-init", 0))
	require.NoError(t, db.LinkVolume("lib-java", "vol-1"))
	require.NoError(t, db.LinkVolume("lib-java", "vol-2"))
	require.NoError(t, db.LinkVolume("lib-php", "vol-3"))
	require.NoError(t, db.Close())

	// Bringing up a manager on the same base path must succeed; this also
	// resynchronizes the library_volume_links gauge in the background.
	lm, err := librarymanager.NewLibraryManager(basePath,
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, lm.Stop())
	}()

	// The persisted volume links remain visible after restart.
	for _, volumeID := range []string{"vol-1", "vol-2", "vol-3"} {
		has, err := lm.HasVolume(volumeID)
		require.NoError(t, err)
		require.Truef(t, has, "expected %s to still be tracked after manager restart", volumeID)
	}
}

// fakeListener is a thread-safe librarymanager.EventListener that records
// every event in the order it is observed. Used by the integration test
// below to assert the manager publishes the expected sequence end-to-end.
type fakeListener struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeListener) record(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, s)
}

func (f *fakeListener) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.events))
	copy(out, f.events)
	return out
}

func (f *fakeListener) OnLibraryResolved(r librarymanager.LibraryResolutionResult) {
	f.record(fmt.Sprintf("resolved=%s", r))
}
func (f *fakeListener) OnLibraryDownload(library, _ string, _ time.Duration) {
	f.record(fmt.Sprintf("download=%s", library))
}
func (f *fakeListener) OnLibraryCleanup(s librarymanager.LibraryCleanupStatus, _ string) {
	f.record(fmt.Sprintf("cleanup=%s", s))
}
func (f *fakeListener) OnVolumeLinked(pkg string, n int) {
	f.record(fmt.Sprintf("linked=%s/%d", pkg, n))
}
func (f *fakeListener) OnVolumeUnlinked(pkg string, n int) {
	f.record(fmt.Sprintf("unlinked=%s/%d", pkg, n))
}
func (f *fakeListener) OnLibraryCached(pkg string, count int, _ int64) {
	f.record(fmt.Sprintf("cached=%s/%d", pkg, count))
}
func (f *fakeListener) OnLibraryEvicted(pkg string, count int, _ int64) {
	f.record(fmt.Sprintf("evicted=%s/%d", pkg, count))
}
func (f *fakeListener) OnSnapshot(s librarymanager.Snapshot) {
	f.record(fmt.Sprintf("snapshot=%d/%d/%d",
		len(s.VolumeLinksByPackage), len(s.CachedCountByPackage), len(s.CachedBytesByPackage)))
}

// TestLibraryManagerEventListenerSequence wires a fake listener into a real
// manager backed by a local registry and asserts every lifecycle event is
// published in the expected order across a download, a cache hit, and two
// cleanup paths (skipped_in_use then success).
func TestLibraryManagerEventListenerSequence(t *testing.T) {
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	localRegistry.AddImage(t, "testdata/image.tar", "test-image", "latest")

	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	listener := &fakeListener{}
	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithDownloader(librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))),
		librarymanager.WithEventListener(listener),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()

	// OnSnapshot must fire at startup, even on an empty database.
	require.Equal(t, []string{"snapshot=0/0/0"}, listener.snapshot(),
		"OnSnapshot must be invoked exactly once at NewLibraryManager")

	lib, err := librarymanager.NewLibrary("test-image", localRegistry.Registry(t), "latest", false)
	require.NoError(t, err)

	ctx := context.Background()

	// First mount: download → cached → linked → resolved.
	_, err = lm.GetLibraryForVolume(ctx, "vol-1", lib)
	require.NoError(t, err)

	// Second mount on the same library: cache hit → linked → resolved.
	_, err = lm.GetLibraryForVolume(ctx, "vol-2", lib)
	require.NoError(t, err)

	// First unlink: should NOT trigger eviction since vol-2 still uses
	// the library; cleanup must report skipped_in_use.
	require.NoError(t, lm.RemoveVolume(ctx, "vol-1"))

	// Second unlink: the library is now orphan, eviction fires.
	require.NoError(t, lm.RemoveVolume(ctx, "vol-2"))

	got := listener.snapshot()
	require.Equal(t, []string{
		"snapshot=0/0/0",
		"download=test-image",
		"cached=test-image/1",
		"linked=test-image/1",
		"resolved=downloaded",
		"linked=test-image/2",
		"resolved=cache_hit",
		"unlinked=test-image/1",
		"cleanup=skipped_in_use",
		"unlinked=test-image/0",
		"evicted=test-image/0",
		"cleanup=success",
	}, got, "the listener must observe the full lifecycle in the expected order")
}

// TestLibraryManagerGetLibraryForVolumeValidatesInputs covers the two
// "user error" branches that the production wiring should never hit but the
// API still has to guard.
func TestLibraryManagerGetLibraryForVolumeValidatesInputs(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()

	_, err = lm.GetLibraryForVolume(context.Background(), "", &librarymanager.Library{})
	require.Error(t, err, "empty volumeID must be rejected")

	_, err = lm.GetLibraryForVolume(context.Background(), "vol", nil)
	require.Error(t, err, "nil library must be rejected")
}

// TestLibraryManagerRemoveVolumeIsNoopForUnknownVolume covers the
// NodeUnpublish-after-failed-NodePublish path: kubelet calls RemoveVolume
// on a volume that was never successfully linked, and we must not error.
func TestLibraryManagerRemoveVolumeIsNoopForUnknownVolume(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	listener := &fakeListener{}
	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithEventListener(listener),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()

	require.NoError(t, lm.RemoveVolume(context.Background(), "vol-never-linked"),
		"RemoveVolume on an unknown volume must be a no-op, not an error")

	// Beyond the startup snapshot, nothing should have been emitted.
	require.Equal(t, []string{"snapshot=0/0/0"}, listener.snapshot(),
		"a no-op RemoveVolume must not publish any lifecycle event")
}

// TestLibraryManagerWithCleanupStrategyAndEventListener exercises both
// options in isolation by constructing a manager with non-default values
// and asserting the options were applied.
func TestLibraryManagerWithCleanupStrategyAndEventListener(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	listener := &fakeListener{}
	strategy := librarymanager.NewDelayedCleanupStrategy(50 * time.Millisecond)

	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithCleanupStrategy(strategy),
		librarymanager.WithEventListener(listener),
	)
	require.NoError(t, err)

	// Listener wiring is confirmed by the OnSnapshot recorded at startup.
	require.Equal(t, []string{"snapshot=0/0/0"}, listener.snapshot())

	// CleanupStrategy wiring is confirmed by Stop draining the strategy
	// (DelayedCleanupStrategy.Stop is the only way Stop can fail to be a
	// no-op).
	require.NoError(t, lm.Stop())
}

// TestLibraryManagerCleanupFailedWhenStoreRemoveFails surfaces the
// LibraryCleanupFailed branch of tryCleanupLibrary by wrapping the
// filesystem so RemoveAll fails for paths under the store directory. We
// avoid POSIX chmod because root (the user the tests run as on CI)
// silently ignores permission bits and the cleanup would succeed anyway.
func TestLibraryManagerCleanupFailedWhenStoreRemoveFails(t *testing.T) {
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	localRegistry.AddImage(t, "testdata/image.tar", "test-image", "latest")

	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	storeDir := filepath.Join(tsd.Path(t), librarymanager.StoreDirectory)
	fs := afero.Afero{Fs: &failingStoreRemovalFs{Fs: afero.NewOsFs(), storePrefix: storeDir}}

	listener := &fakeListener{}
	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(fs),
		librarymanager.WithDownloader(librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))),
		librarymanager.WithEventListener(listener),
	)
	require.NoError(t, err)
	defer func() { _ = lm.Stop() }()

	lib, err := librarymanager.NewLibrary("test-image", localRegistry.Registry(t), "latest", false)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = lm.GetLibraryForVolume(ctx, "vol-1", lib)
	require.NoError(t, err)

	require.NoError(t, lm.RemoveVolume(ctx, "vol-1"),
		"RemoveVolume itself succeeds; only the async cleanup fails")

	require.Contains(t, listener.snapshot(), "cleanup=failed",
		"the cleanup-failed event must reach the listener so dashboards can alert")
}
