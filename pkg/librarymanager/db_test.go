// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

// volumeCount is a small helper to read the live volume count of a library
// through the public GetLibrary accessor.
func volumeCount(t *testing.T, db *librarymanager.Database, libraryID string) int {
	t.Helper()
	info, _, err := db.GetLibrary(libraryID)
	require.NoError(t, err)
	return info.VolumeCount
}

// link is a small helper to link a volume and assert success.
func link(t *testing.T, db *librarymanager.Database, libraryID, volumeID string) {
	t.Helper()
	require.NoError(t, db.LinkVolume(libraryID, volumeID, false))
}

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
	require.Equal(t, 0, volumeCount(t, db, libraryID), "there should be no volumes linked")

	// Ensure that an unlink does not produce an error if it has not been linked.
	unlinked, pkg, err := db.UnlinkVolume(volumeID)
	require.NoError(t, err)
	require.Empty(t, unlinked, "unlinking an unknown volume reports no library")
	require.Empty(t, pkg, "unlinking an unknown volume reports no package")

	// Ensure a linked volume is linked.
	err = db.LinkVolume(libraryID, volumeID, false)
	require.NoError(t, err)
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	require.Equal(t, 1, volumeCount(t, db, libraryID), "there should be one volume linked")

	// Ensure a second call to link the same volume does nothing.
	err = db.LinkVolume(libraryID, volumeID, false)
	require.NoError(t, err)
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	require.Equal(t, 1, volumeCount(t, db, libraryID), "there should still be one volume linked")

	// Ensure a second linked volume shows both.
	secondVolumeID := "test-volume-id-two"
	err = db.LinkVolume(libraryID, secondVolumeID, false)
	require.NoError(t, err)
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	require.Equal(t, 2, volumeCount(t, db, libraryID), "there should be two volumes linked")

	// Ensure an unlinked volume only has one volume linked.
	unlinked, _, err = db.UnlinkVolume(secondVolumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, unlinked, "unlink reports the library the volume used")
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	require.Equal(t, 1, volumeCount(t, db, libraryID), "there should be one volume linked")

	// Ensure all unlinks completely zeros out the count.
	unlinked, _, err = db.UnlinkVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, unlinked)
	require.Equal(t, 0, volumeCount(t, db, libraryID), "there should be no volumes linked")
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

	// An unknown library reports no record.
	info, found, err := db.GetLibrary("unknown")
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, info.Package)
	require.Equal(t, int64(0), info.SizeBytes)

	// Two versions of the same package aggregate together in the snapshot.
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 100))
	require.NoError(t, db.AddLibrary("lib-id-2", "dd-lib-java-init", 200))
	snap, err := db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 2, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(300), snap.CachedBytesByLibrary["dd-lib-java-init"])

	info, found, err = db.GetLibrary("lib-id-1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "dd-lib-java-init", info.Package)
	require.Equal(t, int64(100), info.SizeBytes)

	// AddLibrary on the same library ID is idempotent: it overwrites the
	// previous record without double-counting.
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 150))
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 2, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(350), snap.CachedBytesByLibrary["dd-lib-java-init"])

	// A different package is tracked independently.
	require.NoError(t, db.AddLibrary("php-id", "dd-lib-php-init", 42))
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 2, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(350), snap.CachedBytesByLibrary["dd-lib-java-init"])
	require.Equal(t, 1, snap.CachedCountByLibrary["dd-lib-php-init"])
	require.Equal(t, int64(42), snap.CachedBytesByLibrary["dd-lib-php-init"])

	// Removing one of two versions keeps the package present.
	require.NoError(t, db.RemoveLibrary("lib-id-1"))
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 1, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(200), snap.CachedBytesByLibrary["dd-lib-java-init"])

	// Removing the last version drops the package entry from the aggregates.
	require.NoError(t, db.RemoveLibrary("lib-id-2"))
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 0, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(0), snap.CachedBytesByLibrary["dd-lib-java-init"])

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

	snap, err := db2.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 1, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(1024), snap.CachedBytesByLibrary["dd-lib-java-init"])
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
	_, _, err = db.GetLibrary("")
	require.Error(t, err)
	_, _, err = db.UnlinkVolume("")
	require.Error(t, err)
	_, err = db.GetLibraryForVolume("")
	require.Error(t, err)
}

func TestDatabaseVolumeLinksAggregateByPackage(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Aggregate is zero before any link is created.
	snap, err := db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 0, snap.VolumeLinksByLibrary["dd-lib-java-init"])

	// Without metadata the aggregate is not touched even after a link
	// (a library without a package label is excluded from the snapshot).
	link(t, db, "lib-id-1", "vol-1")
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 0, snap.VolumeLinksByLibrary["dd-lib-java-init"])

	// Once metadata is recorded, the aggregate reflects the links.
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 100))
	require.NoError(t, db.AddLibrary("lib-id-2", "dd-lib-java-init", 200))
	link(t, db, "lib-id-2", "vol-3")
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 2, snap.VolumeLinksByLibrary["dd-lib-java-init"])

	// Re-linking the same volume is a no-op for the aggregate.
	link(t, db, "lib-id-1", "vol-1")
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 2, snap.VolumeLinksByLibrary["dd-lib-java-init"])

	// Unlinks decrement and eventually drop the package entry. The unlink
	// also reports the package the volume used.
	_, unlinkedPkg, err := db.UnlinkVolume("vol-1")
	require.NoError(t, err)
	require.Equal(t, "dd-lib-java-init", unlinkedPkg)
	_, _, err = db.UnlinkVolume("vol-3")
	require.NoError(t, err)
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 0, snap.VolumeLinksByLibrary["dd-lib-java-init"])

	// Unlinking an unknown volume is a no-op for the aggregate.
	_, _, err = db.UnlinkVolume("never-existed")
	require.NoError(t, err)
	snap, err = db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 0, snap.VolumeLinksByLibrary["dd-lib-java-init"])
}

func TestDatabaseVolumeLinksReloadedFromDisk(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	require.NoError(t, db.AddLibrary("lib-id-1", "dd-lib-java-init", 100))
	link(t, db, "lib-id-1", "vol-1")
	link(t, db, "lib-id-1", "vol-2")
	require.NoError(t, db.Close())

	db2, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db2.Close()

	snap, err := db2.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 2, snap.VolumeLinksByLibrary["dd-lib-java-init"])
}

// TestDatabaseMigrationDeduplicatesVolumeAcrossLibraries verifies that a
// legacy database which mapped the same volume under more than one library
// (possible because the legacy LinkVolume only checked the per-library bucket)
// migrates the volume exactly once, so the owning library carries the only
// count and the others are left at zero and remain cleanable.
func TestDatabaseMigrationDeduplicatesVolumeAcrossLibraries(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	dbPath := filepath.Join(tsd.Path(t), librarymanager.DatabaseFileName)

	legacy, err := bbolt.Open(dbPath, 0600, nil)
	require.NoError(t, err)
	require.NoError(t, legacy.Update(func(tx *bbolt.Tx) error {
		mappings, err := tx.CreateBucketIfNotExists([]byte("library-mappings"))
		if err != nil {
			return err
		}
		// The same volume appears under two libraries.
		for _, libID := range []string{"lib-a", "lib-b"} {
			libBkt, err := mappings.CreateBucketIfNotExists([]byte(libID))
			if err != nil {
				return err
			}
			if err := libBkt.Put([]byte("shared-vol"), []byte("{}")); err != nil {
				return err
			}
		}
		return nil
	}))
	require.NoError(t, legacy.Close())

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// The volume maps to exactly one library.
	lib, err := db.GetLibraryForVolume("shared-vol")
	require.NoError(t, err)
	require.Contains(t, []string{"lib-a", "lib-b"}, lib)

	// The shared volume is counted exactly once, by the owning library only.
	require.Equal(t, 1, volumeCount(t, db, "lib-a")+volumeCount(t, db, "lib-b"),
		"the shared volume is counted exactly once across libraries")
	require.Equal(t, 1, volumeCount(t, db, lib), "the owning library carries the only count")
}

// TestDatabaseMigratesLegacySchema seeds a database with the legacy
// nested-bucket schema (library-mappings/volume-mappings/library-metadata)
// and verifies NewDatabase rewrites it into the flat volumes/libraries schema
// without losing link or metadata information.
func TestDatabaseMigratesLegacySchema(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	dbPath := filepath.Join(tsd.Path(t), librarymanager.DatabaseFileName)

	// Build a legacy-shaped database by hand.
	legacy, err := bbolt.Open(dbPath, 0600, nil)
	require.NoError(t, err)
	require.NoError(t, legacy.Update(func(tx *bbolt.Tx) error {
		meta, err := tx.CreateBucketIfNotExists([]byte("library-metadata"))
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(map[string]any{"package": "dd-lib-java-init", "size_bytes": 512})
		if err != nil {
			return err
		}
		if err := meta.Put([]byte("lib-id-1"), encoded); err != nil {
			return err
		}

		mappings, err := tx.CreateBucketIfNotExists([]byte("library-mappings"))
		if err != nil {
			return err
		}
		libBkt, err := mappings.CreateBucketIfNotExists([]byte("lib-id-1"))
		if err != nil {
			return err
		}
		if err := libBkt.Put([]byte("vol-1"), []byte("{}")); err != nil {
			return err
		}
		if err := libBkt.Put([]byte("vol-2"), []byte("{}")); err != nil {
			return err
		}

		// A legacy library without metadata: it must still migrate its links.
		orphanBkt, err := mappings.CreateBucketIfNotExists([]byte("lib-id-orphan"))
		if err != nil {
			return err
		}
		if err := orphanBkt.Put([]byte("vol-3"), []byte("{}")); err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte("volume-mappings"))
		return err
	}))
	require.NoError(t, legacy.Close())

	// Reopen through NewDatabase, which runs the migration.
	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)

	// Library metadata and the migrated volume count are preserved.
	info, found, err := db.GetLibrary("lib-id-1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "dd-lib-java-init", info.Package)
	require.Equal(t, int64(512), info.SizeBytes)
	require.Equal(t, 2, info.VolumeCount)

	// Volume -> library links survive the migration.
	for _, vol := range []string{"vol-1", "vol-2"} {
		lib, err := db.GetLibraryForVolume(vol)
		require.NoError(t, err)
		require.Equal(t, "lib-id-1", lib)
	}

	// The orphan library (no metadata) keeps its links and a count.
	orphan, found, err := db.GetLibrary("lib-id-orphan")
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, orphan.Package)
	require.Equal(t, 1, orphan.VolumeCount)

	// The snapshot reflects only labelled libraries.
	snap, err := db.Snapshot()
	require.NoError(t, err)
	require.Equal(t, 1, snap.CachedCountByLibrary["dd-lib-java-init"])
	require.Equal(t, int64(512), snap.CachedBytesByLibrary["dd-lib-java-init"])
	require.Equal(t, 2, snap.VolumeLinksByLibrary["dd-lib-java-init"])

	// The legacy buckets are gone after migration.
	require.NoError(t, db.Close())
	check, err := bbolt.Open(dbPath, 0600, nil)
	require.NoError(t, err)
	defer check.Close()
	require.NoError(t, check.View(func(tx *bbolt.Tx) error {
		require.Nil(t, tx.Bucket([]byte("library-mappings")))
		require.Nil(t, tx.Bucket([]byte("volume-mappings")))
		require.Nil(t, tx.Bucket([]byte("library-metadata")))
		require.NotNil(t, tx.Bucket([]byte("volumes")))
		require.NotNil(t, tx.Bucket([]byte("libraries")))
		return nil
	}))
}
