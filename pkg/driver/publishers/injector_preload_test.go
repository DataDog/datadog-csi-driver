// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"sync"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/mount"
)

func TestInjectorPreloadPublisher_Publish_TypeSelection(t *testing.T) {
	// These tests verify the type selection logic only.
	// We only test cases that return (nil, nil) before doing any work.
	tests := map[string]struct {
		volumeContext map[string]string
	}{
		"APMSocket type is not supported": {
			volumeContext: map[string]string{"type": "APMSocket"},
		},
		"DatadogLibrary type is not supported": {
			volumeContext: map[string]string{"type": "DatadogLibrary"},
		},
		"unknown type is not supported": {
			volumeContext: map[string]string{"type": "Unknown"},
		},
		"empty context is not supported": {
			volumeContext: map[string]string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			publisher := &injectorPreloadPublisher{}

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

func TestInjectorPreloadPublisher_Publish_DisabledRejectsRequest(t *testing.T) {
	publisher := &injectorPreloadPublisher{disabled: true}

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume",
		TargetPath:    "/target/ld.so.preload",
		Readonly:      true,
		VolumeContext: map[string]string{"type": "DatadogInjectorPreload"},
	}

	resp, err := publisher.Publish(req)

	assert.NotNil(t, resp, "response should be non-nil for metrics")
	assert.Equal(t, DatadogInjectorPreload, resp.VolumeType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSI is disabled")
}

func TestInjectorPreloadPublisher_Publish_RejectsNonReadOnly(t *testing.T) {
	publisher := &injectorPreloadPublisher{}

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume",
		TargetPath:    "/target/ld.so.preload",
		Readonly:      false, // Should be rejected
		VolumeContext: map[string]string{"type": "DatadogInjectorPreload"},
	}

	resp, err := publisher.Publish(req)

	assert.NotNil(t, resp, "response should be non-nil for metrics")
	assert.Equal(t, DatadogInjectorPreload, resp.VolumeType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be mounted in read-only mode")
}

func TestInjectorPreloadPublisher_Publish_Success(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	publisher := newInjectorPreloadPublisher(fs, mounter, "/var/datadog", false)

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume",
		TargetPath:    "/target/ld.so.preload",
		Readonly:      true, // Injector preload volumes must be mounted read-only
		VolumeContext: map[string]string{"type": "DatadogInjectorPreload"},
	}

	resp, err := publisher.Publish(req)

	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, DatadogInjectorPreload, resp.VolumeType)
	assert.Equal(t, "/target/ld.so.preload", resp.VolumePath)

	// Verify preload file was created with correct content
	content, err := fs.ReadFile("/var/datadog/ld.so.preload")
	assert.NoError(t, err)
	assert.Equal(t, defaultPreloadContent, string(content))

	// Verify mount was called
	log := mounter.GetLog()
	require.Len(t, log, 1)
	assert.Equal(t, "mount", log[0].Action)
	assert.Equal(t, "/var/datadog/ld.so.preload", log[0].Source)
	assert.Equal(t, "/target/ld.so.preload", log[0].Target)
}

func TestInjectorPreloadPublisher_Publish_Idempotent(t *testing.T) {
	// Verifies that calling Publish multiple times doesn't fail
	// and reuses the same preload file
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	publisher := newInjectorPreloadPublisher(fs, mounter, "/var/datadog", false)

	req := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume-1",
		TargetPath:    "/target1/ld.so.preload",
		Readonly:      true,
		VolumeContext: map[string]string{"type": "DatadogInjectorPreload"},
	}

	// First publish
	resp, err := publisher.Publish(req)
	assert.NoError(t, err)
	require.NotNil(t, resp)

	// Second publish with different volume
	req2 := &csi.NodePublishVolumeRequest{
		VolumeId:      "test-volume-2",
		TargetPath:    "/target2/ld.so.preload",
		Readonly:      true,
		VolumeContext: map[string]string{"type": "DatadogInjectorPreload"},
	}

	resp, err = publisher.Publish(req2)
	assert.NoError(t, err)
	require.NotNil(t, resp)

	// Verify mount was called twice
	log := mounter.GetLog()
	assert.Len(t, log, 2)
}

func TestInjectorPreloadPublisher_Publish_Concurrent(t *testing.T) {
	// Verifies that concurrent publishes don't cause race conditions
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	publisher := newInjectorPreloadPublisher(fs, mounter, "/var/datadog", false)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/ld.so.preload",
				Readonly:      true,
				VolumeContext: map[string]string{"type": "DatadogInjectorPreload"},
			}
			_, err := publisher.Publish(req)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent publish failed: %v", err)
	}

	// Verify preload file exists and has correct content
	content, err := fs.ReadFile("/var/datadog/ld.so.preload")
	assert.NoError(t, err)
	assert.Equal(t, defaultPreloadContent, string(content))
}

func TestInjectorPreloadPublisher_Unpublish_DelegatesToUnmount(t *testing.T) {
	publisher := &injectorPreloadPublisher{}
	resp, err := publisher.Unpublish(&csi.NodeUnpublishVolumeRequest{})
	assert.Nil(t, resp, "injectorPreload should delegate Unpublish to unmountPublisher")
	assert.NoError(t, err)
}
