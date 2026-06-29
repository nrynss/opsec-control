package contracts

import (
	"context"
	"encoding/json"
)

// interfaces.go — the interface seams every package depends on instead of each
// other's implementations (SPEC §0.2 rule 3). Change only via the §0.5 step.

// EventBus distributes events (pub/sub); it owns no state (SPEC §7).
type EventBus interface {
	Publish(Event)
	// Subscribe returns a receive channel and an unsubscribe func.
	Subscribe() (events <-chan Event, cancel func())
}

// StateStore is the sole owner of world state: a read snapshot plus the single
// mutator path (SPEC §8, §14.2).
type StateStore interface {
	Snapshot() WorldState
	Version() StateVersion
	// Apply runs the §14.2 validation contract and, if the event is accepted,
	// mutates state and returns the new version. A rejected event returns a
	// *RejectionError and does not change state.
	Apply(Event) (StateVersion, error)
}

// Cell is one specialist agent: a pure function of input → structured output
// (SPEC §9). Implementations must not mutate state, touch the bus, or call
// other Cells.
type Cell interface {
	Kind() CellKind
	Analyze(context.Context, CellInput) (CellOutput, error)
}

// Orchestrator runs the concurrent fan-out and Commander synthesis (SPEC §6).
// It is the only invoker of Cells and invokes them concurrently — never
// sequentially.
type Orchestrator interface {
	FanOut(ctx context.Context, snapshot WorldState, trigger Event, wake []CellKind) (CommonOperationalPicture, error)
}

// Classifier decides which specialist Cells should be woken for parallel
// analysis when an accepted event changes world state (§6). The returned
// slice contains only specialist CellKinds — the orchestrator unconditionally
// invokes the Commander as a phase-2 synthesis step after all specialists
// return.
type Classifier interface {
	Classify(snapshot WorldState, trigger Event) []CellKind
}

// LLMClient is the Cerebras/Gemma client; all provider-specific types stay
// behind it (SPEC §16.1).
type LLMClient interface {
	Complete(context.Context, LLMRequest) (LLMResponse, error)
}

// LLMRequest is a structured-output completion request.
type LLMRequest struct {
	System string `json:"system"`
	User   string `json:"user"`
	// Schema, if set, is the JSON schema the completion must satisfy.
	Schema json.RawMessage `json:"schema,omitempty"`
}

// LLMResponse carries the completion plus the throughput metrics shown on the
// HUD (SPEC §15.1). LatencyMS is the wall-clock for the call (real client: the
// HTTP round-trip; mock: the simulated inference time).
type LLMResponse struct {
	Content      string  `json:"content"`
	TokensIn     int     `json:"tokensIn"`
	TokensOut    int     `json:"tokensOut"`
	TokensPerSec float64 `json:"tokensPerSec"`
	LatencyMS    int64   `json:"latencyMs"`
}

// Perception turns a sensor image into structured events (SPEC §6, §14.4).
type Perception interface {
	Interpret(context.Context, ImageInput) ([]Event, error)
}

// ImageInput is a satellite or drone frame for the perception layer.
type ImageInput struct {
	Source string `json:"source"` // "satellite" | "drone"
	Data   []byte `json:"data"`
}

// --- P19: Simulation controls and stats (additive §0.5 contract change) ---

// SimulationStatus enumerates the possible states for simulation progress
// reporting (used in SimulationStats and future /scenario/stats responses).
type SimulationStatus string

const (
	SimStatusRunning  SimulationStatus = "running"
	SimStatusPaused   SimulationStatus = "paused"
	SimStatusComplete SimulationStatus = "complete"
)

// SimulationInfo provides scenario name and time bounds (start/end) for the
// UI simulation clock/progress bar. All times are SimTime (scenario seconds;
// see contracts/events.go for definition and determinism guarantees).
type SimulationInfo struct {
	Name      string  `json:"name"`
	StartTime SimTime `json:"startTime"`
	EndTime   SimTime `json:"endTime"`
}

// SimulationStats is returned by GET /scenario/stats. It drives the live
// metrics widget (wall elapsed, events replayed, tokens, inferences) and
// progress display. WallElapsed is milliseconds for UI display only (never
// used for logic or determinism). ElapsedTime provides explicit elapsed
// SimTime (CurrentTime - StartTime) for the metrics grid.
type SimulationStats struct {
	Status         SimulationStatus `json:"status"`
	CurrentTime    SimTime          `json:"currentTime"`
	ElapsedTime    SimTime          `json:"elapsedTime"`
	WallElapsed    int64            `json:"wallElapsed"` // ms (display only)
	EventsReplayed int              `json:"eventsReplayed"`
	TokensIn       int              `json:"tokensIn"`
	TokensOut      int              `json:"tokensOut"`
	Inferences     int              `json:"inferences"` // LLM calls / completions (via LLMClient)
	Speed          float64          `json:"speed"`
}

// SimulationController allows the API edge to invoke simulation controls
// (reset, playback) and query info without importing the simulation package.
// Implemented by the cmd/eoc integration root.
type SimulationController interface {
	Reset()
	Pause()
	Resume()
	Step() (bool, error)
	SetSpeed(float64)
	Info() SimulationInfo
	WallElapsedMS() int64
	Status() string
	CurrentTime() SimTime
}

// TokenStatsProvider provides aggregated LLM usage counters for stats
// without leaking llm package types or impl details.
type TokenStatsProvider interface {
	TotalTokens() (in, out int)
	TotalRequests() int
}
