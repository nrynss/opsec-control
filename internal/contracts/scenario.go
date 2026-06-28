package contracts

// scenario.go — the deterministic scenario file format the offline compiler
// emits and the simulation engine replays (SPEC §14, §11). Change only via the
// §0.5 coordinated step.

// Scenario is a versioned, replayable artifact: the static substrate at t=0
// (SPEC §8.3, expressed as the initial WorldState) plus an ordered, validated
// event stream. Same file → same run every time (Principle 7).
type Scenario struct {
	SchemaVersion string `json:"schemaVersion"`
	Name          string `json:"name"`
	// Seed seeds all RNG so replay is deterministic (§0.2 rule 5).
	Seed int64 `json:"seed"`
	// Initial is the static substrate at t=0 (sectors, bridges, dam, levee,
	// hospitals, shelters, resources).
	Initial WorldState `json:"initial"`
	// Events is the stream, ordered by ascending SimTime (temporal monotonicity,
	// §14.2). The simulation engine replays it onto the bus.
	Events []Event `json:"events"`
}
