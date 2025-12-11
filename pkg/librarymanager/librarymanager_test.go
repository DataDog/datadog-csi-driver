// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/stretchr/testify/require"
)

type testImage struct {
	tarPath string
	name    string
	tag     string
}

type testVolume struct {
	name          string
	version       string
	path          string
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
					path:     "/data/datadog-init/package",
					pull:     false,
					volumeID: "test-volume-001",
					expectedFiles: []string{
						"library.txt",
					},
				},
			},
			expectedManagerFiles: []string{
				"db/datadog-csi-driver.db",
				"store/32ea291b55c8556199ec22906034cc296f20ae69866f8c8031aecb7d9fd765b8/library.txt",
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
					path:     "/data/datadog-init/package",
					pull:     false,
					volumeID: "test-volume-001",
					expectedFiles: []string{
						"library.txt",
					},
				},
				{
					name:     "test-image",
					version:  "latest",
					path:     "/data/datadog-init/package",
					pull:     false,
					volumeID: "test-volume-002",
					expectedFiles: []string{
						"library.txt",
					},
				},
			},
			expectedManagerFiles: []string{
				"db/datadog-csi-driver.db",
				"store/32ea291b55c8556199ec22906034cc296f20ae69866f8c8031aecb7d9fd765b8/library.txt",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup local registry
			localRegistry := NewLocalRegistry(t)
			defer localRegistry.Stop()
			for _, img := range test.images {
				localRegistry.AddImage(t, img.tarPath, img.name, img.tag)
			}

			// Create downloader.
			d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))

			// Create scratch space.
			tsd := NewTempScratchDirectory(t)
			defer tsd.Cleanup(t)
			basePath := tsd.Path(t)

			// Setup library manager.
			ctx := context.Background()
			lm, err := librarymanager.NewLibraryManagerWithDownloader(basePath, d)
			require.NoError(t, err)
			defer func() {
				err := lm.Stop()
				require.NoError(t, err)
			}()

			// Setup all volumes and ensure they have expected files.
			for _, volume := range test.volumes {
				// Get library for the volume.
				lib := CreateTestLibrary(t, volume, localRegistry.Registry(t))
				path, err := lm.GetLibraryForVolume(ctx, volume.volumeID, lib)
				require.NoError(t, err)

				// Ensure the volume path returned contains the expected files.
				actualFiles := listFiles(t, path)
				require.ElementsMatch(t, volume.expectedFiles, actualFiles)
			}

			// Ensure the manager file system is as expected.
			actualFiles := listFiles(t, tsd.Path(t))
			require.ElementsMatch(t, test.expectedManagerFiles, actualFiles)

			// Delete the volumes.
			for _, volume := range test.volumes {
				err = lm.RemoveVolume(ctx, volume.volumeID)
				require.NoError(t, err)
			}

			// Ensure the store is empty.
			actualFiles = listFiles(t, filepath.Join(tsd.Path(t), librarymanager.StoreDirectory))
			require.Empty(t, actualFiles)
		})
	}
}

func CreateTestLibrary(t *testing.T, tl *testVolume, registry string) *librarymanager.Library {
	t.Helper()
	lib, err := librarymanager.NewLibrary(tl.name, registry, tl.version, tl.path, tl.pull)
	require.NoError(t, err)
	return lib
}
