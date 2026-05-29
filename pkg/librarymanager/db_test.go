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
	require.NoError(t, db.UnlinkVolume(libraryID, volumeID))

	// Register the library metadata (mirrors a successful download in
	// production: AddLibrary is the canonical writer for the
	// per-library Package label).
	pkg := "dd-lib-java-init"
	require.NoError(t, db.AddLibrary(libraryID, pkg, 0))

	// Ensure a linked volume is linked.
	require.NoError(t, db.LinkVolume(libraryID, volumeID))
	require.Equal(t, 1, db.VolumeLinkCount(pkg), "first link must take the package count to 1")
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Equal(t, libraryID, lib, "there should be a library linked")
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "there should be one volume linked")

	// A second call to link the same (library, volume) must be idempotent
	// for the aggregate counters.
	require.NoError(t, db.LinkVolume(libraryID, volumeID))
	require.Equal(t, 1, db.VolumeLinkCount(pkg), "idempotent re-link must not move the package count")
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "there should still be one volume linked")

	// Linking a second volume to the same library bumps the package count.
	secondVolumeID := "test-volume-id-two"
	require.NoError(t, db.LinkVolume(libraryID, secondVolumeID))
	require.Equal(t, 2, db.VolumeLinkCount(pkg))
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 2, count, "there should be two volumes linked")

	// Unlinking one volume drops the count by one.
	require.NoError(t, db.UnlinkVolume(libraryID, secondVolumeID))
	require.Equal(t, 1, db.VolumeLinkCount(pkg))
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "there should be one volume linked")

	// Unlinking the last volume zeros the package count but keeps the
	// metadata entry around so the package can still be resolved.
	require.NoError(t, db.UnlinkVolume(libraryID, volumeID))
	require.Equal(t, 0, db.VolumeLinkCount(pkg))
	count, err = db.GetVolumeCount(libraryID)
	require.NoError(t, err)
	require.Equal(t, 0, count, "there should be no volumes linked")
	lib, err = db.GetLibraryForVolume(volumeID)
	require.NoError(t, err)
	require.Empty(t, lib, "there should be no libs linked")

	// A second unlink of the same (libraryID, volumeID) is a no-op.
	require.NoError(t, db.UnlinkVolume(libraryID, volumeID))
	require.Equal(t, 0, db.VolumeLinkCount(pkg), "repeated unlink must not move the package count")
}

func TestDatabasePackageTracking(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Two libraries, three packages worth of links. The metadata must be
	// registered (AddLibrary) before any LinkVolume to ensure the
	// per-package aggregate uses the right label.
	require.NoError(t, db.AddLibrary("lib-java", "dd-lib-java-init", 0))
	require.NoError(t, db.AddLibrary("lib-php", "dd-lib-php-init", 0))
	mustLink(t, db, "lib-java", "vol-1")
	mustLink(t, db, "lib-java", "vol-2")
	mustLink(t, db, "lib-php", "vol-3")

	require.Equal(t, map[string]int{
		"dd-lib-java-init": 2,
		"dd-lib-php-init":  1,
	}, db.Snapshot().VolumeLinksByPackage)

	pkg, err := db.GetPackageForLibrary("lib-java")
	require.NoError(t, err)
	require.Equal(t, "dd-lib-java-init", pkg)

	pkg, err = db.GetPackageForLibrary("lib-php")
	require.NoError(t, err)
	require.Equal(t, "dd-lib-php-init", pkg)

	// Unknown library yields an empty package and no error.
	pkg, err = db.GetPackageForLibrary("lib-unknown")
	require.NoError(t, err)
	require.Empty(t, pkg)

	// Unlinking the last php volume drops the package count to zero. The
	// entry is kept in the snapshot at zero rather than removed: dashboards
	// can then observe the "no volumes" transition explicitly.
	require.NoError(t, db.UnlinkVolume("lib-php", "vol-3"))
	require.Equal(t, map[string]int{
		"dd-lib-java-init": 2,
		"dd-lib-php-init":  0,
	}, db.Snapshot().VolumeLinksByPackage)
}

func TestDatabaseLinkWithoutRegistrationFallsBackToEmptyPackage(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Simulate the upgrade-from-1.2.2 case: a library is linked before any
	// AddLibrary has run, so no Package is registered.
	mustLink(t, db, "lib-legacy", "vol-legacy")

	require.Equal(t, map[string]int{"": 1}, db.Snapshot().VolumeLinksByPackage, "unregistered libraries surface under the empty key")

	pkg, err := db.GetPackageForLibrary("lib-legacy")
	require.NoError(t, err)
	require.Empty(t, pkg)
}

func TestDatabaseLibraryMetadataCacheLifecycle(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Linking a volume without prior AddLibrary is the legacy /
	// upgrade-from-pre-feature scenario: no Package or cache state is
	// registered.
	mustLink(t, db, "lib-java", "vol-1")
	snap := db.Snapshot()
	require.Empty(t, snap.CachedCountByPackage)
	require.Empty(t, snap.CachedBytesByPackage)

	// First AddLibrary registers the package and publishes the
	// library at its measured size.
	require.NoError(t, db.AddLibrary("lib-java", "dd-lib-java-init", 12_345))
	count, byteTotal := db.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 1, count)
	require.Equal(t, int64(12_345), byteTotal)

	snap = db.Snapshot()
	require.Equal(t, map[string]int{"dd-lib-java-init": 1}, snap.CachedCountByPackage)
	require.Equal(t, map[string]int64{"dd-lib-java-init": 12_345}, snap.CachedBytesByPackage)

	// Re-caching the same library replaces the size, not the count. This is
	// a defensive path for any future in-place re-cache; the package totals
	// must track the size delta.
	require.NoError(t, db.AddLibrary("lib-java", "dd-lib-java-init", 99_999))
	count, byteTotal = db.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 1, count, "re-caching must not bump the package count")
	require.Equal(t, int64(99_999), byteTotal)

	require.Equal(t, map[string]int64{"dd-lib-java-init": 99_999}, db.Snapshot().CachedBytesByPackage)

	// RemoveLibrary drops the cache state and wipes the metadata entry.
	require.NoError(t, db.RemoveLibrary("lib-java"))
	count, byteTotal = db.PackageCacheStats("dd-lib-java-init")
	require.Equal(t, 0, count)
	require.Equal(t, int64(0), byteTotal)

	pkg, err := db.GetPackageForLibrary("lib-java")
	require.NoError(t, err)
	require.Empty(t, pkg, "metadata entry must be gone after eviction")
}

func TestDatabaseRemoveLibraryUnknownIsNoop(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Evicting a library that was never marked cached must not change the
	// snapshot and must not materialize a metadata entry for it.
	require.NoError(t, db.RemoveLibrary("lib-unknown"))

	snap := db.Snapshot()
	require.Empty(t, snap.CachedCountByPackage)
	require.Empty(t, snap.CachedBytesByPackage)

	pkg, err := db.GetPackageForLibrary("lib-unknown")
	require.NoError(t, err)
	require.Empty(t, pkg, "RemoveLibrary must not materialize an entry for an unknown library")
}

func TestDatabaseAggregatesAcrossMultipleVersionsOfTheSamePackage(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	// Two distinct PHP library versions share the same package. Each version
	// has its own library_id (e.g. a digest of the image) but contributes to
	// the same per-package gauges.
	const (
		phpPackage = "dd-lib-php-init"
		php81      = "lib-php-8.1-hash"
		php82      = "lib-php-8.2-hash"
	)

	// Register each version, then link its volumes. Order mirrors production:
	// AddLibrary at download time, then LinkVolume for each mount.
	require.NoError(t, db.AddLibrary(php81, phpPackage, 5_000_000))
	mustLink(t, db, php81, "vol-a")
	mustLink(t, db, php81, "vol-b")
	require.NoError(t, db.AddLibrary(php82, phpPackage, 5_500_000))
	mustLink(t, db, php82, "vol-c")

	snap := db.Snapshot()
	require.Equal(t, map[string]int{phpPackage: 3}, snap.VolumeLinksByPackage,
		"volume links must be summed across every version of the same package")
	require.Equal(t, map[string]int{phpPackage: 2}, snap.CachedCountByPackage)
	require.Equal(t, map[string]int64{phpPackage: 10_500_000}, snap.CachedBytesByPackage)

	count, bytesTotal := db.PackageCacheStats(phpPackage)
	require.Equal(t, 2, count, "the package must count both cached versions")
	require.Equal(t, int64(10_500_000), bytesTotal, "package bytes must sum every cached version")

	// Unlinking the volumes of one version drops the link count for the
	// package by one each; the package label stays the same.
	require.NoError(t, db.UnlinkVolume(php81, "vol-a"))
	require.NoError(t, db.UnlinkVolume(php81, "vol-b"))
	require.Equal(t, map[string]int{phpPackage: 1}, db.Snapshot().VolumeLinksByPackage)

	// Evicting one version drops the package count by one and removes only
	// the corresponding bytes; the other version is untouched. In practice
	// eviction is only ever called after the last volume of a library has
	// been unlinked, hence the ordering here.
	require.NoError(t, db.RemoveLibrary(php81))
	count, bytesTotal = db.PackageCacheStats(phpPackage)
	require.Equal(t, 1, count, "evicting one version must not affect the others")
	require.Equal(t, int64(5_500_000), bytesTotal)
}

func TestDatabaseAddLibraryValidatesInputs(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	require.Error(t, db.AddLibrary("", "pkg", 1), "empty libraryID must be rejected")
	require.Error(t, db.AddLibrary("lib", "pkg", -1), "negative size must be rejected")
}

// TestDatabaseValidatesBlankInputs asserts every public method rejects empty
// IDs up front so a bug at the caller never silently corrupts the database
// (e.g. linking under an empty libraryID would aggregate orphan series).
func TestDatabaseValidatesBlankInputs(t *testing.T) {
	tsd := testutil.NewTempScratchDirectory(t)
	defer tsd.Cleanup(t)

	db, err := librarymanager.NewDatabase(tsd.Path(t))
	require.NoError(t, err)
	defer db.Close()

	require.Error(t, db.LinkVolume("", "vol"), "blank libraryID must be rejected")
	require.Error(t, db.LinkVolume("lib", ""), "blank volumeID must be rejected")
	require.Error(t, db.UnlinkVolume("", "vol"), "blank libraryID must be rejected")
	require.Error(t, db.UnlinkVolume("lib", ""), "blank volumeID must be rejected")

	_, err = db.GetVolumeCount("")
	require.Error(t, err, "blank libraryID must be rejected")
	_, err = db.GetLibraryForVolume("")
	require.Error(t, err, "blank volumeID must be rejected")
	_, err = db.GetPackageForLibrary("")
	require.Error(t, err, "blank libraryID must be rejected")

	require.Error(t, db.RemoveLibrary(""), "blank libraryID must be rejected")
}

// mustLink is a tiny test helper that asserts LinkVolume succeeded.
func mustLink(t *testing.T, db *librarymanager.Database, libraryID, volumeID string) {
	t.Helper()
	require.NoError(t, db.LinkVolume(libraryID, volumeID))
}
