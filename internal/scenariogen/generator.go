package scenariogen

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/state"
)

// Generator handles the compilation of a scenario.
type Generator struct {
	llm contracts.LLMClient
}

func NewGenerator(llm contracts.LLMClient) *Generator {
	return &Generator{llm: llm}
}

// Compile produces a validated Scenario based on the 3-act cascade (SPEC §8.5).
func (g *Generator) Compile(ctx context.Context, seed int64) (*contracts.Scenario, error) {
	initial := g.createSubstrate()

	// The gatekeeper: replay every candidate against a throwaway store built from
	// a SEPARATE substrate copy. state.New shares the map references of the
	// WorldState it's given, so validating against `initial` would mutate the very
	// substrate we freeze as Scenario.Initial — corrupting t=0 into the post-cascade
	// end state and making the artifact un-replayable.
	st := state.New(g.createSubstrate())

	var finalEvents []contracts.Event

	// Acts 1-3
	acts := []struct {
		name   string
		prompt string
	}{
		{"Act 1: Mainshock", "M6.8 earthquake. Highgate collapses. Vora and Iron bridges closed. Power off in Highgate/Central. Casualty surge at Central General."},
		{"Act 2: Aftershock & Fire", "M5.9 aftershock. South Span closes (isolating Westside/Southport). Ironworks gas main fire. Westside Clinic overwhelmed. Dam stress rises."},
		{"Act 3: Levee Breach", "Mainor dam release. Southport levee breaches. Flooding spreads. Shelter filling in Greenfield."},
	}

	currentSimTime := contracts.SimTime(0)
	eventIDCounter := 1

	for _, act := range acts {
		candidates, err := g.DraftEvents(ctx, act.prompt, currentSimTime)
		if err != nil {
			return nil, fmt.Errorf("failed to draft %s: %w", act.name, err)
		}

		for _, ev := range candidates {
			// Fix metadata for validation
			ev.ID = contracts.EventID(fmt.Sprintf("evt-%d", eventIDCounter))
			eventIDCounter++

			// Ensure monotonic time (simulated spacing)
			currentSimTime += 30 // 30s between beats
			ev.Timestamp = currentSimTime

			// Validation gate (§14.2)
			if _, err := st.Apply(ev); err != nil {
				log.Printf("[scenariogen] dropping invalid event %s (%s): %v", ev.ID, ev.Type, err)
				continue
			}
			finalEvents = append(finalEvents, ev)
		}
	}

	// Ensure sorted by timestamp (though we added them monotonically)
	sort.Slice(finalEvents, func(i, j int) bool {
		return finalEvents[i].Timestamp < finalEvents[j].Timestamp
	})

	return &contracts.Scenario{
		SchemaVersion: "1.0.0",
		Name:          "Cerebro Earthquake Cascade",
		Seed:          seed,
		Initial:       initial,
		Events:        finalEvents,
	}, nil
}

func (g *Generator) DraftEvents(ctx context.Context, prompt string, startTime contracts.SimTime) ([]contracts.Event, error) {
	// We ask the LLM for a list of "beats" (event type + payload).
	// In a real implementation, we'd provide a JSON schema.
	req := contracts.LLMRequest{
		System: "You are a disaster scenario author. Output a JSON array of events for an EOC simulation. Each event must have 'type', 'confidence' (0-1), and 'payload'.",
		User:   fmt.Sprintf("Draft 5-10 key event beats for the following phase: %s. Start time: %d", prompt, startTime),
	}

	resp, err := g.llm.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	var rawEvents []struct {
		Type       contracts.EventType `json:"type"`
		Confidence float64             `json:"confidence"`
		Payload    json.RawMessage     `json:"payload"`
	}

	if err := json.Unmarshal([]byte(resp.Content), &rawEvents); err != nil {
		// If LLM fails to provide clean JSON, we log and return empty instead of crashing.
		log.Printf("[scenariogen] LLM output not valid JSON: %v", err)
		return nil, nil
	}

	events := make([]contracts.Event, len(rawEvents))
	for i, re := range rawEvents {
		events[i] = contracts.Event{
			Source:     "Gemma4-ScenarioGen",
			Type:       re.Type,
			Confidence: re.Confidence,
			Payload:    re.Payload,
		}
	}

	return events, nil
}

func (g *Generator) createSubstrate() contracts.WorldState {
	// Fixed, hand-coded substrate based on SPEC §8.2-8.3.
	return contracts.WorldState{
		Version: 0,
		Time:    0,
		Sectors: map[contracts.SectorID]contracts.Sector{
			"S-HIGHGATE":   {ID: "S-HIGHGATE", Name: "Highgate", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 150000},
			"S-CENTRAL":    {ID: "S-CENTRAL", Name: "Central", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 300000},
			"S-IRONWORKS":  {ID: "S-IRONWORKS", Name: "Ironworks", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 100000},
			"S-HARBORSIDE": {ID: "S-HARBORSIDE", Name: "Harborside", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 120000},
			"S-WESTBANK":   {ID: "S-WESTBANK", Name: "Westside", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 200000},
			"S-SOUTHPORT":  {ID: "S-SOUTHPORT", Name: "Southport", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 180000},
			"S-GREENFIELD": {ID: "S-GREENFIELD", Name: "Greenfield", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 100000},
			"S-MAINOR":     {ID: "S-MAINOR", Name: "Mainor Heights", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp, Population: 50000},
		},
		Bridges: map[contracts.BridgeID]contracts.Bridge{
			"B-VORA":  {ID: "B-VORA", Name: "Vora Bridge", Status: contracts.BridgeOpen},
			"B-IRON":  {ID: "B-IRON", Name: "Iron Bridge", Status: contracts.BridgeOpen},
			"B-SOUTH": {ID: "B-SOUTH", Name: "South Span", Status: contracts.BridgeOpen},
		},
		Dam: contracts.Dam{
			ID: "D-MAINOR", Status: contracts.DamNormal, ReservoirPct: 0.75, StressRating: 0.1,
		},
		Levee: contracts.Levee{
			ID: "L-SOUTHPORT", Status: contracts.LeveeIntact, Height: 5.0, Integrity: 0.9,
		},
		Hospitals: map[contracts.HospitalID]contracts.Hospital{
			"H-CENTRAL":  {ID: "H-CENTRAL", Name: "Central General", Sector: "S-CENTRAL", Beds: 500, ICU: 50, ER: 100, Occupancy: 300, Band: contracts.HospitalNormal, OnGenerator: false},
			"H-WESTBANK": {ID: "H-WESTBANK", Name: "Westside Clinic", Sector: "S-WESTBANK", Beds: 100, ICU: 10, ER: 20, Occupancy: 60, Band: contracts.HospitalNormal, OnGenerator: false},
		},
		Shelters: map[contracts.ShelterID]contracts.Shelter{
			"SH-GREENFIELD-1": {ID: "SH-GREENFIELD-1", Name: "Greenfield Uni Shelter", Sector: "S-GREENFIELD", Capacity: 2000, Occupancy: 0, Full: false},
			"SH-GREENFIELD-2": {ID: "SH-GREENFIELD-2", Name: "Greenfield Civic Center", Sector: "S-GREENFIELD", Capacity: 1500, Occupancy: 0, Full: false},
		},
		FireZones: map[contracts.FireZoneID]contracts.FireZone{},
		Flood: contracts.Flood{
			Polygons: []contracts.FloodPolygon{},
		},
		Resources: map[contracts.ResourceID]contracts.Resource{
			"R-AMB-1":  {ID: "R-AMB-1", Kind: contracts.ResourceAmbulance, HomeBase: "S-CENTRAL", Count: 50, Deployed: 0},
			"R-FIRE-1": {ID: "R-FIRE-1", Kind: contracts.ResourceFireEngine, HomeBase: "S-HARBORSIDE", Count: 30, Deployed: 0},
			"R-USAR-1": {ID: "R-USAR-1", Kind: contracts.ResourceUSARTeam, HomeBase: "S-HARBORSIDE", Count: 10, Deployed: 0},
			"R-HELI-1": {ID: "R-HELI-1", Kind: contracts.ResourceHelicopter, HomeBase: "S-CENTRAL", Count: 5, Deployed: 0},
		},
	}
}
