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

func TestLocalLegacyPublisher_Publish_ModeSelection(t *testing.T) {
	// These tests verify the mode selection logic only.
	// We only test cases that return (false, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext   map[string]string
		expectSupported bool
	}{
		"socket mode is not supported": {
			volumeContext:   map[string]string{"mode": "socket", "path": "/some/path"},
			expectSupported: false,
		},
		"type schema is not supported (handled by new publishers)": {
			volumeContext:   map[string]string{"type": "APMSocketDirectory"},
			expectSupported: false,
		},
		"mode without path is not supported": {
			volumeContext:   map[string]string{"mode": "local"},
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
			publisher := localLegacyPublisher{}

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

func TestLocalLegacyPublisher_Publish_PathValidation(t *testing.T) {
	// apmSocketPath=/var/run/datadog/apm.sock → allowed dir = /var/run/datadog
	// dsdSocketPath=/opt/datadog/dsd.sock → allowed dir = /opt/datadog
	tests := map[string]struct {
		path string
	}{
		"disallowed path fails": {
			path: "/etc/passwd",
		},
		"similar but different path fails": {
			path: "/var/run/datadog/subdir",
		},
		"empty path fails": {
			path: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			publisher := localLegacyPublisher{
				apmSocketPath: "/var/run/datadog/apm.sock",
				dsdSocketPath: "/opt/datadog/dsd.sock",
			}

			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/path",
				VolumeContext: map[string]string{"mode": "local", "path": tc.path},
			}

			supported, err := publisher.Publish(req)
			assert.True(t, supported, "local mode should be supported")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not allowed")
		})
	}
}

func TestLocalLegacyPublisher_Stage_NotSupported(t *testing.T) {
	publisher := localLegacyPublisher{}
	supported, err := publisher.Stage(&csi.NodeStageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}

func TestLocalLegacyPublisher_Unstage_NotSupported(t *testing.T) {
	publisher := localLegacyPublisher{}
	supported, err := publisher.Unstage(&csi.NodeUnstageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}
