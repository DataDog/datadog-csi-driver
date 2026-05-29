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
