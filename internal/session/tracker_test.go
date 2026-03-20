package session

import (
	"testing"
	"time"
)

func TestTrackerSetGet(t *testing.T) {
	tr := NewTracker(time.Minute)
	defer tr.Stop()

	// Empty key is a no-op.
	tr.Set("", "resp_001", 3)
	if _, ok := tr.Get(""); ok {
		t.Error("expected no entry for empty key")
	}

	// Normal set/get round-trip.
	tr.Set("session1", "resp_abc", 4)
	entry, ok := tr.Get("session1")
	if !ok {
		t.Fatal("expected entry for session1")
	}
	if entry.ResponseID != "resp_abc" {
		t.Errorf("got ResponseID %q, want %q", entry.ResponseID, "resp_abc")
	}
	if entry.MessageCount != 4 {
		t.Errorf("got MessageCount %d, want 4", entry.MessageCount)
	}
}

func TestTrackerOverwrite(t *testing.T) {
	tr := NewTracker(time.Minute)
	defer tr.Stop()

	tr.Set("session1", "resp_v1", 2)
	tr.Set("session1", "resp_v2", 5)

	entry, ok := tr.Get("session1")
	if !ok {
		t.Fatal("expected entry")
	}
	if entry.ResponseID != "resp_v2" {
		t.Errorf("expected resp_v2, got %q", entry.ResponseID)
	}
	if entry.MessageCount != 5 {
		t.Errorf("expected 5, got %d", entry.MessageCount)
	}
}

func TestTrackerExpiry(t *testing.T) {
	ttl := 50 * time.Millisecond
	tr := NewTracker(ttl)
	defer tr.Stop()

	tr.Set("session1", "resp_abc", 2)

	// Should exist before TTL.
	if _, ok := tr.Get("session1"); !ok {
		t.Fatal("expected entry before expiry")
	}

	time.Sleep(ttl + 10*time.Millisecond)

	// Should be expired on next Get.
	if _, ok := tr.Get("session1"); ok {
		t.Error("expected entry to be expired")
	}
}

func TestTrackerDelete(t *testing.T) {
	tr := NewTracker(time.Minute)
	defer tr.Stop()

	tr.Set("session1", "resp_abc", 3)
	tr.Delete("session1")

	if _, ok := tr.Get("session1"); ok {
		t.Error("expected entry to be deleted")
	}
}

func TestTrackerLen(t *testing.T) {
	tr := NewTracker(time.Minute)
	defer tr.Stop()

	if tr.Len() != 0 {
		t.Errorf("expected 0, got %d", tr.Len())
	}
	tr.Set("a", "resp_1", 1)
	tr.Set("b", "resp_2", 2)
	if tr.Len() != 2 {
		t.Errorf("expected 2, got %d", tr.Len())
	}
	tr.Delete("a")
	if tr.Len() != 1 {
		t.Errorf("expected 1, got %d", tr.Len())
	}
}

func TestTrackerEmptyResponseIDIgnored(t *testing.T) {
	tr := NewTracker(time.Minute)
	defer tr.Stop()

	// Empty responseID should not be stored.
	tr.Set("session1", "", 3)
	if _, ok := tr.Get("session1"); ok {
		t.Error("expected no entry when responseID is empty")
	}
}

func TestTrackerNoTTL(t *testing.T) {
	// TTL of 0 means entries never expire via Get check.
	tr := NewTracker(0)
	defer tr.Stop()

	tr.Set("session1", "resp_abc", 3)
	entry, ok := tr.Get("session1")
	if !ok {
		t.Fatal("expected entry")
	}
	if entry.ResponseID != "resp_abc" {
		t.Errorf("got %q, want resp_abc", entry.ResponseID)
	}
}
