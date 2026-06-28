package timeline

import (
	"errors"
	"testing"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/events"
)

func TestListenAppendsEventsFromBus(t *testing.T) {
	tl := New()
	bus := events.New(4)

	cancel := Listen(bus, tl)
	defer cancel()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	ev1 := contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test", Type: contracts.EventMainshockOccurred, Confidence: 1}
	ev2 := contracts.Event{ID: "evt-2", Timestamp: 200, Source: "test", Type: contracts.EventBridgeClosed, Confidence: 0.8}

	bus.Publish(ev1)
	bus.Publish(ev2)

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	if tl.Len() != 2 {
		t.Fatalf("expected 2 events in timeline, got %d", tl.Len())
	}

	all := tl.All()
	if all[0].Event.ID != "evt-1" || all[1].Event.ID != "evt-2" {
		t.Fatalf("events not in expected order: got %s, %s", all[0].Event.ID, all[1].Event.ID)
	}
}

func TestListenCancelStopsGoroutine(t *testing.T) {
	tl := New()
	bus := events.New(4)

	cancel := Listen(bus, tl)
	time.Sleep(10 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	// Publish after cancel - should not be received
	bus.Publish(contracts.Event{ID: "evt-after-cancel", Timestamp: 100, Source: "test"})
	time.Sleep(50 * time.Millisecond)

	// Timeline should still be functional but not receive new events
	ev := contracts.Event{ID: "evt-manual", Timestamp: 200, Source: "test"}
	tl.Append(ev)
	if tl.Len() != 1 {
		t.Fatalf("expected 1 event after manual append, got %d", tl.Len())
	}
}

func TestReplayAppliesEventsInOrder(t *testing.T) {
	// Create a timeline with events
	tl := New()
	for i := 1; i <= 5; i++ {
		ev := contracts.Event{
			ID:         "evt-",
			Timestamp:  contracts.SimTime(i * 100),
			Source:     "test",
			Type:       contracts.EventBridgeClosed,
			Confidence: 1,
		}
		tl.Append(ev)
	}

	// Create a mock state store that tracks applied events
	var applied []contracts.EventID
	mockStore := &mockStateStore{applyFunc: func(ev contracts.Event) (contracts.StateVersion, error) {
		applied = append(applied, ev.ID)
		return contracts.StateVersion(len(applied)), nil
	}}

	finalVer, rejected, err := Replay(tl, mockStore, 400)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int(finalVer) != 4 {
		t.Fatalf("expected final version 4, got %d", finalVer)
	}
	if len(rejected) != 0 {
		t.Fatalf("expected no rejections, got %d", len(rejected))
	}
	if len(applied) != 4 {
		t.Fatalf("expected 4 applied events, got %d", len(applied))
	}
}

func TestReplayStopsAtUpToTimestamp(t *testing.T) {
	tl := New()
	for i := 1; i <= 5; i++ {
		ev := contracts.Event{
			ID:        "evt-",
			Timestamp: contracts.SimTime(i * 100),
			Source:    "test",
		}
		tl.Append(ev)
	}

	var applied []contracts.EventID
	mockStore := &mockStateStore{applyFunc: func(ev contracts.Event) (contracts.StateVersion, error) {
		applied = append(applied, ev.ID)
		return contracts.StateVersion(len(applied)), nil
	}}

	_, _, err := Replay(tl, mockStore, 250)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only apply events 1, 2 (timestamps 100, 200)
	// Event 3 at timestamp 300 should be skipped
	if len(applied) != 2 {
		t.Fatalf("expected 2 events applied (up to 250), got %d: %v", len(applied), applied)
	}
}

func TestReplayHandlesRejectedEvents(t *testing.T) {
	tl := New()
	tl.Append(contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test"})
	tl.Append(contracts.Event{ID: "evt-2", Timestamp: 200, Source: "test"})

	rejectionErr := &contracts.RejectionError{EventID: "evt-2", Reason: contracts.RejectDuplicate, Detail: "duplicate"}
	mockStore := &mockStateStore{applyFunc: func(ev contracts.Event) (contracts.StateVersion, error) {
		if ev.ID == "evt-2" {
			return 0, rejectionErr
		}
		return contracts.StateVersion(1), nil
	}}

	finalVer, rejected, err := Replay(tl, mockStore, 300)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int(finalVer) != 1 {
		t.Fatalf("expected version 1, got %d", finalVer)
	}
	if len(rejected) != 1 || rejected[0].ID != "evt-2" {
		t.Fatalf("expected 1 rejected event (evt-2), got %d: %v", len(rejected), rejected)
	}
}

func TestReplayReturnsUnexpectedErrors(t *testing.T) {
	tl := New()
	tl.Append(contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test"})
	tl.Append(contracts.Event{ID: "evt-2", Timestamp: 200, Source: "test"})

	unexpectedErr := errors.New("internal failure")
	mockStore := &mockStateStore{applyFunc: func(ev contracts.Event) (contracts.StateVersion, error) {
		if ev.ID == "evt-2" {
			return 0, unexpectedErr
		}
		return contracts.StateVersion(1), nil
	}}

	finalVer, rejected, err := Replay(tl, mockStore, 300)

	if err != unexpectedErr {
		t.Fatalf("expected unexpected error to be returned, got: %v", err)
	}
	if int(finalVer) != 1 {
		t.Fatalf("expected version 1 (stopped at error), got %d", finalVer)
	}
	if len(rejected) != 0 {
		t.Fatalf("expected no rejections on unexpected error, got %d", len(rejected))
	}
}

// mockStateStore implements contracts.StateStore for testing
type mockStateStore struct {
	applyFunc func(ev contracts.Event) (contracts.StateVersion, error)
}

func (m *mockStateStore) Snapshot() contracts.WorldState {
	return contracts.WorldState{}
}

func (m *mockStateStore) Version() contracts.StateVersion {
	return contracts.StateVersion(0)
}

func (m *mockStateStore) Apply(ev contracts.Event) (contracts.StateVersion, error) {
	return m.applyFunc(ev)
}
