// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/google/go-containerregistry/pkg/crane"
	imageref "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type LocalRegistry struct {
	srv        *httptest.Server
	stopped    bool
	registry   string
	Downloader *librarymanager.Downloader
}

func NewLocalRegistry(t *testing.T) *LocalRegistry {
	// Create the test server
	srv := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", log.LstdFlags))))

	// Create downloader.
	d := librarymanager.NewDownloaderWithRoundTripper(srv.Client().Transport)
	return &LocalRegistry{
		Downloader: d,
		srv:        srv,
		registry:   strings.TrimPrefix(srv.URL, "http://"),
	}
}

func (lr *LocalRegistry) Stop() {
	if !lr.stopped {
		lr.stopped = true
		lr.srv.Close()
	}
}

func (lr *LocalRegistry) Registry(t *testing.T) string {
	return lr.registry
}

func (lr *LocalRegistry) AddImage(t *testing.T, tarPath string, name string, version string) string {
	// Validate state.
	require.False(t, lr.stopped, "cannot add image to stopped local registry")

	// Create image ref.
	image := fmt.Sprintf("%s/%s:%s", lr.registry, name, version)
	ref, err := imageref.NewTag(image, imageref.Insecure)
	require.NoError(t, err, "could not generate image ref")

	// Load image from tarball.
	img, err := tarball.ImageFromPath(tarPath, nil)
	require.NoError(t, err, "could not load tarball image")

	// Push image to test server.
	err = crane.Push(img, ref.String(), crane.WithTransport(lr.srv.Client().Transport))
	require.NoError(t, err, "could not load tarball image")

	// Return image string.
	return image
}

func (lr *LocalRegistry) GetRoundTripper(t *testing.T) http.RoundTripper {
	// Validate state.
	require.False(t, lr.stopped, "cannot add image to stopped local registry")
	return lr.srv.Client().Transport
}

func TestDownload(t *testing.T) {
	tests := map[string]struct {
		imagePath      string
		source         string
		expectedFiles  []string
		expectedDigest string
	}{
		"test image can be downloaded": {
			imagePath: "testdata/image.tar",
			source:    "/data/datadog-init/package",
			expectedFiles: []string{
				"library.txt",
			},
			expectedDigest: "32ea291b55c8556199ec22906034cc296f20ae69866f8c8031aecb7d9fd765b8",
		},
		"test image can be downloaded with other source": {
			imagePath: "testdata/image.tar",
			source:    "/data/other",
			expectedFiles: []string{
				"other.txt",
			},
			expectedDigest: "32ea291b55c8556199ec22906034cc296f20ae69866f8c8031aecb7d9fd765b8",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup local registry
			localRegistry := NewLocalRegistry(t)
			defer localRegistry.Stop()
			image := localRegistry.AddImage(t, test.imagePath, "test", "latest")

			// Create downloader.
			d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))

			// Create scratch space.
			tsd := NewTempScratchDirectory(t)
			defer tsd.Cleanup(t)

			// Ensure digest matches expected.
			ctx := context.Background()
			digest, err := d.FetchDigest(ctx, image)
			require.NoError(t, err, "error found when getting digest")
			require.Equal(t, test.expectedDigest, digest)

			// Ensure downloaded files match the expected.
			err = d.Download(ctx, afero.Afero{Fs: afero.NewOsFs()}, image, test.source, tsd.Path(t))
			require.NoError(t, err, "error found when downloading")
			actual := listFiles(t, tsd.Path(t))
			require.ElementsMatch(t, test.expectedFiles, actual)
		})
	}
}
