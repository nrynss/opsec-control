package api

import (
	"bytes"
	"context"
	"mime/multipart"
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

// mockBus records published events for assertions in tests.
type mockBus struct {
	published []contracts.Event
}

func (m *mockBus) Publish(ev contracts.Event) { m.published = append(m.published, ev) }
func (m *mockBus) Subscribe() (<-chan contracts.Event, func()) {
	ch := make(chan contracts.Event)
	return ch, func() { close(ch) }
}

func TestRegisterAndState(t *testing.T) {
	store := &mockStore{ws: contracts.WorldState{Version: 42}}
	bus := &mockBus{}
	log := &mockLog{}

	srv := New(store, bus, log, nil, nil)
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

// mockPerception implements contracts.Perception for api tests.
type mockPerception struct {
	events []contracts.Event
	err    error
}

func (m *mockPerception) Interpret(ctx context.Context, input contracts.ImageInput) ([]contracts.Event, error) {
	return m.events, m.err
}

func TestPerceptionNil(t *testing.T) {
	store := &mockStore{ws: contracts.WorldState{Version: 1, Time: 42}}
	bus := &mockBus{}
	log := &mockLog{}

	srv := New(store, bus, log, nil, nil)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest("POST", "/perception", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil perception: expected 503, got %d", w.Code)
	}
}

func TestPerceptionRawAndPublish(t *testing.T) {
	store := &mockStore{ws: contracts.WorldState{Version: 5, Time: 123}}
	bus := &mockBus{}
	log := &mockLog{}
	mp := &mockPerception{
		events: []contracts.Event{
			{ID: "p1", Timestamp: 0, Source: "test", Type: contracts.EventLeveeBreached, Confidence: 0.95, Payload: []byte(`{"sector":"southport"}`)},
			{ID: "p2", Timestamp: 0, Source: "test", Type: contracts.EventRoadBlocked, Confidence: 0.9},
		},
	}

	srv := New(store, bus, log, nil, mp)
	mux := http.NewServeMux()
	srv.Register(mux)

	body := []byte("fake-image-bytes-containing-southport-or-bridge")
	req := httptest.NewRequest("POST", "/perception?source=drone", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("raw: expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	if len(bus.published) != 2 {
		t.Fatalf("expected 2 events published, got %d", len(bus.published))
	}
	// timestamp was stamped
	if bus.published[0].Timestamp != 123 {
		t.Errorf("expected stamped ts=123, got %d", bus.published[0].Timestamp)
	}
}

func TestPerceptionMultipart(t *testing.T) {
	store := &mockStore{ws: contracts.WorldState{Time: 200}}
	bus := &mockBus{}
	log := &mockLog{}
	mp := &mockPerception{events: []contracts.Event{{ID: "mp1", Type: contracts.EventBuildingCollapsed, Confidence: 0.88}}}

	srv := New(store, bus, log, nil, mp)
	mux := http.NewServeMux()
	srv.Register(mux)

	// build multipart body
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("source", "satellite")
	part, _ := mw.CreateFormFile("image", "sat.png")
	part.Write([]byte("png-bytes-here"))
	mw.Close()

	req := httptest.NewRequest("POST", "/perception", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("multipart: expected 202, got %d", w.Code)
	}
	if len(bus.published) != 1 {
		t.Fatalf("multipart: expected 1 published, got %d", len(bus.published))
	}
}

func TestPerceptionBadSourceAndError(t *testing.T) {
	store := &mockStore{}
	bus := &mockBus{}
	log := &mockLog{}
	mp := &mockPerception{err: context.DeadlineExceeded} // simulate failure

	srv := New(store, bus, log, nil, mp)
	mux := http.NewServeMux()
	srv.Register(mux)

	// bad source
	req := httptest.NewRequest("POST", "/perception?source=foo", bytes.NewReader([]byte("data")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad source: expected 400, got %d", w.Code)
	}

	// empty body
	req = httptest.NewRequest("POST", "/perception", bytes.NewReader([]byte{}))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty: expected 400, got %d", w.Code)
	}

	// perception err
	req = httptest.NewRequest("POST", "/perception", bytes.NewReader([]byte("data")))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("perception err: expected 500, got %d", w.Code)
	}
}

func TestPerceptionPayloadTooLarge(t *testing.T) {
	store := &mockStore{}
	bus := &mockBus{}
	log := &mockLog{}
	mp := &mockPerception{events: []contracts.Event{{ID: "big", Type: contracts.EventRoadBlocked, Confidence: 0.5}}}

	srv := New(store, bus, log, nil, mp)
	mux := http.NewServeMux()
	srv.Register(mux)

	// Raw body larger than maxImageSize — should return 413 instead of silent truncate
	oversized := make([]byte, maxImageSize+100)
	req := httptest.NewRequest("POST", "/perception", bytes.NewReader(oversized))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized raw: expected 413 Payload Too Large, got %d", w.Code)
	}
	if len(bus.published) != 0 {
		t.Errorf("oversized should not have published any events")
	}
}
