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

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
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
	// listener is notified of every significant lifecycle event. Defaults to
	// a no-op so the manager can invoke it unconditionally; the production
	// wiring (cmd/driver) passes a metrics-publishing listener from
	// pkg/metrics. The listener handles any observability concern (metrics,
	// audit, etc.); the manager itself stays free of any dependency on a
	// concrete backend.
	listener libraryevents.Listener
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

// WithEventListener injects a Listener. Without this option the manager
// uses a no-op listener; the production wiring should always pass an
// implementation that publishes metrics (or any other observability
// signal).
func WithEventListener(l libraryevents.Listener) LibraryManagerOption {
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
		listener:        libraryevents.NoopListener{},
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

	// Seed listener gauges from the persisted state, so dashboards reflect
	// reality immediately after a driver restart instead of waiting for the
	// first event.
	if snap, err := lm.db.Snapshot(); err != nil {
		log.Error("could not seed library metrics from persisted state", "error", err)
	} else {
		lm.listener.OnSnapshot(snap)
	}

	return lm, nil
}

// packageStats reads the per-package aggregates used to label gauge events by
// indexing a fresh Snapshot. Metrics are best-effort: a read error is logged
// and reported as zeroed stats so a metrics hiccup never fails the
// mount/unmount that triggered it.
func (lm *LibraryManager) packageStats(library string) (cachedCount int, cachedBytes int64, volumeLinks int) {
	snap, err := lm.db.Snapshot()
	if err != nil {
		log.Error("could not read library stats for metrics", "library", library, "error", err)
		return 0, 0, 0
	}
	return snap.CachedCountByLibrary[library], snap.CachedBytesByLibrary[library], snap.VolumeLinksByLibrary[library]
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
	// Track the resolution outcome through a deferred listener call. The
	// default "failed" reflects any early return; success paths overwrite it
	// before returning.
	result := libraryevents.ResolutionFailed
	defer func() {
		library := ""
		if lib != nil {
			library = lib.Name()
		}
		lm.listener.OnLibraryResolved(library, result)
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

	// Lock the package. The locker prevents cleanup from running while
	// we resolve, so we can defer LinkVolume to after we have confirmed
	// the library is on disk and recorded in the metadata bucket.
	lm.locker.Lock(libraryID)
	defer lm.locker.Unlock(libraryID)

	// If the library already exists, return it.
	path, err := lm.store.Get(libraryID)
	if err != nil && !errors.Is(err, ErrItemNotFound) {
		return "", err
	}
	if path != "" {
		log.Info("Library already cached", "image", lib.Image(), "path", path)
		if err := lm.linkVolume(libraryID, volumeID, lib.Name()); err != nil {
			return "", err
		}
		result = libraryevents.ResolutionCacheHit
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
	sizeBytes, err := lm.downloader.Download(ctx, lm.fs, lib.Image(), scratch)
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

	// Record the library so its package name and on-disk size are persisted.
	// AddLibrary is the canonical writer for the per-library record; it must
	// run before LinkVolume so the library record exists when the volume
	// count is incremented.
	if err := lm.db.AddLibrary(libraryID, lib.Name(), sizeBytes); err != nil {
		return "", fmt.Errorf("could not record library metadata: %w", err)
	}
	count, totalBytes, _ := lm.packageStats(lib.Name())
	lm.listener.OnLibraryCached(lib.Name(), count, totalBytes)

	if err := lm.linkVolume(libraryID, volumeID, lib.Name()); err != nil {
		return "", err
	}

	result = libraryevents.ResolutionDownloaded
	return storePath, nil
}

// linkVolume persists the library/volume link and notifies the listener with
// the resulting per-library volume count. It is intentionally a tiny helper:
// keeping the listener invocation paired with the LinkVolume call avoids the
// easy mistake of forgetting one of the two.
func (lm *LibraryManager) linkVolume(libraryID, volumeID, library string) error {
	previousLibraryID, err := lm.db.LinkVolume(libraryID, volumeID)
	if err != nil {
		return err
	}
	// If the volume was previously linked to a different library, the transfer
	// may have dropped that library's volume count to zero. RemoveVolume is the
	// only other place that schedules cleanup, so do it here too; otherwise the
	// displaced library would linger on disk until the next restart.
	if previousLibraryID != "" {
		lm.cleanupStrategy.ScheduleCleanup(previousLibraryID, lm.tryCleanupLibrary)
	}
	_, _, volumeLinks := lm.packageStats(library)
	lm.listener.OnVolumeLinked(library, volumeLinks)
	return nil
}

// RemoveVolume removes the link between the LibraryID and the VolumeID in the database.
// If there are no more uses of the library, it is also removed from disk.
// Calling RemoveVolume for a volume that was never linked is a no-op.
func (lm *LibraryManager) RemoveVolume(_ context.Context, volumeID string) error {
	// Unlink the volume. UnlinkVolume returns both the library it was linked
	// to and that library's package name (read off the persisted record while
	// decrementing the count), so we get the gauge label without a separate
	// lookup before the link is wiped.
	// Note: No lock needed here because:
	// - UnlinkVolume is atomic (database has its own locking)
	// - tryCleanupLibrary acquires the lock before checking and removing
	libraryID, library, err := lm.db.UnlinkVolume(volumeID)
	if err != nil {
		return fmt.Errorf("could not unlink volume ID %s: %w", volumeID, err)
	}
	if libraryID == "" {
		// Nothing to do: the volume was never linked or has already been
		// removed. We return nil here to keep idempotency.
		return nil
	}
	if library != "" {
		_, _, volumeLinks := lm.packageStats(library)
		lm.listener.OnVolumeUnlinked(library, volumeLinks)
	}

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

	// Read the library record once: it carries both the package label every
	// cleanup event needs and the live volume count. Legacy entries that
	// predate per-library metadata resolve to an empty package; the gauges
	// were never published for them, but the cleanup counter still fires.
	info, _, err := lm.db.GetLibrary(libraryID)
	if err != nil {
		lm.listener.OnLibraryCleanup("", libraryevents.CleanupFailed, strategy)
		return fmt.Errorf("could not get library for ID %s: %w", libraryID, err)
	}

	// Check if the library is still in use
	if info.VolumeCount > 0 {
		log.Info("Library still in use, skipping cleanup", "library_id", libraryID, "count", info.VolumeCount)
		lm.listener.OnLibraryCleanup(info.Package, libraryevents.CleanupSkippedInUse, strategy)
		return nil
	}
	log.Info("Removing library from disk", "library_id", libraryID)

	if err := lm.store.Remove(libraryID); err != nil {
		lm.listener.OnLibraryCleanup(info.Package, libraryevents.CleanupFailed, strategy)
		return err
	}
	if err := lm.db.RemoveLibrary(libraryID); err != nil {
		lm.listener.OnLibraryCleanup(info.Package, libraryevents.CleanupFailed, strategy)
		return fmt.Errorf("could not remove library metadata for %s: %w", libraryID, err)
	}
	if info.Package != "" {
		newCount, newBytes, _ := lm.packageStats(info.Package)
		lm.listener.OnLibraryEvicted(info.Package, newCount, newBytes)
	}
	lm.listener.OnLibraryCleanup(info.Package, libraryevents.CleanupSuccess, strategy)
	return nil
}
