// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import "sync"

type lockEntry struct {
	mu   sync.Mutex
	refs int
}

// Locker is a sharded mutex to be able to perform concurrent operations on unrelated keys.
type Locker struct {
	mu      sync.Mutex
	entries map[string]*lockEntry
}

// NewLocker initializes a new locker.
func NewLocker() *Locker {
	return &Locker{
		entries: map[string]*lockEntry{},
	}
}

// Lock will acquire a lock for the given ID. The caller MUST call unlock.
func (l *Locker) Lock(id string) {
	// Get the entry.
	l.mu.Lock()
	entry, ok := l.entries[id]
	if !ok {
		entry = &lockEntry{}
		l.entries[id] = entry
	}
	entry.refs++
	l.mu.Unlock()

	// Lock the entry.
	entry.mu.Lock()
}

// Unlock will release a lock for the given ID.
func (l *Locker) Unlock(id string) {
	// Get the entry and remove it from the map if there are no more references.
	l.mu.Lock()
	entry, ok := l.entries[id]
	if !ok {
		l.mu.Unlock()
		return
	}
	entry.refs--
	if entry.refs == 0 {
		delete(l.entries, id)
	}
	l.mu.Unlock()

	// Unlock the entry.
	entry.mu.Unlock()
}
