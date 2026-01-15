// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestDownload(t *testing.T) {
	tests := map[string]struct {
		imagePath      string
		checkFiles     []string // files that should exist after download (relative to extract root)
		expectedDigest string
	}{
		"test image can be downloaded": {
			imagePath: "testdata/image.tar",
			checkFiles: []string{
				"data/datadog-init/package/library.txt",
				"data/other/other.txt",
			},
			expectedDigest: "32ea291b55c8556199ec22906034cc296f20ae69866f8c8031aecb7d9fd765b8",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup local registry
			localRegistry := testutil.NewLocalRegistry(t)
			defer localRegistry.Stop()
			image := localRegistry.AddImage(t, test.imagePath, "test", "latest")

			// Create downloader.
			d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))

			// Create scratch space.
			tsd := testutil.NewTempScratchDirectory(t)
			defer tsd.Cleanup(t)

			// Ensure digest matches expected.
			ctx := context.Background()
			digest, err := d.FetchDigest(ctx, image)
			require.NoError(t, err, "error found when getting digest")
			require.Equal(t, test.expectedDigest, digest)

			// Ensure downloaded files match the expected.
			err = d.Download(ctx, afero.Afero{Fs: afero.NewOsFs()}, image, tsd.Path(t))
			require.NoError(t, err, "error found when downloading")

			// Ensure expected files exist.
			for _, checkFile := range test.checkFiles {
				checkPath := filepath.Join(tsd.Path(t), checkFile)
				_, err := os.Stat(checkPath)
				require.NoError(t, err, "expected file %s to exist", checkPath)
			}
		})
	}
}
