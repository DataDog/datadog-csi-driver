// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/mount"
)

func TestSocketLegacyPublisher_Publish_ModeSelection(t *testing.T) {
	// These tests verify the mode selection logic only.
	// We only test cases that return (nil, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext map[string]string
	}{
		"local mode is not supported": {
			volumeContext: map[string]string{"mode": "local", "path": "/some/path"},
		},
		"type schema is not supported (handled by new publishers)": {
			volumeContext: map[string]string{"type": "APMSocket"},
		},
		"mode without path is not supported": {
			volumeContext: map[string]string{"mode": "socket"},
		},
		"path without mode is not supported": {
			volumeContext: map[string]string{"path": "/some/path"},
		},
		"empty context is not supported": {
			volumeContext: map[string]string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			publisher := socketLegacyPublisher{}

			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/path",
				VolumeContext: tc.volumeContext,
			}

			resp, err := publisher.Publish(req)
			assert.Nil(t, resp)
			assert.NoError(t, err)
		})
	}
}

func TestSocketLegacyPublisher_Publish_PathValidation(t *testing.T) {
	tests := map[string]struct {
		path string
	}{
		"disallowed path fails": {
			path: "/etc/passwd",
		},
		"similar but different path fails": {
			path: "/var/run/other.sock",
		},
		"empty path fails": {
			path: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			publisher := socketLegacyPublisher{
				apmSocketPath: "/var/run/apm.sock",
				dsdSocketPath: "/var/run/dsd.sock",
			}

			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/path",
				VolumeContext: map[string]string{"mode": "socket", "path": tc.path},
			}

			resp, err := publisher.Publish(req)
			assert.NotNil(t, resp, "socket mode should be supported")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not allowed")
		})
	}
}

func TestSocketLegacyPublisher_Publish_SocketNotFound(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	publisher := newSocketLegacyPublisher(fs, mounter, "/var/run/apm.sock", "/var/run/dsd.sock")

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume",
		TargetPath:    "/target/apm.sock",
		VolumeContext: map[string]string{"mode": "socket", "path": "/var/run/apm.sock"},
	}

	resp, err := publisher.Publish(req)

	// Should return a response (for metrics) and an error
	assert.NotNil(t, resp)
	assert.Equal(t, VolumeType("socket"), resp.VolumeType)
	assert.Equal(t, "/var/run/apm.sock", resp.VolumePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestSocketLegacyPublisher_Publish_NotASocket(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	// Create a regular file instead of a socket
	require.NoError(t, fs.MkdirAll("/var/run", 0755))
	_, err := fs.Create("/var/run/apm.sock")
	require.NoError(t, err)

	publisher := newSocketLegacyPublisher(fs, mounter, "/var/run/apm.sock", "/var/run/dsd.sock")

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume",
		TargetPath:    "/target/apm.sock",
		VolumeContext: map[string]string{"mode": "socket", "path": "/var/run/apm.sock"},
	}

	resp, err := publisher.Publish(req)

	// Should return a response and an error because it's not a socket
	assert.NotNil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "socket not found")
}

func TestSocketLegacyPublisher_Unpublish_DelegatesToUnmount(t *testing.T) {
	publisher := socketLegacyPublisher{}
	resp, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.Nil(t, resp, "socket legacy should delegate Unpublish to unmountPublisher")
	assert.NoError(t, err)
}
