package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/timeline"
)

// mockStore for testing.
type mockStore struct {
	ws contracts.WorldState
}

func (m *mockStore) Snapshot() contracts.WorldState                           { return m.ws }
func (m *mockStore) Version() contracts.StateVersion                          { return m.ws.Version }
func (m *mockStore) Apply(ev contracts.Event) (contracts.StateVersion, error) { return 0, nil }

// mockBus .
type mockBus struct{}

func (m *mockBus) Publish(ev contracts.Event) {}
func (m *mockBus) Subscribe() (<-chan contracts.Event, func()) {
	ch := make(chan contracts.Event)
	return ch, func() { close(ch) }
}

// mockOrch .
type mockOrch struct{}

func (m *mockOrch) FanOut(ctx context.Context, snapshot contracts.WorldState, trigger contracts.Event, wake []contracts.CellKind) (contracts.CommonOperationalPicture, error) {
	return contracts.CommonOperationalPicture{}, nil
}

func TestRegisterAndState(t *testing.T) {
	store := &mockStore{ws: contracts.WorldState{Version: 42}}
	bus := &mockBus{}
	orch := &mockOrch{}
	log := &mockLog{}

	srv := New(store, bus, orch, log, nil)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest("GET", "/state", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// mockLog implements api.EventLog for tests.
type mockLog struct{}

func (m *mockLog) All() []timeline.Entry                       { return nil }
func (m *mockLog) Since(ts contracts.SimTime) []timeline.Entry { return nil }
