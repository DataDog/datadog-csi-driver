// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"testing"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestRecordLibraryResolution(t *testing.T) {
	libraryResolutions.Reset()

	RecordLibraryResolution(libraryevents.ResolutionCacheHit)
	RecordLibraryResolution(libraryevents.ResolutionCacheHit)
	RecordLibraryResolution(libraryevents.ResolutionDownloaded)
	RecordLibraryResolution(libraryevents.ResolutionFailed)

	require.Equal(t, float64(2), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(libraryevents.ResolutionCacheHit))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(libraryevents.ResolutionDownloaded))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(libraryevents.ResolutionFailed))))
}

func TestRecordLibraryCleanup(t *testing.T) {
	libraryCleanup.Reset()

	RecordLibraryCleanup(libraryevents.CleanupSuccess, "immediate")
	RecordLibraryCleanup(libraryevents.CleanupSuccess, "immediate")
	RecordLibraryCleanup(libraryevents.CleanupFailed, "immediate")
	RecordLibraryCleanup(libraryevents.CleanupSkippedInUse, "delayed")

	require.Equal(t, float64(2), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(libraryevents.CleanupSuccess), "immediate")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(libraryevents.CleanupFailed), "immediate")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(libraryevents.CleanupSkippedInUse), "delayed")))
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
