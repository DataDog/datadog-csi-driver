// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/mount"
)

func TestLibraryPublisher_Publish_TypeSelection(t *testing.T) {
	// These tests verify the type selection logic only.
	// We only test cases that return (nil, nil) before doing any work.
	tests := map[string]struct {
		volumeContext map[string]string
	}{
		"APMSocket type is not supported (handled by socket publisher)": {
			volumeContext: map[string]string{"type": "APMSocket"},
		},
		"DatadogInjectorPreload type is not supported": {
			volumeContext: map[string]string{"type": "DatadogInjectorPreload"},
		},
		"APMSocketDirectory type is not supported": {
			volumeContext: map[string]string{"type": "APMSocketDirectory"},
		},
		"unknown type is not supported": {
			volumeContext: map[string]string{"type": "Unknown"},
		},
		"empty context is not supported": {
			volumeContext: map[string]string{},
		},
		"legacy mode/path is not supported": {
			volumeContext: map[string]string{"mode": "local", "path": "/some/path"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// We can test type selection without a real LibraryManager
			// because the check happens before any LibraryManager calls
			publisher := libraryPublisher{}

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

func TestLibraryPublisher_Publish_DisabledRejectsRequest(t *testing.T) {
	publisher := libraryPublisher{disabled: true}

	req := &csi.NodePublishVolumeRequest{
		VolumeId:   "test-volume",
		TargetPath: "/target/path",
		Readonly:   true,
		VolumeContext: map[string]string{
			"type":                                "DatadogLibrary",
			"dd.csi.datadog.com/library.package":  "test-image",
			"dd.csi.datadog.com/library.registry": "gcr.io/example",
			"dd.csi.datadog.com/library.version":  "v1.0.0",
		},
	}

	resp, err := publisher.Publish(req)

	assert.NotNil(t, resp, "response should be non-nil for metrics")
	assert.Equal(t, DatadogLibrary, resp.VolumeType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSI is disabled")
}

func TestLibraryPublisher_Publish_RejectsNonReadOnly(t *testing.T) {
	publisher := libraryPublisher{}

	req := &csi.NodePublishVolumeRequest{
		VolumeId:   "test-volume",
		TargetPath: "/target/path",
		Readonly:   false, // Should be rejected
		VolumeContext: map[string]string{
			"type":                                "DatadogLibrary",
			"dd.csi.datadog.com/library.package":  "test-image",
			"dd.csi.datadog.com/library.registry": "gcr.io/example",
			"dd.csi.datadog.com/library.version":  "v1.0.0",
		},
	}

	resp, err := publisher.Publish(req)

	assert.NotNil(t, resp, "response should be non-nil for metrics")
	assert.Equal(t, DatadogLibrary, resp.VolumeType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be mounted in read-only mode")
}

func TestLibraryPublisher_Publish_InvalidLibraryConfig(t *testing.T) {
	// Test that invalid library configuration returns an error
	tests := map[string]struct {
		volumeContext map[string]string
		expectedError string
	}{
		"missing package": {
			volumeContext: map[string]string{
				"type":                               "DatadogLibrary",
				"dd.csi.datadog.com/library.version": "v1.0.0",
			},
			expectedError: "invalid library configuration",
		},
		"missing version": {
			volumeContext: map[string]string{
				"type":                               "DatadogLibrary",
				"dd.csi.datadog.com/library.package": "dd-lib-java-init",
			},
			expectedError: "invalid library configuration",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// We don't need a LibraryManager for these tests because
			// NewLibrary validation fails before we call the manager
			publisher := libraryPublisher{}

			req := &csi.NodePublishVolumeRequest{
				VolumeId:      "test-volume",
				TargetPath:    "/target/path",
				Readonly:      true, // Library volumes must be mounted read-only
				VolumeContext: tc.volumeContext,
			}

			resp, err := publisher.Publish(req)
			assert.NotNil(t, resp, "response should be non-nil even on error for metrics")
			assert.Equal(t, DatadogLibrary, resp.VolumeType)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestLibraryPublisher_Publish_Success(t *testing.T) {
	// Setup local registry with test image
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	localRegistry.AddImage(t, "../../librarymanager/testdata/image.tar", "test-image", "v1.0.0")

	// Create temp directory for library manager
	basePath := t.TempDir()

	// Create library manager with test downloader
	fs := afero.Afero{Fs: afero.NewOsFs()}
	lm, err := librarymanager.NewLibraryManager(basePath,
		librarymanager.WithFilesystem(fs),
		librarymanager.WithDownloader(librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))),
	)
	require.NoError(t, err)
	defer lm.Stop()

	// Create publisher
	mounter := mount.NewFakeMounter(nil)
	publisher := newLibraryPublisher(fs, mounter, lm, false)

	// Create target directory
	targetPath := filepath.Join(t.TempDir(), "target", "library")

	req := &csi.NodePublishVolumeRequest{
		VolumeId:   "test-volume-123",
		TargetPath: targetPath,
		Readonly:   true, // Library volumes must be mounted read-only
		VolumeContext: map[string]string{
			"type":                                "DatadogLibrary",
			"dd.csi.datadog.com/library.package":  "test-image",
			"dd.csi.datadog.com/library.registry": localRegistry.Registry(t),
			"dd.csi.datadog.com/library.version":  "v1.0.0",
		},
	}

	resp, err := publisher.Publish(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, DatadogLibrary, resp.VolumeType)
	assert.Contains(t, resp.VolumePath, "test-image:v1.0.0")

	// Verify mount was called
	mountLog := mounter.GetLog()
	require.Len(t, mountLog, 1)
	assert.Equal(t, "mount", mountLog[0].Action)
	assert.Equal(t, targetPath, mountLog[0].Target)
	assert.True(t, strings.Contains(mountLog[0].Source, "datadog-init/package"),
		"mount source should contain library path, got: %s", mountLog[0].Source)
}

func TestLibraryPublisher_Unpublish_Success(t *testing.T) {
	// Setup local registry with test image
	localRegistry := testutil.NewLocalRegistry(t)
	defer localRegistry.Stop()
	localRegistry.AddImage(t, "../../librarymanager/testdata/image.tar", "test-image", "v1.0.0")

	// Create temp directory for library manager
	basePath := t.TempDir()

	// Create library manager with test downloader
	fs := afero.Afero{Fs: afero.NewOsFs()}
	lm, err := librarymanager.NewLibraryManager(basePath,
		librarymanager.WithFilesystem(fs),
		librarymanager.WithDownloader(librarymanager.NewDownloaderWithRoundTripper(localRegistry.GetRoundTripper(t))),
	)
	require.NoError(t, err)
	defer lm.Stop()

	// Create publisher
	mounter := mount.NewFakeMounter(nil)
	publisher := newLibraryPublisher(fs, mounter, lm, false)

	// First, publish a volume
	targetPath := filepath.Join(t.TempDir(), "target", "library")
	publishReq := &csi.NodePublishVolumeRequest{
		VolumeId:   "test-volume-456",
		TargetPath: targetPath,
		Readonly:   true, // Library volumes must be mounted read-only
		VolumeContext: map[string]string{
			"type":                                "DatadogLibrary",
			"dd.csi.datadog.com/library.package":  "test-image",
			"dd.csi.datadog.com/library.registry": localRegistry.Registry(t),
			"dd.csi.datadog.com/library.version":  "v1.0.0",
		},
	}

	_, err = publisher.Publish(publishReq)
	require.NoError(t, err)

	// Verify volume is tracked
	hasVolume, err := lm.HasVolume("test-volume-456")
	require.NoError(t, err)
	require.True(t, hasVolume, "volume should be tracked after publish")

	// Now unpublish
	unpublishReq := &csi.NodeUnpublishVolumeRequest{
		VolumeId:   "test-volume-456",
		TargetPath: targetPath,
	}

	resp, err := publisher.Unpublish(unpublishReq)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, DatadogLibrary, resp.VolumeType)

	// Verify volume is no longer tracked
	hasVolume, err = lm.HasVolume("test-volume-456")
	require.NoError(t, err)
	assert.False(t, hasVolume, "volume should not be tracked after unpublish")

	// Verify library was removed from store (since it was the only volume using it)
	storeDir := filepath.Join(basePath, librarymanager.StoreDirectory)
	entries, err := os.ReadDir(storeDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "store should be empty after last volume is unpublished")
}
