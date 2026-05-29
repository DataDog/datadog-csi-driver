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
}

// noopEventListener is the default listener used when no observer is
// configured. It discards every event so the LibraryManager can invoke the
// listener unconditionally and keep the call sites clean.
type noopEventListener struct{}

func (noopEventListener) OnLibraryResolved(LibraryResolutionResult)       {}
func (noopEventListener) OnLibraryDownload(string, string, time.Duration) {}
func (noopEventListener) OnLibraryCleanup(LibraryCleanupStatus, string)   {}
