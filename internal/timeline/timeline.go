package timeline

import (
	"slices"
	"sync"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Entry is an immutable log entry containing an event.
// Logical ordering is determined by the event's Timestamp (SimTime) field,
// never by wall time (determinism per §0.2 r5).
type Entry struct {
	Event contracts.Event
}

// Timeline is an append-only log of events (SPEC §16.1).
// It is queried for the dashboard timeline and deterministic replay.
// Thread-safe: multiple goroutines may safely call Append and query methods.
type Timeline struct {
	mu      sync.RWMutex
	entries []Entry
}

// New returns an empty timeline.
func New() *Timeline {
	return &Timeline{entries: make([]Entry, 0)}
}

// Append adds an event to the log. The event is stored by value with its
// Payload deep-cloned so later mutation by the caller cannot corrupt the log.
func (t *Timeline) Append(ev contracts.Event) {
	// Clone Payload to ensure true immutability (json.RawMessage is []byte).
	// slices.Clone(nil) returns nil, so no nil check needed.
	ev.Payload = slices.Clone(ev.Payload)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, Entry{Event: ev})
}

// All returns a copy of all entries in chronological order.
// The returned slice is safe to modify without affecting the timeline.
// Note: Payload fields in returned entries are still aliases to cloned storage;
// callers should treat them as read-only to preserve immutability guarantees.
func (t *Timeline) All() []Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Entry, len(t.entries))
	copy(out, t.entries)
	return out
}

// Len returns the number of entries in the log.
func (t *Timeline) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}

// Last returns the last event, or nil if the log is empty.
// The returned event is a copy; modifications to it do not affect the log.
// Note: Payload should still be treated as read-only to preserve the
// immutability contract.
func (t *Timeline) Last() *contracts.Event {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.entries) == 0 {
		return nil
	}
	e := t.entries[len(t.entries)-1].Event
	return &e
}

// Since returns all entries with Timestamp >= t.
// Useful for dashboard incremental updates.
func (t *Timeline) Since(ts contracts.SimTime) []Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []Entry
	for _, entry := range t.entries {
		if entry.Event.Timestamp >= ts {
			out = append(out, entry)
		}
	}
	return out
}

// UpTo returns all entries with Timestamp <= t.
// Useful for replay initialization.
func (t *Timeline) UpTo(ts contracts.SimTime) []Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []Entry
	for _, entry := range t.entries {
		if entry.Event.Timestamp <= ts {
			out = append(out, entry)
		}
	}
	return out
}
