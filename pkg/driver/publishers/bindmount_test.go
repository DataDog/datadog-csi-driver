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

type recordingMounter struct {
	*mount.FakeMounter
	mounts       []recordedMount
	alreadyMount bool
}

type recordedMount struct {
	source  string
	target  string
	fstype  string
	options []string
}

func (m *recordingMounter) Mount(source string, target string, fstype string, options []string) error {
	m.mounts = append(m.mounts, recordedMount{
		source:  source,
		target:  target,
		fstype:  fstype,
		options: append([]string(nil), options...),
	})
	return m.FakeMounter.Mount(source, target, fstype, options)
}

func (m *recordingMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	if m.alreadyMount {
		return false, nil
	}
	return m.FakeMounter.IsLikelyNotMountPoint(file)
}

func TestBindMount_CreatesTargetDirectory(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	// Create source directory
	require.NoError(t, fs.MkdirAll("/host/dir", 0755))

	err := bindMount(fs, mounter, "/host/dir", "/target/dir", false, false)

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

	// Create source file
	require.NoError(t, fs.MkdirAll("/host", 0755))
	_, err := fs.Create("/host/socket.sock")
	require.NoError(t, err)

	err = bindMount(fs, mounter, "/host/socket.sock", "/target/socket.sock", true, false)

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

	// Create source directory
	require.NoError(t, fs.MkdirAll("/host/path", 0755))

	err := bindMount(fs, mounter, "/host/path", "/target/path", false, false)

	assert.NoError(t, err)
	log := mounter.GetLog()
	require.Len(t, log, 1)
	// Verify mount action was called with correct source/target
	assert.Equal(t, "mount", log[0].Action)
	assert.Equal(t, "/host/path", log[0].Source)
	assert.Equal(t, "/target/path", log[0].Target)
}

func TestBindMount_ReadOnlyRemountsBind(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := &recordingMounter{FakeMounter: mount.NewFakeMounter(nil)}

	require.NoError(t, fs.MkdirAll("/host/path", 0755))

	err := bindMount(fs, mounter, "/host/path", "/target/path", false, true)

	require.NoError(t, err)
	require.Len(t, mounter.mounts, 1)
	assert.Equal(t, recordedMount{source: "/host/path", target: "/target/path", options: []string{"bind", "ro"}}, mounter.mounts[0])
}

func TestBindMount_ReadOnlyRemountsAlreadyMountedTarget(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := &recordingMounter{FakeMounter: mount.NewFakeMounter(nil), alreadyMount: true}
	var remounted []recordedMount
	originalRemountReadOnlyBind := remountReadOnlyBind
	remountReadOnlyBind = func(hostPath, targetPath string) error {
		remounted = append(remounted, recordedMount{source: hostPath, target: targetPath})
		return nil
	}
	t.Cleanup(func() {
		remountReadOnlyBind = originalRemountReadOnlyBind
	})

	require.NoError(t, fs.MkdirAll("/host/path", 0755))
	require.NoError(t, fs.MkdirAll("/target/path", 0755))

	err := bindMount(fs, mounter, "/host/path", "/target/path", false, true)

	require.NoError(t, err)
	assert.Empty(t, mounter.mounts)
	assert.Equal(t, []recordedMount{{source: "/host/path", target: "/target/path"}}, remounted)
}
