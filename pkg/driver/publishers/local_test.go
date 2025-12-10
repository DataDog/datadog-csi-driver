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

func TestLocalPublisher_Publish_TypeSelection(t *testing.T) {
	// These tests verify the type selection logic only.
	// We only test cases that return (false, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext   map[string]string
		expectSupported bool
	}{
		"APMSocket type is not supported (handled by socket publisher)": {
			volumeContext:   map[string]string{"type": "APMSocket"},
			expectSupported: false,
		},
		"DSDSocket type is not supported (handled by socket publisher)": {
			volumeContext:   map[string]string{"type": "DSDSocket"},
			expectSupported: false,
		},
		"unknown type is not supported": {
			volumeContext:   map[string]string{"type": "Unknown"},
			expectSupported: false,
		},
		"legacy mode/path is not supported (handled by legacy publisher)": {
			volumeContext:   map[string]string{"mode": "local", "path": "/some/path"},
			expectSupported: false,
		},
		"empty context is not supported": {
			volumeContext:   map[string]string{},
			expectSupported: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			publisher := localPublisher{
				apmSocketPath: "/var/run/apm.sock",
				dsdSocketPath: "/var/run/dsd.sock",
			}

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

func TestLocalPublisher_Stage_NotSupported(t *testing.T) {
	publisher := localPublisher{}
	supported, err := publisher.Stage(&csi.NodeStageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}

func TestLocalPublisher_Unstage_NotSupported(t *testing.T) {
	publisher := localPublisher{}
	supported, err := publisher.Unstage(&csi.NodeUnstageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}

func TestLocalPublisher_Unpublish_DelegatesToLegacy(t *testing.T) {
	publisher := localPublisher{}
	supported, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.False(t, supported, "local should delegate Unpublish to legacy")
	assert.NoError(t, err)
}
