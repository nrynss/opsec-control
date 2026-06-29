package timeline

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func TestNewReturnsEmptyTimeline(t *testing.T) {
	tl := New()
	if tl.Len() != 0 {
		t.Fatalf("expected empty timeline, got %d entries", tl.Len())
	}
	if tl.Last() != nil {
		t.Fatalf("expected nil Last on empty timeline")
	}
}

func TestAppendIncreasesLen(t *testing.T) {
	tl := New()
	ev := contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test", Type: contracts.EventMainshockOccurred, Confidence: 1}
	tl.Append(ev)
	if tl.Len() != 1 {
		t.Fatalf("expected 1 entry after append, got %d", tl.Len())
	}
}

func TestLastReturnsMostRecentEvent(t *testing.T) {
	tl := New()
	ev1 := contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test", Type: contracts.EventMainshockOccurred, Confidence: 1}
	ev2 := contracts.Event{ID: "evt-2", Timestamp: 200, Source: "test", Type: contracts.EventAftershockOccurred, Confidence: 0.8}
	tl.Append(ev1)
	tl.Append(ev2)

	got := tl.Last()
	if got == nil {
		t.Fatalf("expected non-nil Last")
	}
	if got.ID != ev2.ID {
		t.Fatalf("expected last event ID to be %s, got %s", ev2.ID, got.ID)
	}
	if got.Timestamp != ev2.Timestamp {
		t.Fatalf("expected last event timestamp to be %d, got %d", ev2.Timestamp, got.Timestamp)
	}
}

func TestAllReturnsCopy(t *testing.T) {
	tl := New()
	ev := contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test", Type: contracts.EventMainshockOccurred, Confidence: 1}
	tl.Append(ev)

	entries := tl.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Modify returned slice; original should be unaffected
	entries[0].Event.ID = "modified"
	if tl.Last().ID == "modified" {
		t.Fatalf("All() returned slice that shares storage with internal state")
	}
}

func TestSinceReturnsEventsAtOrAfterTimestamp(t *testing.T) {
	tl := New()
	events := []contracts.Event{
		{ID: "evt-1", Timestamp: 100, Source: "test"},
		{ID: "evt-2", Timestamp: 200, Source: "test"},
		{ID: "evt-3", Timestamp: 300, Source: "test"},
		{ID: "evt-4", Timestamp: 400, Source: "test"},
	}
	for _, ev := range events {
		tl.Append(ev)
	}

	got := tl.Since(250)
	if len(got) != 2 {
		t.Fatalf("expected 2 events since 250, got %d", len(got))
	}
	wantIDs := map[contracts.EventID]bool{"evt-3": true, "evt-4": true}
	for _, entry := range got {
		if !wantIDs[entry.Event.ID] {
			t.Fatalf("unexpected event in Since(250): %s", entry.Event.ID)
		}
	}
}

func TestSinceReturnsAllWhenTimestampIsZero(t *testing.T) {
	tl := New()
	ev := contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test"}
	tl.Append(ev)

	got := tl.Since(0)
	if len(got) != 1 {
		t.Fatalf("expected 1 event since 0, got %d", len(got))
	}
}

func TestUpToReturnsEventsAtOrBeforeTimestamp(t *testing.T) {
	tl := New()
	events := []contracts.Event{
		{ID: "evt-1", Timestamp: 100, Source: "test"},
		{ID: "evt-2", Timestamp: 200, Source: "test"},
		{ID: "evt-3", Timestamp: 300, Source: "test"},
		{ID: "evt-4", Timestamp: 400, Source: "test"},
	}
	for _, ev := range events {
		tl.Append(ev)
	}

	got := tl.UpTo(250)
	if len(got) != 2 {
		t.Fatalf("expected 2 events up to 250, got %d", len(got))
	}
	wantIDs := map[contracts.EventID]bool{"evt-1": true, "evt-2": true}
	for _, entry := range got {
		if !wantIDs[entry.Event.ID] {
			t.Fatalf("unexpected event in UpTo(250): %s", entry.Event.ID)
		}
	}
}

func TestUpToReturnsNoneWhenTimestampBeforeFirst(t *testing.T) {
	tl := New()
	ev := contracts.Event{ID: "evt-1", Timestamp: 100, Source: "test"}
	tl.Append(ev)

	got := tl.UpTo(50)
	if len(got) != 0 {
		t.Fatalf("expected 0 events up to 50, got %d", len(got))
	}
}

func TestConcurrentAppendAndQuery(t *testing.T) {
	tl := New()
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := range 100 {
			ev := contracts.Event{ID: "evt-", Timestamp: contracts.SimTime(i)}
			tl.Append(ev)
		}
		close(done)
	}()

	// Reader goroutine
	go func() {
		for i := range 100 {
			_ = tl.Len()
			_ = tl.All()
			_ = tl.Since(contracts.SimTime(i))
		}
	}()

	<-done
}

func TestAllPreservesOrder(t *testing.T) {
	tl := New()
	want := []contracts.Event{
		{ID: "evt-1", Timestamp: 100, Source: "test"},
		{ID: "evt-2", Timestamp: 200, Source: "test"},
		{ID: "evt-3", Timestamp: 300, Source: "test"},
	}
	for _, ev := range want {
		tl.Append(ev)
	}

	got := tl.All()
	if !reflect.DeepEqual(got, []Entry{
		{want[0]}, {want[1]}, {want[2]},
	}) {
		t.Fatalf("All() did not preserve order: got %+v", got)
	}
}

func TestPayloadIsClonedOnAppend(t *testing.T) {
	tl := New()
	// Create event with payload bytes
	origPayload := json.RawMessage([]byte(`{"bridgeId":"BR-12"}`))
	ev := contracts.Event{
		ID:        "evt-1",
		Timestamp: 100,
		Source:    "test",
		Type:      contracts.EventBridgeClosed,
		Payload:   origPayload,
	}
	tl.Append(ev)

	// Mutate the original payload
	origPayload[2] = 'X' // modify "bridgeId" -> "XridgeId"

	// Check stored event is unchanged
	saved := tl.Last()
	if string(saved.Payload) == string(origPayload) {
		t.Fatalf("Payload was not cloned; stored event was corrupted by caller mutation")
	}
	if string(saved.Payload) != `{"bridgeId":"BR-12"}` {
		t.Fatalf("Payload mutated unexpectedly: got %s", saved.Payload)
	}
}

func TestPayloadCloneIsNilSafe(t *testing.T) {
	tl := New()
	ev := contracts.Event{
		ID:        "evt-1",
		Timestamp: 100,
		Source:    "test",
		Type:      contracts.EventMainshockOccurred,
		Payload:   nil, // nil payload
	}
	tl.Append(ev)

	saved := tl.Last()
	if saved.Payload != nil {
		t.Fatalf("nil payload should remain nil after clone")
	}
}

func TestTruncate(t *testing.T) {
	tl := New()
	tl.Append(contracts.Event{ID: "e1", Timestamp: 100})
	tl.Append(contracts.Event{ID: "e2", Timestamp: 200})

	if tl.Len() != 2 {
		t.Fatalf("expected 2 entries before truncate, got %d", tl.Len())
	}

	tl.Truncate()

	if tl.Len() != 0 {
		t.Fatalf("expected 0 entries after truncate, got %d", tl.Len())
	}
	if tl.Last() != nil {
		t.Fatalf("expected Last() to be nil after truncate")
	}
}
