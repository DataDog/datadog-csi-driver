// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
)

// LibraryListener publishes Datadog CSI driver Prometheus metrics in
// response to library lifecycle events. It implements
// libraryevents.Listener; the librarymanager package itself does not
// depend on this package, which keeps the manager's domain logic free
// from any observability backend.
//
// The listener is stateless and safe for concurrent use.
type LibraryListener struct{}

// NewLibraryListener constructs a LibraryListener ready to be passed to
// librarymanager.WithEventListener.
func NewLibraryListener() *LibraryListener {
	return &LibraryListener{}
}

// OnLibraryResolved publishes the resolution outcome counter.
func (*LibraryListener) OnLibraryResolved(result libraryevents.ResolutionResult) {
	RecordLibraryResolution(result)
}

// OnLibraryDownload observes the download duration histogram.
func (*LibraryListener) OnLibraryDownload(library, registry string, duration time.Duration) {
	ObserveLibraryDownloadDuration(library, registry, duration)
}

// OnLibraryCleanup publishes the cleanup outcome counter.
func (*LibraryListener) OnLibraryCleanup(status libraryevents.CleanupStatus, strategy string) {
	RecordLibraryCleanup(status, strategy)
}

// OnLibraryCached updates the per-package cached gauges with the new
// aggregate values reported by the manager. The manager guarantees the
// counts are post-update.
func (*LibraryListener) OnLibraryCached(packageName string, cachedCount int, cachedBytes int64) {
	SetLibrariesCachedForPackage(packageName, cachedCount)
	SetLibrariesCachedBytesForPackage(packageName, cachedBytes)
}

// OnLibraryEvicted updates the per-package cached gauges with the new
// aggregate values reported by the manager. When the last version of a
// package is evicted both counts are zero, which materialises in
// Prometheus as a series of value 0 (kept intentionally so dashboards can
// show "cache is empty" rather than gap).
func (*LibraryListener) OnLibraryEvicted(packageName string, cachedCount int, cachedBytes int64) {
	SetLibrariesCachedForPackage(packageName, cachedCount)
	SetLibrariesCachedBytesForPackage(packageName, cachedBytes)
}

// OnSnapshot seeds the per-package gauges from the persisted state. Reset
// is used so packages that disappeared between two driver runs are not
// stuck reporting stale values.
func (*LibraryListener) OnSnapshot(s libraryevents.Snapshot) {
	librariesCached.Reset()
	librariesCachedBytes.Reset()
	for pkg, count := range s.CachedCountByPackage {
		SetLibrariesCachedForPackage(pkg, count)
	}
	for pkg, bytes := range s.CachedBytesByPackage {
		SetLibrariesCachedBytesForPackage(pkg, bytes)
	}
}
