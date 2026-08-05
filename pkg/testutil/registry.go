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

	"github.com/google/go-containerregistry/pkg/authn"
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
	username string
	password string
}

// NewLocalRegistry creates a new local OCI registry for testing.
func NewLocalRegistry(t *testing.T) *LocalRegistry {
	t.Helper()
	return newLocalRegistry(t, "", "")
}

// NewAuthenticatedLocalRegistry creates a local OCI registry protected by Basic Auth.
func NewAuthenticatedLocalRegistry(t *testing.T, username, password string) *LocalRegistry {
	t.Helper()
	return newLocalRegistry(t, username, password)
}

func newLocalRegistry(t *testing.T, username, password string) *LocalRegistry {
	t.Helper()
	r := &LocalRegistry{username: username, password: password}
	registryHandler := registry.New(registry.Logger(log.New(io.Discard, "", log.LstdFlags)))
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if username != "" {
			gotUsername, gotPassword, ok := req.BasicAuth()
			if !ok || gotUsername != username || gotPassword != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		registryHandler.ServeHTTP(w, req)
	}))
	r.registry = strings.TrimPrefix(r.srv.URL, "http://")
	return r
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

	options := []crane.Option{crane.WithTransport(r.srv.Client().Transport)}
	if r.username != "" {
		options = append(options, crane.WithAuth(authn.FromConfig(authn.AuthConfig{
			Username: r.username,
			Password: r.password,
		})))
	}
	err = crane.Push(img, ref.String(), options...)
	require.NoError(t, err, "could not push image to test registry")

	return image
}
