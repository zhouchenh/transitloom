package status

import (
	"testing"
	"time"
)

func TestEventHistory_BoundedGrowth(t *testing.T) {
	limit := 3
	history := NewEventHistory(limit)

	// Add more events than the limit
	for i := 0; i < 5; i++ {
		history.Record(Event{
			Type:    EventChosenPathChanged,
			Message: "event " + string(rune('A'+i)),
		})
	}

	snap := history.Snapshot()
	if len(snap) != limit {
		t.Fatalf("expected snapshot length %d, got %d", limit, len(snap))
	}

	// Should contain the last 3 events: C, D, E
	if snap[0].Message != "event C" {
		t.Errorf("expected oldest to be 'event C', got %q", snap[0].Message)
	}
	if snap[2].Message != "event E" {
		t.Errorf("expected newest to be 'event E', got %q", snap[2].Message)
	}
}

func TestEventHistory_ZeroLimit(t *testing.T) {
	history := NewEventHistory(0)
	history.Record(Event{Type: EventChosenPathChanged})

	snap := history.Snapshot()
	if len(snap) != 0 {
		t.Fatalf("expected length 0 for zero-limit history, got %d", len(snap))
	}
}

func TestEventHistory_TimestampAutoSet(t *testing.T) {
	history := NewEventHistory(10)
	history.Record(Event{Type: EventChosenPathChanged})

	snap := history.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected length 1, got %d", len(snap))
	}
	if snap[0].Timestamp.IsZero() {
		t.Error("expected timestamp to be automatically set, got zero time")
	}

	// Explicit timestamp should be preserved
	explicitTime := time.Now().Add(-1 * time.Hour)
	history.Record(Event{
		Type:      EventFallbackToRelay,
		Timestamp: explicitTime,
	})
	snap2 := history.Snapshot()
	if !snap2[1].Timestamp.Equal(explicitTime) {
		t.Errorf("expected explicit time to be preserved, got %v", snap2[1].Timestamp)
	}
}
