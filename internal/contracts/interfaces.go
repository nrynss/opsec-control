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
// HUD (SPEC §15.1).
type LLMResponse struct {
	Content      string  `json:"content"`
	TokensIn     int     `json:"tokensIn"`
	TokensOut    int     `json:"tokensOut"`
	TokensPerSec float64 `json:"tokensPerSec"`
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
