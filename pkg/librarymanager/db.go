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
	// LibraryMetadataBucket stores per-library metadata keyed by library ID.
	// It is the canonical place to look up the package name for a library
	// and holds the cache-state fields used by the libraries_cached* gauges.
	//     /library-metadata/{{ library_id }} -> libraryMetadata
	LibraryMetadataBucket = "library-metadata"
)

// linkedVolume is the value stored in LibraryMappingBucket[libraryID][volumeID].
type linkedVolume struct{}

// linkedLibrary is the value stored in VolumeMappingBucket[volumeID][libraryID].
type linkedLibrary struct{}

// libraryMetadata is the value stored in LibraryMetadataBucket[libraryID]
type libraryMetadata struct {
	// Package is the library package name (e.g. dd-lib-java-init).
	Package string `json:"package,omitempty"`
	// SizeBytes is the size of the library payload in bytes.
	SizeBytes int64 `json:"size_bytes,omitempty"`
	// IsCached is whether the library payload is currently present on disk.
	IsCached bool `json:"is_cached,omitempty"`
	// VolumeCount is the number of volumes currently linked to this library.
	// This is not persisted because bbolt is always the source of truth for the link set itself.
	// It is rebuilt from LibraryMappingBucket at startup and kept in sync by
	// LinkVolume / UnlinkVolume. Lets GetVolumeCount answer in O(1) without
	// re-scanning bbolt on every cleanup attempt.
	VolumeCount int `json:"-"`
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
// # In-memory caches
//
// Database also maintains in-memory aggregates derived from bbolt so the
// caller can publish stateful gauges without re-scanning the database on
// every mutation. The caches are seeded at NewDatabase from the persisted
// state and kept in sync under cacheMu, which is held for the duration of
// each mutation so external observers never see bbolt and the caches drift.
//
// # External Locking Requirements
//
// While individual database transactions are atomic, operations that span multiple transactions
// or combine database operations with filesystem operations (e.g., LinkVolume + store.Add)
// require external synchronization. The LibraryManager uses a Locker to coordinate these
// compound operations on a per-library basis.
type Database struct {
	bbolt *bbolt.DB

	// cacheMu protects the in-memory derived state below. It is held for the
	// full duration of any mutation that touches bbolt so the cache and the
	// persisted state never disagree as seen from the outside.
	cacheMu sync.Mutex

	// metadataByLibrary mirrors LibraryMetadataBucket. It holds every library
	// the database has ever seen (whether currently cached or not), keyed by
	// libraryID. Used to resolve a library's package in O(1) without going
	// back to bbolt and as the source for derived aggregates.
	metadataByLibrary map[string]libraryMetadata

	// volumeLinksByPackage is the per-package count of LibraryMappingBucket
	// entries. Drives the library_volume_links gauge.
	volumeLinksByPackage map[string]int

	// cachedCountByPackage and cachedBytesByPackage are the per-package
	// aggregates for libraries with IsCached=true. They drive the
	// libraries_cached and libraries_cached_bytes gauges respectively.
	cachedCountByPackage map[string]int
	cachedBytesByPackage map[string]int64
}

// NewDatabase initializes a new database. If a database file exists, it will re-use the existing file. Call Close when
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
		bbolt:                db,
		metadataByLibrary:    make(map[string]libraryMetadata),
		volumeLinksByPackage: make(map[string]int),
		cachedCountByPackage: make(map[string]int),
		cachedBytesByPackage: make(map[string]int64),
	}
	if err := database.loadCaches(); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not load library caches: %w", err)
	}
	return database, nil
}

// loadCaches populates the in-memory derived state from bbolt. Called once at
// startup; subsequent writes keep the cache in sync directly.
func (db *Database) loadCaches() error {
	return db.bbolt.View(func(tx *bbolt.Tx) error {
		metaBkt := tx.Bucket([]byte(LibraryMetadataBucket))
		if metaBkt == nil {
			return fmt.Errorf("library metadata bucket does not exist")
		}
		mappingBkt := tx.Bucket([]byte(LibraryMappingBucket))
		if mappingBkt == nil {
			return fmt.Errorf("library mapping bucket does not exist")
		}

		if err := metaBkt.ForEach(func(k, v []byte) error {
			if v == nil {
				return nil
			}
			var meta libraryMetadata
			if err := json.Unmarshal(v, &meta); err != nil {
				return fmt.Errorf("could not unmarshal library metadata for %s: %w", string(k), err)
			}
			db.metadataByLibrary[string(k)] = meta
			if meta.IsCached {
				db.cachedCountByPackage[meta.Package]++
				db.cachedBytesByPackage[meta.Package] += meta.SizeBytes
			}
			return nil
		}); err != nil {
			return err
		}

		// Count the volumes linked to each library directly from bbolt and
		// seed both the per-library VolumeCount (used by GetVolumeCount on the
		// cleanup path) and the per-package aggregate (used by the gauge).
		return mappingBkt.ForEach(func(libraryID, v []byte) error {
			if v != nil {
				return nil
			}
			bkt := mappingBkt.Bucket(libraryID)
			if bkt == nil {
				return nil
			}
			n := 0
			if err := bkt.ForEach(func(_, _ []byte) error {
				n++
				return nil
			}); err != nil {
				return fmt.Errorf("could not iterate library bucket %s: %w", libraryID, err)
			}
			meta := db.metadataByLibrary[string(libraryID)]
			meta.VolumeCount = n
			db.metadataByLibrary[string(libraryID)] = meta
			db.volumeLinksByPackage[meta.Package] += n
			return nil
		})
	})
}

// Close will clean up the database and should be called before exiting.
func (db *Database) Close() error {
	return db.bbolt.Close()
}

// LinkVolume creates a bidirectional mapping between the library and volume.
// The package the library belongs to must have been registered beforehand by
// AddLibrary; LinkVolume itself only writes the link buckets.
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

	// Create the bucket for the volume if it does not exist.
	volumeBkt, err := volumeMappingBkt.CreateBucketIfNotExists([]byte(volumeID))
	if err != nil {
		return fmt.Errorf("could not create bucket for volume %s: %w", volumeID, err)
	}

	// Re-linking the same (library, volume) pair is a no-op: both link
	// buckets already exist (otherwise the Get would have returned nil) and
	// the aggregate counters must not move. Bail out before doing any
	// write so we skip the commit entirely.
	if libraryBkt.Get([]byte(volumeID)) != nil {
		return nil
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

	// Hold cacheMu across Commit and the cache update so readers never
	// observe bbolt and the in-memory aggregates disagree.
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()

	// Commit the transaction.
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

	db.updateVolumeLinked(libraryID)
	return nil
}

// UnlinkVolume removes the link for a given volume. Idempotent: unlinking a
// non-existent link is a no-op and does not return an error.
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

	// Track whether bbolt actually changed so the in-memory counters mirror
	// the persisted state precisely. bbolt.Delete returns nil for both
	// "deleted" and "did not exist", hence the explicit lookup.
	wasRemoved := false

	// Check if the library bucket exists.
	libraryBucket := libraryMappingBkt.Bucket([]byte(libraryID))
	if libraryBucket != nil {
		wasRemoved = libraryBucket.Get([]byte(volumeID)) != nil
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

	// See LinkVolume for the cacheMu/Commit ordering rationale.
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()

	// Commit the transaction.
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

	if wasRemoved {
		db.updateVolumeUnlinked(libraryID)
	}
	return nil
}

// GetVolumeCount returns the number of volumes linked to a library. Reads
// from the in-memory libraryMetadata.VolumeCount, which is seeded from bbolt
// at startup and kept in sync by LinkVolume / UnlinkVolume.
func (db *Database) GetVolumeCount(libraryID string) (int, error) {
	// Validate input.
	if libraryID == "" {
		return 0, fmt.Errorf("library ID cannot be blank")
	}

	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	return db.metadataByLibrary[libraryID].VolumeCount, nil
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

// GetPackageForLibrary returns the package name persisted for a library, or an
// empty string if the library is not tracked. Reads from the in-memory cache.
func (db *Database) GetPackageForLibrary(libraryID string) (string, error) {
	if libraryID == "" {
		return "", fmt.Errorf("library ID cannot be blank")
	}
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	return db.metadataByLibrary[libraryID].Package, nil
}

// AddLibrary registers a library in the metadata bucket and marks it as
// cached on disk. It is the canonical writer for the library metadata
// entry: the package name, payload size and cache flag are all set here.
// Intended to be called once per successful download, before LinkVolume.
// Re-calling AddLibrary on an already-cached library updates the size
// (used as a defensive path for any future in-place re-cache).
//
// Callers that need to publish the per-package aggregate after the mutation
// should read it via PackageCacheStats.
func (db *Database) AddLibrary(libraryID string, packageName string, sizeBytes int64) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}
	if sizeBytes < 0 {
		return fmt.Errorf("sizeBytes must be non-negative, got %d", sizeBytes)
	}

	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()

	// Build the bbolt payload from the current cache state (VolumeCount is
	// in-memory only and stripped by the json tags).
	next := db.metadataByLibrary[libraryID]
	next.Package = packageName
	next.SizeBytes = sizeBytes
	next.IsCached = true

	if err := db.bbolt.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(LibraryMetadataBucket))
		if bkt == nil {
			return fmt.Errorf("library metadata bucket does not exist")
		}
		return writeLibraryMetadata(bkt, []byte(libraryID), next)
	}); err != nil {
		return err
	}

	db.updateLibraryCached(libraryID, packageName, sizeBytes)
	return nil
}

// RemoveLibrary drops a library's metadata entry, mirroring an eviction
// from the on-disk store. Intended to be called by the cleanup path once
// the library has no remaining volume links and its payload has been
// deleted from disk. Idempotent: if the library was never added (unknown
// or already removed), the call is a no-op.
//
// Callers that need to publish the per-package aggregate after the mutation
// should read it via PackageCacheStats.
func (db *Database) RemoveLibrary(libraryID string) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}

	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()

	previous, known := db.metadataByLibrary[libraryID]
	if !known || !previous.IsCached {
		// Skip the bbolt write: there is no observable state to flip and
		// writing here would materialize a spurious metadata entry for an
		// unknown libraryID.
		return nil
	}

	if err := db.bbolt.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(LibraryMetadataBucket))
		if bkt == nil {
			return fmt.Errorf("library metadata bucket does not exist")
		}
		return bkt.Delete([]byte(libraryID))
	}); err != nil {
		return err
	}

	db.updateLibraryEvicted(libraryID)
	return nil
}

// PackageCacheStats returns the current per-package cached library count and
// byte total. Reads from the in-memory aggregate and returns zero values for
// unknown packages.
func (db *Database) PackageCacheStats(pkg string) (count int, bytes int64) {
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	return db.cachedCountByPackage[pkg], db.cachedBytesByPackage[pkg]
}

// VolumeLinkCount returns the current number of volumes linked to any
// library belonging to the given package. Reads from the in-memory aggregate
// and returns 0 for unknown packages.
func (db *Database) VolumeLinkCount(pkg string) int {
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	return db.volumeLinksByPackage[pkg]
}

// Snapshot returns a deep copy of the per-package aggregates captured under a
// single lock so the three maps reflect a consistent point-in-time view.
// Used by observers that want to resync without going through bbolt
// themselves (e.g. metrics listeners on startup).
func (db *Database) Snapshot() Snapshot {
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	s := Snapshot{
		VolumeLinksByPackage: make(map[string]int, len(db.volumeLinksByPackage)),
		CachedCountByPackage: make(map[string]int, len(db.cachedCountByPackage)),
		CachedBytesByPackage: make(map[string]int64, len(db.cachedBytesByPackage)),
	}
	for k, v := range db.volumeLinksByPackage {
		s.VolumeLinksByPackage[k] = v
	}
	for k, v := range db.cachedCountByPackage {
		s.CachedCountByPackage[k] = v
	}
	for k, v := range db.cachedBytesByPackage {
		s.CachedBytesByPackage[k] = v
	}
	return s
}

// updateVolumeLinked mirrors a new (library, volume) link in the in-memory
// caches. The caller must hold cacheMu. The per-package aggregate is keyed
// on whatever Package was already registered for this library (empty if
// AddLibrary has not run yet, e.g. upgrade from a release that had
// no metadata bucket).
func (db *Database) updateVolumeLinked(libraryID string) {
	meta := db.metadataByLibrary[libraryID]
	meta.VolumeCount++
	db.metadataByLibrary[libraryID] = meta
	db.volumeLinksByPackage[meta.Package]++
}

// updateVolumeUnlinked mirrors a removed (library, volume) link in the
// in-memory caches. Both counters are defensively clamped at zero so any
// drift between bbolt and the cache cannot surface as a negative gauge.
// The caller must hold cacheMu.
func (db *Database) updateVolumeUnlinked(libraryID string) {
	meta := db.metadataByLibrary[libraryID]
	if meta.VolumeCount > 0 {
		meta.VolumeCount--
		db.metadataByLibrary[libraryID] = meta
	}
	if db.volumeLinksByPackage[meta.Package] > 0 {
		db.volumeLinksByPackage[meta.Package]--
	}
}

// updateLibraryCached mirrors a successful library cache (download) in the
// in-memory caches and adjusts the per-package aggregates. A re-cache of an
// already-cached library only adjusts the byte delta, leaving the count
// alone. The caller must hold cacheMu.
func (db *Database) updateLibraryCached(libraryID, packageName string, sizeBytes int64) {
	previous := db.metadataByLibrary[libraryID]
	next := previous
	next.Package = packageName
	next.SizeBytes = sizeBytes
	next.IsCached = true
	db.metadataByLibrary[libraryID] = next

	switch {
	case !previous.IsCached:
		db.cachedCountByPackage[next.Package]++
		db.cachedBytesByPackage[next.Package] += next.SizeBytes
	case previous.IsCached && previous.Package == next.Package:
		db.cachedBytesByPackage[next.Package] += next.SizeBytes - previous.SizeBytes
	}
}

// updateLibraryEvicted mirrors a library eviction in the in-memory caches:
// the metadata entry is dropped and the per-package aggregates are
// decremented (clamped at zero). The caller must hold cacheMu and have
// verified that the library was actually cached.
func (db *Database) updateLibraryEvicted(libraryID string) {
	previous := db.metadataByLibrary[libraryID]
	delete(db.metadataByLibrary, libraryID)
	if db.cachedCountByPackage[previous.Package] > 0 {
		db.cachedCountByPackage[previous.Package]--
	}
	db.cachedBytesByPackage[previous.Package] -= previous.SizeBytes
	if db.cachedBytesByPackage[previous.Package] < 0 {
		db.cachedBytesByPackage[previous.Package] = 0
	}
}

// writeLibraryMetadata serializes and stores the metadata in the given bucket.
func writeLibraryMetadata(bkt *bbolt.Bucket, libraryID []byte, meta libraryMetadata) error {
	encoded, err := json.Marshal(&meta)
	if err != nil {
		return fmt.Errorf("could not marshal library metadata: %w", err)
	}
	if err := bkt.Put(libraryID, encoded); err != nil {
		return fmt.Errorf("could not write library metadata: %w", err)
	}
	return nil
}
