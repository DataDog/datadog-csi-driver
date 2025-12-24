// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/mount"
)

func TestBindMount_CreatesTargetDirectory(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	err := bindMount(fs, mounter, "/host/dir", "/target/dir", false)

	assert.NoError(t, err)
	// Verify target directory was created in afero
	exists, err := fs.DirExists("/target/dir")
	assert.NoError(t, err)
	assert.True(t, exists)
	// Verify mount was called (FakeMounter always mounts because it doesn't check real FS)
	log := mounter.GetLog()
	assert.Len(t, log, 1)
	assert.Equal(t, "mount", log[0].Action)
	assert.Equal(t, "/host/dir", log[0].Source)
	assert.Equal(t, "/target/dir", log[0].Target)
}

func TestBindMount_CreatesTargetFile(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	err := bindMount(fs, mounter, "/host/socket.sock", "/target/socket.sock", true)

	assert.NoError(t, err)
	// Verify target file was created in afero
	exists, err := fs.Exists("/target/socket.sock")
	assert.NoError(t, err)
	assert.True(t, exists)
	// Verify it's a file, not a directory
	isDir, err := fs.IsDir("/target/socket.sock")
	assert.NoError(t, err)
	assert.False(t, isDir)
}

func TestBindMount_CallsMounter(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	err := bindMount(fs, mounter, "/host/path", "/target/path", false)

	assert.NoError(t, err)
	log := mounter.GetLog()
	require.Len(t, log, 1)
	// Verify mount action was called with correct source/target
	assert.Equal(t, "mount", log[0].Action)
	assert.Equal(t, "/host/path", log[0].Source)
	assert.Equal(t, "/target/path", log[0].Target)
}
