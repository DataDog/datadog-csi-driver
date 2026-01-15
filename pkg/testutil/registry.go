// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

// Package testutil provides testing utilities shared across packages.
package testutil

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	imageref "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
)

// LocalRegistry is a local OCI registry for testing.
type LocalRegistry struct {
	srv      *httptest.Server
	registry string
}

// NewLocalRegistry creates a new local OCI registry for testing.
func NewLocalRegistry(t *testing.T) *LocalRegistry {
	t.Helper()
	srv := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", log.LstdFlags))))
	return &LocalRegistry{
		srv:      srv,
		registry: strings.TrimPrefix(srv.URL, "http://"),
	}
}

// Stop stops the test registry server.
func (r *LocalRegistry) Stop() {
	r.srv.Close()
}

// GetRoundTripper returns the HTTP transport for the test registry.
// Use this to configure a Downloader to use the test registry.
func (r *LocalRegistry) GetRoundTripper(t *testing.T) http.RoundTripper {
	t.Helper()
	return r.srv.Client().Transport
}

// Registry returns the registry address.
func (r *LocalRegistry) Registry(t *testing.T) string {
	t.Helper()
	return r.registry
}

// AddImage loads an image from a tar file and pushes it to the test registry.
// Returns the full image reference (e.g., "127.0.0.1:12345/name:version").
func (r *LocalRegistry) AddImage(t *testing.T, tarPath, name, version string) string {
	t.Helper()

	image := fmt.Sprintf("%s/%s:%s", r.registry, name, version)
	ref, err := imageref.NewTag(image, imageref.Insecure)
	require.NoError(t, err, "could not generate image ref")

	img, err := tarball.ImageFromPath(tarPath, nil)
	require.NoError(t, err, "could not load tarball image")

	err = crane.Push(img, ref.String(), crane.WithTransport(r.srv.Client().Transport))
	require.NoError(t, err, "could not push image to test registry")

	return image
}
