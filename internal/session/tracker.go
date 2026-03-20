// Package session manages conversation session state for Responses API compaction.
package session

import (
	"sync"
	"time"
)

// Entry stores the compaction state for a session.
type Entry struct {
	// ResponseID is the Responses API response ID from the last completed turn.
	ResponseID string
	// MessageCount is the number of messages in the Anthropic request when this
	// response was generated. Used to determine which messages are "new" on the
	// next continuation request.
	MessageCount int
	// LastSeen is updated each time the session is accessed.
	LastSeen time.Time
}

// Tracker maintains in-memory session state for Responses API compaction.
// It maps session keys to their last response IDs and message counts.
// Sessions expire after the configured TTL.
type Tracker struct {
	mu      sync.Mutex
	entries map[string]*Entry
	ttl     time.Duration
	stopCh  chan struct{}
}

// NewTracker creates a Tracker with the given TTL and starts background cleanup.
func NewTracker(ttl time.Duration) *Tracker {
	t := &Tracker{
		entries: make(map[string]*Entry),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	go t.cleanupLoop()
	return t
}

// Get returns the session entry for key if it exists and has not expired.
func (t *Tracker) Get(key string) (*Entry, bool) {
	if key == "" {
		return nil, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.entries[key]
	if !ok {
		return nil, false
	}
	if t.ttl > 0 && time.Since(entry.LastSeen) > t.ttl {
		delete(t.entries, key)
		return nil, false
	}
	return entry, true
}

// Set stores the response ID and message count for a session key, refreshing LastSeen.
func (t *Tracker) Set(key, responseID string, messageCount int) {
	if key == "" || responseID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[key] = &Entry{
		ResponseID:   responseID,
		MessageCount: messageCount,
		LastSeen:     time.Now(),
	}
}

// Delete removes a session entry.
func (t *Tracker) Delete(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// Len returns the number of active session entries.
func (t *Tracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.entries)
}

// Stop shuts down the background cleanup goroutine.
func (t *Tracker) Stop() {
	close(t.stopCh)
}

// cleanupLoop periodically removes expired entries.
func (t *Tracker) cleanupLoop() {
	if t.ttl <= 0 {
		return
	}
	ticker := time.NewTicker(t.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.evictExpired()
		case <-t.stopCh:
			return
		}
	}
}

func (t *Tracker) evictExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for key, entry := range t.entries {
		if now.Sub(entry.LastSeen) > t.ttl {
			delete(t.entries, key)
		}
	}
}
