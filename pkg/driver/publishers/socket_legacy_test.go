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
	// We only test cases that return (false, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext   map[string]string
		expectSupported bool
	}{
		"local mode is not supported": {
			volumeContext:   map[string]string{"mode": "local", "path": "/some/path"},
			expectSupported: false,
		},
		"type schema is not supported (handled by new publishers)": {
			volumeContext:   map[string]string{"type": "APMSocket"},
			expectSupported: false,
		},
		"mode without path is not supported": {
			volumeContext:   map[string]string{"mode": "socket"},
			expectSupported: false,
		},
		"path without mode is not supported": {
			volumeContext:   map[string]string{"path": "/some/path"},
			expectSupported: false,
		},
		"empty context is not supported": {
			volumeContext:   map[string]string{},
			expectSupported: false,
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

			supported, err := publisher.Publish(req)
			assert.Equal(t, tc.expectSupported, supported)
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

			supported, err := publisher.Publish(req)
			assert.True(t, supported, "socket mode should be supported")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not allowed")
		})
	}
}

func TestSocketLegacyPublisher_Stage_NotSupported(t *testing.T) {
	publisher := socketLegacyPublisher{}
	supported, err := publisher.Stage(&csi.NodeStageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}

func TestSocketLegacyPublisher_Unstage_NotSupported(t *testing.T) {
	publisher := socketLegacyPublisher{}
	supported, err := publisher.Unstage(&csi.NodeUnstageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}

func TestSocketLegacyPublisher_Unpublish_DelegatesToLocal(t *testing.T) {
	publisher := socketLegacyPublisher{}
	supported, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.False(t, supported, "socket legacy should delegate Unpublish to local legacy")
	assert.NoError(t, err)
}
