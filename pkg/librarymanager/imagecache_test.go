// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"context"
	"testing"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/stretchr/testify/require"
)

func TestImageCache(t *testing.T) {
	// Setup local registry
	localRegistry := NewLocalRegistry(t)
	defer localRegistry.Stop()
	image := localRegistry.AddImage(t, "testdata/image.tar", "test", "latest")

	// Create downloader.
	d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))

	// Create the image cache.
	ic := librarymanager.NewImageCache(d, 1*time.Hour)

	// Ensure digest matches expected.
	ctx := context.Background()
	digest, err := ic.FetchDigest(ctx, image, true)
	require.NoError(t, err, "error found when getting digest")
	require.Equal(t, "32ea291b55c8556199ec22906034cc296f20ae69866f8c8031aecb7d9fd765b8", digest)

	// Ensure the digest is cached by fetching after the server is stopped.
	localRegistry.Stop()
	digest, err = ic.FetchDigest(ctx, image, false)
	require.NoError(t, err, "error found when getting digest")
	require.Equal(t, "32ea291b55c8556199ec22906034cc296f20ae69866f8c8031aecb7d9fd765b8", digest)

	// Ensure pull true attempts to pull the image.
	digest, err = ic.FetchDigest(ctx, image, true)
	require.Error(t, err, "error should be returned")
	require.Empty(t, digest, "no digest should be returned")
}

func TestImageCacheInvalidImageReference(t *testing.T) {
	tests := map[string]string{
		"empty image reference": "",
		"image with spaces":     "nginx latest",
		"image with newline":    "nginx:latest\n",
		"invalid characters":    "INVALID@@@",
	}

	// Create a minimal downloader (won't be called for validation errors).
	d := librarymanager.NewDownloader()
	ic := librarymanager.NewImageCache(d, 1*time.Hour)
	ctx := context.Background()

	for name, image := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ic.FetchDigest(ctx, image, false)
			require.Error(t, err, "expected error for image %q", image)
			require.Contains(t, err.Error(), "invalid image reference")
		})
	}
}

func TestImageCacheTTLExpiration(t *testing.T) {
	// Setup local registry
	localRegistry := NewLocalRegistry(t)
	defer localRegistry.Stop()
	image := localRegistry.AddImage(t, "testdata/image.tar", "test", "latest")

	// Create downloader.
	d := librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))

	// Create the image cache with a very short TTL.
	ic := librarymanager.NewImageCache(d, 1*time.Millisecond)

	ctx := context.Background()

	// Fetch the digest to populate the cache.
	digest1, err := ic.FetchDigest(ctx, image, false)
	require.NoError(t, err)
	require.NotEmpty(t, digest1)

	// Wait for the cache to expire.
	time.Sleep(10 * time.Millisecond)

	// Stop the registry to verify we're not using cache.
	localRegistry.Stop()

	// Fetch again - should fail because cache expired and registry is stopped.
	_, err = ic.FetchDigest(ctx, image, false)
	require.Error(t, err, "expected error after cache expiration with stopped registry")
}
