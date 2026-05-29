// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestRecordLibraryResolution(t *testing.T) {
	libraryResolutions.Reset()

	RecordLibraryResolution(ResolutionCacheHit)
	RecordLibraryResolution(ResolutionCacheHit)
	RecordLibraryResolution(ResolutionDownloaded)
	RecordLibraryResolution(ResolutionFailed)

	require.Equal(t, float64(2), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(ResolutionCacheHit))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(ResolutionDownloaded))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(ResolutionFailed))))
}

func TestSetLibraryVolumeLinksForPackage(t *testing.T) {
	libraryVolumeLinks.Reset()

	SetLibraryVolumeLinksForPackage("dd-lib-java-init", 3)
	SetLibraryVolumeLinksForPackage("dd-lib-php-init", 1)
	SetLibraryVolumeLinksForPackage("dd-lib-java-init", 7) // overwrites the previous value
	SetLibraryVolumeLinksForPackage("", 4)                 // empty package maps to "unknown"

	require.Equal(t, float64(7), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-php-init")), "other series must be untouched")
	require.Equal(t, float64(4), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues(UnknownLibrary)))
}

func TestSetLibrariesCachedForPackage(t *testing.T) {
	librariesCached.Reset()

	SetLibrariesCachedForPackage("dd-lib-java-init", 2)
	SetLibrariesCachedForPackage("dd-lib-php-init", 1)
	SetLibrariesCachedForPackage("dd-lib-java-init", 5)
	SetLibrariesCachedForPackage("", 3)

	require.Equal(t, float64(5), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-php-init")), "other series must be untouched")
	require.Equal(t, float64(3), testutil.ToFloat64(librariesCached.WithLabelValues(UnknownLibrary)))
}

func TestSetLibrariesCachedBytesForPackage(t *testing.T) {
	librariesCachedBytes.Reset()

	SetLibrariesCachedBytesForPackage("dd-lib-java-init", 1_000_000)
	SetLibrariesCachedBytesForPackage("dd-lib-php-init", 500_000)
	SetLibrariesCachedBytesForPackage("dd-lib-java-init", 3_000_000)
	SetLibrariesCachedBytesForPackage("", 42)

	require.Equal(t, float64(3_000_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(500_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-php-init")), "other series must be untouched")
	require.Equal(t, float64(42), testutil.ToFloat64(librariesCachedBytes.WithLabelValues(UnknownLibrary)))
}

func TestRecordLibraryCleanup(t *testing.T) {
	libraryCleanup.Reset()

	RecordLibraryCleanup(CleanupSuccess, "immediate")
	RecordLibraryCleanup(CleanupSuccess, "immediate")
	RecordLibraryCleanup(CleanupFailed, "immediate")
	RecordLibraryCleanup(CleanupSkippedInUse, "delayed")

	require.Equal(t, float64(2), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(CleanupSuccess), "immediate")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(CleanupFailed), "immediate")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(CleanupSkippedInUse), "delayed")))
}

func TestObserveLibraryDownloadDuration(t *testing.T) {
	libraryDownloadDuration.Reset()

	ObserveLibraryDownloadDuration("dd-lib-java-init", "gcr.io", 200*time.Millisecond)
	ObserveLibraryDownloadDuration("dd-lib-java-init", "gcr.io", 3*time.Second)
	ObserveLibraryDownloadDuration("dd-lib-php-init", "gcr.io", 1*time.Second)
	ObserveLibraryDownloadDuration("apm-inject", "docker.io", 75*time.Second)

	cases := []struct {
		library, registry string
		wantCount         uint64
		wantSum           float64
	}{
		{"dd-lib-java-init", "gcr.io", 2, 3.2},
		{"dd-lib-php-init", "gcr.io", 1, 1.0},
		{"apm-inject", "docker.io", 1, 75.0},
	}
	for _, tc := range cases {
		t.Run(tc.library+"_"+tc.registry, func(t *testing.T) {
			count, sum := histogramCountAndSum(t, libraryDownloadDuration, tc.library, tc.registry)
			require.Equal(t, tc.wantCount, count)
			require.InDelta(t, tc.wantSum, sum, 1e-9)
		})
	}
}

// histogramCountAndSum returns the observed sample count and sum for the given
// (library, registry) series of a HistogramVec. This intentionally ignores
// bucket layout so the test stays robust when buckets evolve.
func histogramCountAndSum(t *testing.T, vec *prometheus.HistogramVec, lvs ...string) (uint64, float64) {
	t.Helper()
	m, err := vec.GetMetricWithLabelValues(lvs...)
	require.NoError(t, err)

	pb := &dto.Metric{}
	require.NoError(t, m.(prometheus.Histogram).Write(pb))
	require.NotNil(t, pb.Histogram, "metric is not a histogram")

	return pb.Histogram.GetSampleCount(), pb.Histogram.GetSampleSum()
}
