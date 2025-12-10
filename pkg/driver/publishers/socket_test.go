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

func TestSocketPublisher_Publish_TypeSelection(t *testing.T) {
	// These tests verify the type selection logic only.
	// We only test cases that return (false, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext   map[string]string
		expectSupported bool
	}{
		"APMSocketDirectory type is not supported (handled by local publisher)": {
			volumeContext:   map[string]string{"type": "APMSocketDirectory"},
			expectSupported: false,
		},
		"DSDSocketDirectory type is not supported (handled by local publisher)": {
			volumeContext:   map[string]string{"type": "DSDSocketDirectory"},
			expectSupported: false,
		},
		"unknown type is not supported": {
			volumeContext:   map[string]string{"type": "Unknown"},
			expectSupported: false,
		},
		"legacy mode/path is not supported (handled by legacy publisher)": {
			volumeContext:   map[string]string{"mode": "socket", "path": "/some/socket.sock"},
			expectSupported: false,
		},
		"empty context is not supported": {
			volumeContext:   map[string]string{},
			expectSupported: false,
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

			supported, err := publisher.Publish(req)
			assert.Equal(t, tc.expectSupported, supported)
			assert.NoError(t, err)
		})
	}
}

func TestSocketPublisher_Stage_NotSupported(t *testing.T) {
	publisher := socketPublisher{}
	supported, err := publisher.Stage(&csi.NodeStageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}

func TestSocketPublisher_Unstage_NotSupported(t *testing.T) {
	publisher := socketPublisher{}
	supported, err := publisher.Unstage(&csi.NodeUnstageVolumeRequest{})
	assert.False(t, supported)
	assert.NoError(t, err)
}

func TestSocketPublisher_Unpublish_DelegatesToLegacy(t *testing.T) {
	publisher := socketPublisher{}
	supported, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.False(t, supported, "socket should delegate Unpublish to legacy")
	assert.NoError(t, err)
}
