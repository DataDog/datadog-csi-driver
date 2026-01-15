// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
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
			tsd := testutil.NewTempScratchDirectory(t)
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
			actual := testutil.ListFiles(t, tsd.Path(t))

			// Ensure the desitnation files match the expected files.
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestExtractSymlink(t *testing.T) {
	// Create a temporary directory structure with a symlink
	srcDir := t.TempDir()

	// Create target file
	targetDir := filepath.Join(srcDir, "lib")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "library.so"), []byte("content"), 0644))

	// Create symlink: latest -> lib/library.so
	require.NoError(t, os.Symlink("lib/library.so", filepath.Join(srcDir, "latest")))

	// Create tar archive
	archivePath := filepath.Join(t.TempDir(), "symlink.tar")
	createTarFromDir(t, srcDir, archivePath)

	// Extract
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	f, err := os.Open(archivePath)
	require.NoError(t, err)
	defer f.Close()

	ctx := context.Background()
	ae, err := librarymanager.NewArchiveExtractor(afero.Afero{Fs: afero.NewOsFs()}, "/", tsd.Path(t))
	require.NoError(t, err)
	require.NoError(t, ae.Extract(ctx, f))

	// Verify symlink was created with correct target
	linkPath := filepath.Join(tsd.Path(t), "latest")
	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	require.Equal(t, "lib/library.so", linkTarget)

	// Verify the target file exists and symlink resolves correctly
	content, err := os.ReadFile(linkPath)
	require.NoError(t, err)
	require.Equal(t, "content", string(content))
}

func TestExtractSymlinkPathTraversal(t *testing.T) {
	// This test verifies that path traversal attempts in archive entries are safely handled.
	// The archive extractor normalizes paths via filepath.Clean, so malicious paths like
	// "../escape" become "/escape" and are safely extracted within the destination directory.
	tests := map[string]struct {
		symlinkPath     string // path of the symlink in the archive (potentially malicious)
		expectedSymlink string // expected normalized path in destination
	}{
		"symlink with .. in path is normalized": {
			symlinkPath:     "../escape",
			expectedSymlink: "escape",
		},
		"symlink with nested .. in path is normalized": {
			symlinkPath:     "subdir/../../escape",
			expectedSymlink: "escape",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create tar archive with potentially malicious symlink path
			archivePath := createTarWithSymlink(t, tc.symlinkPath, "target.txt")

			tsd := testutil.NewTempScratchDirectory(t)
			defer tsd.Cleanup(t)

			f, err := os.Open(archivePath)
			require.NoError(t, err)
			defer f.Close()

			ctx := context.Background()
			ae, err := librarymanager.NewArchiveExtractor(afero.Afero{Fs: afero.NewOsFs()}, "/", tsd.Path(t))
			require.NoError(t, err)

			// Extraction should succeed - malicious paths are normalized, not rejected
			err = ae.Extract(ctx, f)
			require.NoError(t, err)

			// Verify the symlink was created inside the destination (path was normalized)
			expectedPath := filepath.Join(tsd.Path(t), tc.expectedSymlink)
			linkTarget, err := os.Readlink(expectedPath)
			require.NoError(t, err, "symlink should be created inside destination directory")
			require.Equal(t, "target.txt", linkTarget)

			// Verify nothing was created outside the destination directory
			parentDir := filepath.Dir(tsd.Path(t))
			escapedPath := filepath.Join(parentDir, "escape")
			_, err = os.Lstat(escapedPath)
			require.True(t, os.IsNotExist(err), "no file should be created outside destination directory")
		})
	}
}

func TestExtractSymlinkIdempotent(t *testing.T) {
	// Verifies that extracting the same archive twice doesn't fail
	// when symlinks already exist with the same target (lines 92-95 of archive.go)
	srcDir := t.TempDir()

	// Create target file and symlink
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "target.txt"), []byte("content"), 0644))
	require.NoError(t, os.Symlink("target.txt", filepath.Join(srcDir, "link")))

	// Create tar archive
	archivePath := filepath.Join(t.TempDir(), "idempotent.tar")
	createTarFromDir(t, srcDir, archivePath)

	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	ctx := context.Background()
	ae, err := librarymanager.NewArchiveExtractor(afero.Afero{Fs: afero.NewOsFs()}, "/", tsd.Path(t))
	require.NoError(t, err)

	// First extraction
	f1, err := os.Open(archivePath)
	require.NoError(t, err)
	require.NoError(t, ae.Extract(ctx, f1))
	f1.Close()

	// Verify symlink exists
	linkPath := filepath.Join(tsd.Path(t), "link")
	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	require.Equal(t, "target.txt", linkTarget)

	// Second extraction should not fail (symlink already exists with same target)
	f2, err := os.Open(archivePath)
	require.NoError(t, err)
	defer f2.Close()

	ae2, err := librarymanager.NewArchiveExtractor(afero.Afero{Fs: afero.NewOsFs()}, "/", tsd.Path(t))
	require.NoError(t, err)
	require.NoError(t, ae2.Extract(ctx, f2), "second extraction should succeed when symlink already exists with same target")
}

func TestExtractSymlinkNoTarget(t *testing.T) {
	// Verifies that a symlink with empty target returns an error (lines 81-83 of archive.go)
	archivePath := filepath.Join(t.TempDir(), "no-target.tar")
	f, err := os.Create(archivePath)
	require.NoError(t, err)

	tw := tar.NewWriter(f)
	header := &tar.Header{
		Name:     "broken-link",
		Typeflag: tar.TypeSymlink,
		Linkname: "", // empty target
		Mode:     0777,
	}
	require.NoError(t, tw.WriteHeader(header))
	tw.Close()
	f.Close()

	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	archive, err := os.Open(archivePath)
	require.NoError(t, err)
	defer archive.Close()

	ctx := context.Background()
	ae, err := librarymanager.NewArchiveExtractor(afero.Afero{Fs: afero.NewOsFs()}, "/", tsd.Path(t))
	require.NoError(t, err)

	err = ae.Extract(ctx, archive)
	require.Error(t, err)
	require.Contains(t, err.Error(), "has no target")
}

// createTarFromDir creates a tar archive from a directory, preserving symlinks.
func createTarFromDir(t *testing.T, srcDir, archivePath string) {
	t.Helper()

	f, err := os.Create(archivePath)
	require.NoError(t, err)
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = linkTarget
			header.Typeflag = tar.TypeSymlink
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if _, err := tw.Write(content); err != nil {
				return err
			}
		}

		return nil
	})
	require.NoError(t, err)
}

// createTarWithSymlink creates a tar archive with a single symlink.
func createTarWithSymlink(t *testing.T, symlinkPath, linkTarget string) string {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), "malicious.tar")
	f, err := os.Create(archivePath)
	require.NoError(t, err)
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	header := &tar.Header{
		Name:     symlinkPath,
		Typeflag: tar.TypeSymlink,
		Linkname: linkTarget,
		Mode:     0777,
	}
	require.NoError(t, tw.WriteHeader(header))

	return archivePath
}
