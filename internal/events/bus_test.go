package events

import (
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func TestBusPublishesToAllCurrentSubscribers(t *testing.T) {
	bus := New(4)
	first, cancelFirst := bus.Subscribe()
	defer cancelFirst()
	second, cancelSecond := bus.Subscribe()
	defer cancelSecond()

	event := contracts.Event{ID: "evt-1", Timestamp: 10, Source: "test", Type: contracts.EventMainshockOccurred, Confidence: 1}
	bus.Publish(event)

	assertReceive(t, first, event)
	assertReceive(t, second, event)
}

func TestSubscriberReceivesEventsInPublishOrder(t *testing.T) {
	bus := New(4)
	events, cancel := bus.Subscribe()
	defer cancel()

	want := []contracts.Event{
		{ID: "evt-1", Timestamp: 1, Source: "test", Type: contracts.EventBridgeDamaged, Confidence: 1},
		{ID: "evt-2", Timestamp: 2, Source: "test", Type: contracts.EventBridgeClosed, Confidence: 1},
		{ID: "evt-3", Timestamp: 3, Source: "test", Type: contracts.EventPowerFailure, Confidence: 1},
	}
	for _, event := range want {
		bus.Publish(event)
	}
	for _, event := range want {
		assertReceive(t, events, event)
	}
}

func TestSlowSubscriberDoesNotBlockOthers(t *testing.T) {
	bus := New(1)
	_, cancelSlow := bus.Subscribe()
	defer cancelSlow()

	// Fill the slow subscriber's output buffer and leave it unread.
	bus.Publish(contracts.Event{ID: "evt-fill", Timestamp: 1, Source: "test", Type: contracts.EventMainshockOccurred, Confidence: 1})

	fast, cancelFast := bus.Subscribe()
	defer cancelFast()

	wantFast := contracts.Event{ID: "evt-fast", Timestamp: 2, Source: "test", Type: contracts.EventPowerFailure, Confidence: 1}
	done := make(chan struct{})
	go func() {
		bus.Publish(wantFast)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("publish blocked behind a slow subscriber")
	}

	assertReceive(t, fast, wantFast)
}

func TestCancelStopsDeliveryAndClosesChannel(t *testing.T) {
	bus := New(1)
	events, cancel := bus.Subscribe()
	cancel()
	cancel()

	select {
	case _, ok := <-events:
		if ok {
			t.Fatalf("subscriber channel should be closed after cancel")
		}
	case <-time.After(time.Second):
		t.Fatalf("subscriber channel was not closed")
	}

	bus.Publish(contracts.Event{ID: "evt-ignored", Timestamp: 1, Source: "test", Type: contracts.EventCommsOutage, Confidence: 1})
}

func TestNewSubscriberOnlyReceivesFutureEvents(t *testing.T) {
	bus := New(2)
	bus.Publish(contracts.Event{ID: "evt-before", Timestamp: 1, Source: "test", Type: contracts.EventMainshockOccurred, Confidence: 1})

	events, cancel := bus.Subscribe()
	defer cancel()

	want := contracts.Event{ID: "evt-after", Timestamp: 2, Source: "test", Type: contracts.EventAftershockOccurred, Confidence: 1}
	bus.Publish(want)
	assertReceive(t, events, want)
}

func assertReceive(t *testing.T, events <-chan contracts.Event, want contracts.Event) {
	t.Helper()
	select {
	case got, ok := <-events:
		if !ok {
			t.Fatalf("subscriber channel closed before receiving %s", want.ID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("event mismatch: got %+v want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", want.ID)
	}
}
