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

// Listener is notified by the library manager of significant lifecycle
// events. The default implementation is a no-op (see NoopListener) so the
// manager can invoke it unconditionally; the production wiring registers
// the metrics-publishing listener from pkg/metrics.
//
// All methods are expected to be quick and non-blocking: they run on the
// caller goroutine, sometimes under a per-library lock. Implementations
// that need to do non-trivial work should fan out internally.
type Listener interface {
	// OnLibraryResolved is called once a library resolution attempt
	// finishes. result reflects whether the library was served from the
	// cache, freshly downloaded, or the resolution failed.
	OnLibraryResolved(result ResolutionResult)

	// OnLibraryDownload is called when a library has been fetched from a
	// registry, with the time spent on the download. Not called on cache
	// hits or failures before the download step.
	OnLibraryDownload(library, registry string, duration time.Duration)

	// OnLibraryCleanup is called for every cleanup attempt, including ones
	// that were skipped because the library is still in use.
	OnLibraryCleanup(status CleanupStatus, strategy string)
}

// NoopListener is the default Listener used when no observer is
// configured. It discards every event.
type NoopListener struct{}

func (NoopListener) OnLibraryResolved(ResolutionResult)             {}
func (NoopListener) OnLibraryDownload(string, string, time.Duration) {}
func (NoopListener) OnLibraryCleanup(CleanupStatus, string)         {}
