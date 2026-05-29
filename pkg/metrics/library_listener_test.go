// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"testing"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/libraryevents"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

// TestLibraryListenerImplementsInterface fails to compile if the listener
// drifts from the interface contract. It does not need to be run.
func TestLibraryListenerImplementsInterface(t *testing.T) {
	var _ libraryevents.Listener = (*LibraryListener)(nil)
}

func TestLibraryListenerPublishesResolutionAndCleanup(t *testing.T) {
	libraryResolutions.Reset()
	libraryCleanup.Reset()
	libraryDownloadDuration.Reset()

	l := NewLibraryListener()

	l.OnLibraryResolved("dd-lib-java-init", libraryevents.ResolutionCacheHit)
	l.OnLibraryResolved("dd-lib-java-init", libraryevents.ResolutionDownloaded)
	l.OnLibraryResolved("dd-lib-php-init", libraryevents.ResolutionFailed)
	l.OnLibraryCleanup("dd-lib-java-init", libraryevents.CleanupSuccess, "immediate")
	l.OnLibraryCleanup("dd-lib-php-init", libraryevents.CleanupSkippedInUse, "delayed")
	l.OnLibraryDownload("dd-lib-java-init", "gcr.io", 250*time.Millisecond)

	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues("dd-lib-java-init", string(libraryevents.ResolutionCacheHit))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues("dd-lib-java-init", string(libraryevents.ResolutionDownloaded))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues("dd-lib-php-init", string(libraryevents.ResolutionFailed))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues("dd-lib-java-init", string(libraryevents.CleanupSuccess), "immediate")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues("dd-lib-php-init", string(libraryevents.CleanupSkippedInUse), "delayed")))

	// The download histogram is still populated; we just check the sample count
	// exists for the right labels (bucket layout is tested elsewhere).
	require.Equal(t, 1, testutil.CollectAndCount(libraryDownloadDuration))
}

func TestLibraryListenerCachedAndEvictedSetGauges(t *testing.T) {
	librariesCached.Reset()
	librariesCachedBytes.Reset()

	l := NewLibraryListener()
	l.OnLibraryCached("dd-lib-java-init", 2, 1024)
	require.Equal(t, float64(2), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1024), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))

	// Evicting back to zero leaves the series at 0 so dashboards do not see a gap.
	l.OnLibraryEvicted("dd-lib-java-init", 0, 0)
	require.Equal(t, float64(0), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(0), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
}

func TestLibraryListenerVolumeLinkedAndUnlinkedSetGauge(t *testing.T) {
	libraryVolumeLinks.Reset()

	l := NewLibraryListener()
	l.OnVolumeLinked("dd-lib-java-init", 3)
	require.Equal(t, float64(3), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))

	l.OnVolumeUnlinked("dd-lib-java-init", 0)
	require.Equal(t, float64(0), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))
}

func TestLibraryListenerOnSnapshotSeedsGauges(t *testing.T) {
	librariesCached.Reset()
	librariesCachedBytes.Reset()
	libraryVolumeLinks.Reset()

	// Pre-existing stale series should be wiped by Reset inside OnSnapshot.
	librariesCached.WithLabelValues("stale").Set(7)
	libraryVolumeLinks.WithLabelValues("stale").Set(7)

	l := NewLibraryListener()
	l.OnSnapshot(libraryevents.Snapshot{
		CachedCountByLibrary: map[string]int{"dd-lib-java-init": 2, "dd-lib-php-init": 1},
		CachedBytesByLibrary: map[string]int64{"dd-lib-java-init": 4096, "dd-lib-php-init": 64},
		VolumeLinksByLibrary: map[string]int{"dd-lib-java-init": 5},
	})

	require.Equal(t, float64(2), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(4096), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-php-init")))
	require.Equal(t, float64(64), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-php-init")))
	require.Equal(t, float64(5), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))

	require.Equal(t, 2, testutil.CollectAndCount(librariesCached), "stale series should be evicted")
	require.Equal(t, 1, testutil.CollectAndCount(libraryVolumeLinks), "stale series should be evicted")
}
