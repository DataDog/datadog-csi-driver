// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
	"go.etcd.io/bbolt"
)

const (
	// DatabaseFileName is the name of the database file created by bbolt.
	DatabaseFileName = "datadog-csi-driver.db"

	// VolumesBucket maps a volume to the library it uses. Key = volume ID,
	// value = JSON volumeRecord. The relationship is 1:1 (a volume mounts
	// exactly one library), so a flat bucket is all that is needed.
	VolumesBucket = "volumes"
	// LibrariesBucket holds one record per cached library. Key = library ID
	// (the image digest), value = JSON libraryRecord. It is the single
	// source of truth for the package label, the on-disk size and the number
	// of volumes currently using the library.
	LibrariesBucket = "libraries"

	// The following buckets belong to the legacy nested-bucket schema and are
	// only referenced by migrate() to upgrade existing databases in place.
	legacyLibraryMappingBucket  = "library-mappings"
	legacyVolumeMappingBucket   = "volume-mappings"
	legacyLibraryMetadataBucket = "library-metadata"
)

// volumeRecord is the value stored in VolumesBucket. It is a struct (rather
// than a bare library ID) so that fields can be added later without breaking
// existing databases.
type volumeRecord struct {
	// LibraryID is the ID of the library the volume is mounted from.
	LibraryID string `json:"library_id"`
	// CreatedAt is when the link was recorded. Records migrated from the
	// legacy schema have a zero value.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// FromCache reports whether the publish that created this link reused an
	// already-cached library (true) or had to download it first (false).
	FromCache bool `json:"from_cache,omitempty"`
}

// libraryRecord is the value stored in LibrariesBucket.
type libraryRecord struct {
	// Package is the canonical package name (e.g. "dd-lib-java-init") shared
	// by every version of the library and used as the metric label. It may
	// be empty for libraries migrated from a database that predates the
	// per-library metadata.
	Package string `json:"package,omitempty"`
	// SizeBytes is the on-disk size of the library, in bytes.
	SizeBytes int64 `json:"size_bytes,omitempty"`
	// VolumeCount is the number of volumes currently linked to this library.
	// It replaces the per-library volume sub-bucket of the legacy schema.
	VolumeCount int `json:"volume_count,omitempty"`
}

// LibraryInfo is the public, read-only view of a library record returned by
// GetLibrary.
type LibraryInfo struct {
	// Package is the canonical package name used as the metric label. It is
	// empty for legacy entries that predate per-library metadata.
	Package string
	// SizeBytes is the on-disk size of the library, in bytes.
	SizeBytes int64
	// VolumeCount is the number of volumes currently linked to the library.
	VolumeCount int
}

// Database is a thin wrapper around bbolt.
//
// # Transaction consistency
//
// bbolt provides serializable isolation: write transactions are mutually
// exclusive, reads see a consistent snapshot, and every transaction is
// atomic. As a result each method here is a single self-contained
// transaction and the database keeps no in-memory bookkeeping: the
// per-package aggregates the metrics listener needs are derived on demand by
// scanning LibrariesBucket (see Snapshot), which is cheap because a node only
// ever caches a handful of libraries.
//
// # External locking
//
// Operations that combine a database write with a filesystem operation (e.g.
// LinkVolume followed by store.Add) still require external synchronisation;
// the LibraryManager uses a per-library Locker for that.
type Database struct {
	bbolt *bbolt.DB
}

// NewDatabase initializes a new database. If a database file exists it is
// reused (and migrated from the legacy schema if necessary). Call Close when
// you are done.
func NewDatabase(basePath string) (*Database, error) {
	path := filepath.Join(basePath, DatabaseFileName)
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open database at %s: %w", path, err)
	}

	if err := db.Update(migrate); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("could not initialize database: %w", err)
	}

	return &Database{bbolt: db}, nil
}

// migrate ensures the current buckets exist and, on the first run after an
// upgrade, rewrites the legacy nested-bucket schema into the flat
// volumes/libraries schema. It is idempotent: once the legacy buckets are
// gone it only creates the current buckets if they are missing.
func migrate(tx *bbolt.Tx) error {
	volumesBkt, err := tx.CreateBucketIfNotExists([]byte(VolumesBucket))
	if err != nil {
		return fmt.Errorf("could not create bucket %s: %w", VolumesBucket, err)
	}
	librariesBkt, err := tx.CreateBucketIfNotExists([]byte(LibrariesBucket))
	if err != nil {
		return fmt.Errorf("could not create bucket %s: %w", LibrariesBucket, err)
	}

	// Seed library records from the legacy metadata bucket.
	if metaBkt := tx.Bucket([]byte(legacyLibraryMetadataBucket)); metaBkt != nil {
		if err := metaBkt.ForEach(func(k, v []byte) error {
			var meta struct {
				Package   string `json:"package,omitempty"`
				SizeBytes int64  `json:"size_bytes,omitempty"`
			}
			if err := json.Unmarshal(v, &meta); err != nil {
				return fmt.Errorf("could not unmarshal legacy library metadata: %w", err)
			}
			return putLibrary(librariesBkt, string(k), libraryRecord{
				Package:   meta.Package,
				SizeBytes: meta.SizeBytes,
			})
		}); err != nil {
			return err
		}
	}

	// Rebuild volume records and volume counts from the legacy mapping bucket.
	if mappingBkt := tx.Bucket([]byte(legacyLibraryMappingBucket)); mappingBkt != nil {
		if err := mappingBkt.ForEachBucket(func(libraryKey []byte) error {
			libraryID := string(libraryKey)
			libBkt := mappingBkt.Bucket(libraryKey)
			if libBkt == nil {
				return nil
			}
			return libBkt.ForEach(func(volumeKey, _ []byte) error {
				// The legacy LinkVolume only checked the per-library
				// sub-bucket, so the same volume could be mapped under
				// several libraries. Migrate each volume exactly once
				// (first writer wins) so the flat record and the owning
				// library's VolumeCount stay consistent. Counting every
				// occurrence would leave the overwritten libraries with a
				// positive count for a volume they no longer own, so they
				// would never be cleaned up and their gauges would stay
				// inflated.
				if _, migrated, err := getVolume(volumesBkt, string(volumeKey)); err != nil {
					return err
				} else if migrated {
					return nil
				}
				if err := putVolume(volumesBkt, string(volumeKey), volumeRecord{LibraryID: libraryID}); err != nil {
					return err
				}
				rec, err := getLibrary(librariesBkt, libraryID)
				if err != nil {
					return err
				}
				rec.VolumeCount++
				return putLibrary(librariesBkt, libraryID, rec)
			})
		}); err != nil {
			return err
		}
	}

	// Drop the legacy buckets now that their content has been migrated.
	for _, name := range []string{legacyLibraryMappingBucket, legacyVolumeMappingBucket, legacyLibraryMetadataBucket} {
		if tx.Bucket([]byte(name)) != nil {
			if err := tx.DeleteBucket([]byte(name)); err != nil {
				return fmt.Errorf("could not delete legacy bucket %s: %w", name, err)
			}
		}
	}

	return nil
}

// Close cleans up the database and should be called before exiting.
func (db *Database) Close() error {
	return db.bbolt.Close()
}

// AddLibrary records a freshly-cached library by persisting its package name
// and on-disk size. It is idempotent and preserves the volume count of an
// existing record, so it can safely be called again (for instance when the
// size changed) without disturbing the link bookkeeping.
func (db *Database) AddLibrary(libraryID, packageName string, sizeBytes int64) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}
	if packageName == "" {
		return fmt.Errorf("package name cannot be blank")
	}

	return db.bbolt.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(LibrariesBucket))
		if bkt == nil {
			return fmt.Errorf("libraries bucket does not exist")
		}
		rec, err := getLibrary(bkt, libraryID)
		if err != nil {
			return err
		}
		rec.Package = packageName
		rec.SizeBytes = sizeBytes
		return putLibrary(bkt, libraryID, rec)
	})
}

// RemoveLibrary deletes the record for a library. It is a no-op when the
// library is unknown.
func (db *Database) RemoveLibrary(libraryID string) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}

	return db.bbolt.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(LibrariesBucket))
		if bkt == nil {
			return fmt.Errorf("libraries bucket does not exist")
		}
		return bkt.Delete([]byte(libraryID))
	})
}

// LinkVolume records that volumeID uses libraryID and increments the
// library's volume count. fromCache notes whether the publish reused an
// already-cached library or had to download it; it is persisted on the record
// together with a creation timestamp.
//
// A volume maps to exactly one library for its whole lifetime: callers resolve
// an already-linked volume from its existing record instead of re-resolving the
// image, so LinkVolume is only reached for volumes that are not yet linked.
// Linking a volume that is already tracked is therefore treated as an
// idempotent no-op rather than re-pointing it, which keeps the per-library
// counts from drifting even if the function is called twice.
func (db *Database) LinkVolume(libraryID, volumeID string, fromCache bool) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}
	if volumeID == "" {
		return fmt.Errorf("volume ID cannot be blank")
	}

	return db.bbolt.Update(func(tx *bbolt.Tx) error {
		volumesBkt := tx.Bucket([]byte(VolumesBucket))
		librariesBkt := tx.Bucket([]byte(LibrariesBucket))
		if volumesBkt == nil || librariesBkt == nil {
			return fmt.Errorf("database buckets do not exist")
		}

		// A volume is only ever linked once (re-publishes reuse the existing
		// record upstream). If a record already exists, stay idempotent and
		// do not touch the counts.
		if _, linked, err := getVolume(volumesBkt, volumeID); err != nil {
			return err
		} else if linked {
			return nil
		}

		if err := putVolume(volumesBkt, volumeID, volumeRecord{
			LibraryID: libraryID,
			CreatedAt: time.Now().UTC(),
			FromCache: fromCache,
		}); err != nil {
			return err
		}
		rec, err := getLibrary(librariesBkt, libraryID)
		if err != nil {
			return err
		}
		rec.VolumeCount++
		return putLibrary(librariesBkt, libraryID, rec)
	})
}

// UnlinkVolume removes the link for a volume and decrements the owning
// library's volume count. It returns the library ID and package name the
// volume was linked to (both empty when the volume was not tracked, in which
// case it is a no-op). The package is read off the library record that is
// loaded to decrement the count, so callers get the metric label for free
// without an extra lookup.
func (db *Database) UnlinkVolume(volumeID string) (libraryID, packageName string, err error) {
	if volumeID == "" {
		return "", "", fmt.Errorf("volume ID cannot be blank")
	}

	err = db.bbolt.Update(func(tx *bbolt.Tx) error {
		volumesBkt := tx.Bucket([]byte(VolumesBucket))
		librariesBkt := tx.Bucket([]byte(LibrariesBucket))
		if volumesBkt == nil || librariesBkt == nil {
			return fmt.Errorf("database buckets do not exist")
		}

		rec, linked, err := getVolume(volumesBkt, volumeID)
		if err != nil {
			return err
		}
		if !linked {
			return nil
		}
		libraryID = rec.LibraryID

		if err := volumesBkt.Delete([]byte(volumeID)); err != nil {
			return fmt.Errorf("could not delete volume record %s: %w", volumeID, err)
		}

		libRec, err := getLibrary(librariesBkt, libraryID)
		if err != nil {
			return err
		}
		packageName = libRec.Package
		if libRec.VolumeCount > 0 {
			libRec.VolumeCount--
			if err := putLibrary(librariesBkt, libraryID, libRec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return libraryID, packageName, nil
}

// GetLibraryForVolume returns the library ID a volume is linked to, or an
// empty string when the volume is not tracked.
func (db *Database) GetLibraryForVolume(volumeID string) (string, error) {
	if volumeID == "" {
		return "", fmt.Errorf("volume ID cannot be blank")
	}

	var libraryID string
	err := db.bbolt.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(VolumesBucket))
		if bkt == nil {
			return fmt.Errorf("volumes bucket does not exist")
		}
		rec, _, err := getVolume(bkt, volumeID)
		if err != nil {
			return err
		}
		libraryID = rec.LibraryID
		return nil
	})
	return libraryID, err
}

// GetLibrary returns the stored information for a library. The boolean is
// false when the library has no record (for instance a legacy entry on disk
// that was never tracked with metadata).
func (db *Database) GetLibrary(libraryID string) (LibraryInfo, bool, error) {
	if libraryID == "" {
		return LibraryInfo{}, false, fmt.Errorf("library ID cannot be blank")
	}

	var (
		info  LibraryInfo
		found bool
	)
	err := db.bbolt.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(LibrariesBucket))
		if bkt == nil {
			return fmt.Errorf("libraries bucket does not exist")
		}
		raw := bkt.Get([]byte(libraryID))
		if raw == nil {
			return nil
		}
		var rec libraryRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return fmt.Errorf("could not unmarshal library record: %w", err)
		}
		found = true
		info = LibraryInfo(rec)
		return nil
	})
	return info, found, err
}

// Snapshot derives the per-package aggregates the metrics listener needs by
// scanning LibrariesBucket. It is cheap because a node only ever caches a
// small number of libraries. Libraries without a package label (legacy
// entries) are left out because they were never published as gauges.
func (db *Database) Snapshot() (libraryevents.Snapshot, error) {
	snap := libraryevents.Snapshot{
		CachedCountByLibrary: map[string]int{},
		CachedBytesByLibrary: map[string]int64{},
		VolumeLinksByLibrary: map[string]int{},
	}
	err := db.bbolt.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(LibrariesBucket))
		if bkt == nil {
			return nil
		}
		return bkt.ForEach(func(_, v []byte) error {
			var rec libraryRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("could not unmarshal library record: %w", err)
			}
			if rec.Package == "" {
				return nil
			}
			snap.CachedCountByLibrary[rec.Package]++
			snap.CachedBytesByLibrary[rec.Package] += rec.SizeBytes
			snap.VolumeLinksByLibrary[rec.Package] += rec.VolumeCount
			return nil
		})
	})
	if err != nil {
		return libraryevents.Snapshot{}, err
	}
	return snap, nil
}

// getLibrary reads and decodes a library record. A missing key yields a zero
// record and no error so callers can treat it as an upsert base.
func getLibrary(bkt *bbolt.Bucket, libraryID string) (libraryRecord, error) {
	var rec libraryRecord
	raw := bkt.Get([]byte(libraryID))
	if raw == nil {
		return rec, nil
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return rec, fmt.Errorf("could not unmarshal library record: %w", err)
	}
	return rec, nil
}

// putLibrary encodes and writes a library record.
func putLibrary(bkt *bbolt.Bucket, libraryID string, rec libraryRecord) error {
	encoded, err := json.Marshal(&rec)
	if err != nil {
		return fmt.Errorf("could not marshal library record: %w", err)
	}
	if err := bkt.Put([]byte(libraryID), encoded); err != nil {
		return fmt.Errorf("could not write library record: %w", err)
	}
	return nil
}

// getVolume reads and decodes a volume record. The boolean reports whether
// the record exists.
func getVolume(bkt *bbolt.Bucket, volumeID string) (volumeRecord, bool, error) {
	var rec volumeRecord
	raw := bkt.Get([]byte(volumeID))
	if raw == nil {
		return rec, false, nil
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return rec, false, fmt.Errorf("could not unmarshal volume record: %w", err)
	}
	return rec, true, nil
}

// putVolume encodes and writes a volume record.
func putVolume(bkt *bbolt.Bucket, volumeID string, rec volumeRecord) error {
	encoded, err := json.Marshal(&rec)
	if err != nil {
		return fmt.Errorf("could not marshal volume record: %w", err)
	}
	if err := bkt.Put([]byte(volumeID), encoded); err != nil {
		return fmt.Errorf("could not write volume record: %w", err)
	}
	return nil
}
