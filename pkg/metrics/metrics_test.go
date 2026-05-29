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

func TestSetLibraryVolumeLinks(t *testing.T) {
	libraryVolumeLinks.Reset()

	SetLibraryVolumeLinks(map[string]int{
		"dd-lib-java-init": 3,
		"dd-lib-php-init":  1,
		"":                 2, // legacy entries without a persisted package
	})

	require.Equal(t, float64(3), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-php-init")))
	require.Equal(t, float64(2), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues(UnknownLibrary)))

	// A subsequent call replaces stale series instead of leaving them behind.
	SetLibraryVolumeLinks(map[string]int{"dd-lib-java-init": 5})

	require.Equal(t, float64(5), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, 1, testutil.CollectAndCount(libraryVolumeLinks), "stale series must be cleared")
}

func TestSetLibraryVolumeLinksForPackage(t *testing.T) {
	libraryVolumeLinks.Reset()

	// Pre-populate two packages to make sure the targeted update only touches one.
	SetLibraryVolumeLinks(map[string]int{
		"dd-lib-java-init": 3,
		"dd-lib-php-init":  1,
	})

	SetLibraryVolumeLinksForPackage("dd-lib-java-init", 7)

	require.Equal(t, float64(7), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-php-init")), "other series must be untouched")
	require.Equal(t, 2, testutil.CollectAndCount(libraryVolumeLinks))

	// An empty package name maps to "unknown".
	SetLibraryVolumeLinksForPackage("", 4)
	require.Equal(t, float64(4), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues(UnknownLibrary)))
}

func TestDeleteLibraryVolumeLinksForPackage(t *testing.T) {
	libraryVolumeLinks.Reset()

	SetLibraryVolumeLinks(map[string]int{
		"dd-lib-java-init": 3,
		"dd-lib-php-init":  1,
	})
	require.Equal(t, 2, testutil.CollectAndCount(libraryVolumeLinks))

	DeleteLibraryVolumeLinksForPackage("dd-lib-php-init")

	require.Equal(t, 1, testutil.CollectAndCount(libraryVolumeLinks), "the deleted series must be gone")
	require.Equal(t, float64(3), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))

	// Deleting a missing package is a no-op.
	DeleteLibraryVolumeLinksForPackage("dd-lib-php-init")
	require.Equal(t, 1, testutil.CollectAndCount(libraryVolumeLinks))
}

func TestSetLibrariesCached(t *testing.T) {
	librariesCached.Reset()

	SetLibrariesCached(map[string]int{
		"dd-lib-java-init": 2,
		"dd-lib-php-init":  1,
		"":                 1, // legacy entries without a persisted package
	})

	require.Equal(t, float64(2), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-php-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(librariesCached.WithLabelValues(UnknownLibrary)))

	// A subsequent call replaces stale series rather than leaving them behind.
	SetLibrariesCached(map[string]int{"dd-lib-java-init": 4})

	require.Equal(t, float64(4), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, 1, testutil.CollectAndCount(librariesCached), "stale series must be cleared")
}

func TestSetLibrariesCachedForPackage(t *testing.T) {
	librariesCached.Reset()

	SetLibrariesCached(map[string]int{
		"dd-lib-java-init": 2,
		"dd-lib-php-init":  1,
	})

	SetLibrariesCachedForPackage("dd-lib-java-init", 5)

	require.Equal(t, float64(5), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-php-init")), "other series must be untouched")
	require.Equal(t, 2, testutil.CollectAndCount(librariesCached))

	SetLibrariesCachedForPackage("", 3)
	require.Equal(t, float64(3), testutil.ToFloat64(librariesCached.WithLabelValues(UnknownLibrary)))
}

func TestDeleteLibrariesCachedForPackage(t *testing.T) {
	librariesCached.Reset()

	SetLibrariesCached(map[string]int{
		"dd-lib-java-init": 2,
		"dd-lib-php-init":  1,
	})
	require.Equal(t, 2, testutil.CollectAndCount(librariesCached))

	DeleteLibrariesCachedForPackage("dd-lib-php-init")

	require.Equal(t, 1, testutil.CollectAndCount(librariesCached), "the deleted series must be gone")
	require.Equal(t, float64(2), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
}

func TestSetLibrariesCachedBytes(t *testing.T) {
	librariesCachedBytes.Reset()

	SetLibrariesCachedBytes(map[string]int64{
		"dd-lib-java-init": 1_000_000,
		"dd-lib-php-init":  500_000,
	})

	require.Equal(t, float64(1_000_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(500_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-php-init")))

	SetLibrariesCachedBytes(map[string]int64{"dd-lib-java-init": 2_000_000})
	require.Equal(t, float64(2_000_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, 1, testutil.CollectAndCount(librariesCachedBytes), "stale series must be cleared")
}

func TestSetLibrariesCachedBytesForPackage(t *testing.T) {
	librariesCachedBytes.Reset()

	SetLibrariesCachedBytes(map[string]int64{
		"dd-lib-java-init": 1_000_000,
		"dd-lib-php-init":  500_000,
	})

	SetLibrariesCachedBytesForPackage("dd-lib-java-init", 3_000_000)
	require.Equal(t, float64(3_000_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(500_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-php-init")), "other series must be untouched")

	SetLibrariesCachedBytesForPackage("", 42)
	require.Equal(t, float64(42), testutil.ToFloat64(librariesCachedBytes.WithLabelValues(UnknownLibrary)))
}

func TestDeleteLibrariesCachedBytesForPackage(t *testing.T) {
	librariesCachedBytes.Reset()

	SetLibrariesCachedBytes(map[string]int64{
		"dd-lib-java-init": 1_000_000,
		"dd-lib-php-init":  500_000,
	})
	require.Equal(t, 2, testutil.CollectAndCount(librariesCachedBytes))

	DeleteLibrariesCachedBytesForPackage("dd-lib-php-init")

	require.Equal(t, 1, testutil.CollectAndCount(librariesCachedBytes), "the deleted series must be gone")
	require.Equal(t, float64(1_000_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
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
