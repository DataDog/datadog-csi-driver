// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
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

var librariesCached = newGaugeVec(
	"libraries_cached",
	"Number of library versions currently stored on disk, per package",
	"library",
)

var librariesCachedBytes = newGaugeVec(
	"libraries_cached_bytes",
	"Cumulative on-disk size of cached libraries, in bytes, per package",
	"library",
)

func init() {
	prometheus.MustRegister(nodeVolumeMountAttempts)
	prometheus.MustRegister(nodeVolumeUnmountAttempts)
	prometheus.MustRegister(libraryResolutions)
	prometheus.MustRegister(libraryDownloadDuration)
	prometheus.MustRegister(libraryCleanup)
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
func RecordLibraryResolution(result libraryevents.ResolutionResult) {
	libraryResolutions.WithLabelValues(string(result)).Inc()
}

// ObserveLibraryDownloadDuration records the duration of a successful library download.
// The library and registry labels allow breaking down latency per package and per registry endpoint.
func ObserveLibraryDownloadDuration(library, registry string, d time.Duration) {
	libraryDownloadDuration.WithLabelValues(library, registry).Observe(d.Seconds())
}

// RecordLibraryCleanup records the outcome of a cleanup attempt for an unused library.
// The strategy label captures which cleanup policy was active (e.g. "immediate", "delayed").
func RecordLibraryCleanup(status libraryevents.CleanupStatus, strategy string) {
	libraryCleanup.WithLabelValues(string(status), strategy).Inc()
}

// SetLibrariesCachedForPackage sets the number of cached library versions for a package.
func SetLibrariesCachedForPackage(library string, count int) {
	librariesCached.WithLabelValues(library).Set(float64(count))
}

// SetLibrariesCachedBytesForPackage sets the cumulative on-disk size of cached libraries
// for a given package, in bytes.
func SetLibrariesCachedBytesForPackage(library string, bytes int64) {
	librariesCachedBytes.WithLabelValues(library).Set(float64(bytes))
}
