package anomaly

import (
	"slices"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func makeBaseWorld() contracts.WorldState {
	return contracts.WorldState{
		Version: 42,
		Time:    300,
		Sectors: map[contracts.SectorID]contracts.Sector{
			"central": {ID: "central", Power: contracts.PowerOn, Comms: contracts.UtilityUp, Water: contracts.UtilityUp, Gas: contracts.UtilityUp},
		},
		Bridges: map[contracts.BridgeID]contracts.Bridge{
			"vora": {ID: "vora", Status: contracts.BridgeOpen},
		},
		Dam:   contracts.Dam{ID: "mainor", Status: contracts.DamNormal, ReservoirPct: 0.8, StressRating: 0.2},
		Levee: contracts.Levee{ID: "south", Status: contracts.LeveeIntact, Height: 5.0, Integrity: 1.0},
		Hospitals: map[contracts.HospitalID]contracts.Hospital{
			"central-h": {ID: "central-h", Beds: 400, Occupancy: 200, Band: contracts.HospitalNormal},
		},
		Shelters: map[contracts.ShelterID]contracts.Shelter{
			"green": {ID: "green", Capacity: 1000, Occupancy: 300, Full: false},
		},
		FireZones: map[contracts.FireZoneID]contracts.FireZone{},
		Flood:     contracts.Flood{},
		Resources: map[contracts.ResourceID]contracts.Resource{},
	}
}

func contains(cells []contracts.CellKind, want contracts.CellKind) bool {
	return slices.Contains(cells, want)
}

func TestClassify_Seismic(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ev := contracts.Event{Type: contracts.EventMainshockOccurred, Confidence: 0.99}

	got := d.Classify(ws, ev)
	if len(got) < 4 {
		t.Errorf("mainshock should wake many cells, got %d: %v", len(got), got)
	}
	if !contains(got, contracts.CellIntelligence) {
		t.Error("mainshock should wake Intelligence at minimum")
	}
}

func TestClassify_BridgeClosed(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.Bridges["vora"] = contracts.Bridge{ID: "vora", Status: contracts.BridgeClosed}
	ev := contracts.Event{Type: contracts.EventBridgeClosed, Confidence: 0.95}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellInfrastructure) {
		t.Error("bridge closed should wake Infrastructure")
	}
	if !contains(got, contracts.CellIntelligence) {
		t.Error("bridge closed should wake Intelligence")
	}
}

func TestClassify_HospitalCritical(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.Hospitals["central-h"] = contracts.Hospital{
		ID: "central-h", Beds: 400, Occupancy: 380, Band: contracts.HospitalCritical,
	}
	ev := contracts.Event{Type: contracts.EventHospitalCapacityChanged, Confidence: 0.9}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellMedical) {
		t.Error("critical hospital should wake Medical")
	}
}

func TestClassify_ShelterFull(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.Shelters["green"] = contracts.Shelter{ID: "green", Capacity: 1000, Occupancy: 1000, Full: true}
	ev := contracts.Event{Type: contracts.EventShelterFull, Confidence: 1.0}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellPopulation) {
		t.Error("shelter full should wake Population")
	}
}

func TestClassify_FireSpreading(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.FireZones["f1"] = contracts.FireZone{ID: "f1", Status: contracts.FireStatusSpreading}
	ev := contracts.Event{Type: contracts.EventFireSpread, Confidence: 0.8}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellInfrastructure) || !contains(got, contracts.CellPopulation) {
		t.Error("spreading fire should wake Infrastructure and Population")
	}
}

func TestClassify_Flood(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.Flood = contracts.Flood{Polygons: []contracts.FloodPolygon{{Sector: "south", DepthM: 1.5}}}
	ev := contracts.Event{Type: contracts.EventFloodExtentUpdated, Confidence: 0.85}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellIntelligence) || !contains(got, contracts.CellPopulation) {
		t.Error("flood should wake Intelligence and Population")
	}
}

func TestClassify_PowerFailure(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.Sectors["central"] = contracts.Sector{ID: "central", Power: contracts.PowerOff}
	ev := contracts.Event{Type: contracts.EventPowerFailure, Confidence: 1.0}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellInfrastructure) {
		t.Error("power failure should wake Infrastructure")
	}
}

func TestClassify_DamStressed(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.Dam.Status = contracts.DamStressed
	ev := contracts.Event{Type: contracts.EventDamStressElevated, Confidence: 0.9}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellInfrastructure) {
		t.Error("dam stress should wake Infrastructure")
	}
}

func TestClassify_BridgeCollapsed(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ws.Bridges["vora"] = contracts.Bridge{ID: "vora", Status: contracts.BridgeCollapsed}
	ev := contracts.Event{Type: contracts.EventBridgeCollapsed, Confidence: 0.95}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellInfrastructure) {
		t.Error("bridge collapsed should wake Infrastructure")
	}
	if !contains(got, contracts.CellIntelligence) {
		t.Error("bridge collapsed should wake Intelligence")
	}
}

func TestClassify_PowerDegraded(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ev := contracts.Event{Type: contracts.EventPowerDegraded, Confidence: 0.9}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellInfrastructure) {
		t.Error("power degraded should wake Infrastructure")
	}
	if !contains(got, contracts.CellIntelligence) {
		t.Error("power degraded should wake Intelligence")
	}
}

func TestClassify_RoadBlocked(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ev := contracts.Event{Type: contracts.EventRoadBlocked, Confidence: 0.88}

	got := d.Classify(ws, ev)
	if !contains(got, contracts.CellInfrastructure) || !contains(got, contracts.CellIntelligence) {
		t.Error("road blocked should wake Infrastructure and Intelligence")
	}
}

func TestClassify_Deterministic(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	ev := contracts.Event{Type: contracts.EventAftershockOccurred, Confidence: 0.7}

	got1 := d.Classify(ws, ev)
	got2 := d.Classify(ws, ev)
	if len(got1) != len(got2) {
		t.Fatal("non-deterministic length")
	}
	for i := range got1 {
		if got1[i] != got2[i] {
			t.Errorf("order changed: %v vs %v", got1, got2)
		}
	}
}

func TestClassify_NoWakeForMinor(t *testing.T) {
	d := New()
	ws := makeBaseWorld()
	// A low-impact or bookkeeping event (resource handled in Commander phase)
	ev := contracts.Event{Type: contracts.EventResourceDeployed, Confidence: 0.6}

	got := d.Classify(ws, ev)
	// Per agreement, anomaly returns specialists only; Commander is orchestrator phase-2.
	// Resource may legitimately return empty from anomaly's view.
	if len(got) != 0 {
		t.Errorf("resource event woke specialists from anomaly: %v (should be empty)", got)
	}
}
