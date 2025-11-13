// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package downloader_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/downloader"
	"github.com/google/go-containerregistry/pkg/crane"
	imageref "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
)

func TestDownload(t *testing.T) {
	tests := map[string]struct {
		imagePath string
		source    string
		expected  []string
	}{
		"test image can be downloaded": {
			imagePath: "testdata/image.tar",
			source:    "/data/datadog-init/package",
			expected: []string{
				"library.txt",
			},
		},
		"test image can be downloaded with other source": {
			imagePath: "testdata/image.tar",
			source:    "/data/other",
			expected: []string{
				"other.txt",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(registry.New())
			defer srv.Close()

			// Create image ref.
			registry := strings.TrimPrefix(srv.URL, "http://")
			image := fmt.Sprintf("%s/test-image:latest", registry)
			ref, err := imageref.NewTag(image, imageref.Insecure)
			require.NoError(t, err, "could not generate image ref")

			// Load image from tarball.
			img, err := tarball.ImageFromPath(test.imagePath, nil)
			require.NoError(t, err, "could not load tarball image")

			// Push image to test server.
			err = crane.Push(img, ref.String(), crane.WithTransport(srv.Client().Transport))
			require.NoError(t, err, "could not load tarball image")

			// Create temp dir.
			dst, err := os.MkdirTemp("", "csi-driver-test-*")
			require.NoError(t, err, "could not setup destination dir for the test")
			defer os.RemoveAll(dst)

			// Download and extract contents.
			d := downloader.NewDownloaderWithRoundTripper(srv.Client().Transport)
			ctx := context.Background()
			err = d.Download(ctx, ref.String(), test.source, dst)
			require.NoError(t, err, "error found when downloading")

			// List files in the destination by path.
			actual := listFiles(t, dst)

			// Ensure the desitnation files match the expected files.
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}
