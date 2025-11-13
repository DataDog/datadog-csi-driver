// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

// Package downloader provides the ability to download and extract a source directory from inside the container to a
// destination directory on the local filesystem.
package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-containerregistry/pkg/crane"
)

const (
	// UserAgent is used during the crane HTTP operations to identify the Datadog CSI Driver.
	UserAgent = "datadog-csi-driver"
)

// Downloader enables downloading and extracting directories from container images.
type Downloader struct {
	roundTripper http.RoundTripper
}

// NewDownloader creates a new downloader with the default settings.
func NewDownloader() *Downloader {
	return NewDownloaderWithRoundTripper(http.DefaultTransport)
}

// NewDownloaderWithRoundTripper creates a new downloader with the provided round tripper.
func NewDownloaderWithRoundTripper(roundTripper http.RoundTripper) *Downloader {
	return &Downloader{
		roundTripper: roundTripper,
	}
}

// Download will stream a container image and extract the source directory from inside of the image to the destination
// directory on disk.
func (d *Downloader) Download(ctx context.Context, image string, src string, dst string) error {
	img, err := crane.Pull(image, crane.WithContext(ctx), crane.WithUserAgent(UserAgent), crane.WithTransport(d.roundTripper))
	if err != nil {
		return fmt.Errorf("could not pull %s: %w", image, err)
	}

	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(crane.Export(img, pw))
	}()

	fp := NewArchiveExtractor(src, dst)
	err = fp.Extract(ctx, pr)
	if err != nil {
		return fmt.Errorf("could not extract archive: %w", err)
	}

	return nil
}
