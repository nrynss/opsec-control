package scenario

import (
	"encoding/json"
	"fmt"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// LoadJSON parses a scenario from JSON bytes.
// It performs minimal structural checks (events are sorted by non-decreasing
// SimTime). Full §14.2 validation (referential integrity, legal transitions,
// etc.) is the responsibility of internal/validation and must be done before
// a scenario is considered safe to replay.
func LoadJSON(data []byte) (*contracts.Scenario, error) {
	var sc contracts.Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("unmarshal scenario: %w", err)
	}
	if err := validateBasic(&sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

func validateBasic(sc *contracts.Scenario) error {
	if sc.SchemaVersion == "" {
		return fmt.Errorf("scenario missing schemaVersion")
	}
	if len(sc.Events) == 0 {
		return nil
	}
	prev := sc.Events[0].Timestamp
	for i := 1; i < len(sc.Events); i++ {
		t := sc.Events[i].Timestamp
		if t < prev {
			return fmt.Errorf("events not sorted by ascending timestamp at index %d", i)
		}
		prev = t
	}
	return nil
}
