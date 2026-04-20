// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/mount"
)

func TestCreateStorageDir(t *testing.T) {
	t.Run("returns empty path without error when input is blank", func(t *testing.T) {
		fs := afero.Afero{Fs: afero.NewMemMapFs()}

		path, err := createStorageDir(fs, "   ")

		assert.NoError(t, err)
		assert.Empty(t, path)
	})

	t.Run("returns error when directory cannot be created", func(t *testing.T) {
		fs := afero.Afero{Fs: afero.NewReadOnlyFs(afero.NewMemMapFs())}

		path, err := createStorageDir(fs, "/var/datadog")

		assert.Empty(t, path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create storage base path")
	})

	t.Run("returns error when directory is not writable", func(t *testing.T) {
		baseFs := afero.NewMemMapFs()
		err := baseFs.MkdirAll("/var/datadog", 0o755)
		assert.NoError(t, err)

		fs := afero.Afero{Fs: afero.NewReadOnlyFs(baseFs)}

		path, err := createStorageDir(fs, "/var/datadog")

		assert.Empty(t, path)
		assert.Error(t, err)
	})

	t.Run("returns configured path when writable", func(t *testing.T) {
		fs := afero.Afero{Fs: afero.NewMemMapFs()}

		path, err := createStorageDir(fs, "/var/datadog")

		assert.NoError(t, err)
		assert.Equal(t, "/var/datadog", path)
	})
}

func TestNewDatadogCSIDriver_CreatesLibraryManagerWhenStoragePathIsWritable(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	mounter := mount.NewFakeMounter(nil)
	storageBasePath := t.TempDir()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	driver, err := newDatadogCSIDriver(
		fs,
		mounter,
		logger,
		"test-driver",
		"/tmp/apm.sock",
		"/tmp/dsd.sock",
		storageBasePath,
		"test-version",
		true,
	)
	require.NoError(t, err)

	assert.NotNil(t, driver.libraryManager)
	assert.NotContains(t, logs.String(), "Disabling SSI storage")
}

func TestNewDatadogCSIDriver_DisablesSSIStorageWhenStoragePathIsNotWritable(t *testing.T) {
	baseFs := afero.NewMemMapFs()
	err := baseFs.MkdirAll("/var/datadog", 0o755)
	require.NoError(t, err)

	fs := afero.Afero{Fs: afero.NewReadOnlyFs(baseFs)}
	mounter := mount.NewFakeMounter(nil)

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	driver, err := newDatadogCSIDriver(
		fs,
		mounter,
		logger,
		"test-driver",
		"/tmp/apm.sock",
		"/tmp/dsd.sock",
		"/var/datadog",
		"test-version",
		true,
	)
	require.NoError(t, err)

	assert.Nil(t, driver.libraryManager)
	assert.Contains(t, logs.String(), "Disabling SSI storage")
	assert.Contains(t, logs.String(), "storage_base_path=/var/datadog")

	t.Run("library volume is ignored", func(t *testing.T) {
		resp, err := driver.publisher.Publish(&csi.NodePublishVolumeRequest{
			VolumeId:   "library-volume",
			TargetPath: "/target/library",
			Readonly:   true,
			VolumeContext: map[string]string{
				"type": "DatadogLibrary",
			},
		})

		assert.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("injector preload volume is ignored", func(t *testing.T) {
		resp, err := driver.publisher.Publish(&csi.NodePublishVolumeRequest{
			VolumeId:   "preload-volume",
			TargetPath: "/target/ld.so.preload",
			Readonly:   true,
			VolumeContext: map[string]string{
				"type": "DatadogInjectorPreload",
			},
		})

		assert.NoError(t, err)
		assert.Nil(t, resp)
	})
}
