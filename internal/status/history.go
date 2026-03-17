package status

import (
	"sync"
	"time"
)

// EventType categorizes the architectural meaning of a path-related event.
type EventType string

const (
	EventChosenPathChanged   EventType = "chosen-path-changed"
	EventFallbackToRelay     EventType = "fallback-to-relay"
	EventRecoveryToDirect    EventType = "recovery-to-direct"
	EventCandidateExcluded   EventType = "candidate-excluded"
	EventCandidateRestored   EventType = "candidate-restored"
	EventEndpointStale       EventType = "endpoint-stale"
	EventEndpointVerified    EventType = "endpoint-verified"
	EventEndpointFailed      EventType = "endpoint-failed"
	EventRevalidationStarted EventType = "revalidation-started"
	EventRevalidationSuccess EventType = "revalidation-success"
	EventRevalidationFailed  EventType = "revalidation-failed"
	EventPolicyHold          EventType = "policy-hold"
)

// Event represents a single point-in-time state transition or significant
// path-related occurrence. It is explicit and typed to prevent flattening
// all context into a single opaque log string.
type Event struct {
	Timestamp     time.Time
	Type          EventType
	AssociationID string
	CandidateID   string
	Message       string
}

// EventHistory is a bounded, concurrency-safe store for recent path events.
// It bridges the gap between current status and long-term analytics by
// answering "what just happened and why" without unbounded memory growth.
type EventHistory struct {
	mu     sync.RWMutex
	events []Event
	limit  int
}

// NewEventHistory creates a new bounded event history store.
// A limit of 0 disables history recording entirely.
func NewEventHistory(limit int) *EventHistory {
	return &EventHistory{
		events: make([]Event, 0, limit),
		limit:  limit,
	}
}

// Record appends a new event, evicting the oldest if the limit is reached.
func (h *EventHistory) Record(e Event) {
	if h.limit <= 0 {
		return
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.events) >= h.limit {
		// Shift elements left to make room at the end
		copy(h.events, h.events[1:])
		h.events[len(h.events)-1] = e
	} else {
		h.events = append(h.events, e)
	}
}

// Snapshot returns a copy of all currently held events in chronological order
// (oldest first).
func (h *EventHistory) Snapshot() []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]Event, len(h.events))
	copy(out, h.events)
	return out
}
