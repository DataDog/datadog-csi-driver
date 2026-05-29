// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"path/filepath"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/testutil"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

// TestNewDatabaseRecoversFromMissingMetadataBucket simulates a database
// written by an older driver version (links present, no LibraryMetadataBucket
// at all). NewDatabase must recreate the bucket without dropping existing
// volume mappings; the per-library package is not recovered until the next
// AddLibrary runs (typically at the next download for that library).
func TestNewDatabaseRecoversFromMissingMetadataBucket(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	dir := tsd.Path(t)

	db, err := NewDatabase(dir)
	require.NoError(t, err)
	// The pre-migration driver had no metadata bucket, so links are created
	// without any associated package.
	mustInternalLink(t, db, "lib-java", "vol-1")
	mustInternalLink(t, db, "lib-java", "vol-2")

	// Strip the LibraryMetadataBucket entirely to mimic what an older driver
	// would have produced.
	require.NoError(t, db.bbolt.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket([]byte(LibraryMetadataBucket))
	}))
	require.NoError(t, db.Close())

	// Reopen: NewDatabase must recreate the bucket. Existing links survive,
	// even though their package is now unknown until AddLibrary runs.
	db2, err := NewDatabase(dir)
	require.NoError(t, err)
	defer db2.Close()

	lib, err := db2.GetLibraryForVolume("vol-1")
	require.NoError(t, err)
	require.Equal(t, "lib-java", lib, "existing mappings must be preserved across the migration")

	// The per-library volume count must be reseeded from bbolt at startup so
	// the cleanup path never wrongly removes a library that still has volumes
	// linked to it.
	count, err := db2.GetVolumeCount("lib-java")
	require.NoError(t, err)
	require.Equal(t, 2, count, "GetVolumeCount must be reseeded from bbolt across restarts")

	pkg, err := db2.GetPackageForLibrary("lib-java")
	require.NoError(t, err)
	require.Empty(t, pkg, "package is not recovered until the next AddLibrary after migration")

	// Pre-migration links are surfaced under an empty package label until
	// the library is re-registered. Tagging the gauge series with "" lets
	// observers remap it to "unknown" at publish time.
	require.Equal(t, map[string]int{"": 2}, db2.Snapshot().VolumeLinksByPackage,
		"links carried over from the old layout must be counted under an empty package")

	// Re-registering the library (e.g. at the next download) sets the
	// package label going forward; previously-aggregated link counts stay
	// under "" until they are recycled (unlink + re-link).
	require.NoError(t, db2.AddLibrary("lib-java", "dd-lib-java-init", 0))
	mustInternalLink(t, db2, "lib-java", "vol-3")
	pkg, err = db2.GetPackageForLibrary("lib-java")
	require.NoError(t, err)
	require.Equal(t, "dd-lib-java-init", pkg)
	require.Equal(t, map[string]int{"": 2, "dd-lib-java-init": 1}, db2.Snapshot().VolumeLinksByPackage,
		"only the post-migration link is moved to the package aggregate")
}

// TestDatabaseFileEnsureSchema checks that DatabaseFileName is created under
// the provided directory and the LibraryMetadataBucket is present after a
// fresh open. This is a small sanity check that doubles as documentation of
// where the database file lands.
func TestDatabaseFileEnsureSchema(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	dir := tsd.Path(t)
	db, err := NewDatabase(dir)
	require.NoError(t, err)
	defer db.Close()

	// The DB file exists at the expected location.
	_, err = db.bbolt.Path(), error(nil)
	require.Equal(t, filepath.Join(dir, DatabaseFileName), db.bbolt.Path())

	// All three buckets are present right after a fresh open.
	require.NoError(t, db.bbolt.View(func(tx *bbolt.Tx) error {
		for _, name := range []string{LibraryMappingBucket, VolumeMappingBucket, LibraryMetadataBucket} {
			require.NotNil(t, tx.Bucket([]byte(name)), "bucket %s must exist after NewDatabase", name)
		}
		return nil
	}))
}

func mustInternalLink(t *testing.T, db *Database, libraryID, volumeID string) {
	t.Helper()
	require.NoError(t, db.LinkVolume(libraryID, volumeID))
}
