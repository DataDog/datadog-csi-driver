// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator   = "_"
	subsystem   = "datadog_csi_driver"
	MetricsPort = 5000
)

// Status represents the status of an operation
type Status string

const (
	// StatusSuccess represents the success of an operation
	StatusSuccess Status = "success"
	// StatusFailed represents the failure of an operation
	StatusFailed = "failed"
	// StatusUnsupported represents an operation not supported by any publisher
	StatusUnsupported = "unsupported"
)

// ResolutionResult represents the outcome of attempting to resolve a library for a volume.
type ResolutionResult string

const (
	// ResolutionCacheHit indicates the library was already present in the local store.
	ResolutionCacheHit ResolutionResult = "cache_hit"
	// ResolutionDownloaded indicates the library was downloaded from the registry.
	ResolutionDownloaded ResolutionResult = "downloaded"
	// ResolutionFailed indicates the resolution failed at any step.
	ResolutionFailed ResolutionResult = "failed"
)

// CleanupStatus represents the outcome of a library cleanup attempt.
type CleanupStatus string

const (
	// CleanupSuccess indicates the library was successfully removed from disk.
	CleanupSuccess CleanupStatus = "success"
	// CleanupFailed indicates the cleanup attempt failed.
	CleanupFailed CleanupStatus = "failed"
	// CleanupSkippedInUse indicates the cleanup was skipped because the library is still in use.
	CleanupSkippedInUse CleanupStatus = "skipped_in_use"
)

// downloadDurationBuckets covers the range from very fast cached/local downloads
// (~100ms) up to slow registry pulls (~5 minutes).
var downloadDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300}

func newCounterVec(name, help string, labels ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: subsystem + separator + name,
		Help: help,
	}, labels)
}

func newHistogramVec(name, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    subsystem + separator + name,
		Help:    help,
		Buckets: buckets,
	}, labels)
}

func newGaugeVec(name, help string, labels ...string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: subsystem + separator + name,
		Help: help,
	}, labels)
}

var nodeVolumeMountAttempts = newCounterVec(
	"node_publish_volume_attempts",
	"Counts the number of publish volume requests received by the csi node server",
	"type",
	"path",
	"status",
)

var nodeVolumeUnmountAttempts = newCounterVec(
	"node_unpublish_volume_attempts",
	"Counts the number of unpublish volume requests received by the csi node server",
	"status",
)

var libraryResolutions = newCounterVec(
	"library_resolutions_total",
	"Counts the outcome of attempts to resolve a library for a volume",
	"result",
)

var libraryDownloadDuration = newHistogramVec(
	"library_download_duration_seconds",
	"Time spent downloading a library from the registry",
	downloadDurationBuckets,
	"library",
	"registry",
)

var libraryCleanup = newCounterVec(
	"library_cleanup_total",
	"Counts cleanup attempts for unused libraries",
	"status",
	"strategy",
)

var libraryVolumeLinks = newGaugeVec(
	"library_volume_links",
	"Number of volumes currently linked to each library package",
	"library",
)

var librariesCached = newGaugeVec(
	"libraries_cached",
	"Number of library versions currently cached on disk for each package",
	"library",
)

var librariesCachedBytes = newGaugeVec(
	"libraries_cached_bytes",
	"Total size in bytes of the library versions currently cached on disk for each package",
	"library",
)

func init() {
	prometheus.MustRegister(nodeVolumeMountAttempts)
	prometheus.MustRegister(nodeVolumeUnmountAttempts)
	prometheus.MustRegister(libraryResolutions)
	prometheus.MustRegister(libraryDownloadDuration)
	prometheus.MustRegister(libraryCleanup)
	prometheus.MustRegister(libraryVolumeLinks)
	prometheus.MustRegister(librariesCached)
	prometheus.MustRegister(librariesCachedBytes)
}

// RecordVolumeMountAttempt records a volume mount attempt
func RecordVolumeMountAttempt(volumeType, path string, status Status) {
	nodeVolumeMountAttempts.WithLabelValues(volumeType, path, string(status)).Inc()
}

// RecordVolumeUnMountAttempt records a volume unmount attempt
func RecordVolumeUnMountAttempt(status Status) {
	nodeVolumeUnmountAttempts.WithLabelValues(string(status)).Inc()
}

// RecordLibraryResolution records the outcome of an attempt to resolve a library for a volume.
func RecordLibraryResolution(result ResolutionResult) {
	libraryResolutions.WithLabelValues(string(result)).Inc()
}

// ObserveLibraryDownloadDuration records the duration of a successful library download.
// The library and registry labels allow breaking down latency per package and per registry endpoint.
func ObserveLibraryDownloadDuration(library, registry string, d time.Duration) {
	libraryDownloadDuration.WithLabelValues(library, registry).Observe(d.Seconds())
}

// RecordLibraryCleanup records the outcome of a cleanup attempt for an unused library.
// The strategy label captures which cleanup policy was active (e.g. "immediate", "delayed").
func RecordLibraryCleanup(status CleanupStatus, strategy string) {
	libraryCleanup.WithLabelValues(string(status), strategy).Inc()
}

// UnknownLibrary is used as the library label when the package cannot be determined,
// typically for entries persisted by an older driver version.
const UnknownLibrary = "unknown"

// SetLibraryVolumeLinksForPackage sets the library_volume_links gauge for a
// single package. Empty package names are reported under "unknown".
func SetLibraryVolumeLinksForPackage(library string, n int) {
	libraryVolumeLinks.WithLabelValues(libraryLabel(library)).Set(float64(n))
}

// SetLibrariesCachedForPackage sets the libraries_cached gauge for a single
// package. Empty package names are reported under "unknown".
func SetLibrariesCachedForPackage(library string, n int) {
	librariesCached.WithLabelValues(libraryLabel(library)).Set(float64(n))
}

// SetLibrariesCachedBytesForPackage sets the libraries_cached_bytes gauge
// for a single package. Empty package names are reported under "unknown".
func SetLibrariesCachedBytesForPackage(library string, sizeBytes int64) {
	librariesCachedBytes.WithLabelValues(libraryLabel(library)).Set(float64(sizeBytes))
}

// libraryLabel maps an empty package name to the "unknown" sentinel used for
// entries persisted by an older driver version.
func libraryLabel(pkg string) string {
	if pkg == "" {
		return UnknownLibrary
	}
	return pkg
}
