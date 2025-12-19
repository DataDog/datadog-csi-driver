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

func TestSocketPublisher_Publish_TypeSelection(t *testing.T) {
	// These tests verify the type selection logic only.
	// We only test cases that return (nil, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext map[string]string
	}{
		"APMSocketDirectory type is not supported (handled by local publisher)": {
			volumeContext: map[string]string{"type": "APMSocketDirectory"},
		},
		"DSDSocketDirectory type is not supported (handled by local publisher)": {
			volumeContext: map[string]string{"type": "DSDSocketDirectory"},
		},
		"unknown type is not supported": {
			volumeContext: map[string]string{"type": "Unknown"},
		},
		"legacy mode/path is not supported (handled by legacy publisher)": {
			volumeContext: map[string]string{"mode": "socket", "path": "/some/socket.sock"},
		},
		"empty context is not supported": {
			volumeContext: map[string]string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			publisher := socketPublisher{
				apmSocketPath: "/var/run/apm.sock",
				dsdSocketPath: "/var/run/dsd.sock",
			}

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

func TestSocketPublisher_Publish_SocketNotFound(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	publisher := newSocketPublisher(fs, mounter, "/var/run/apm.sock", "/var/run/dsd.sock")

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume",
		TargetPath:    "/target/apm.sock",
		VolumeContext: map[string]string{"type": "APMSocket"},
	}

	resp, err := publisher.Publish(req)

	// Should return a response (for metrics) and an error
	assert.NotNil(t, resp)
	assert.Equal(t, VolumeType("APMSocket"), resp.VolumeType)
	assert.Equal(t, "/var/run/apm.sock", resp.VolumePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestSocketPublisher_Publish_NotASocket(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	// Create a regular file instead of a socket
	require.NoError(t, fs.MkdirAll("/var/run", 0755))
	_, err := fs.Create("/var/run/apm.sock")
	require.NoError(t, err)

	publisher := newSocketPublisher(fs, mounter, "/var/run/apm.sock", "/var/run/dsd.sock")

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume",
		TargetPath:    "/target/apm.sock",
		VolumeContext: map[string]string{"type": "APMSocket"},
	}

	resp, err := publisher.Publish(req)

	// Should return a response and an error because it's not a socket
	assert.NotNil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "socket not found")
}

func TestSocketPublisher_Publish_ReturnsCorrectResponse(t *testing.T) {
	tests := []struct {
		name         string
		volumeType   string
		expectedPath string
	}{
		{"APMSocket", "APMSocket", "/var/run/apm.sock"},
		{"DSDSocket", "DSDSocket", "/var/run/dsd.sock"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			mounter := mount.NewFakeMounter(nil)

			publisher := newSocketPublisher(fs, mounter, "/var/run/apm.sock", "/var/run/dsd.sock")

			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/socket",
				VolumeContext: map[string]string{"type": tc.volumeType},
			}

			resp, _ := publisher.Publish(req)

			// Verify response has correct metadata (even if mount fails)
			assert.NotNil(t, resp)
			assert.Equal(t, VolumeType(tc.volumeType), resp.VolumeType)
			assert.Equal(t, tc.expectedPath, resp.VolumePath)
		})
	}
}

func TestSocketPublisher_Stage_NotSupported(t *testing.T) {
	publisher := socketPublisher{}
	resp, err := publisher.Stage(&csi.NodeStageVolumeRequest{})
	assert.Nil(t, resp)
	assert.NoError(t, err)
}

func TestSocketPublisher_Unstage_NotSupported(t *testing.T) {
	publisher := socketPublisher{}
	resp, err := publisher.Unstage(&csi.NodeUnstageVolumeRequest{})
	assert.Nil(t, resp)
	assert.NoError(t, err)
}

func TestSocketPublisher_Unpublish_DelegatesToUnmount(t *testing.T) {
	publisher := socketPublisher{}
	resp, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.Nil(t, resp, "socket should delegate Unpublish to unmountPublisher")
	assert.NoError(t, err)
}
