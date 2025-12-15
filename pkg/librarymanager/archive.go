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
		// Handle symbolic links
		linkTarget := f.LinkTarget
		if linkTarget == "" {
			return fmt.Errorf("symlink %s has no target", destPath)
		}
		// Create the symlink in the destination
		fullDestPath := filepath.Join(fp.dst, destPath)
		// Remove existing file/symlink if it exists
		_ = os.Remove(fullDestPath)
		if err := os.Symlink(linkTarget, fullDestPath); err != nil {
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
