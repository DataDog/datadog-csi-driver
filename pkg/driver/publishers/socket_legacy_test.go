// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
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

func TestSocketLegacyPublisher_Stage_NotSupported(t *testing.T) {
	publisher := socketLegacyPublisher{}
	resp, err := publisher.Stage(&csi.NodeStageVolumeRequest{})
	assert.Nil(t, resp)
	assert.NoError(t, err)
}

func TestSocketLegacyPublisher_Unstage_NotSupported(t *testing.T) {
	publisher := socketLegacyPublisher{}
	resp, err := publisher.Unstage(&csi.NodeUnstageVolumeRequest{})
	assert.Nil(t, resp)
	assert.NoError(t, err)
}

func TestSocketLegacyPublisher_Unpublish_DelegatesToUnmount(t *testing.T) {
	publisher := socketLegacyPublisher{}
	resp, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.Nil(t, resp, "socket legacy should delegate Unpublish to unmountPublisher")
	assert.NoError(t, err)
}
