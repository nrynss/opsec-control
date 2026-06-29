package contracts

// agentio.go — the Cell input envelope and the per-Cell structured output
// schema (SPEC §9). Change only via the §0.5 coordinated step.

// CellKind identifies one of the six specialist cells (SPEC §9).
type CellKind string

const (
	CellIntelligence   CellKind = "Intelligence"
	CellInfrastructure CellKind = "Infrastructure"
	CellMedical        CellKind = "Medical"
	CellPopulation     CellKind = "Population"
	CellCommunications CellKind = "Communications"
	CellCommander      CellKind = "Commander"
)

// CellInput is the envelope the orchestrator hands a Cell on fan-out. A Cell is
// a pure function of (Snapshot + Trigger) → CellOutput and receives nothing else
// (SPEC §9). Peers carries other cells' outputs and is populated only for the
// Commander; specialist cells never see each other's output and never call each
// other.
type CellInput struct {
	Snapshot     WorldState   `json:"snapshot"`
	Trigger      Event        `json:"trigger"`
	StateVersion StateVersion `json:"stateVersion"`
	Peers        []CellOutput `json:"peers,omitempty"`
}

// RiskLevel is the coarse severity a Cell assigns.
type RiskLevel string

const (
	RiskLow      RiskLevel = "Low"
	RiskMedium   RiskLevel = "Medium"
	RiskHigh     RiskLevel = "High"
	RiskCritical RiskLevel = "Critical"
)

// CellMetrics carries the Cerebras throughput telemetry for one Cell call, shown
// on the HUD (SPEC §15.1). It is computed by the LLM client from the response
// (NOT part of the model's structured output schema) and stamped onto the
// CellOutput by the agent. For a multi-turn (plan→critique) cell, the agent
// aggregates across turns (summed tokens/latency).
type CellMetrics struct {
	TokensIn     int     `json:"tokensIn"`
	TokensOut    int     `json:"tokensOut"`
	TokensPerSec float64 `json:"tokensPerSec"`
	LatencyMS    int64   `json:"latencyMs"`
}

// CellOutput is the structured result every Cell emits — never free-form prose
// (SPEC §9). Each Cell records the StateVersion it analyzed (SPEC §8).
type CellOutput struct {
	Cell            CellKind     `json:"agent"`
	Summary         string       `json:"summary"`
	RiskLevel       RiskLevel    `json:"riskLevel"`
	Confidence      float64      `json:"confidence"` // ∈ [0,1]
	StateVersion    StateVersion `json:"stateVersion"`
	Recommendations []string     `json:"recommendations"`
	Evidence        []string     `json:"evidence"`
	Metrics         CellMetrics  `json:"metrics"`
}

// COPMetrics aggregates the throughput telemetry across a whole fan-out, shown on
// the HUD (SPEC §15.1). FanOutLatencyMS is the orchestrator's wall-clock for the
// entire fan-out (specialists + Commander); the token totals and peak rate are
// summed/maxed across all cell calls. AggregateTokensPerSec is total output tokens
// over the fan-out wall time — the headline "wafer-scale" number.
type COPMetrics struct {
	FanOutLatencyMS       int64   `json:"fanOutLatencyMs"`
	TotalTokensIn         int     `json:"totalTokensIn"`
	TotalTokensOut        int     `json:"totalTokensOut"`
	PeakTokensPerSec      float64 `json:"peakTokensPerSec"`
	AggregateTokensPerSec float64 `json:"aggregateTokensPerSec"`
	CellCount             int     `json:"cellCount"`
}

// CommonOperationalPicture is the Commander's synthesis over the whole world
// state plus all specialist outputs (SPEC §6, §9).
type CommonOperationalPicture struct {
	Summary            string              `json:"summary"`
	StateVersion       StateVersion        `json:"stateVersion"`
	OverallRisk        RiskLevel           `json:"overallRisk"`
	PrioritizedActions []PrioritizedAction `json:"prioritizedActions"`
	CellOutputs        []CellOutput        `json:"cellOutputs"`
	Metrics            COPMetrics          `json:"metrics"`
}

// PrioritizedAction is one executive recommendation; lower Priority is more
// urgent (1 = top).
type PrioritizedAction struct {
	Priority int      `json:"priority"`
	Action   string   `json:"action"`
	Owner    CellKind `json:"owner,omitempty"`
}
