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
}

// CommonOperationalPicture is the Commander's synthesis over the whole world
// state plus all specialist outputs (SPEC §6, §9).
type CommonOperationalPicture struct {
	Summary            string              `json:"summary"`
	StateVersion       StateVersion        `json:"stateVersion"`
	OverallRisk        RiskLevel           `json:"overallRisk"`
	PrioritizedActions []PrioritizedAction `json:"prioritizedActions"`
	CellOutputs        []CellOutput        `json:"cellOutputs"`
}

// PrioritizedAction is one executive recommendation; lower Priority is more
// urgent (1 = top).
type PrioritizedAction struct {
	Priority int      `json:"priority"`
	Action   string   `json:"action"`
	Owner    CellKind `json:"owner,omitempty"`
}
