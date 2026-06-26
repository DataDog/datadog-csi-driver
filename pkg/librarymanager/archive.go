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
)

// ArchiveExtractor extracts directories from a tar archive.
type ArchiveExtractor struct {
	src    string
	dst    string // Absolute path to destination
	format archives.Tar

	// bytesExtracted tracks the cumulative size of regular files written
	// during Extract. Reset on each Extract call.
	bytesExtracted int64

	root *os.Root
}

// NewArchiveExtractor initializes a new archive extractor.
func NewArchiveExtractor(src string, dst string) (*ArchiveExtractor, error) {
	destination, err := filepath.Abs(filepath.Clean(dst))
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path for destination %s: %w", dst, err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return nil, fmt.Errorf("could not create destination %s: %w", destination, err)
	}
	return &ArchiveExtractor{
		src:    filepath.Clean("/" + src),
		dst:    destination,
		format: archives.Tar{},
	}, nil
}

// Extract will copy files from the configured source directory inside the archive to the desitnation directory outside
// of the archive through the reader provided. Returns the cumulative size of
// the regular files written; symlinks and directories are not counted.
func (fp *ArchiveExtractor) Extract(ctx context.Context, reader io.Reader) (int64, error) {
	fp.bytesExtracted = 0
	root, err := os.OpenRoot(fp.dst)
	if err != nil {
		return 0, fmt.Errorf("could not open destination root %s: %w", fp.dst, err)
	}
	defer func() {
		_ = root.Close()
	}()
	fp.root = root
	defer func() { fp.root = nil }()

	if err := fp.format.Extract(ctx, reader, fp.processFile); err != nil {
		return 0, err
	}
	return fp.bytesExtracted, nil
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

	mode := f.Mode()
	switch {
	case mode.IsDir():
		return fp.root.Mkdir(destPath, 0o755)
	case mode&os.ModeSymlink != 0:
		// Handle symbolic links.
		// Some packages use symlinks (e.g., dd-lib-python-init for deduplication, apm-inject for versioning).
		// We preserve them as-is because once bind-mounted in a pod, symlinks resolve within the
		// container's filesystem namespace, not the host's.
		linkTarget := f.LinkTarget
		if linkTarget == "" {
			return fmt.Errorf("symlink %s has no target", destPath)
		}
		if err := fp.root.Symlink(linkTarget, destPath); err != nil {
			// If symlink already exists with the same target, ignore the error
			if existing, readErr := fp.root.Readlink(destPath); readErr == nil && existing == linkTarget {
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
		defer func() {
			_ = in.Close()
		}()

		out, err := fp.root.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("could not create destination file: %w", err)
		}
		defer func() {
			_ = out.Close()
		}()

		n, err := io.Copy(out, in)
		if err != nil {
			return fmt.Errorf("could not copy destination file: %w", err)
		}
		fp.bytesExtracted += n

		return nil
	default:
		return nil
	}
}
