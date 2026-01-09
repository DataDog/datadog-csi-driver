// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestExtract(t *testing.T) {
	tests := map[string]struct {
		archive  string
		source   string
		expected []string
	}{
		"sample archive extracts expected files with source": {
			archive: "testdata/rootfs.tar",
			source:  "/contents/datadog-init/package",
			expected: []string{
				"library.txt",
			},
		},
		"sample archive extracts expected files with relative source": {
			archive: "testdata/rootfs.tar",
			source:  "contents/datadog-init/package",
			expected: []string{
				"library.txt",
			},
		},
		"sample archive extracts expected files when it ends in a slash": {
			archive: "testdata/rootfs.tar",
			source:  "/contents/datadog-init/package/",
			expected: []string{
				"library.txt",
			},
		},
		"sample archive extracts all files with root": {
			archive: "testdata/rootfs.tar",
			source:  "/",
			expected: []string{
				"contents/datadog-init/package/library.txt",
				"contents/other/other.txt",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Create scratch space.
			tsd := NewTempScratchDirectory(t)
			defer tsd.Cleanup(t)

			// Open archive.
			f, err := os.Open(test.archive)
			require.NoError(t, err, "could not test archive for the test")
			defer f.Close()

			// Extract archive.
			ctx := context.Background()
			ae, err := librarymanager.NewArchiveExtractor(afero.Afero{Fs: afero.NewOsFs()}, test.source, tsd.Path(t))
			require.NoError(t, err, "could not setup extractor")
			err = ae.Extract(ctx, f)
			require.NoError(t, err, "could not extract archive")

			// List files in the destination by path.
			actual := listFiles(t, tsd.Path(t))

			// Ensure the desitnation files match the expected files.
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	files := []string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err, "could not traverse destination")
		if path == dir {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		require.NoError(t, err, "could not determine relative path")
		files = append(files, rel)
		return nil
	})
	require.NoError(t, err, "could not list all files in the destination")
	return files
}
