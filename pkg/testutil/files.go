// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package testutil

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ListFiles returns a list of all files (not directories) in a directory, relative to the directory.
func ListFiles(t *testing.T, dir string) []string {
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
