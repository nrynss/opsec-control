package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/timeline"
)

// maxImageSize is the hard limit for perception image uploads (raw body or
// multipart file). 10 MiB as used in handlePostPerception.
const maxImageSize = 10 << 20

// COPProvider allows the API to serve the current CommonOperationalPicture
// without owning state. Typically provided by cmd/eoc which retains the last
// result from orchestrator.FanOut.
type COPProvider interface {
	Current() contracts.CommonOperationalPicture
}

// EventLog is a small read interface satisfied by *timeline.Timeline.
// api depends on this interface (not the concrete impl) per §0.2 r3.
type EventLog interface {
	All() []timeline.Entry
	Since(ts contracts.SimTime) []timeline.Entry
}

// ProviderSwitcher exposes read/set for the active LLM backend (P10).
// Uses string values "cerebras" | "openrouter". cmd/eoc adapts *llm.Client
// so that api does not import llm or leak provider types.
type ProviderSwitcher interface {
	Provider() string
	SetProvider(p string)
}

// Broadcaster is satisfied by *websocket.Server. Used to notify connected
// clients of provider switches over WS (P10).
type Broadcaster interface {
	Broadcast(msg any)
}

// toFlatEvents converts timeline entries to flat contract.Events for the wire
// (avoids nested {"Event": {...}} shape).
func toFlatEvents(entries []timeline.Entry) []contracts.Event {
	if entries == nil {
		return nil
	}
	flat := make([]contracts.Event, len(entries))
	for i, e := range entries {
		flat[i] = e.Event
	}
	return flat
}

// Server provides the HTTP API edge.
// It only serializes contract types and forwards to the bus. No logic, no state.
//
// The orchestrator is intentionally NOT a dependency: fan-out is driven by the
// reasoning loop in cmd/eoc, and the API only serves the latest COP via
// COPProvider. Keeping the orchestrator out of the edge enforces "no logic" here.
type Server struct {
	store      contracts.StateStore
	bus        contracts.EventBus
	log        EventLog
	cop        COPProvider
	perception contracts.Perception
	provider   ProviderSwitcher
	bcast      Broadcaster
}

// New creates the API server.
func New(store contracts.StateStore, bus contracts.EventBus, log EventLog, cop COPProvider, perception contracts.Perception, provider ProviderSwitcher, bcast Broadcaster) *Server {
	return &Server{
		store:      store,
		bus:        bus,
		log:        log,
		cop:        cop,
		perception: perception,
		provider:   provider,
		bcast:      bcast,
	}
}

// Register mounts the handlers on the given mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /state", s.handleState)
	mux.HandleFunc("GET /agents", s.handleAgents)
	mux.HandleFunc("GET /timeline", s.handleTimeline)
	mux.HandleFunc("GET /events", s.handleGetEvents)
	mux.HandleFunc("POST /events", s.handlePostEvent)
	mux.HandleFunc("POST /perception", s.handlePostPerception)
	mux.HandleFunc("GET /provider", s.handleGetProvider)
	mux.HandleFunc("POST /provider", s.handlePostProvider)
	mux.HandleFunc("POST /scenario/load", s.handleScenarioLoad)
	mux.HandleFunc("POST /scenario/reset", s.handleScenarioReset)
	mux.HandleFunc("POST /scenario/pause", s.handleScenarioPause)
	mux.HandleFunc("POST /scenario/resume", s.handleScenarioResume)
	mux.HandleFunc("POST /scenario/step", s.handleScenarioStep)
	mux.HandleFunc("POST /scenario/speed", s.handleScenarioSpeed)
}

// handleState returns the current world state.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	ws := s.store.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ws)
}

// handleAgents returns the current CommonOperationalPicture if available.
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cop != nil {
		json.NewEncoder(w).Encode(s.cop.Current())
		return
	}
	// Fallback for when no COP provider is wired yet (MVD)
	json.NewEncoder(w).Encode(map[string]string{"status": "agents endpoint - see orchestrator for COP"})
}

// handleTimeline serves the append-only log (for replay / dashboard timeline).
func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if s.log == nil {
		http.Error(w, "timeline not wired", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toFlatEvents(s.log.All()))
}

// handleGetEvents serves recent events (for now same as timeline; can be refined).
func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	if s.log == nil {
		http.Error(w, "event log not wired", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toFlatEvents(s.log.All()))
}

// handlePostEvent accepts an event and publishes to bus.
func (s *Server) handlePostEvent(w http.ResponseWriter, r *http.Request) {
	var ev contracts.Event
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "bad event", http.StatusBadRequest)
		return
	}
	// A manually-injected event (e.g. a dashboard preset trigger) may omit the
	// timestamp (0). It is stamped to the live world time at apply time by the
	// reasoning loop (cmd/eoc handle), which keeps it monotonic-valid regardless
	// of where the scenario replay has advanced to.
	s.bus.Publish(ev)
	w.WriteHeader(http.StatusAccepted)
}

// respondWithScenarioStub is a helper for the MVD scenario control endpoints.
// All scenario playback controls (load/reset/pause/resume/step/speed) are
// intentionally no-ops here; the real engine lives in cmd/eoc + simulation.
// Returns 202 Accepted + JSON so the frontend treats the calls as successful
// (callEndpoint returns true) while we wait for real wiring in P6.
func (s *Server) respondWithScenarioStub(w http.ResponseWriter, op string, extra ...map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	resp := map[string]any{
		"status": "accepted",
		"op":     op,
		"note":   "MVD stub - no-op until wired in cmd/eoc",
	}
	if len(extra) > 0 {
		for k, v := range extra[0] {
			resp[k] = v
		}
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleScenarioLoad(w http.ResponseWriter, r *http.Request) {
	// Best-effort parse name for future real impl
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	extra := map[string]any{}
	if body.Name != "" {
		extra["name"] = body.Name
	}
	s.respondWithScenarioStub(w, "load", extra)
}

func (s *Server) handleScenarioReset(w http.ResponseWriter, r *http.Request) {
	s.respondWithScenarioStub(w, "reset")
}

func (s *Server) handleScenarioPause(w http.ResponseWriter, r *http.Request) {
	s.respondWithScenarioStub(w, "pause")
}

func (s *Server) handleScenarioResume(w http.ResponseWriter, r *http.Request) {
	s.respondWithScenarioStub(w, "resume")
}

func (s *Server) handleScenarioStep(w http.ResponseWriter, r *http.Request) {
	s.respondWithScenarioStub(w, "step")
}

func (s *Server) handleScenarioSpeed(w http.ResponseWriter, r *http.Request) {
	// Best-effort parse speed
	var body struct {
		Speed float64 `json:"speed"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	extra := map[string]any{}
	if body.Speed != 0 {
		extra["speed"] = body.Speed
	}
	s.respondWithScenarioStub(w, "speed", extra)
}

// handlePostPerception accepts a satellite/drone image via raw request body
// (application/octet-stream or image/*) or multipart/form-data (field "image"
// or "file"), with source via ?source= or form field ("drone" | "satellite").
// It delegates to the injected Perception (per P5), stamps a current sim
// timestamp (so events pass the §14.2 temporal gate), and publishes the
// resulting events onto the bus (triggering anomaly → orchestrator fan-out).
func (s *Server) handlePostPerception(w http.ResponseWriter, r *http.Request) {
	if s.perception == nil {
		http.Error(w, "perception not wired", http.StatusServiceUnavailable)
		return
	}

	ct := r.Header.Get("Content-Type")
	source := r.URL.Query().Get("source")

	var data []byte
	var readErr error

	if strings.HasPrefix(ct, "multipart/") {
		if perr := r.ParseMultipartForm(maxImageSize); perr != nil {
			http.Error(w, "bad multipart form: "+perr.Error(), http.StatusBadRequest)
			return
		}
		if source == "" {
			source = r.FormValue("source")
		}
		file, _, ferr := r.FormFile("image")
		if ferr != nil {
			file, _, ferr = r.FormFile("file")
		}
		if ferr != nil {
			http.Error(w, "missing image file (use form field 'image' or 'file')", http.StatusBadRequest)
			return
		}
		defer file.Close()
		lr := io.LimitReader(file, maxImageSize+1)
		data, readErr = io.ReadAll(lr)
		if readErr == nil && len(data) > maxImageSize {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
	} else {
		// raw bytes body
		// Read one byte past the limit so we can distinguish truncation.
		// Per review observation: avoid silent truncation; return 413 instead.
		lr := io.LimitReader(r.Body, maxImageSize+1)
		data, readErr = io.ReadAll(lr)
		if readErr == nil && len(data) > maxImageSize {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
	}

	if readErr != nil {
		http.Error(w, "read image data: "+readErr.Error(), http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "empty image data", http.StatusBadRequest)
		return
	}

	if source == "" {
		source = "drone"
	}
	if source != "drone" && source != "satellite" {
		http.Error(w, `source must be "drone" or "satellite"`, http.StatusBadRequest)
		return
	}

	input := contracts.ImageInput{
		Source: source,
		Data:   data,
	}

	events, perr := s.perception.Interpret(r.Context(), input)
	if perr != nil {
		http.Error(w, "perception failed: "+perr.Error(), http.StatusInternalServerError)
		return
	}

	// Stamp timestamp from current snapshot so Apply will accept it (perception
	// impls conventionally use 0 as "fill me in"). Use >= current time.
	snap := s.store.Snapshot()
	for i := range events {
		if events[i].Timestamp == 0 {
			events[i].Timestamp = snap.Time
		}
		s.bus.Publish(events[i])
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accepted": len(events),
		"events":   events,
	})
}

// handleGetProvider reports the current LLM provider (P10).
// Returns 200 on success or 503 if no ProviderSwitcher wired.
func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	if s.provider == nil {
		http.Error(w, "provider switch not wired", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"provider": s.provider.Provider(),
	})
}

// handlePostProvider switches the active LLM provider (P10).
// Accepts {"provider": "cerebras" | "openrouter"}.
// Broadcasts {"kind":"provider","payload":{"provider":"..."}} when bcast wired.
// Returns 503/400/202 as appropriate.
func (s *Server) handlePostProvider(w http.ResponseWriter, r *http.Request) {
	if s.provider == nil {
		http.Error(w, "provider switch not wired", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Provider == "" {
		http.Error(w, `bad request: expected {"provider":"cerebras"|"openrouter"}`, http.StatusBadRequest)
		return
	}
	p := body.Provider
	if p != "cerebras" && p != "openrouter" {
		http.Error(w, `provider must be "cerebras" or "openrouter"`, http.StatusBadRequest)
		return
	}
	s.provider.SetProvider(p)
	if s.bcast != nil {
		s.bcast.Broadcast(map[string]any{
			"kind":    "provider",
			"payload": map[string]any{"provider": p},
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":   "accepted",
		"provider": p,
	})
}
