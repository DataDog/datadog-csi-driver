// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {
	// Create scratch space.
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	// Create the database.
	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Ensure there are no volumes linked when none have been linked.
	volumeID := "test-volume-id"
	lib, err := db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Empty(t, lib, "there should be no libs linked")

	// Ensure there are no libraries linked when none have been linked.
	libraryID := "test-library-id"
	count, err := db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 0, count, "there should be no volumes linked")

	// Ensure that an unlink does not produce an error if it has not been linked.
	err = db.UnlinkVolume(libraryID, volumeID)
	require.NoError(t, err)

	// Ensure a linked volume is linked.
	err = db.LinkVolume(libraryID, volumeID)
	require.NoError(t, err)
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "there should be one volume linked")

	// Ensure a second call to link the same volume does nothing
	err = db.LinkVolume(libraryID, volumeID)
	require.NoError(t, err)
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "there should still be one volume linked")

	// Ensure a second linked volume shows both.
	secondVolumeID := "test-volume-id-two"
	err = db.LinkVolume(libraryID, secondVolumeID)
	require.NoError(t, err)
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 2, count, "there should be two volumes linked")

	// Ensure an unlinked volume only has one volume linked.
	err = db.UnlinkVolume(libraryID, secondVolumeID)
	require.NoError(t, err)
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "there should be one volume linked")

	// Ensure all unlinks completely zeros out the count.
	err = db.UnlinkVolume(libraryID, volumeID)
	require.NoError(t, err)
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 0, count, "there should be no volumes linked")
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Empty(t, lib, "there should be no libs linked")
}
