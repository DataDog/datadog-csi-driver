// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// StoreDirectory is the subdirectory where active libraries are stored.
	StoreDirectory = "store"
	// DatabaseDirectory is the subdirectory where the databse file will be stored.
	DatabaseDirectory = "db"
	// ScratchDirectory is the subdirectory used for scratch download space for libraries.
	ScratchDirectory = "scratch"
	// DefaultImageCacheTTL is the max amount of time before we fetch a new image digest.
	DefaultImageCacheTTL = 1 * time.Hour
)

// LibraryManager is a high level object to manage fetching libraries for volumes. It will download, extract, store, and
// track libraries and how they map to a volume.
type LibraryManager struct {
	// downloader is used to download container images.
	downloader *Downloader
	// cache is used to cache container image digests.
	cache *ImageCache
	// store is used to store libraries on disk.
	store *Store
	// db is used to track library and volume mappings.
	db *Database
	// locker is used to synchronize access to the library manager.
	locker *Locker
	// scratchDir is the directory used for scratch download space for libraries.
	scratchDir string
}

// NewLibraryManager creates a new library manager with all of the required dependencies. The base path will be used
// by the manager to setup scratch space, library storage, and the database file.
func NewLibraryManager(basePath string) (*LibraryManager, error) {
	return NewLibraryManagerWithDownloader(basePath, NewDownloader())
}

// NewLibraryManagerWithDownloader is exposed primarily for testing purposes.
func NewLibraryManagerWithDownloader(basePath string, d *Downloader) (*LibraryManager, error) {
	// Setup scratch directory.
	scratchDir := filepath.Join(basePath, ScratchDirectory)
	err := os.MkdirAll(scratchDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("could not create base directory %s: %w", scratchDir, err)
	}

	// Setup store.
	storeDir := filepath.Join(basePath, StoreDirectory)
	err = os.MkdirAll(storeDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("could not create store directory %s: %w", storeDir, err)
	}
	store, err := NewStore(storeDir)
	if err != nil {
		return nil, fmt.Errorf("could not create store: %w", err)
	}

	// Setup database.
	dbDir := filepath.Join(basePath, DatabaseDirectory)
	err = os.MkdirAll(dbDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("could not create db directory %s: %w", dbDir, err)
	}
	db, err := NewDatabase(dbDir)
	if err != nil {
		return nil, fmt.Errorf("could not create database: %w", err)
	}

	// Return library manager.
	return &LibraryManager{
		downloader: d,
		cache:      NewImageCache(d, DefaultImageCacheTTL),
		store:      store,
		db:         db,
		scratchDir: scratchDir,
		locker:     NewLocker(),
	}, nil
}

// Stop ensures all dependencies are stopped correctly.
func (lm *LibraryManager) Stop() error {
	return lm.db.Close()
}

// GetLibraryForVolume fetches the remote library if it doesn't exist, records its usage, and returns the path on disk
// that can be mounted for the volume.
func (lm *LibraryManager) GetLibraryForVolume(ctx context.Context, volumeID string, lib *Library) (string, error) {
	// Validate the input.
	if volumeID == "" {
		return "", fmt.Errorf("volume ID cannot be empty")
	}
	if lib == nil {
		return "", fmt.Errorf("library cannot be nil")
	}

	// Fetch the library ID.
	libraryID, err := lm.cache.FetchDigest(ctx, lib.Image(), lib.Pull())
	if err != nil {
		return "", fmt.Errorf("could not determine library ID: %w", err)
	}

	// Lock the package.
	lm.locker.Lock(libraryID)
	defer lm.locker.Unlock(libraryID)

	// Link the library as a first step to ensure any cleanup processes know we need this library.
	err = lm.db.LinkVolume(libraryID, volumeID)
	if err != nil {
		return "", err
	}

	// If the library already exists, return it.
	path, err := lm.store.Get(libraryID)
	if err != nil && !errors.Is(err, ErrItemNotFound) {
		return "", err
	}
	if path != "" {
		return path, nil
	}

	// Otherwise, create a scratch space.
	scratch, err := os.MkdirTemp(lm.scratchDir, "datadog-csi-driver-*")
	if err != nil {
		return "", fmt.Errorf("could not create scratch directory: %w", err)
	}
	defer os.RemoveAll(scratch)

	// Download the library into the scratch space.
	err = lm.downloader.Download(ctx, lib.Image(), lib.Source(), scratch)
	if err != nil {
		return "", err
	}

	// Copy the library into the store.
	return lm.store.Add(libraryID, scratch)
}

// RemoveVolume removes the link in the database for the volume. If there are no more uses of the library, it is also
// removed from disk.
func (lm *LibraryManager) RemoveVolume(ctx context.Context, volumeID string) error {
	// Look up the linked library.
	libraryID, err := lm.db.GetLibraryForVolume(volumeID)
	if err != nil {
		return fmt.Errorf("could not determine which library was linked for volume %s: %w", volumeID, err)
	}

	// Lock the package.
	lm.locker.Lock(libraryID)
	defer lm.locker.Unlock(libraryID)

	// Unlink the volume from the database.
	err = lm.db.UnlinkVolume(libraryID, volumeID)
	if err != nil {
		return fmt.Errorf("could not unlink volume ID %s: %w", volumeID, err)
	}

	// If there are no other linked libraries, remove it from disk.
	count, err := lm.db.GetVolumeCount(libraryID)
	if err != nil {
		return fmt.Errorf("could not get linked library count")
	}
	if count == 0 {
		return lm.store.Remove(libraryID)
	}

	return nil
}
