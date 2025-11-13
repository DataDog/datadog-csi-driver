// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package downloader_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/downloader"
	"github.com/stretchr/testify/require"
)

func TestExtract(t *testing.T) {
	tests := map[string]struct {
		archive  string
		source   string
		expected []string
	}{
		"sameple archive extracts expected files with source": {
			archive: "testdata/rootfs.tar",
			source:  "/contents/datadog-init/package",
			expected: []string{
				"library.txt",
			},
		},
		"sameple archive extracts expected files with relative source": {
			archive: "testdata/rootfs.tar",
			source:  "contents/datadog-init/package",
			expected: []string{
				"library.txt",
			},
		},
		"sameple archive extracts expected files when it ends in a slash": {
			archive: "testdata/rootfs.tar",
			source:  "/contents/datadog-init/package/",
			expected: []string{
				"library.txt",
			},
		},
		"sameple archive extracts all files with root": {
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
			// Create temp dir.
			dst, err := os.MkdirTemp("", "csi-driver-test-*")
			require.NoError(t, err, "could not setup destination dir for the test")
			defer os.RemoveAll(dst)

			// Open archive.
			f, err := os.Open(test.archive)
			require.NoError(t, err, "could not test archive for the test")
			defer f.Close()

			// Extract archive.
			ctx := context.Background()
			ae := downloader.NewArchiveExtractor(test.source, dst)
			err = ae.Extract(ctx, f)
			require.NoError(t, err, "could not extract archive")

			// List files in the destination by path.
			actual := listFiles(t, dst)

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
