// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func testFs() afero.Afero {
	return afero.Afero{Fs: afero.NewOsFs()}
}

func TestNewStore(t *testing.T) {
	tests := map[string]struct {
		createDir bool
		copyFiles bool
	}{
		"new store creates directory if it does not exist": {
			createDir: false,
			copyFiles: false,
		},
		"new store is ok if the directory already exists but is empty": {
			createDir: true,
			copyFiles: false,
		},
		"new store is ok if the directory already exists with files": {
			createDir: true,
			copyFiles: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Validate test input.
			if test.copyFiles {
				require.True(t, test.createDir, "test is not properly configured, copying files requires the directory to exist")
			}

			// Create scratch space.
			tsd := testutil.NewTempScratchDirectory(t)
			defer tsd.Cleanup(t)

			// Optionally create the base directory.
			basePath := filepath.Join(tsd.Path(t), "base-path")
			if test.createDir {
				err := os.MkdirAll(basePath, 0o755)
				require.NoError(t, err, "could not setup base path for the test")
			}

			// Optionally copy files.
			if test.copyFiles {
				testFile := filepath.Join(basePath, "test.txt")
				err := os.WriteFile(testFile, []byte("test"), 0o755)
				require.NoError(t, err, "could not copy files for test")
			}

			// Require store to create successfully and path to exist.
			store, err := librarymanager.NewStore(testFs(), basePath)
			require.NoError(t, err, "no error was expected")
			require.NotNil(t, store, "store should not be empty when no error is returned")
			require.DirExists(t, basePath, "directory should be created")
		})
	}
}

func TestStore(t *testing.T) {
	// Create scratch space.
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	basePath := filepath.Join(tsd.Path(t), "base-path")
	store, err := librarymanager.NewStore(testFs(), basePath)
	require.NoError(t, err, "no error was expected")

	id := "test-library-id"

	// Verify the package does not exist.
	exists, err := store.Exists(id)
	require.NoError(t, err, "no error was expected")
	require.False(t, exists, "package should not exist in store")

	// Verify the package cannot be got.
	actual, err := store.Get(id)
	require.Error(t, err, "error is expected when package doesn't exist")
	require.Empty(t, actual, "path should be empty if an error is returned")

	// Verify the package that doesnt exist will not error
	err = store.Remove(id)
	require.NoError(t, err, "remove for a package that does not exist should not error")

	// Verify nonexistent source should error.
	packagePath := filepath.Join(basePath, "package-path")
	actual, err = store.Add(id, packagePath)
	require.Error(t, err, "add for a package that does not exist on the filesystem should error")
	require.Empty(t, actual, "path should be empty if an error is returned")

	// Setup test package.
	err = os.MkdirAll(packagePath, 0o755)
	require.NoError(t, err, "could not setup package path for the test")

	// Verify empty directory should error.
	actual, err = store.Add(id, packagePath)
	require.Error(t, err, "add for an empty package directory should error")
	require.Empty(t, actual, "path should be empty if an error is returned")

	// Create a test file in test package.
	testFile := filepath.Join(packagePath, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0o755)
	require.NoError(t, err, "could not copy files for test")

	// Add the package.
	actual, err = store.Add(id, packagePath)
	require.NoError(t, err, "no error was expected")
	require.Equal(t, filepath.Join(basePath, id), actual)

	// Verify that adding the package again should not error.
	actual, err = store.Add(id, packagePath)
	require.NoError(t, err, "a package that already exists should not error")
	require.Equal(t, filepath.Join(basePath, id), actual)

	// Verify that the package now exists.
	exists, err = store.Exists(id)
	require.True(t, exists, "package should exist")
	require.NoError(t, err, "no error should be returned")

	// Verify that the package can be got.
	actual, err = store.Get(id)
	require.NoError(t, err, "no error was expected")
	require.Equal(t, filepath.Join(basePath, id), actual)

	// Verfiy the package can be removed.
	err = store.Remove(id)
	require.NoError(t, err, "no error was expected")

	// Verify the package no longer exists.
	exists, err = store.Exists(id)
	require.False(t, exists, "package should no longer exist exist")
	require.NoError(t, err, "no error should be returned")
}

// TestStoreValidatesBlankID asserts every public method rejects an empty
// ID, which is the only invariant a caller can break by accident.
func TestStoreValidatesBlankID(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	store, err := librarymanager.NewStore(testFs(), filepath.Join(tsd.Path(t), "base-path"))
	require.NoError(t, err)

	_, err = store.Add("", "/tmp/whatever")
	require.Error(t, err, "blank id must be rejected by Add")
	_, err = store.Get("")
	require.Error(t, err, "blank id must be rejected by Get")
	require.Error(t, store.Remove(""), "blank id must be rejected by Remove")
	_, err = store.Exists("")
	require.Error(t, err, "blank id must be rejected by Exists")
}

// TestNewStoreRejectsNonDirectoryPath documents that pointing the store at
// an existing file (a misconfiguration) surfaces as a NewStore error rather
// than corrupting the file with a later Add.
func TestNewStoreRejectsNonDirectoryPath(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	// Create a regular file where the store base path is supposed to live.
	filePath := filepath.Join(tsd.Path(t), "not-a-dir")
	require.NoError(t, os.WriteFile(filePath, []byte("placeholder"), 0o600))

	store, err := librarymanager.NewStore(testFs(), filePath)
	require.Error(t, err, "NewStore must refuse a base path that is a regular file")
	require.Nil(t, store, "store must be nil on error")
}

