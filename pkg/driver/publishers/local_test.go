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

func TestLocalPublisher_Publish_Success(t *testing.T) {
	const (
		apmSocketPath    = "/var/run/datadog/apm.sock"
		dsdSocketPath    = "/var/run/datadog/dsd.sock"
		expectedHostPath = "/var/run/datadog"
	)

	volumeTypes := []string{"APMSocketDirectory", "DSDSocketDirectory", "DatadogSocketsDirectory"}

	for _, volumeType := range volumeTypes {
		t.Run(volumeType, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			mounter := mount.NewFakeMounter(nil)

			publisher := newLocalPublisher(fs, mounter, apmSocketPath, dsdSocketPath)

			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/datadog",
				VolumeContext: map[string]string{"type": volumeType},
			}

			resp, err := publisher.Publish(req)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, VolumeType(volumeType), resp.VolumeType)
			assert.Equal(t, expectedHostPath, resp.VolumePath)

			// Verify mount was called
			log := mounter.GetLog()
			require.Len(t, log, 1)
			assert.Equal(t, "mount", log[0].Action)
			assert.Equal(t, expectedHostPath, log[0].Source)
			assert.Equal(t, "/target/datadog", log[0].Target)

			// Verify target directory was created
			exists, err := fs.DirExists("/target/datadog")
			assert.NoError(t, err)
			assert.True(t, exists)
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
