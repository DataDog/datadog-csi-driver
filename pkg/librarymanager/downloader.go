// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/afero"
)

const (
	// userAgent is used during the crane HTTP operations to identify the Datadog CSI Driver.
	userAgent = "datadog-csi-driver"
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
func (d *Downloader) Download(ctx context.Context, afs afero.Afero, image string, src string, dst string) error {
	img, err := crane.Pull(image, crane.WithContext(ctx), crane.WithUserAgent(userAgent), crane.WithTransport(d.roundTripper))
	if err != nil {
		return fmt.Errorf("could not pull %s: %w", image, err)
	}

	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(crane.Export(img, pw))
	}()

	fp, err := NewArchiveExtractor(afs, src, dst)
	if err != nil {
		return fmt.Errorf("could not setup archive extractor: %w", err)
	}
	err = fp.Extract(ctx, pr)
	if err != nil {
		return fmt.Errorf("could not extract archive: %w", err)
	}

	return nil
}

// FetchDigest will fetch a sha256 sum of the image and return it.
func (d *Downloader) FetchDigest(ctx context.Context, image string) (string, error) {
	digest, err := crane.Digest(image, crane.WithContext(ctx), crane.WithUserAgent(userAgent), crane.WithTransport(d.roundTripper))
	if err != nil {
		return "", fmt.Errorf("could not get digetst %s: %w", image, err)
	}
	if !strings.HasPrefix(digest, "sha256:") {
		return "", fmt.Errorf("digest does not have expected prefix: %s", digest)
	}
	return strings.TrimPrefix(digest, "sha256:"), nil
}
