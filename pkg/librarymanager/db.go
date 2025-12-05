// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"encoding/json"
	"fmt"
	"path/filepath"

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
)

type linkedVolume struct{}

type linkedLibrary struct{}

// Database is a wrapper around bbolt with business logic for the library manager.
type Database struct {
	bbolt *bbolt.DB
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
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not create initial buckets: %w", err)
	}

	return &Database{
		bbolt: db,
	}, nil
}

// Close will clean up the database and should be called before exiting.
func (db *Database) Close() error {
	return db.bbolt.Close()
}

// LinkVolume creates a bidrectional mapping between the library and volume.
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
	defer tx.Rollback()

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

	// Commit the transaction.
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

	return nil
}

// UnlinkVolume removes the link for a given volume.
func (db *Database) UnlinkVolume(libraryID string, volumeID string) error {
	// Validate input.
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be blank")
	}
	if volumeID == "" {
		return fmt.Errorf(" volume ID cannot be blank")
	}

	// Start a transaction.
	tx, err := db.bbolt.Begin(true)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

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

	// Commit the transaction.
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

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
	defer tx.Rollback()

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
	bkt.ForEach(func(k, v []byte) error {
		count++
		return nil
	})

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
	defer tx.Rollback()

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
