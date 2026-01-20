// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"log/slog"
	"sync"
	"time"
)

// CleanupFunc is a function that performs cleanup for a library.
// It receives the libraryID and should re-check if cleanup is still needed.
type CleanupFunc func(libraryID string) error

// CleanupStrategy defines how libraries are cleaned up when no longer in use.
type CleanupStrategy interface {
	// ScheduleCleanup is called when a library has no more volumes using it.
	// The cleanupFunc will be called either immediately or after a delay,
	// depending on the strategy implementation.
	ScheduleCleanup(libraryID string, cleanupFunc CleanupFunc)

	// Stop stops the strategy and executes all pending cleanups.
	Stop()
}

// ImmediateCleanupStrategy executes cleanup immediately when a library is no longer used.
// This is the default behavior.
type ImmediateCleanupStrategy struct{}

// NewImmediateCleanupStrategy creates a new immediate cleanup strategy.
func NewImmediateCleanupStrategy() *ImmediateCleanupStrategy {
	return &ImmediateCleanupStrategy{}
}

func (s *ImmediateCleanupStrategy) ScheduleCleanup(libraryID string, cleanupFunc CleanupFunc) {
	slog.Info("ImmediateCleanup: executing cleanup", "library_id", libraryID)
	if err := cleanupFunc(libraryID); err != nil {
		slog.Error("ImmediateCleanup: cleanup failed", "library_id", libraryID, "error", err)
	}
}

func (s *ImmediateCleanupStrategy) Stop() {
	// No-op: nothing to stop
}

// DelayedCleanupStrategy waits for a configurable delay before executing cleanup.
// This allows rolling updates to reuse libraries without re-downloading them.
type DelayedCleanupStrategy struct {
	delay time.Duration

	mu      sync.Mutex
	pending map[string]*pendingCleanup
	stopped bool
}

type pendingCleanup struct {
	timer       *time.Timer
	cleanupFunc CleanupFunc
}

// NewDelayedCleanupStrategy creates a new delayed cleanup strategy.
// The delay parameter specifies how long to wait before cleaning up unused libraries.
func NewDelayedCleanupStrategy(delay time.Duration) *DelayedCleanupStrategy {
	return &DelayedCleanupStrategy{
		delay:   delay,
		pending: make(map[string]*pendingCleanup),
	}
}

func (s *DelayedCleanupStrategy) ScheduleCleanup(libraryID string, cleanupFunc CleanupFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		// If stopped, execute immediately
		slog.Info("DelayedCleanup: strategy stopped, executing cleanup immediately", "library_id", libraryID)
		if err := cleanupFunc(libraryID); err != nil {
			slog.Error("DelayedCleanup: cleanup failed", "library_id", libraryID, "error", err)
		}
		return
	}

	// Cancel any existing pending cleanup for this library
	if existing, ok := s.pending[libraryID]; ok {
		existing.timer.Stop()
		delete(s.pending, libraryID)
	}

	slog.Info("DelayedCleanup: scheduling cleanup", "library_id", libraryID, "delay", s.delay)

	timer := time.AfterFunc(s.delay, func() {
		s.mu.Lock()
		// Check if still pending (might have been cleared by Stop())
		if _, ok := s.pending[libraryID]; !ok {
			s.mu.Unlock()
			return
		}
		delete(s.pending, libraryID)
		s.mu.Unlock()

		slog.Info("DelayedCleanup: executing scheduled cleanup", "library_id", libraryID)
		if err := cleanupFunc(libraryID); err != nil {
			slog.Error("DelayedCleanup: cleanup failed", "library_id", libraryID, "error", err)
		}
	})

	s.pending[libraryID] = &pendingCleanup{
		timer:       timer,
		cleanupFunc: cleanupFunc,
	}
}

func (s *DelayedCleanupStrategy) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}
	s.stopped = true

	// Execute all pending cleanups immediately
	for libraryID, pending := range s.pending {
		pending.timer.Stop()
		slog.Info("DelayedCleanup: stop - executing pending cleanup", "library_id", libraryID)
		if err := pending.cleanupFunc(libraryID); err != nil {
			slog.Error("DelayedCleanup: cleanup failed during stop", "library_id", libraryID, "error", err)
		}
	}
	s.pending = make(map[string]*pendingCleanup)
}
