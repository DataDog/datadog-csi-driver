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

func TestLocalLegacyPublisher_Publish_ModeSelection(t *testing.T) {
	// These tests verify the mode selection logic only.
	// We only test cases that return (nil, nil) before calling bindMount.
	tests := map[string]struct {
		volumeContext map[string]string
	}{
		"socket mode is not supported": {
			volumeContext: map[string]string{"mode": "socket", "path": "/some/path"},
		},
		"type schema is not supported (handled by new publishers)": {
			volumeContext: map[string]string{"type": "APMSocketDirectory"},
		},
		"mode without path is not supported": {
			volumeContext: map[string]string{"mode": "local"},
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
			publisher := localLegacyPublisher{}

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

			resp, err := publisher.Publish(req)
			assert.NotNil(t, resp, "local mode should be supported")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not allowed")
		})
	}
}

func TestLocalLegacyPublisher_Publish_Success(t *testing.T) {
	const (
		apmSocketPath = "/var/run/datadog/apm.sock"
		dsdSocketPath = "/opt/datadog/dsd.sock"
	)

	// Allowed paths are parent directories of the socket paths
	hostPaths := []string{"/var/run/datadog", "/opt/datadog"}

	for _, hostPath := range hostPaths {
		t.Run(hostPath, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			mounter := mount.NewFakeMounter(nil)

			// Create source directory
			require.NoError(t, fs.MkdirAll(hostPath, 0755))

			publisher := newLocalLegacyPublisher(fs, mounter, apmSocketPath, dsdSocketPath)

			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/datadog",
				VolumeContext: map[string]string{"mode": "local", "path": hostPath},
			}

			resp, err := publisher.Publish(req)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, VolumeType("local"), resp.VolumeType)
			assert.Equal(t, hostPath, resp.VolumePath)

			// Verify mount was called
			log := mounter.GetLog()
			require.Len(t, log, 1)
			assert.Equal(t, "mount", log[0].Action)
			assert.Equal(t, hostPath, log[0].Source)
			assert.Equal(t, "/target/datadog", log[0].Target)
		})
	}
}

func TestLocalLegacyPublisher_Unpublish_DelegatesToUnmount(t *testing.T) {
	publisher := localLegacyPublisher{}
	resp, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.Nil(t, resp, "local legacy should delegate Unpublish to unmountPublisher")
	assert.NoError(t, err)
}
