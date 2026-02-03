// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestImmediateCleanupStrategy_ExecutesImmediately(t *testing.T) {
	strategy := NewImmediateCleanupStrategy()

	var executed bool
	var receivedID string
	strategy.ScheduleCleanup("lib-123", func(libraryID string) error {
		executed = true
		receivedID = libraryID
		return nil
	})

	assert.True(t, executed, "cleanup should be executed immediately")
	assert.Equal(t, "lib-123", receivedID, "libraryID should be passed to cleanup function")
}

func TestDelayedCleanupStrategy_ExecutesAfterDelay(t *testing.T) {
	strategy := NewDelayedCleanupStrategy(50 * time.Millisecond)
	defer strategy.Stop()

	var executed atomic.Bool
	strategy.ScheduleCleanup("lib-123", func(libraryID string) error {
		executed.Store(true)
		return nil
	})

	// Should not be executed immediately
	assert.False(t, executed.Load(), "cleanup should not be executed immediately")

	// Wait for the delay
	time.Sleep(100 * time.Millisecond)

	assert.True(t, executed.Load(), "cleanup should be executed after delay")
}

func TestDelayedCleanupStrategy_StopExecutesPendingCleanups(t *testing.T) {
	strategy := NewDelayedCleanupStrategy(1 * time.Hour) // Long delay

	var executed1, executed2 atomic.Bool
	strategy.ScheduleCleanup("lib-1", func(libraryID string) error {
		executed1.Store(true)
		return nil
	})
	strategy.ScheduleCleanup("lib-2", func(libraryID string) error {
		executed2.Store(true)
		return nil
	})

	// Stop should execute all pending cleanups immediately
	strategy.Stop()

	assert.True(t, executed1.Load(), "pending cleanup 1 should be executed on stop")
	assert.True(t, executed2.Load(), "pending cleanup 2 should be executed on stop")
}

func TestDelayedCleanupStrategy_ReschedulingResetsTimer(t *testing.T) {
	strategy := NewDelayedCleanupStrategy(50 * time.Millisecond)
	defer strategy.Stop()

	var executeCount atomic.Int32
	cleanupFunc := func(libraryID string) error {
		executeCount.Add(1)
		return nil
	}

	// Schedule first cleanup
	strategy.ScheduleCleanup("lib-123", cleanupFunc)

	// Wait a bit, then reschedule (should reset the timer)
	time.Sleep(30 * time.Millisecond)
	strategy.ScheduleCleanup("lib-123", cleanupFunc)

	// Wait less than the new delay
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, int32(0), executeCount.Load(), "cleanup should not be executed yet")

	// Wait for the new delay to complete
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), executeCount.Load(), "cleanup should be executed exactly once")
}

func TestDelayedCleanupStrategy_MultipleLibrariesIndependent(t *testing.T) {
	strategy := NewDelayedCleanupStrategy(50 * time.Millisecond)
	defer strategy.Stop()

	var executed1, executed2 atomic.Bool
	strategy.ScheduleCleanup("lib-1", func(libraryID string) error {
		executed1.Store(true)
		return nil
	})
	strategy.ScheduleCleanup("lib-2", func(libraryID string) error {
		executed2.Store(true)
		return nil
	})

	// Wait for the delay
	time.Sleep(100 * time.Millisecond)

	assert.True(t, executed1.Load(), "lib-1 cleanup should be executed")
	assert.True(t, executed2.Load(), "lib-2 cleanup should be executed")
}

func TestDelayedCleanupStrategy_ScheduleAfterStopExecutesImmediately(t *testing.T) {
	strategy := NewDelayedCleanupStrategy(1 * time.Hour)
	strategy.Stop()

	var executed atomic.Bool
	strategy.ScheduleCleanup("lib-123", func(libraryID string) error {
		executed.Store(true)
		return nil
	})

	// Should be executed immediately since strategy is stopped
	assert.True(t, executed.Load(), "cleanup should be executed immediately after stop")
}

func TestDelayedCleanupStrategy_ConcurrentAccess(t *testing.T) {
	strategy := NewDelayedCleanupStrategy(10 * time.Millisecond)
	defer strategy.Stop()

	var executeCount atomic.Int32
	const numGoroutines = 100

	// Spawn many goroutines that schedule cleanups concurrently
	done := make(chan struct{})
	for i := 0; i < numGoroutines; i++ {
		go func() {
			libID := "lib-123" // Same library for contention
			strategy.ScheduleCleanup(libID, func(libraryID string) error {
				executeCount.Add(1)
				return nil
			})
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatalf("timeout waiting for goroutine %d/%d to complete", i+1, numGoroutines)
		}
	}

	// Wait for any pending cleanup
	time.Sleep(50 * time.Millisecond)

	// Should execute exactly once (last schedule wins, timer resets)
	assert.Equal(t, int32(1), executeCount.Load(), "cleanup should be executed exactly once")
}
