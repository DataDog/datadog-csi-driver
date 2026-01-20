// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
	"github.com/spf13/afero"
)

// ArchiveExtractor extracts directories from a tar archive.
type ArchiveExtractor struct {
	src    string
	dst    string      // Absolute path to destination (needed for symlinks, as afero doesn't support them)
	dstFs  afero.Afero // BasePathFs rooted at destination
	format archives.Tar
}

// NewArchiveExtractor initializes a new archive extractor.
func NewArchiveExtractor(afs afero.Afero, src string, dst string) (*ArchiveExtractor, error) {
	destination, err := filepath.Abs(filepath.Clean(dst))
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path for destination %s: %w", dst, err)
	}
	// Create a BasePathFs rooted at the destination to prevent path traversal
	baseFs := afero.NewBasePathFs(afs.Fs, destination)
	return &ArchiveExtractor{
		src:    filepath.Clean("/" + src),
		dst:    destination,
		dstFs:  afero.Afero{Fs: baseFs},
		format: archives.Tar{},
	}, nil
}

// Extract will copy files from the configured source directory inside the archive to the desitnation directory outside
// of the archive through the reader provided.
func (fp *ArchiveExtractor) Extract(ctx context.Context, reader io.Reader) error {
	return fp.format.Extract(ctx, reader, fp.processFile)
}

// processFile is a helper function that is called for every file extracted.
func (fp *ArchiveExtractor) processFile(ctx context.Context, f archives.FileInfo) error {
	// Determine the current file or directory name. Skip it if the current file does not match the prefix for copy.
	archivePath := filepath.Clean("/" + f.NameInArchive)
	if !strings.HasPrefix(archivePath, fp.src) {
		return nil
	}

	// Determine the relative path for the current file.
	relativePath, err := filepath.Rel(fp.src, archivePath)
	if err != nil {
		return fmt.Errorf("could not determine relative path: %w", err)
	}
	destPath := filepath.Clean(relativePath)

	// Return if the destination path is relative to the root.
	if destPath == "." {
		return nil
	}

	// The dstFs is a BasePathFs rooted at the destination, preventing path traversal
	mode := f.FileInfo.Mode()
	switch {
	case mode.IsDir():
		return fp.dstFs.Mkdir(destPath, 0o755)
	case mode&os.ModeSymlink != 0:
		// Handle symbolic links.
		// Some packages use symlinks (e.g., dd-lib-python-init for deduplication, apm-inject for versioning).
		// We preserve them as-is because once bind-mounted in a pod, symlinks resolve within the
		// container's filesystem namespace, not the host's.
		linkTarget := f.LinkTarget
		if linkTarget == "" {
			return fmt.Errorf("symlink %s has no target", destPath)
		}
		fullDestPath := filepath.Clean(filepath.Join(fp.dst, destPath))
		// Verify the path stays within the destination root
		if !strings.HasPrefix(fullDestPath, fp.dst+string(filepath.Separator)) {
			return fmt.Errorf("symlink path %q escapes destination directory", destPath)
		}
		// Create the symlink in the destination.
		// Note: afero does not support symlinks, so we use os.Symlink directly.
		if err := os.Symlink(linkTarget, fullDestPath); err != nil {
			// If symlink already exists with the same target, ignore the error
			if existing, readErr := os.Readlink(fullDestPath); readErr == nil && existing == linkTarget {
				return nil
			}
			return fmt.Errorf("could not create symlink %s -> %s: %w", destPath, linkTarget, err)
		}
		return nil
	case mode.IsRegular():
		in, err := f.Open()
		if err != nil {
			return fmt.Errorf("could not open file in archive: %w", err)
		}
		defer in.Close()

		out, err := fp.dstFs.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("could not create destination file: %w", err)
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		if err != nil {
			return fmt.Errorf("could not copy destination file: %w", err)
		}

		return nil
	default:
		return nil
	}
}
