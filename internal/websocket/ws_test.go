package websocket

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nrynss/opsec-control/internal/contracts"
)

// mockBus for tests.
type mockBus struct {
	ch chan contracts.Event
}

func newMockBus() *mockBus {
	return &mockBus{ch: make(chan contracts.Event, 10)}
}

func (m *mockBus) Publish(ev contracts.Event) { m.ch <- ev }
func (m *mockBus) Subscribe() (<-chan contracts.Event, func()) {
	return m.ch, func() {}
}

func TestWSReceivesEvent(t *testing.T) {
	bus := newMockBus()
	srv := New(bus)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Dial
	dialer := websocket.DefaultDialer
	wsURL := "ws" + ts.URL[4:]
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Publish
	ev := contracts.Event{ID: "evt-1", Type: contracts.EventMainshockOccurred}
	bus.Publish(ev)

	// Read with timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) == "" {
		t.Error("expected some data")
	}
}

func TestBroadcastConcurrent(t *testing.T) {
	bus := newMockBus()
	s := New(bus)

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	wsURL := "ws" + ts.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Exercise concurrent Broadcast + publishes (run under -race to verify fix).
	done := make(chan struct{})
	go func() {
		for i := range 50 {
			s.Broadcast(map[string]int{"seq": i})
		}
		close(done)
	}()

	for range 50 {
		bus.Publish(contracts.Event{ID: "e", Type: contracts.EventMainshockOccurred})
	}

	<-done
	// Drain a couple messages to confirm no panic/crash.
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	for range 2 {
		_, _, _ = conn.ReadMessage()
	}
}
