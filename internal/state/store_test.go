package state

import (
	"encoding/json"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func initial() contracts.WorldState {
	return contracts.WorldState{
		Version: 0, Time: 0,
		Sectors:   map[contracts.SectorID]contracts.Sector{"ironworks": {ID: "ironworks", Name: "Ironworks", Power: contracts.PowerOn, Gas: contracts.UtilityUp}},
		Bridges:   map[contracts.BridgeID]contracts.Bridge{"vora": {ID: "vora", Name: "Vora Bridge", Status: contracts.BridgeOpen}},
		Dam:       contracts.Dam{ID: "mainor", Status: contracts.DamNormal},
		Levee:     contracts.Levee{ID: "southport", Status: contracts.LeveeIntact},
		Hospitals: map[contracts.HospitalID]contracts.Hospital{"cg": {ID: "cg", Beds: 100, Occupancy: 50, Band: contracts.HospitalNormal}},
		Shelters:  map[contracts.ShelterID]contracts.Shelter{"gf": {ID: "gf", Capacity: 10}},
	}
}

func ev(id string, ts contracts.SimTime, typ contracts.EventType, payload any) contracts.Event {
	b, _ := json.Marshal(payload)
	return contracts.Event{ID: contracts.EventID(id), Timestamp: ts, Type: typ, Confidence: 1, Payload: b}
}

func TestApply_BridgeCloseAndVersionBump(t *testing.T) {
	s := New(initial())
	v, err := s.Apply(ev("e1", 10, contracts.EventBridgeClosed, map[string]string{"bridgeId": "vora"}))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if v != 1 {
		t.Fatalf("version=%d, want 1", v)
	}
	if got := s.Snapshot().Bridges["vora"].Status; got != contracts.BridgeClosed {
		t.Fatalf("bridge status=%s", got)
	}
}

func TestApply_Rejections(t *testing.T) {
	s := New(initial())
	_, _ = s.Apply(ev("e1", 10, contracts.EventBridgeClosed, map[string]string{"bridgeId": "vora"}))

	tests := []struct {
		name string
		ev   contracts.Event
		want contracts.RejectionReason
	}{
		{"duplicate", ev("e1", 20, contracts.EventBridgeClosed, map[string]string{"bridgeId": "vora"}), contracts.RejectDuplicate},
		{"temporal", ev("e2", 5, contracts.EventDamStressElevated, struct{}{}), contracts.RejectTemporalMonotonicity},
		{"unknown bridge", ev("e3", 30, contracts.EventBridgeClosed, map[string]string{"bridgeId": "ghost"}), contracts.RejectReferentialIntegrity},
		{"illegal transition", ev("e4", 40, contracts.EventBridgeClosed, map[string]string{"bridgeId": "vora"}), contracts.RejectIllegalTransition},
		{"bad confidence", contracts.Event{ID: "e5", Timestamp: 50, Type: contracts.EventDamStressElevated, Confidence: 9}, contracts.RejectSchema},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			before := s.Version()
			_, err := s.Apply(c.ev)
			re, ok := err.(*contracts.RejectionError)
			if !ok || re.Reason != c.want {
				t.Fatalf("got %v, want %s", err, c.want)
			}
			if s.Version() != before {
				t.Fatalf("rejected event changed version")
			}
		})
	}
}

func TestApply_HospitalBand(t *testing.T) {
	s := New(initial())
	_, err := s.Apply(ev("h1", 10, contracts.EventHospitalCapacityChanged, map[string]any{"hospitalId": "cg", "occupancy": 95}))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Hospitals["cg"].Band; got != contracts.HospitalCritical {
		t.Fatalf("band=%s, want critical", got)
	}
}

func TestSnapshotIsolation(t *testing.T) {
	s := New(initial())
	snap := s.Snapshot()
	snap.Bridges["vora"] = contracts.Bridge{Status: contracts.BridgeCollapsed}
	if s.Snapshot().Bridges["vora"].Status != contracts.BridgeOpen {
		t.Fatal("mutating snapshot leaked into live state")
	}
}

func TestApply_FireUpsertThenSpread(t *testing.T) {
	s := New(initial())
	if _, err := s.Apply(ev("f1", 10, contracts.EventFireIgnited, map[string]string{"fireZoneId": "fz1", "sector": "ironworks"})); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(ev("f2", 20, contracts.EventFireSpread, map[string]string{"fireZoneId": "fz1"})); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().FireZones["fz1"].Status; got != contracts.FireStatusSpreading {
		t.Fatalf("fire status=%s", got)
	}
}
