// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package testutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TempScratchDirectory is a temporary directory for tests.
type TempScratchDirectory struct {
	path string
}

// NewTempScratchDirectory creates a new temporary directory for testing.
func NewTempScratchDirectory(t *testing.T) *TempScratchDirectory {
	t.Helper()
	testPath, err := os.MkdirTemp("", "csi-driver-test-*")
	require.NoError(t, err, "could not setup destination dir for the test")
	return &TempScratchDirectory{
		path: testPath,
	}
}

// Path returns the path to the temporary directory.
func (tsd *TempScratchDirectory) Path(t *testing.T) string {
	return tsd.path
}

// Cleanup removes the temporary directory.
func (tsd *TempScratchDirectory) Cleanup(t *testing.T) {
	t.Helper()
	err := os.RemoveAll(tsd.path)
	require.NoError(t, err, "could not clean up scratch space")
}
