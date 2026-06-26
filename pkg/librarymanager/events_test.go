// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

// recordedEvent is a flattened record of a single Listener call. Only the
// fields relevant to the invoked method are populated.
type recordedEvent struct {
	kind     string
	library  string
	result   libraryevents.ResolutionResult
	status   libraryevents.CleanupStatus
	strategy string
	registry string
	duration time.Duration
	count    int
	bytes    int64
	links    int
	snapshot libraryevents.Snapshot
}

// recordingListener captures every Listener call so tests can assert which
// events the LibraryManager emits, in which order, and with which arguments.
type recordingListener struct {
	mu     sync.Mutex
	events []recordedEvent
}

// compile-time check that the test double satisfies the interface.
var _ libraryevents.Listener = (*recordingListener)(nil)

func (r *recordingListener) record(e recordedEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

// drain returns every event captured since the last drain and clears the
// buffer, so each step of a test can assert only the events it triggered.
func (r *recordingListener) drain() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.events
	r.events = nil
	return out
}

func (r *recordingListener) OnLibraryResolved(library string, result libraryevents.ResolutionResult) {
	r.record(recordedEvent{kind: "resolved", library: library, result: result})
}

func (r *recordingListener) OnLibraryDownload(library, registry string, d time.Duration) {
	r.record(recordedEvent{kind: "download", library: library, registry: registry, duration: d})
}

func (r *recordingListener) OnLibraryCleanup(library string, status libraryevents.CleanupStatus, strategy string) {
	r.record(recordedEvent{kind: "cleanup", library: library, status: status, strategy: strategy})
}

func (r *recordingListener) OnLibraryCached(library string, count int, bytes int64) {
	r.record(recordedEvent{kind: "cached", library: library, count: count, bytes: bytes})
}

func (r *recordingListener) OnLibraryEvicted(library string, count int, bytes int64) {
	r.record(recordedEvent{kind: "evicted", library: library, count: count, bytes: bytes})
}

func (r *recordingListener) OnVolumeLinked(library string, links int) {
	r.record(recordedEvent{kind: "linked", library: library, links: links})
}

func (r *recordingListener) OnVolumeUnlinked(library string, links int) {
	r.record(recordedEvent{kind: "unlinked", library: library, links: links})
}

func (r *recordingListener) OnSnapshot(s libraryevents.Snapshot) {
	r.record(recordedEvent{kind: "snapshot", snapshot: s})
}

func eventsOfKind(events []recordedEvent, kind string) []recordedEvent {
	var out []recordedEvent
	for _, e := range events {
		if e.kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// singleEvent asserts exactly one event of the given kind was recorded and
// returns it.
func singleEvent(t *testing.T, events []recordedEvent, kind string) recordedEvent {
	t.Helper()
	matched := eventsOfKind(events, kind)
	require.Lenf(t, matched, 1, "expected exactly one %q event, got %d in %+v", kind, len(matched), events)
	return matched[0]
}

// TestLibraryManagerEmitsLifecycleEvents walks a full download -> share ->
// release lifecycle for two volumes on the same library and asserts the
// listener sees the right events at each step. It also covers the
// "skipped_in_use" cleanup path (the library must stay on disk while a second
// volume still uses it).
func TestLibraryManagerEmitsLifecycleEvents(t *testing.T) {
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	localRegistry.AddImage(t, "testdata/image.tar", "test-image", "latest")

	d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)
	basePath := tsd.Path(t)
	ctx := context.Background()

	rec := &recordingListener{}
	lm, err := librarymanager.NewLibraryManager(basePath,
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithDownloader(d),
		librarymanager.WithEventListener(rec),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()

	// Construction seeds an (empty) snapshot.
	require.Empty(t, singleEvent(t, rec.drain(), "snapshot").snapshot.CachedCountByLibrary)

	lib, err := librarymanager.NewLibrary("test-image", localRegistry.Registry(t), "latest", false)
	require.NoError(t, err)

	// First volume: cache miss -> download.
	_, err = lm.GetLibraryForVolume(ctx, "vol-1", lib)
	require.NoError(t, err)
	first := rec.drain()
	require.Equal(t, libraryevents.ResolutionDownloaded, singleEvent(t, first, "resolved").result)
	require.Equal(t, "test-image", singleEvent(t, first, "download").library)
	cached := singleEvent(t, first, "cached")
	require.Equal(t, 1, cached.count)
	require.Greater(t, cached.bytes, int64(0))
	require.Equal(t, 1, singleEvent(t, first, "linked").links)

	// Second volume, same library: cache hit, no new download/cached.
	_, err = lm.GetLibraryForVolume(ctx, "vol-2", lib)
	require.NoError(t, err)
	second := rec.drain()
	require.Equal(t, libraryevents.ResolutionCacheHit, singleEvent(t, second, "resolved").result)
	require.Equal(t, 2, singleEvent(t, second, "linked").links)
	require.Empty(t, eventsOfKind(second, "download"), "a cache hit must not download")
	require.Empty(t, eventsOfKind(second, "cached"), "a cache hit must not re-cache")

	// Remove the first of two volumes: unlink, cleanup skipped (still in use),
	// the library must remain on disk.
	require.NoError(t, lm.RemoveVolume(ctx, "vol-1"))
	afterFirst := rec.drain()
	require.Equal(t, 1, singleEvent(t, afterFirst, "unlinked").links)
	require.Equal(t, libraryevents.CleanupSkippedInUse, singleEvent(t, afterFirst, "cleanup").status)
	require.Empty(t, eventsOfKind(afterFirst, "evicted"), "a library still in use must not be evicted")
	require.NotEmpty(t, testutil.ListFiles(t, filepath.Join(basePath, librarymanager.StoreDirectory)),
		"the library should remain on disk while a volume still uses it")

	// Remove the last volume: unlink, eviction, cleanup success, library gone.
	require.NoError(t, lm.RemoveVolume(ctx, "vol-2"))
	afterLast := rec.drain()
	require.Equal(t, 0, singleEvent(t, afterLast, "unlinked").links)
	evicted := singleEvent(t, afterLast, "evicted")
	require.Equal(t, 0, evicted.count)
	require.Equal(t, int64(0), evicted.bytes)
	require.Equal(t, libraryevents.CleanupSuccess, singleEvent(t, afterLast, "cleanup").status)
	require.Empty(t, testutil.ListFiles(t, filepath.Join(basePath, librarymanager.StoreDirectory)),
		"the library should be removed once unused")
}

// TestLibraryManagerReusesLibraryForRepublishedVolume verifies that
// re-publishing an already-linked volume reuses the library it was first linked
// to, without re-resolving the image. Even when the (mutable) tag no longer
// resolves, the volume keeps its original library: the call succeeds with a
// cache hit and records no extra download, cache, or link.
func TestLibraryManagerReusesLibraryForRepublishedVolume(t *testing.T) {
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	localRegistry.AddImage(t, "testdata/image.tar", "test-image", "latest")

	d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)
	ctx := context.Background()

	rec := &recordingListener{}
	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithDownloader(d),
		librarymanager.WithEventListener(rec),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()
	rec.drain() // discard the startup snapshot

	lib, err := librarymanager.NewLibrary("test-image", localRegistry.Registry(t), "latest", false)
	require.NoError(t, err)

	// First publish: cache miss -> download and link.
	firstPath, err := lm.GetLibraryForVolume(ctx, "vol-1", lib)
	require.NoError(t, err)
	first := rec.drain()
	require.Equal(t, libraryevents.ResolutionDownloaded, singleEvent(t, first, "resolved").result)
	require.Equal(t, 1, singleEvent(t, first, "linked").links)

	// Re-publish the same volume with a tag that no longer resolves (pull
	// forced, so a resolution attempt would fail). The volume is already
	// linked, so the manager must reuse its library without touching the
	// registry: no error, no download, no new link.
	stale, err := librarymanager.NewLibrary("test-image", localRegistry.Registry(t), "does-not-exist", true)
	require.NoError(t, err)
	reusedPath, err := lm.GetLibraryForVolume(ctx, "vol-1", stale)
	require.NoError(t, err)
	require.Equal(t, firstPath, reusedPath, "a re-published volume must reuse its original library path")

	second := rec.drain()
	require.Equal(t, libraryevents.ResolutionCacheHit, singleEvent(t, second, "resolved").result)
	require.Empty(t, eventsOfKind(second, "download"), "reusing a linked volume must not download")
	require.Empty(t, eventsOfKind(second, "cached"), "reusing a linked volume must not re-cache")
	require.Empty(t, eventsOfKind(second, "linked"), "reusing a linked volume must not add a link")

	// The library was referenced exactly once, so removing the volume evicts it.
	require.NoError(t, lm.RemoveVolume(ctx, "vol-1"))
	afterRemove := rec.drain()
	require.Equal(t, 0, singleEvent(t, afterRemove, "unlinked").links)
	require.Equal(t, libraryevents.CleanupSuccess, singleEvent(t, afterRemove, "cleanup").status)
}

// TestLibraryManagerEmitsFailedResolution checks that a resolution failure
// (here, an image that cannot be resolved) surfaces as a "failed" resolution
// and does not link a volume.
func TestLibraryManagerEmitsFailedResolution(t *testing.T) {
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	// No image is added, so the digest lookup fails.

	d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	rec := &recordingListener{}
	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithDownloader(d),
		librarymanager.WithEventListener(rec),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()
	rec.drain() // discard the startup snapshot

	lib, err := librarymanager.NewLibrary("missing-image", localRegistry.Registry(t), "v0.0.0", true)
	require.NoError(t, err)

	_, err = lm.GetLibraryForVolume(context.Background(), "vol-x", lib)
	require.Error(t, err)
	events := rec.drain()
	require.Equal(t, libraryevents.ResolutionFailed, singleEvent(t, events, "resolved").result)
	require.Empty(t, eventsOfKind(events, "linked"), "a failed resolution must not link a volume")
	require.Empty(t, eventsOfKind(events, "cached"), "a failed resolution must not cache a library")
}

// TestLibraryManagerSeedsSnapshotOnRestart verifies that a freshly built
// LibraryManager seeds its listener from the persisted state, so gauges are
// correct immediately after a driver restart.
func TestLibraryManagerSeedsSnapshotOnRestart(t *testing.T) {
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	localRegistry.AddImage(t, "testdata/image.tar", "test-image", "latest")
	d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))

	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)
	basePath := tsd.Path(t)
	ctx := context.Background()

	// First run: download and link a volume, then stop.
	lm, err := librarymanager.NewLibraryManager(basePath,
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithDownloader(d),
	)
	require.NoError(t, err)
	lib, err := librarymanager.NewLibrary("test-image", localRegistry.Registry(t), "latest", false)
	require.NoError(t, err)
	_, err = lm.GetLibraryForVolume(ctx, "vol-1", lib)
	require.NoError(t, err)
	require.NoError(t, lm.Stop())

	// Second run: a fresh listener must be seeded from the persisted state.
	rec := &recordingListener{}
	lm2, err := librarymanager.NewLibraryManager(basePath,
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithDownloader(d),
		librarymanager.WithEventListener(rec),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm2.Stop()) }()

	snap := singleEvent(t, rec.drain(), "snapshot").snapshot
	require.Equal(t, 1, snap.CachedCountByLibrary["test-image"])
	require.Equal(t, 1, snap.VolumeLinksByLibrary["test-image"])
	require.Greater(t, snap.CachedBytesByLibrary["test-image"], int64(0))
}

// TestLibraryManagerRemoveUnknownVolumeIsNoop checks that removing a volume
// that was never linked is a no-op that emits no events.
func TestLibraryManagerRemoveUnknownVolumeIsNoop(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	rec := &recordingListener{}
	lm, err := librarymanager.NewLibraryManager(tsd.Path(t),
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithEventListener(rec),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()
	rec.drain() // discard the startup snapshot

	require.NoError(t, lm.RemoveVolume(context.Background(), "never-linked"))
	require.Empty(t, rec.drain(), "removing an untracked volume must emit no events")
}

// TestLibraryManagerCleanupEmitsEmptyLabelForMigratedLibrary covers a library
// migrated from the legacy schema without per-library metadata: its cleanup
// must still fire (with an empty package label) and must not publish gauge
// events, since it was never tracked as a labelled series.
func TestLibraryManagerCleanupEmitsEmptyLabelForMigratedLibrary(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)
	basePath := tsd.Path(t)

	const (
		libraryID = "legacy-lib-id"
		volumeID  = "legacy-vol-id"
	)

	// Seed a legacy-schema DB (a link without metadata) plus the matching
	// store directory on disk.
	dbDir := filepath.Join(basePath, librarymanager.DatabaseDirectory)
	require.NoError(t, os.MkdirAll(dbDir, 0o755))
	seedLegacyLink(t, filepath.Join(dbDir, librarymanager.DatabaseFileName), libraryID, volumeID)

	storeLibDir := filepath.Join(basePath, librarymanager.StoreDirectory, libraryID)
	require.NoError(t, os.MkdirAll(storeLibDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(storeLibDir, "library.txt"), []byte("x"), 0o644))

	rec := &recordingListener{}
	lm, err := librarymanager.NewLibraryManager(basePath,
		librarymanager.WithFilesystem(afero.Afero{Fs: afero.NewOsFs()}),
		librarymanager.WithEventListener(rec),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, lm.Stop()) }()
	rec.drain() // a migrated library has no package, so the seed snapshot is empty

	require.NoError(t, lm.RemoveVolume(context.Background(), volumeID))
	events := rec.drain()

	cleanup := singleEvent(t, events, "cleanup")
	require.Equal(t, libraryevents.CleanupSuccess, cleanup.status)
	require.Empty(t, cleanup.library, "a migrated library without metadata is cleaned up with an empty label")
	require.Empty(t, eventsOfKind(events, "evicted"), "no eviction gauge for a library without a package label")
	require.Empty(t, eventsOfKind(events, "unlinked"), "no unlink gauge for a library without a package label")
	require.Empty(t, testutil.ListFiles(t, filepath.Join(basePath, librarymanager.StoreDirectory)))
}

// seedLegacyLink writes a single volume->library link into a legacy-schema
// bbolt database (the nested library-mappings bucket).
func seedLegacyLink(t *testing.T, dbPath, libraryID, volumeID string) {
	t.Helper()
	db, err := bbolt.Open(dbPath, 0600, nil)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		mappings, err := tx.CreateBucketIfNotExists([]byte("library-mappings"))
		if err != nil {
			return err
		}
		libBkt, err := mappings.CreateBucketIfNotExists([]byte(libraryID))
		if err != nil {
			return err
		}
		return libBkt.Put([]byte(volumeID), []byte("{}"))
	}))
	require.NoError(t, db.Close())
}
