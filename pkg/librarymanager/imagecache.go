// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
)

// ImageCache provides an in memory cache of container image digests so we don't have to resolve a container tag to
// sha256sum each time.
type ImageCache struct {
	downloader *Downloader
	mu         sync.Mutex
	cache      map[string]*cacheEntry
	ttl        time.Duration
}

// NewImageChace initializes a new, empty image cache.
func NewImageCache(d *Downloader, ttl time.Duration) *ImageCache {
	return &ImageCache{
		downloader: d,
		mu:         sync.Mutex{},
		cache:      map[string]*cacheEntry{},
		ttl:        ttl,
	}
}

// FetchDigest returns the sha256 digest for a container image, using the cache when possible.
//
// The image parameter must be a valid container image reference as accepted by crane
// (https://pkg.go.dev/github.com/google/go-containerregistry/pkg/crane).
// Examples:
//   - "gcr.io/datadoghq/dd-lib-java-init:v1.2.3"
//   - "gcr.io/datadoghq/dd-lib-java-init@sha256:abc123..."
//   - "nginx:latest" (defaults to docker.io registry)
//
// If the image already contains a digest (@sha256:...), this function will still resolve
// the full digest from the registry to ensure it exists and is valid.
//
// If pull is true, the cache is bypassed and a fresh digest is always fetched from the registry.
// If pull is false, the cache is checked first and a remote call is only made on cache miss.
func (ic *ImageCache) FetchDigest(ctx context.Context, image string, pull bool) (string, error) {
	// Validate image format using crane's reference parser.
	if _, err := name.ParseReference(image); err != nil {
		return "", fmt.Errorf("invalid image reference %q: %w", image, err)
	}

	// If pull is false, then we should check the cache for the image first.
	if !pull {
		cached := ic.digestFromCache(image)
		if cached != "" {
			return cached, nil
		}
	}

	// Otherwise, fetch the digest.
	digest, err := ic.downloader.FetchDigest(ctx, image)
	if err != nil {
		return "", err
	}

	// Cache the digest.
	ic.cacheDigest(image, digest)

	// Return it.
	return digest, nil
}

func (ic *ImageCache) cacheDigest(image string, digest string) {
	entry := &cacheEntry{
		validUntil: time.Now().Add(ic.ttl),
		value:      digest,
	}

	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.cache[image] = entry
}

func (ic *ImageCache) digestFromCache(image string) string {
	now := time.Now()

	ic.mu.Lock()
	defer ic.mu.Unlock()

	entry, ok := ic.cache[image]
	if !ok {
		return ""
	}

	if now.After(entry.validUntil) {
		delete(ic.cache, image)
		return ""
	}

	return entry.value
}

type cacheEntry struct {
	validUntil time.Time
	value      string
}
