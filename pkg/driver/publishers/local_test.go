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
	// We only test cases that return (nil, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext map[string]string
	}{
		"APMSocket type is not supported (handled by socket publisher)": {
			volumeContext: map[string]string{"type": "APMSocket"},
		},
		"DSDSocket type is not supported (handled by socket publisher)": {
			volumeContext: map[string]string{"type": "DSDSocket"},
		},
		"unknown type is not supported": {
			volumeContext: map[string]string{"type": "Unknown"},
		},
		"legacy mode/path is not supported (handled by legacy publisher)": {
			volumeContext: map[string]string{"mode": "local", "path": "/some/path"},
		},
		"empty context is not supported": {
			volumeContext: map[string]string{},
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

			resp, err := publisher.Publish(req)
			assert.Nil(t, resp)
			assert.NoError(t, err)
		})
	}
}

func TestLocalPublisher_Stage_NotSupported(t *testing.T) {
	publisher := localPublisher{}
	resp, err := publisher.Stage(&csi.NodeStageVolumeRequest{})
	assert.Nil(t, resp)
	assert.NoError(t, err)
}

func TestLocalPublisher_Unstage_NotSupported(t *testing.T) {
	publisher := localPublisher{}
	resp, err := publisher.Unstage(&csi.NodeUnstageVolumeRequest{})
	assert.Nil(t, resp)
	assert.NoError(t, err)
}

func TestLocalPublisher_Unpublish_DelegatesToUnmount(t *testing.T) {
	publisher := localPublisher{}
	resp, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.Nil(t, resp, "local should delegate Unpublish to unmountPublisher")
	assert.NoError(t, err)
}
