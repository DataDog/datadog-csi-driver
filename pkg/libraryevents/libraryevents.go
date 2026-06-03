// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

// Package libraryevents defines the shared vocabulary used by the library
// manager and its observers (metrics, audit logs, tests, ...). Keeping it
// in a small, dependency-free package lets the manager stay free of any
// concrete observability backend and lets backends consume the events
// without dragging in the rest of the manager's API.
package libraryevents

import "time"

// ResolutionResult enumerates the outcomes of resolving a library for a
// volume. The underlying string values match the labels used by the Datadog
// CSI driver metrics so listeners that publish Prometheus series can pass
// them through without remapping.
type ResolutionResult string

const (
	ResolutionCacheHit   ResolutionResult = "cache_hit"
	ResolutionDownloaded ResolutionResult = "downloaded"
	ResolutionFailed     ResolutionResult = "failed"
)

// CleanupStatus enumerates the outcomes of a cleanup attempt for a library
// that is no longer in use. Underlying string values match the driver
// metric labels for the same reason as ResolutionResult.
type CleanupStatus string

const (
	CleanupSuccess      CleanupStatus = "success"
	CleanupFailed       CleanupStatus = "failed"
	CleanupSkippedInUse CleanupStatus = "skipped_in_use"
)

// Snapshot is a consistent view of every aggregate the listener needs to
// publish gauges at startup. The maps are owned by the caller.
//
// The library key is the package name (e.g. "dd-lib-java-init"), matching
// the "library" label used by the driver's Prometheus metrics.
type Snapshot struct {
	// CachedCountByLibrary maps each library to the number of cached
	// versions (library IDs) currently on disk for that library.
	CachedCountByLibrary map[string]int
	// CachedBytesByLibrary maps each library to the cumulative on-disk
	// size, in bytes, of the cached versions for that library.
	CachedBytesByLibrary map[string]int64
}

// Listener is notified by the library manager of significant lifecycle
// events. The default implementation is a no-op (see NoopListener) so the
// manager can invoke it unconditionally; the production wiring registers
// the metrics-publishing listener from pkg/metrics.
//
// All methods are expected to be quick and non-blocking: they run on the
// caller goroutine, sometimes under a per-library lock. Implementations
// that need to do non-trivial work should fan out internally.
//
// The library parameter is the package name (e.g. "dd-lib-java-init"),
// matching the "library" label used by the driver's Prometheus metrics.
// It may be empty when the manager could not determine the library (for
// example on invalid inputs, or for legacy entries on disk that predate
// the metadata bucket); listeners must tolerate that.
type Listener interface {
	// OnLibraryResolved is called once a library resolution attempt
	// finishes. result reflects whether the library was served from the
	// cache, freshly downloaded, or the resolution failed.
	OnLibraryResolved(library string, result ResolutionResult)

	// OnLibraryDownload is called when a library has been fetched from a
	// registry, with the time spent on the download. Not called on cache
	// hits or failures before the download step.
	OnLibraryDownload(library, registry string, duration time.Duration)

	// OnLibraryCleanup is called for every cleanup attempt, including ones
	// that were skipped because the library is still in use.
	OnLibraryCleanup(library string, status CleanupStatus, strategy string)

	// OnLibraryCached is called when a new library version has been stored
	// on disk. cachedCount and cachedBytes are the per-library aggregates
	// after the addition, suitable for a Gauge.Set.
	OnLibraryCached(library string, cachedCount int, cachedBytes int64)

	// OnLibraryEvicted is called when a library has been removed from
	// disk. cachedCount and cachedBytes are the per-library aggregates
	// after the removal (zero when the last version is gone), suitable
	// for a Gauge.Set.
	OnLibraryEvicted(library string, cachedCount int, cachedBytes int64)

	// OnSnapshot is called once at LibraryManager construction so the
	// listener can seed its gauges with the persisted state and avoid the
	// cold-start gap until the next event.
	OnSnapshot(snapshot Snapshot)
}

// NoopListener is the default Listener used when no observer is
// configured. It discards every event.
type NoopListener struct{}

func (NoopListener) OnLibraryResolved(string, ResolutionResult)      {}
func (NoopListener) OnLibraryDownload(string, string, time.Duration) {}
func (NoopListener) OnLibraryCleanup(string, CleanupStatus, string)  {}
func (NoopListener) OnLibraryCached(string, int, int64)              {}
func (NoopListener) OnLibraryEvicted(string, int, int64)             {}
func (NoopListener) OnSnapshot(Snapshot)                             {}
