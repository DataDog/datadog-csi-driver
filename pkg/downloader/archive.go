// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package downloader

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
	dst    string
	format archives.Tar
}

// NewArchiveExtractor initializes a new archive extractor.
func NewArchiveExtractor(src string, dst string) *ArchiveExtractor {
	return &ArchiveExtractor{
		src:    filepath.Clean("/" + src),
		dst:    filepath.Clean(dst),
		format: archives.Tar{},
	}
}

// Extract will copy files from the configured source directory inside the archive to the desitnation directory outside
// of the archive through the reader provided.
func (fp *ArchiveExtractor) Extract(ctx context.Context, reader io.Reader) error {
	return fp.format.Extract(ctx, reader, fp.processFile)
}

// processFile is a helper function that is called for every file extracted.
func (fp *ArchiveExtractor) processFile(ctx context.Context, f archives.FileInfo) error {
	archivePath := filepath.Clean("/" + f.NameInArchive)
	if !strings.HasPrefix(archivePath, fp.src) {
		return nil
	}

	relativePath, err := filepath.Rel(fp.src, archivePath)
	if err != nil {
		return fmt.Errorf("could not determine relative path: %w", err)
	}
	destPath := filepath.Join(fp.dst, relativePath)

	mode := f.FileInfo.Mode()
	switch {
	case mode.IsDir():
		return os.MkdirAll(destPath, 0o755)
	case mode.IsRegular():
		err := os.MkdirAll(filepath.Dir(destPath), 0o755)
		if err != nil {
			return fmt.Errorf("could not create destination dir: %w", err)
		}

		in, err := f.Open()
		if err != nil {
			return fmt.Errorf("could not open file in archive: %w", err)
		}
		defer in.Close()

		out, err := os.Create(destPath)
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
