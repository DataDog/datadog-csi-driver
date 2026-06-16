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

func TestDatabaseLibraryMetadata(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// A package with no cached library reports zero stats and an empty package name.
	count, bytes := db.PackageCacheStats("unknown")
	require.Equal(t, 0, count)
	require.Equal(t, int64(0), bytes)

	pkg, err := db.PackageForLibrary("unknown")
	require.NoError(t, err)
	require.Empty(t, pkg)

	// Two versions of the same package aggregate together.
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 100))
	require.NoError(t, db.AddLibrary("lib-id-2", "dd-lib-java-init", 200))
	count, bytes = db.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 2, count)
	require.Equal(t, int64(300), bytes)

	pkg, err = db.PackageForLibrary("lib-id-1")
	require.NoError(t, err)
	require.Equal(t, "dd-lib-java-init", pkg)

	// AddLibrary on the same library ID is idempotent: it overwrites the
	// previous record without double-counting.
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 150))
	count, bytes = db.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 2, count)
	require.Equal(t, int64(350), bytes)

	// A different package is tracked independently.
	require.NoError(t, db.AddLibrary("php-id", "dd-lib-php-init", 42))
	snap := db.Snapshot()
	require.Equal(t, 2, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(350), snap.CachedBytesByLibrary["dd-lib-java-init"])
	require.Equal(t, 1, snap.CachedCountByLibrary["dd-lib-php-init"])
	require.Equal(t, int64(42), snap.CachedBytesByLibrary["dd-lib-php-init"])

	// Removing one of two versions keeps the package present.
	require.NoError(t, db.RemoveLibrary("lib-id-1"))
	count, bytes = db.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 1, count)
	require.Equal(t, int64(200), bytes)

	// Removing the last version drops the package entry from the aggregates.
	require.NoError(t, db.RemoveLibrary("lib-id-2"))
	count, bytes = db.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 0, count)
	require.Equal(t, int64(0), bytes)

	// RemoveLibrary is a no-op on an unknown library.
	require.NoError(t, db.RemoveLibrary("never-existed"))
}

func TestDatabaseMetadataPersistsAcrossRestart(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 1024))
	require.NoError(t, db.Close())

	// Reopen and verify the aggregates were rebuilt from the persisted state.
	db2, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db2.Close()

	count, bytes := db2.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 1, count)
	require.Equal(t, int64(1024), bytes)
}

func TestDatabaseValidatesBlankInputsForMetadata(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	require.Error(t, db.AddLibrary("", "pkg", 0))
	require.Error(t, db.AddLibrary("lib", "", 0))
	require.Error(t, db.RemoveLibrary(""))
	_, err = db.PackageForLibrary("")
	require.Error(t, err)
}

func TestDatabaseVolumeLinksAggregateByPackage(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Aggregate is zero before any link is created.
	require.Equal(t, 0, db.VolumeLinkCount("dd-lib-java-init"))

	// Without metadata the aggregate is not touched even after a link.
	require.NoError(t, db.LinkVolume("lib-id-1", "vol-1"))
	require.Equal(t, 0, db.VolumeLinkCount("dd-lib-java-init"))

	// Once metadata is recorded, subsequent links update the aggregate.
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 100))
	require.NoError(t, db.AddLibrary("lib-id-2", "dd-lib-java-init", 200))
	require.NoError(t, db.LinkVolume("lib-id-1", "vol-2"))
	require.NoError(t, db.LinkVolume("lib-id-2", "vol-3"))
	require.Equal(t, 2, db.VolumeLinkCount("dd-lib-java-init"))

	// Re-linking the same volume is a no-op for the aggregate.
	require.NoError(t, db.LinkVolume("lib-id-1", "vol-2"))
	require.Equal(t, 2, db.VolumeLinkCount("dd-lib-java-init"))

	// Unlinks decrement and eventually drop the package entry.
	require.NoError(t, db.UnlinkVolume("lib-id-1", "vol-2"))
	require.NoError(t, db.UnlinkVolume("lib-id-2", "vol-3"))
	require.Equal(t, 0, db.VolumeLinkCount("dd-lib-java-init"))

	// Unlinking an unknown volume is a no-op for the aggregate.
	require.NoError(t, db.UnlinkVolume("lib-id-1", "never-existed"))
	require.Equal(t, 0, db.VolumeLinkCount("dd-lib-java-init"))
}

func TestDatabaseVolumeLinksReloadedFromDisk(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 100))
	require.NoError(t, db.LinkVolume("lib-id-1", "vol-1"))
	require.NoError(t, db.LinkVolume("lib-id-1", "vol-2"))
	require.NoError(t, db.Close())

	db2, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db2.Close()

	snap := db2.Snapshot()
	require.Equal(t, 2, snap.VolumeLinksByLibrary["dd-lib-java-init"])
}
