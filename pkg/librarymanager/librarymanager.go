// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"context"
	"errors"
	"fmt"
	log "log/slog"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
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
	// fs is the filesystem abstraction used for all file operations.
	fs afero.Afero
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
	// cleanupStrategy defines how unused libraries are cleaned up.
	cleanupStrategy CleanupStrategy
	// scratchDir is the directory used for scratch download space for libraries.
	scratchDir string
	// listener is notified of every significant lifecycle event. It defaults
	// to a no-op so the manager can always invoke it unconditionally; the
	// production wiring (cmd/driver) passes a metrics-publishing listener
	// from pkg/metrics. The listener is responsible for any observability
	// concern (metrics, audit, etc.); the manager itself stays free of any
	// dependency on a concrete backend.
	listener EventListener
}

// LibraryManagerOption is a functional option for configuring a LibraryManager.
type LibraryManagerOption func(*LibraryManager)

// WithFilesystem sets the filesystem to use. Useful for testing.
func WithFilesystem(fs afero.Afero) LibraryManagerOption {
	return func(lm *LibraryManager) {
		lm.fs = fs
	}
}

// WithDownloader sets the downloader to use. Useful for testing.
func WithDownloader(d *Downloader) LibraryManagerOption {
	return func(lm *LibraryManager) {
		lm.downloader = d
	}
}

// WithCleanupStrategy sets the cleanup strategy to use.
// If not set, ImmediateCleanupStrategy is used by default.
func WithCleanupStrategy(s CleanupStrategy) LibraryManagerOption {
	return func(lm *LibraryManager) {
		lm.cleanupStrategy = s
	}
}

// WithEventListener injects an EventListener. Without this option the
// manager uses a no-op listener; the production wiring should always pass
// an implementation that publishes metrics (or any other observability
// signal).
func WithEventListener(l EventListener) LibraryManagerOption {
	return func(lm *LibraryManager) {
		if l != nil {
			lm.listener = l
		}
	}
}

// NewLibraryManager creates a new library manager with all of the required dependencies.
// The basePath is required as an absolute path (rather than using afero.NewBasePathFs) because
// bind mounts need absolute paths.
func NewLibraryManager(basePath string, opts ...LibraryManagerOption) (*LibraryManager, error) {
	// Create manager with defaults.
	lm := &LibraryManager{
		fs:              afero.Afero{Fs: afero.NewOsFs()},
		downloader:      NewDownloader(),
		locker:          NewLocker(),
		cleanupStrategy: NewImmediateCleanupStrategy(),
		listener:        noopEventListener{},
	}

	// Apply options.
	for _, opt := range opts {
		opt(lm)
	}

	// Setup scratch directory.
	lm.scratchDir = filepath.Join(basePath, ScratchDirectory)
	err := lm.fs.MkdirAll(lm.scratchDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("could not create scratch directory %s: %w", lm.scratchDir, err)
	}

	// Setup store.
	storeDir := filepath.Join(basePath, StoreDirectory)
	err = lm.fs.MkdirAll(storeDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("could not create store directory %s: %w", storeDir, err)
	}
	lm.store, err = NewStore(lm.fs, storeDir)
	if err != nil {
		return nil, fmt.Errorf("could not create store: %w", err)
	}

	// Setup database.
	dbDir := filepath.Join(basePath, DatabaseDirectory)
	err = lm.fs.MkdirAll(dbDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("could not create db directory %s: %w", dbDir, err)
	}
	lm.db, err = NewDatabase(dbDir)
	if err != nil {
		return nil, fmt.Errorf("could not create database: %w", err)
	}

	// Setup cache.
	lm.cache = NewImageCache(lm.downloader, DefaultImageCacheTTL)

	// Notify the listener of the persisted state so stateful gauges reflect
	// reality immediately after a restart, without waiting for the first
	// lifecycle event to flow through.
	lm.listener.OnSnapshot(lm.db.Snapshot())

	return lm, nil
}

// Stop ensures all dependencies are stopped correctly.
func (lm *LibraryManager) Stop() error {
	lm.cleanupStrategy.Stop()
	return lm.db.Close()
}

// HasVolume returns true if the volume is managed by the library manager.
func (lm *LibraryManager) HasVolume(volumeID string) (bool, error) {
	libraryID, err := lm.db.GetLibraryForVolume(volumeID)
	if err != nil {
		return false, err
	}
	return libraryID != "", nil
}

// GetLibraryForVolume fetches the remote library if it doesn't exist, records its usage, and returns the path on disk
// that can be mounted for the volume.
func (lm *LibraryManager) GetLibraryForVolume(ctx context.Context, volumeID string, lib *Library) (string, error) {
	// Track the resolution outcome for the listener. Default to "failed" so
	// any early return is reported as a failure unless explicitly overridden.
	result := LibraryResolutionFailed
	defer func() {
		lm.listener.OnLibraryResolved(result)
	}()

	// Validate the input.
	if volumeID == "" {
		return "", fmt.Errorf("volume ID cannot be empty")
	}
	if lib == nil {
		return "", fmt.Errorf("library cannot be nil")
	}

	// Fetch the library ID based on the image digest.
	libraryID, err := lm.cache.FetchDigest(ctx, lib.Image(), lib.Pull())
	if err != nil {
		return "", fmt.Errorf("could not determine library ID: %w", err)
	}

	// Lock the package. Held across the entire resolution so that any cleanup
	// scheduled for this libraryID by a concurrent RemoveVolume is serialized
	// with the work below; this is what protects an in-flight download from
	// being evicted out from under us.
	lm.locker.Lock(libraryID)
	defer lm.locker.Unlock(libraryID)

	// If the library already exists on disk, link the volume and return it.
	path, err := lm.store.Get(libraryID)
	if err != nil && !errors.Is(err, ErrItemNotFound) {
		return "", err
	}
	if path != "" {
		log.Info("Library already cached", "image", lib.Image(), "path", path)
		if err := lm.linkVolume(libraryID, volumeID); err != nil {
			return "", err
		}
		result = LibraryResolutionCacheHit
		return path, nil
	}

	// Otherwise, create a scratch space.
	scratch, err := afero.TempDir(lm.fs, lm.scratchDir, "datadog-csi-driver-*")
	if err != nil {
		return "", fmt.Errorf("could not create scratch directory: %w", err)
	}
	defer func() { _ = lm.fs.RemoveAll(scratch) }()

	// Download the library into the scratch space.
	log.Info("Downloading library", "image", lib.Image())
	downloadStart := time.Now()
	size, err := lm.downloader.Download(ctx, lm.fs, lib.Image(), scratch)
	if err != nil {
		return "", err
	}
	lm.listener.OnLibraryDownload(lib.Name(), lib.Registry(), time.Since(downloadStart))

	// Copy the library into the store.
	storePath, err := lm.store.Add(libraryID, scratch)
	if err != nil {
		return "", err
	}
	log.Info("Library downloaded and stored", "image", lib.Image(), "path", storePath)

	// Register the library metadata (package name, size, cached flag) before
	// linking any volume so the per-package aggregates are correct from the
	// very first link.
	lm.recordLibraryCached(libraryID, lib.Name(), size)

	// Link the volume to the library only after the payload is actually on
	// disk so a failed download never leaves a dangling link behind.
	if err := lm.linkVolume(libraryID, volumeID); err != nil {
		return "", err
	}
	result = LibraryResolutionDownloaded
	return storePath, nil
}

// linkVolume persists the volume→library link and publishes the per-package
// volume_links gauge via the listener. The package label is read from the
// database, which is populated by recordLibraryCached at download time.
func (lm *LibraryManager) linkVolume(libraryID, volumeID string) error {
	if err := lm.db.LinkVolume(libraryID, volumeID); err != nil {
		return err
	}
	pkg, _ := lm.db.GetPackageForLibrary(libraryID)
	lm.listener.OnVolumeLinked(pkg, lm.db.VolumeLinkCount(pkg))
	return nil
}

// recordLibraryCached persists the metadata of a newly downloaded library
// (package name, size, cached flag) and publishes the libraries_cached*
// gauges via the listener. Errors are logged but never propagated: a stale
// gauge does not justify failing the mount flow.
func (lm *LibraryManager) recordLibraryCached(libraryID, pkg string, size int64) {
	if err := lm.db.AddLibrary(libraryID, pkg, size); err != nil {
		log.Warn("Could not persist library cache metadata, libraries_cached* gauges may be stale across restarts", "library_id", libraryID, "error", err)
		return
	}
	count, bytes := lm.db.PackageCacheStats(pkg)
	lm.listener.OnLibraryCached(pkg, count, bytes)
}

// RemoveVolume removes the link between the LibraryID and the VolumeID in the database.
// If there are no more uses of the library, it is also removed from disk.
func (lm *LibraryManager) RemoveVolume(ctx context.Context, volumeID string) error {
	// Look up the linked library and its package up front: once UnlinkVolume
	// drops the last reference, future cleanup runs may wipe the metadata
	// entry, so we cache the package here while it is still resolvable.
	libraryID, err := lm.db.GetLibraryForVolume(volumeID)
	if err != nil {
		return fmt.Errorf("could not determine which library was linked for volume %s: %w", volumeID, err)
	}
	if libraryID == "" {
		// No link was ever recorded for this volume (typically because
		// NodePublishVolume failed before the volume could be linked, then
		// kubelet still calls NodeUnpublishVolume on pod deletion). Nothing
		// to unlink, nothing to clean up.
		return nil
	}
	pkg, err := lm.db.GetPackageForLibrary(libraryID)
	if err != nil {
		return fmt.Errorf("could not determine package for library %s: %w", libraryID, err)
	}

	// Unlink the volume from the database.
	// Note: No lock needed here because:
	// - UnlinkVolume is atomic (database has its own locking)
	// - tryCleanupLibrary acquires the lock before checking and removing
	if err := lm.db.UnlinkVolume(libraryID, volumeID); err != nil {
		return fmt.Errorf("could not unlink volume ID %s: %w", volumeID, err)
	}
	lm.listener.OnVolumeUnlinked(pkg, lm.db.VolumeLinkCount(pkg))

	// Schedule cleanup - tryCleanupLibrary will check if the library is still in use
	lm.cleanupStrategy.ScheduleCleanup(libraryID, lm.tryCleanupLibrary)

	return nil
}

// tryCleanupLibrary attempts to remove a library from disk if it's no longer in use.
// It acquires the lock and checks the volume count before removing.
func (lm *LibraryManager) tryCleanupLibrary(libraryID string) error {
	strategy := lm.cleanupStrategy.Name()

	// Acquire lock to prevent race with GetLibraryForVolume
	lm.locker.Lock(libraryID)
	defer lm.locker.Unlock(libraryID)

	// Check if the library is still in use
	count, err := lm.db.GetVolumeCount(libraryID)
	if err != nil {
		lm.listener.OnLibraryCleanup(LibraryCleanupFailed, strategy)
		return fmt.Errorf("could not get linked library count: %w", err)
	}
	if count > 0 {
		log.Info("Library still in use, skipping cleanup", "library_id", libraryID, "count", count)
		lm.listener.OnLibraryCleanup(LibraryCleanupSkippedInUse, strategy)
		return nil
	}
	log.Info("Removing library from disk", "library_id", libraryID)
	if err := lm.store.Remove(libraryID); err != nil {
		lm.listener.OnLibraryCleanup(LibraryCleanupFailed, strategy)
		return err
	}
	// Drop the cache state. The package is captured up front so we can
	// still publish the per-package aggregate after the metadata entry is
	// gone.
	pkg, _ := lm.db.GetPackageForLibrary(libraryID)
	if err := lm.db.RemoveLibrary(libraryID); err != nil {
		log.Warn("Could not remove library metadata, libraries_cached* gauges may be stale", "library_id", libraryID, "error", err)
	} else {
		count, bytes := lm.db.PackageCacheStats(pkg)
		lm.listener.OnLibraryEvicted(pkg, count, bytes)
	}
	lm.listener.OnLibraryCleanup(LibraryCleanupSuccess, strategy)
	return nil
}
