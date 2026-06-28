package contracttest

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/rocknarayan/opsec-control/internal/contracts"
)

// roundJSON marshals v, unmarshals into a fresh value of the same type, and
// returns it — proving the canonical schema round-trips (SPEC §0.6).
func roundJSON[T any](t *testing.T, v T) T {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestEventRoundTrip(t *testing.T) {
	in := contracts.Event{
		ID:         "evt-10042",
		Timestamp:  300,
		Source:     "Gemma4-Perception",
		Type:       contracts.EventBridgeClosed,
		Confidence: 0.96,
		Payload:    json.RawMessage(`{"bridgeId":"BR-12","reason":"Structural failure"}`),
	}
	if got := roundJSON(t, in); !reflect.DeepEqual(got, in) {
		t.Fatalf("event mismatch:\n got %+v\nwant %+v", got, in)
	}
}

func TestWorldStateRoundTrip(t *testing.T) {
	in := contracts.WorldState{
		Version: 42,
		Time:    300,
		Sectors: map[contracts.SectorID]contracts.Sector{
			"highgate": {ID: "highgate", Name: "Highgate", Power: contracts.PowerOff,
				Comms: contracts.UtilityDegraded, Water: contracts.UtilityUp,
				Gas: contracts.UtilityDown, Population: 90000},
		},
		Bridges: map[contracts.BridgeID]contracts.Bridge{
			"vora": {ID: "vora", Name: "Vora Bridge", Status: contracts.BridgeClosed},
		},
		Dam:   contracts.Dam{ID: "mainor", Status: contracts.DamStressed, ReservoirPct: 0.82, StressRating: 0.6},
		Levee: contracts.Levee{ID: "southport", Status: contracts.LeveeIntact, Height: 4.5, Integrity: 1},
		Hospitals: map[contracts.HospitalID]contracts.Hospital{
			"central-general": {ID: "central-general", Name: "Central General", Sector: "central",
				Beds: 400, ICU: 40, ER: 60, Occupancy: 380, Band: contracts.HospitalCritical, OnGenerator: true},
		},
		Shelters: map[contracts.ShelterID]contracts.Shelter{
			"greenfield-1": {ID: "greenfield-1", Name: "Greenfield Arena", Sector: "greenfield", Capacity: 2000, Occupancy: 2000, Full: true},
		},
		FireZones: map[contracts.FireZoneID]contracts.FireZone{
			"ironworks-1": {ID: "ironworks-1", Sector: "ironworks", Status: contracts.FireStatusSpreading},
		},
		Flood: contracts.Flood{Polygons: []contracts.FloodPolygon{
			{Sector: "southport", DepthM: 1.2, Points: []contracts.Point{{X: 10, Y: 20}, {X: 12, Y: 22}}},
		}},
		Resources: map[contracts.ResourceID]contracts.Resource{
			"amb-pool": {ID: "amb-pool", Kind: contracts.ResourceAmbulance, HomeBase: "harborside", Count: 24, Deployed: 18},
		},
	}
	if got := roundJSON(t, in); !reflect.DeepEqual(got, in) {
		t.Fatalf("world state did not round-trip")
	}
}

func TestCOPRoundTrip(t *testing.T) {
	in := contracts.CommonOperationalPicture{
		Summary:      "Two bridges down; Westbank isolated.",
		StateVersion: 42,
		OverallRisk:  contracts.RiskCritical,
		PrioritizedActions: []contracts.PrioritizedAction{
			{Priority: 1, Action: "Airlift casualties from Westbank Clinic", Owner: contracts.CellMedical},
		},
		CellOutputs: []contracts.CellOutput{
			{Cell: contracts.CellInfrastructure, Summary: "Vora + Iron closed", RiskLevel: contracts.RiskHigh,
				Confidence: 0.91, StateVersion: 42, Recommendations: []string{"Reroute via South Span"}, Evidence: []string{"bridge feed"}},
		},
	}
	if got := roundJSON(t, in); !reflect.DeepEqual(got, in) {
		t.Fatalf("COP did not round-trip")
	}
}

func TestScenarioRoundTrip(t *testing.T) {
	in := contracts.Scenario{
		SchemaVersion: "0.1",
		Name:          "cerebro-cascade",
		Seed:          1729,
		Initial:       contracts.WorldState{Version: 0, Time: 0},
		Events: []contracts.Event{
			{ID: "evt-1", Timestamp: 0, Source: "sim", Type: contracts.EventMainshockOccurred, Confidence: 1},
		},
	}
	if got := roundJSON(t, in); !reflect.DeepEqual(got, in) {
		t.Fatalf("scenario did not round-trip")
	}
}

func TestRejectionErrorIsError(t *testing.T) {
	var err error = &contracts.RejectionError{EventID: "evt-9", Reason: contracts.RejectIllegalTransition, Detail: "bridge already closed"}
	var re *contracts.RejectionError
	if !errors.As(err, &re) {
		t.Fatalf("RejectionError should satisfy errors.As")
	}
	if re.Reason != contracts.RejectIllegalTransition {
		t.Fatalf("reason not preserved: %s", re.Reason)
	}
	if err.Error() == "" {
		t.Fatalf("Error() should be non-empty")
	}
}

// TestEventTypeUniqueness guards against accidental duplicate enum values when
// the taxonomy is extended via §0.5.
func TestEventTypeUniqueness(t *testing.T) {
	all := []contracts.EventType{
		contracts.EventMainshockOccurred, contracts.EventAftershockOccurred, contracts.EventAftershockForecastUpdated,
		contracts.EventBuildingCollapsed, contracts.EventBridgeDamaged, contracts.EventBridgeClosed,
		contracts.EventRoadBlocked, contracts.EventTunnelClosed, contracts.EventDamStressElevated, contracts.EventLeveeBreached,
		contracts.EventPowerFailure, contracts.EventGasLeakDetected, contracts.EventWaterMainBreak, contracts.EventCommsOutage,
		contracts.EventFireIgnited, contracts.EventFireSpread, contracts.EventFireContained,
		contracts.EventFloodExtentUpdated,
		contracts.EventCasualtyReportUpdated, contracts.EventMassCasualtyIncident, contracts.EventHospitalCapacityChanged,
		contracts.EventCitizenDistressCall, contracts.EventPersonsTrapped, contracts.EventEvacuationOrdered,
		contracts.EventShelterOccupancyChanged, contracts.EventShelterFull,
		contracts.EventSatelliteImageReceived, contracts.EventDroneImageReceived,
		contracts.EventResourceDeployed, contracts.EventResourceDepleted, contracts.EventRejected,
	}
	seen := make(map[contracts.EventType]bool, len(all))
	for _, e := range all {
		if e == "" {
			t.Fatalf("empty event type in enum")
		}
		if seen[e] {
			t.Fatalf("duplicate event type: %s", e)
		}
		seen[e] = true
	}
}
