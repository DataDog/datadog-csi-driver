// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
)

// LibraryListener publishes Datadog CSI driver Prometheus metrics in response
// to LibraryManager lifecycle events. It implements librarymanager.EventListener;
// the librarymanager package itself does not depend on this package, which
// keeps the manager's domain logic free from any observability backend.
//
// The listener is stateless: every gauge value it sets is computed by the
// LibraryManager's database from its in-memory aggregates. This makes it
// safe for concurrent use and trivially replaceable in tests.
type LibraryListener struct{}

// NewLibraryListener constructs a LibraryListener ready to be passed to
// librarymanager.WithEventListener.
func NewLibraryListener() *LibraryListener {
	return &LibraryListener{}
}

// OnLibraryResolved publishes the resolution outcome counter. The underlying
// string values of librarymanager.LibraryResolutionResult and this package's
// ResolutionResult are kept in sync so the conversion is a no-op cast.
func (*LibraryListener) OnLibraryResolved(result librarymanager.LibraryResolutionResult) {
	RecordLibraryResolution(ResolutionResult(result))
}

// OnLibraryDownload observes the download duration histogram.
func (*LibraryListener) OnLibraryDownload(library, registry string, duration time.Duration) {
	ObserveLibraryDownloadDuration(library, registry, duration)
}

// OnLibraryCleanup publishes the cleanup outcome counter.
func (*LibraryListener) OnLibraryCleanup(status librarymanager.LibraryCleanupStatus, strategy string) {
	RecordLibraryCleanup(CleanupStatus(status), strategy)
}

// OnVolumeLinked publishes the post-link library_volume_links value for the
// affected package. The total is computed by the database and passed in;
// nothing is recomputed here.
func (*LibraryListener) OnVolumeLinked(pkg string, packageVolumeLinkCount int) {
	SetLibraryVolumeLinksForPackage(pkg, packageVolumeLinkCount)
}

// OnVolumeUnlinked publishes the post-unlink library_volume_links value.
// Series that drop to zero are kept (set to 0 rather than deleted) so
// dashboards can observe the "no volumes" state explicitly.
func (*LibraryListener) OnVolumeUnlinked(pkg string, packageVolumeLinkCount int) {
	SetLibraryVolumeLinksForPackage(pkg, packageVolumeLinkCount)
}

// OnLibraryCached publishes the post-cache libraries_cached and
// libraries_cached_bytes values.
func (*LibraryListener) OnLibraryCached(pkg string, packageCachedCount int, packageCachedBytes int64) {
	SetLibrariesCachedForPackage(pkg, packageCachedCount)
	SetLibrariesCachedBytesForPackage(pkg, packageCachedBytes)
}

// OnLibraryEvicted publishes the post-eviction libraries_cached and
// libraries_cached_bytes values. Series that drop to zero are kept at 0
// (same rationale as OnVolumeUnlinked).
func (*LibraryListener) OnLibraryEvicted(pkg string, packageCachedCount int, packageCachedBytes int64) {
	SetLibrariesCachedForPackage(pkg, packageCachedCount)
	SetLibrariesCachedBytesForPackage(pkg, packageCachedBytes)
}

// OnSnapshot resets the stateful gauges to match the persisted state at
// startup. The Reset before each fan-out ensures any series that
// disappeared while the driver was down (e.g. a package no longer cached)
// is cleared rather than left at its pre-restart value.
func (*LibraryListener) OnSnapshot(s librarymanager.Snapshot) {
	libraryVolumeLinks.Reset()
	for pkg, n := range s.VolumeLinksByPackage {
		SetLibraryVolumeLinksForPackage(pkg, n)
	}
	librariesCached.Reset()
	for pkg, n := range s.CachedCountByPackage {
		SetLibrariesCachedForPackage(pkg, n)
	}
	librariesCachedBytes.Reset()
	for pkg, n := range s.CachedBytesByPackage {
		SetLibrariesCachedBytesForPackage(pkg, n)
	}
}
