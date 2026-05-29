// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"testing"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

// TestLibraryListenerImplementsInterface fails to compile if the listener
// drifts from the interface contract. It does not need to be run.
func TestLibraryListenerImplementsInterface(t *testing.T) {
	var _ librarymanager.EventListener = (*LibraryListener)(nil)
}

func TestLibraryListenerPublishesResolutionAndCleanup(t *testing.T) {
	libraryResolutions.Reset()
	libraryCleanup.Reset()
	libraryDownloadDuration.Reset()

	l := NewLibraryListener()

	l.OnLibraryResolved(librarymanager.LibraryResolutionCacheHit)
	l.OnLibraryResolved(librarymanager.LibraryResolutionDownloaded)
	l.OnLibraryResolved(librarymanager.LibraryResolutionFailed)
	l.OnLibraryCleanup(librarymanager.LibraryCleanupSuccess, "immediate")
	l.OnLibraryCleanup(librarymanager.LibraryCleanupSkippedInUse, "delayed")
	l.OnLibraryDownload("dd-lib-java-init", "gcr.io", 250*time.Millisecond)

	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(ResolutionCacheHit))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(ResolutionDownloaded))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryResolutions.WithLabelValues(string(ResolutionFailed))))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(CleanupSuccess), "immediate")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryCleanup.WithLabelValues(string(CleanupSkippedInUse), "delayed")))

	// The download histogram is still populated; we just check the sample count
	// exists for the right labels (bucket layout is tested elsewhere).
	require.Equal(t, 1, testutil.CollectAndCount(libraryDownloadDuration))
}

func TestLibraryListenerPublishesVolumeLinkGauge(t *testing.T) {
	libraryVolumeLinks.Reset()

	l := NewLibraryListener()

	l.OnVolumeLinked("dd-lib-java-init", 3)
	l.OnVolumeLinked("dd-lib-php-init", 1)
	require.Equal(t, float64(3), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-php-init")))

	// OnVolumeUnlinked must publish the new count, including zero, so the
	// "no volumes" state is observable rather than dropped.
	l.OnVolumeUnlinked("dd-lib-php-init", 0)
	require.Equal(t, float64(0), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-php-init")))
	require.Equal(t, 2, testutil.CollectAndCount(libraryVolumeLinks), "zero-valued series must be kept")
}

func TestLibraryListenerPublishesCachedGauges(t *testing.T) {
	librariesCached.Reset()
	librariesCachedBytes.Reset()

	l := NewLibraryListener()

	l.OnLibraryCached("dd-lib-java-init", 2, 1_500_000)
	l.OnLibraryCached("dd-lib-php-init", 1, 500_000)
	require.Equal(t, float64(2), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1_500_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-php-init")))
	require.Equal(t, float64(500_000), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-php-init")))

	// Eviction down to zero must still publish (kept-at-zero contract).
	l.OnLibraryEvicted("dd-lib-php-init", 0, 0)
	require.Equal(t, float64(0), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-php-init")))
	require.Equal(t, float64(0), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-php-init")))
}

func TestLibraryListenerOnSnapshotResetsGauges(t *testing.T) {
	libraryVolumeLinks.Reset()
	librariesCached.Reset()
	librariesCachedBytes.Reset()

	l := NewLibraryListener()

	// Populate a couple of series to simulate the gauge state inherited
	// from a previous publishing cycle.
	l.OnVolumeLinked("dd-lib-java-init", 3)
	l.OnVolumeLinked("stale-package", 7)
	l.OnLibraryCached("dd-lib-java-init", 2, 1_500_000)
	l.OnLibraryCached("stale-package", 4, 9_999)

	// A snapshot that no longer mentions "stale-package" must drop it from
	// every gauge while bringing the kept series back to the snapshot value.
	l.OnSnapshot(
		map[string]int{"dd-lib-java-init": 5},
		map[string]int{"dd-lib-java-init": 1},
		map[string]int64{"dd-lib-java-init": 42},
	)

	require.Equal(t, float64(5), testutil.ToFloat64(libraryVolumeLinks.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(1), testutil.ToFloat64(librariesCached.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, float64(42), testutil.ToFloat64(librariesCachedBytes.WithLabelValues("dd-lib-java-init")))
	require.Equal(t, 1, testutil.CollectAndCount(libraryVolumeLinks), "stale series must be cleared")
	require.Equal(t, 1, testutil.CollectAndCount(librariesCached), "stale series must be cleared")
	require.Equal(t, 1, testutil.CollectAndCount(librariesCachedBytes), "stale series must be cleared")
}
