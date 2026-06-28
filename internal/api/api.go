package api

import (
	"encoding/json"
	"net/http"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/timeline"
)

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
type Server struct {
	store contracts.StateStore
	bus   contracts.EventBus
	orch  contracts.Orchestrator
	log   EventLog
	cop   COPProvider
}

// New creates the API server.
func New(store contracts.StateStore, bus contracts.EventBus, orch contracts.Orchestrator, log EventLog, cop COPProvider) *Server {
	return &Server{
		store: store,
		bus:   bus,
		orch:  orch,
		log:   log,
		cop:   cop,
	}
}

// Register mounts the handlers on the given mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /state", s.handleState)
	mux.HandleFunc("GET /agents", s.handleAgents)
	mux.HandleFunc("GET /timeline", s.handleTimeline)
	mux.HandleFunc("GET /events", s.handleGetEvents)
	mux.HandleFunc("POST /events", s.handlePostEvent)
	mux.HandleFunc("POST /scenario/load", s.handleScenarioLoad)
	mux.HandleFunc("POST /scenario/reset", s.handleScenarioReset)
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
	s.bus.Publish(ev)
	w.WriteHeader(http.StatusAccepted)
}

// handleScenarioLoad placeholder (for MVD, may be no-op or forward).
func (s *Server) handleScenarioLoad(w http.ResponseWriter, r *http.Request) {
	// In full impl would use scenario pkg, but for now stub.
	w.WriteHeader(http.StatusNotImplemented)
}

// handleScenarioReset placeholder.
func (s *Server) handleScenarioReset(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
