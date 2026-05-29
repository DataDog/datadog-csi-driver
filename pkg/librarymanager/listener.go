// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import "time"

// LibraryResolutionResult enumerates the outcomes of GetLibraryForVolume.
// The underlying string values match the labels used by the Datadog CSI
// driver metrics so listeners that publish Prometheus series can pass them
// through without remapping.
type LibraryResolutionResult string

const (
	LibraryResolutionCacheHit   LibraryResolutionResult = "cache_hit"
	LibraryResolutionDownloaded LibraryResolutionResult = "downloaded"
	LibraryResolutionFailed     LibraryResolutionResult = "failed"
)

// LibraryCleanupStatus enumerates the outcomes of a cleanup attempt for a
// library that is no longer in use. Underlying string values match the
// driver metric labels for the same reason as LibraryResolutionResult.
type LibraryCleanupStatus string

const (
	LibraryCleanupSuccess      LibraryCleanupStatus = "success"
	LibraryCleanupFailed       LibraryCleanupStatus = "failed"
	LibraryCleanupSkippedInUse LibraryCleanupStatus = "skipped_in_use"
)

// Snapshot is the full per-package state captured atomically from the
// database. Used by EventListener.OnSnapshot to seed stateful gauges at
// startup. The maps are owned by the caller and must not be retained.
type Snapshot struct {
	// VolumeLinksByPackage is the number of volumes currently linked to
	// each package.
	VolumeLinksByPackage map[string]int
	// CachedCountByPackage is the number of library versions currently
	// cached on disk for each package.
	CachedCountByPackage map[string]int
	// CachedBytesByPackage is the total on-disk size in bytes of the
	// cached library versions for each package.
	CachedBytesByPackage map[string]int64
}

// EventListener is notified by the LibraryManager of significant lifecycle
// events. The interface is deliberately observability-agnostic: the default
// implementation is a no-op (see noopEventListener) and the production code
// wires the metrics-aware listener from pkg/metrics. Other consumers (audit
// logs, custom telemetry, tests) can implement it without dragging in any
// concrete observability backend.
//
// All methods are expected to be quick and non-blocking: they run on the
// LibraryManager's caller goroutine, sometimes under a per-library lock.
// Implementations that need to do non-trivial work should fan out internally.
//
// The "Package*Count" / "Package*Bytes" arguments are the post-mutation
// per-package aggregates, already computed by the manager's database layer.
// Listeners that publish them can do so directly without re-aggregating.
type EventListener interface {
	// OnLibraryResolved is called once GetLibraryForVolume finishes. result
	// reflects whether the library was served from the cache, freshly
	// downloaded, or the resolution failed.
	OnLibraryResolved(result LibraryResolutionResult)

	// OnLibraryDownload is called when a library has been fetched from a
	// registry, with the time spent on the download. Not called on cache
	// hits or failures before the download step.
	OnLibraryDownload(library, registry string, duration time.Duration)

	// OnLibraryCleanup is called for every cleanup attempt, including ones
	// that were skipped because the library is still in use.
	OnLibraryCleanup(status LibraryCleanupStatus, strategy string)

	// OnVolumeLinked is called after every successful LinkVolume call
	// (including idempotent re-links). packageVolumeLinkCount is the
	// current total for the package, computed by the database.
	OnVolumeLinked(pkg string, packageVolumeLinkCount int)

	// OnVolumeUnlinked is called after every successful UnlinkVolume call
	// (including no-op unlinks of a non-existent link). The semantics of
	// packageVolumeLinkCount match OnVolumeLinked.
	OnVolumeUnlinked(pkg string, packageVolumeLinkCount int)

	// OnLibraryCached is called when a library payload has been added to
	// the on-disk store.
	OnLibraryCached(pkg string, packageCachedCount int, packageCachedBytes int64)

	// OnLibraryEvicted is called when a library payload has been removed
	// from the on-disk store.
	OnLibraryEvicted(pkg string, packageCachedCount int, packageCachedBytes int64)

	// OnSnapshot is called once at LibraryManager startup with the full
	// per-package aggregates rebuilt from the persisted state. Listeners
	// publishing stateful gauges should overwrite the gauge values from
	// the snapshot so monitors and dashboards reflect reality immediately
	// after a restart, without waiting for the first lifecycle event to
	// flow through.
	OnSnapshot(s Snapshot)
}

// noopEventListener is the default listener used when no observer is
// configured. It discards every event so the LibraryManager can invoke the
// listener unconditionally and keep the call sites clean.
type noopEventListener struct{}

func (noopEventListener) OnLibraryResolved(LibraryResolutionResult)       {}
func (noopEventListener) OnLibraryDownload(string, string, time.Duration) {}
func (noopEventListener) OnLibraryCleanup(LibraryCleanupStatus, string)   {}
func (noopEventListener) OnVolumeLinked(string, int)                      {}
func (noopEventListener) OnVolumeUnlinked(string, int)                    {}
func (noopEventListener) OnLibraryCached(string, int, int64)              {}
func (noopEventListener) OnLibraryEvicted(string, int, int64)             {}
func (noopEventListener) OnSnapshot(Snapshot)                             {}
