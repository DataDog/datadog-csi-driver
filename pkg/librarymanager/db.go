// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
	"go.etcd.io/bbolt"
)

const (
	// DatabaseFileName is the name of the database file created by bbolt.
	DatabaseFileName = "datadog-csi-driver.db"
	// LibraryMappingBucket is the name of the bucket to map libraries. Conceptually, the key structure is as follows:
	//     /library-mappings/{{ library_id }}/{{ volume_id }}
	LibraryMappingBucket = "library-mappings"
	// VolumeMappingBucket is the bucket to map volumes. Conceptually, the key structure is as follows:
	//     /volume-mappings/{{ volume_id }}/{{ library_id }}
	VolumeMappingBucket = "volume-mappings"
	// LibraryMetadataBucket stores per-library metadata indexed by library_id.
	// The value is a JSON-encoded libraryMetadata. The bucket is needed
	// because the package name is otherwise only known at the moment a
	// library is added to the cache, and is required to publish
	// per-package gauges after a process restart.
	LibraryMetadataBucket = "library-metadata"
)

type linkedVolume struct{}

type linkedLibrary struct{}

// libraryMetadata is the value stored in LibraryMetadataBucket. The struct
// is kept small on purpose: any field added here must be tolerant to being
// missing on records produced by older driver versions.
type libraryMetadata struct {
	// Package is the canonical package name (e.g. "dd-lib-java-init") that
	// shares its value across all versions of the same library and is used
	// as the metric label.
	Package string `json:"package,omitempty"`
	// SizeBytes is the on-disk size of the library, in bytes.
	SizeBytes int64 `json:"size_bytes,omitempty"`
}

// Database is a wrapper around bbolt with business logic for the library manager.
//
// # Transaction Consistency Guarantees
//
// bbolt provides serializable isolation, the highest level of transaction isolation:
//   - Write transactions are mutually exclusive (only one can run at a time)
//   - Read transactions see a consistent snapshot of the database at the time they started
//   - All operations within a single transaction are atomic (all-or-nothing)
//
// The defer tx.Rollback() pattern is safe: if Commit() was already called, Rollback() is a no-op.
//
// # External Locking Requirements
//
// While individual database transactions are atomic, operations that span multiple transactions
// or combine database operations with filesystem operations (e.g., LinkVolume + store.Add)
// require external synchronization. The LibraryManager uses a Locker to coordinate these
// compound operations on a per-library basis.
type Database struct {
	bbolt *bbolt.DB

	// cacheMu guards the in-memory aggregates below. They are eventually
	// consistent with the LibraryMetadataBucket: writers take the lock just
	// before tx.Commit() so callers that only read aggregates never block
	// long-running bbolt transactions.
	cacheMu sync.Mutex
	// cachedLibrariesByPackage counts cached library IDs per package name.
	cachedLibrariesByPackage map[string]int
	// cachedBytesByPackage sums the on-disk size of cached libraries per
	// package name.
	cachedBytesByPackage map[string]int64
	// volumeLinksByPackage counts the number of volumes currently linked
	// to any library of a given package. Updated atomically with the
	// LinkVolume/UnlinkVolume bbolt transactions.
	volumeLinksByPackage map[string]int
}

// NewDatabase initializes a new database. If a database file exists, it will re-use the existing file. Call close when
// you are done.
func NewDatabase(basePath string) (*Database, error) {
	path := filepath.Join(basePath, DatabaseFileName)
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open database at %s: %w", path, err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(LibraryMappingBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", LibraryMappingBucket, err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(VolumeMappingBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", VolumeMappingBucket, err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(LibraryMetadataBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", LibraryMetadataBucket, err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not create initial buckets: %w", err)
	}

	database := &Database{
		bbolt:                    db,
		cachedLibrariesByPackage: map[string]int{},
		cachedBytesByPackage:     map[string]int64{},
		volumeLinksByPackage:     map[string]int{},
	}

	// Seed the in-memory aggregates from the on-disk metadata so the
	// listener can publish accurate gauges immediately after startup.
	if err := database.loadCaches(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("could not load caches: %w", err)
	}

	return database, nil
}

// loadCaches scans LibraryMetadataBucket and LibraryMappingBucket once at
// startup and rebuilds the in-memory aggregates. The two buckets are walked
// in the same read transaction to guarantee a consistent snapshot.
func (db *Database) loadCaches() error {
	return db.bbolt.View(func(tx *bbolt.Tx) error {
		metaBkt := tx.Bucket([]byte(LibraryMetadataBucket))
		mappingBkt := tx.Bucket([]byte(LibraryMappingBucket))

		// libraryToPackage lets us project the per-library volume count
		// from LibraryMappingBucket onto a per-package count via
		// LibraryMetadataBucket.
		libraryToPackage := map[string]string{}
		if metaBkt != nil {
			if err := metaBkt.ForEach(func(k, v []byte) error {
				var meta libraryMetadata
				if err := json.Unmarshal(v, &meta); err != nil {
					return fmt.Errorf("could not unmarshal library metadata: %w", err)
				}
				db.cachedLibrariesByPackage[meta.Package]++
				db.cachedBytesByPackage[meta.Package] += meta.SizeBytes
				libraryToPackage[string(k)] = meta.Package
				return nil
			}); err != nil {
				return err
			}
		}

		if mappingBkt == nil {
			return nil
		}
		return mappingBkt.ForEachBucket(func(libraryKey []byte) error {
			pkg, ok := libraryToPackage[string(libraryKey)]
			if !ok {
				// Library is on disk but predates the metadata bucket;
				// we skip it (no package label available).
				return nil
			}
			libraryBkt := mappingBkt.Bucket(libraryKey)
			if libraryBkt == nil {
				return nil
			}
			return libraryBkt.ForEach(func(_, _ []byte) error {
				db.volumeLinksByPackage[pkg]++
				return nil
			})
		})
	})
}

// Close will clean up the database and should be called before exiting.
func (db *Database) Close() error {
	return db.bbolt.Close()
}

// AddLibrary records a freshly-cached library: it persists the metadata
// (package name + on-disk size) in LibraryMetadataBucket and updates the
// in-memory aggregates. AddLibrary is the canonical writer for library
// metadata; LinkVolume relies on it having been called first so that
// per-package gauges have the right package name.
//
// AddLibrary is idempotent: calling it twice for the same library overwrites
// the previous metadata (useful if the size changed) but does not
// double-count it in the aggregates.
func (db *Database) AddLibrary(libraryID, packageName string, sizeBytes int64) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}
	if packageName == "" {
		return fmt.Errorf("package name cannot be blank")
	}

	tx, err := db.bbolt.Begin(true)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	bkt := tx.Bucket([]byte(LibraryMetadataBucket))
	if bkt == nil {
		return fmt.Errorf("library metadata bucket does not exist")
	}

	// Preserve idempotency: if the library is already recorded we must
	// subtract the previous size from the aggregates before adding the
	// new one.
	var previous libraryMetadata
	if raw := bkt.Get([]byte(libraryID)); raw != nil {
		if err := json.Unmarshal(raw, &previous); err != nil {
			return fmt.Errorf("could not unmarshal existing library metadata: %w", err)
		}
	}

	meta := libraryMetadata{Package: packageName, SizeBytes: sizeBytes}
	encoded, err := json.Marshal(&meta)
	if err != nil {
		return fmt.Errorf("could not marshal library metadata: %w", err)
	}
	if err := bkt.Put([]byte(libraryID), encoded); err != nil {
		return fmt.Errorf("could not write library metadata: %w", err)
	}

	db.cacheMu.Lock()
	if previous.Package != "" {
		db.cachedLibrariesByPackage[previous.Package]--
		db.cachedBytesByPackage[previous.Package] -= previous.SizeBytes
		if db.cachedLibrariesByPackage[previous.Package] <= 0 {
			delete(db.cachedLibrariesByPackage, previous.Package)
			delete(db.cachedBytesByPackage, previous.Package)
		}
	}
	db.cachedLibrariesByPackage[packageName]++
	db.cachedBytesByPackage[packageName] += sizeBytes
	if err := tx.Commit(); err != nil {
		db.cacheMu.Unlock()
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	db.cacheMu.Unlock()

	return nil
}

// RemoveLibrary removes the metadata for a library and decrements the
// in-memory aggregates accordingly. It is a no-op if the library has no
// metadata recorded.
func (db *Database) RemoveLibrary(libraryID string) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}

	tx, err := db.bbolt.Begin(true)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	bkt := tx.Bucket([]byte(LibraryMetadataBucket))
	if bkt == nil {
		return fmt.Errorf("library metadata bucket does not exist")
	}

	raw := bkt.Get([]byte(libraryID))
	if raw == nil {
		return nil
	}
	var meta libraryMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return fmt.Errorf("could not unmarshal library metadata: %w", err)
	}
	if err := bkt.Delete([]byte(libraryID)); err != nil {
		return fmt.Errorf("could not delete library metadata: %w", err)
	}

	db.cacheMu.Lock()
	db.cachedLibrariesByPackage[meta.Package]--
	db.cachedBytesByPackage[meta.Package] -= meta.SizeBytes
	if db.cachedLibrariesByPackage[meta.Package] <= 0 {
		delete(db.cachedLibrariesByPackage, meta.Package)
		delete(db.cachedBytesByPackage, meta.Package)
	}
	if err := tx.Commit(); err != nil {
		db.cacheMu.Unlock()
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	db.cacheMu.Unlock()

	return nil
}

// PackageCacheStats returns the current cached-library count and the total
// on-disk size, in bytes, for a given package name. Both values are zero
// when the package has no cached library.
func (db *Database) PackageCacheStats(packageName string) (int, int64) {
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	return db.cachedLibrariesByPackage[packageName], db.cachedBytesByPackage[packageName]
}

// VolumeLinkCount returns the number of volumes currently linked to any
// library of the given package.
func (db *Database) VolumeLinkCount(packageName string) int {
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	return db.volumeLinksByPackage[packageName]
}

// PackageForLibrary returns the package name recorded for a library, or an
// empty string when the library has no metadata (for instance because it
// was added by an older driver version that did not yet persist
// per-library metadata).
func (db *Database) PackageForLibrary(libraryID string) (string, error) {
	if libraryID == "" {
		return "", fmt.Errorf("library ID cannot be blank")
	}

	var pkg string
	err := db.bbolt.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(LibraryMetadataBucket))
		if bkt == nil {
			return nil
		}
		raw := bkt.Get([]byte(libraryID))
		if raw == nil {
			return nil
		}
		var meta libraryMetadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			return fmt.Errorf("could not unmarshal library metadata: %w", err)
		}
		pkg = meta.Package
		return nil
	})
	return pkg, err
}

// Snapshot returns a consistent snapshot of every aggregate the listener
// needs to publish gauges. The returned maps are owned by the caller and
// safe to retain.
func (db *Database) Snapshot() libraryevents.Snapshot {
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	cachedCount := make(map[string]int, len(db.cachedLibrariesByPackage))
	for pkg, n := range db.cachedLibrariesByPackage {
		cachedCount[pkg] = n
	}
	cachedBytes := make(map[string]int64, len(db.cachedBytesByPackage))
	for pkg, n := range db.cachedBytesByPackage {
		cachedBytes[pkg] = n
	}
	volumeLinks := make(map[string]int, len(db.volumeLinksByPackage))
	for pkg, n := range db.volumeLinksByPackage {
		volumeLinks[pkg] = n
	}
	return libraryevents.Snapshot{
		CachedCountByLibrary: cachedCount,
		CachedBytesByLibrary: cachedBytes,
		VolumeLinksByLibrary: volumeLinks,
	}
}

// LinkVolume creates a bidrectional mapping between the library and volume.
// When per-library metadata is recorded (i.e. AddLibrary has been called
// before LinkVolume), the per-package volume-links aggregate is updated
// atomically with the bbolt transaction.
func (db *Database) LinkVolume(libraryID string, volumeID string) error {
	// Validate input.
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}
	if volumeID == "" {
		return fmt.Errorf("volume ID cannot be blank")
	}

	// Start a transaction.
	tx, err := db.bbolt.Begin(true)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get the bucket for library mappings. If it doesn't exist, we have a system level issue.
	libraryMappingBkt := tx.Bucket([]byte(LibraryMappingBucket))
	if libraryMappingBkt == nil {
		return fmt.Errorf("library mapping bucket does not exist")
	}

	// Get the bucket for volume mappings. If it doesn't exist, we have a system level issue.
	volumeMappingBkt := tx.Bucket([]byte(VolumeMappingBucket))
	if volumeMappingBkt == nil {
		return fmt.Errorf("volume mapping bucket does not exist")
	}

	// Create the bucket for the library if it does not exist.
	libraryBkt, err := libraryMappingBkt.CreateBucketIfNotExists([]byte(libraryID))
	if err != nil {
		return fmt.Errorf("could not create bucket for library %s: %w", libraryID, err)
	}

	// If the volume is already linked there is nothing to do; skip the
	// extra writes and the aggregate update.
	if libraryBkt.Get([]byte(volumeID)) != nil {
		return nil
	}

	// Create the bucket for the volume if it does not exist.
	volumeBkt, err := volumeMappingBkt.CreateBucketIfNotExists([]byte(volumeID))
	if err != nil {
		return fmt.Errorf("could not create bucket for volume %s: %w", volumeID, err)
	}

	// The linked volume is intentially empty at the moment with the expectation that we can add fields at a later
	// point in time without breaking existing databases.
	lp, err := json.Marshal(&linkedVolume{})
	if err != nil {
		return fmt.Errorf("could not marshal linked volume info: %w", err)
	}

	// The linked library is intentially empty at the moment with the expectation that we can add fields at a later
	// point in time without breaking existing databases.
	ll, err := json.Marshal(&linkedLibrary{})
	if err != nil {
		return fmt.Errorf("could not marshal linked library info: %w", err)
	}

	// Link the volume to the library.
	err = libraryBkt.Put([]byte(volumeID), lp)
	if err != nil {
		return fmt.Errorf("could not assign volume with id %s: %w", volumeID, err)
	}

	// Link the library to the volume.
	err = volumeBkt.Put([]byte(libraryID), ll)
	if err != nil {
		return fmt.Errorf("could not assign volume with id %s: %w", volumeID, err)
	}

	// Look up the package name now, while the transaction is still open
	// and we have a consistent view. The lookup falls back to an empty
	// string for libraries that predate the metadata bucket.
	packageName, err := packageForLibraryInTx(tx, libraryID)
	if err != nil {
		return err
	}

	// Take cacheMu just long enough to update the aggregate and commit so
	// readers never observe a state where bbolt and the in-memory cache
	// disagree.
	db.cacheMu.Lock()
	if packageName != "" {
		db.volumeLinksByPackage[packageName]++
	}
	if err := tx.Commit(); err != nil {
		if packageName != "" {
			db.volumeLinksByPackage[packageName]--
		}
		db.cacheMu.Unlock()
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	db.cacheMu.Unlock()

	return nil
}

// packageForLibraryInTx is the transaction-bound flavour of PackageForLibrary
// used internally by LinkVolume/UnlinkVolume to keep the aggregate update
// consistent with the bbolt write.
func packageForLibraryInTx(tx *bbolt.Tx, libraryID string) (string, error) {
	bkt := tx.Bucket([]byte(LibraryMetadataBucket))
	if bkt == nil {
		return "", nil
	}
	raw := bkt.Get([]byte(libraryID))
	if raw == nil {
		return "", nil
	}
	var meta libraryMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "", fmt.Errorf("could not unmarshal library metadata: %w", err)
	}
	return meta.Package, nil
}

// UnlinkVolume removes the link for a given volume.
// When the link existed and per-library metadata is recorded, the
// per-package volume-links aggregate is decremented atomically with the
// bbolt transaction.
func (db *Database) UnlinkVolume(libraryID string, volumeID string) error {
	// Validate input.
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}
	if volumeID == "" {
		return fmt.Errorf("volume ID cannot be blank")
	}

	// Start a transaction.
	tx, err := db.bbolt.Begin(true)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get the bucket for library mappings. If it doesn't exist, we have a system level issue.
	libraryMappingBkt := tx.Bucket([]byte(LibraryMappingBucket))
	if libraryMappingBkt == nil {
		return fmt.Errorf("library mapping bucket does not exist")
	}

	// Get the bucket for volume mappings. If it doesn't exist, we have a system level issue.
	volumeMappingBkt := tx.Bucket([]byte(VolumeMappingBucket))
	if volumeMappingBkt == nil {
		return fmt.Errorf("library mapping bucket does not exist")
	}

	// Detect whether the link actually existed so the aggregate is only
	// decremented for a real removal.
	wasLinked := false
	if libraryBucket := libraryMappingBkt.Bucket([]byte(libraryID)); libraryBucket != nil {
		wasLinked = libraryBucket.Get([]byte(volumeID)) != nil
	}

	// Check if the library bucket exists.
	libraryBucket := libraryMappingBkt.Bucket([]byte(libraryID))
	if libraryBucket != nil {
		// Delete the volume for library. Will return nil if the key does not exist and only returns an error if there was an issue.
		err = libraryBucket.Delete([]byte(volumeID))
		if err != nil {
			return fmt.Errorf("could not delete library mapping for volume %s: %w", volumeID, err)
		}

		// If there are no more mappings for this library, delete the bucket.
		c := libraryBucket.Cursor()
		key, _ := c.First()
		if key == nil {
			err = libraryMappingBkt.DeleteBucket([]byte(libraryID))
			if err != nil {
				return fmt.Errorf("could not delete empty bucket for library %s: %w", libraryID, err)
			}
		}
	}

	// Check if the volume bucket exists.
	volumeBucket := volumeMappingBkt.Bucket([]byte(volumeID))
	if volumeBucket != nil {
		// Delete the library for volume. Will return nil if the key does not exist and only returns an error if there was an issue.
		err = volumeBucket.Delete([]byte(libraryID))
		if err != nil {
			return fmt.Errorf("could not delete volume mapping for library %s: %w", libraryID, err)
		}

		// If there are no more mappings for this volume, delete the bucket.
		c := volumeBucket.Cursor()
		key, _ := c.First()
		if key == nil {
			err = volumeMappingBkt.DeleteBucket([]byte(volumeID))
			if err != nil {
				return fmt.Errorf("could not delete empty bucket for volume %s: %w", volumeID, err)
			}
		}
	}

	// Look up the package while the transaction is still open so the
	// aggregate update is consistent with the bbolt write.
	packageName := ""
	if wasLinked {
		packageName, err = packageForLibraryInTx(tx, libraryID)
		if err != nil {
			return err
		}
	}

	db.cacheMu.Lock()
	if packageName != "" {
		db.volumeLinksByPackage[packageName]--
		if db.volumeLinksByPackage[packageName] <= 0 {
			delete(db.volumeLinksByPackage, packageName)
		}
	}
	if err := tx.Commit(); err != nil {
		if packageName != "" {
			db.volumeLinksByPackage[packageName]++
		}
		db.cacheMu.Unlock()
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	db.cacheMu.Unlock()

	return nil
}

// GetVolumeCount returns the number of volumes linked to a library.
func (db *Database) GetVolumeCount(libraryID string) (int, error) {
	// Validate input.
	if libraryID == "" {
		return 0, fmt.Errorf("library ID cannot be blank")
	}

	// Start a transaction.
	tx, err := db.bbolt.Begin(false)
	if err != nil {
		return 0, fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get the bucket for library mappings. If it doesn't exist, we have a system level issue.
	root := tx.Bucket([]byte(LibraryMappingBucket))
	if root == nil {
		return 0, fmt.Errorf("library mapping bucket does not exist")
	}

	// Get the bucket for the library. If it doesn't exist, then there are no linked volumes for the library.
	bkt := root.Bucket([]byte(libraryID))
	if bkt == nil {
		return 0, nil
	}

	// Count the keys in the bucket.
	count := 0
	err = bkt.ForEach(func(k, v []byte) error {
		count++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("could not count volumes for library %s: %w", libraryID, err)
	}

	// Return number of volumes linked to the bucket.
	return count, nil
}

// GetLibraryForVolume returns the library mapped to a volume. A volume should only ever have one library mapped to it.
func (db *Database) GetLibraryForVolume(volumeID string) (string, error) {
	// Validate input.
	if volumeID == "" {
		return "", fmt.Errorf("volume ID cannot be blank")
	}

	// Start a transaction.
	tx, err := db.bbolt.Begin(false)
	if err != nil {
		return "", fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get the bucket for volume mappings. If it doesn't exist, we have a system level issue.
	root := tx.Bucket([]byte(VolumeMappingBucket))
	if root == nil {
		return "", fmt.Errorf("volume mapping bucket does not exist")
	}

	// Get the bucket for the volume. If it doesn't exist, then there are no linked libraries for the volume.
	bkt := root.Bucket([]byte(volumeID))
	if bkt == nil {
		return "", nil
	}

	c := bkt.Cursor()
	key, _ := c.First()
	if key == nil {
		return "", nil
	}

	return string(key), nil
}
