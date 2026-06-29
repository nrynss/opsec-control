package state

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func initial() contracts.WorldState {
	return contracts.WorldState{
		Version: 0, Time: 0,
		Sectors:   map[contracts.SectorID]contracts.Sector{"ironworks": {ID: "ironworks", Name: "Ironworks", Power: contracts.PowerOn, Gas: contracts.UtilityUp}},
		Bridges:   map[contracts.BridgeID]contracts.Bridge{"vora": {ID: "vora", Name: "Vora Bridge", Status: contracts.BridgeOpen}},
		Roads:     map[contracts.RoadID]contracts.Road{"r-main": {ID: "r-main", Name: "Main Road", Status: contracts.RoadOpen}},
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

func TestApply_RoadBlocked(t *testing.T) {
	s := New(initial())
	v, err := s.Apply(ev("rb1", 10, contracts.EventRoadBlocked, map[string]string{"roadId": "r-main"}))
	if err != nil {
		t.Fatalf("apply road: %v", err)
	}
	if v != 1 {
		t.Fatalf("version=%d want 1", v)
	}
	if got := s.Snapshot().Roads["r-main"].Status; got != contracts.RoadBlocked {
		t.Fatalf("road status=%s", got)
	}
}

func TestApply_RoadRejections(t *testing.T) {
	s := New(initial())
	_, _ = s.Apply(ev("rb1", 10, contracts.EventRoadBlocked, map[string]string{"roadId": "r-main"}))

	// unknown road
	_, err := s.Apply(ev("rb2", 20, contracts.EventRoadBlocked, map[string]string{"roadId": "ghost"}))
	re, ok := err.(*contracts.RejectionError)
	if !ok || re.Reason != contracts.RejectReferentialIntegrity {
		t.Fatalf("unknown road: got %v want referential", err)
	}

	// bad payload
	_, err = s.Apply(ev("rb3", 30, contracts.EventRoadBlocked, map[string]string{"bad": "payload"}))
	re, ok = err.(*contracts.RejectionError)
	if !ok || re.Reason != contracts.RejectSchema {
		t.Fatalf("bad payload: got %v want schema", err)
	}

	// illegal no-op (already blocked)
	_, err = s.Apply(ev("rb4", 40, contracts.EventRoadBlocked, map[string]string{"roadId": "r-main"}))
	re, ok = err.(*contracts.RejectionError)
	if !ok || re.Reason != contracts.RejectIllegalTransition {
		t.Fatalf("no-op: got %v want illegal", err)
	}
}

func TestApply_BridgeCollapsed(t *testing.T) {
	s := New(initial())
	// first close
	if _, err := s.Apply(ev("b1", 10, contracts.EventBridgeClosed, map[string]string{"bridgeId": "vora"})); err != nil {
		t.Fatal(err)
	}
	// then collapse
	v, err := s.Apply(ev("b2", 20, contracts.EventBridgeCollapsed, map[string]string{"bridgeId": "vora"}))
	if err != nil {
		t.Fatalf("collapse apply: %v", err)
	}
	if v != 2 {
		t.Fatalf("ver=%d", v)
	}
	if got := s.Snapshot().Bridges["vora"].Status; got != contracts.BridgeCollapsed {
		t.Fatalf("status=%s", got)
	}
	// second collapse illegal
	_, err = s.Apply(ev("b3", 30, contracts.EventBridgeCollapsed, map[string]string{"bridgeId": "vora"}))
	re, _ := err.(*contracts.RejectionError)
	if re == nil || re.Reason != contracts.RejectIllegalTransition {
		t.Fatalf("second collapse should illegal: %v", err)
	}
}

func TestApply_PowerDegradedChain(t *testing.T) {
	s := New(initial())
	// on -> partial
	if _, err := s.Apply(ev("p1", 10, contracts.EventPowerDegraded, map[string]string{"sector": "ironworks"})); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Sectors["ironworks"].Power; got != contracts.PowerPartial {
		t.Fatalf("partial=%s", got)
	}
	// partial -> off
	if _, err := s.Apply(ev("p2", 20, contracts.EventPowerFailure, map[string]string{"sector": "ironworks"})); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Sectors["ironworks"].Power; got != contracts.PowerOff {
		t.Fatalf("off=%s", got)
	}
}

func TestApply_TriggerOnlyEvents_B1(t *testing.T) {
	s := New(initial())
	snap0 := s.Snapshot()
	ver0 := s.Version()

	// BuildingCollapsed (used by demo) accepted, version++, no entity change.
	v, err := s.Apply(ev("bc", 10, contracts.EventBuildingCollapsed, map[string]string{"sector": "ironworks"}))
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	if v != ver0+1 {
		t.Fatalf("building ver %d", v)
	}
	snap1 := s.Snapshot()
	if !versionsAdvanced(snap0, snap1, 1, 10) || !entitiesUnchanged(snap0, snap1) {
		t.Fatal("BuildingCollapsed mutated state beyond version/time (should be trigger-only)")
	}

	// TunnelClosed same (no payload, no entity).
	v2, err := s.Apply(ev("tc", 20, contracts.EventTunnelClosed, map[string]string{}))
	if err != nil {
		t.Fatalf("tunnel: %v", err)
	}
	if v2 != ver0+2 {
		t.Fatalf("tunnel ver")
	}
	snap2 := s.Snapshot()
	if !versionsAdvanced(snap1, snap2, 1, 20) || !entitiesUnchanged(snap1, snap2) {
		t.Fatal("TunnelClosed mutated state beyond version/time")
	}

	// Payload refs for trigger-only events are *not* validated (by design for B1;
	// no Building/Tunnel entity exists to enforce referential integrity).
	// A typo'd/nonexistent sector ref is accepted (still bumps version/time, zero entity change).
	sBad := New(initial())
	snapBad0 := sBad.Snapshot()
	badVer0 := sBad.Version()
	if _, err := sBad.Apply(ev("bc-bad", 5, contracts.EventBuildingCollapsed, map[string]string{"sector": "NOPE"})); err != nil {
		t.Fatalf("bad sector ref for BuildingCollapsed must be accepted (trigger-only): %v", err)
	}
	snapBad1 := sBad.Snapshot()
	if snapBad1.Version != badVer0+1 || snapBad1.Time != 5 {
		t.Fatal("bad-ref trigger event must still advance version/time")
	}
	if !entitiesUnchanged(snapBad0, snapBad1) {
		t.Fatal("bad-ref BuildingCollapsed must not mutate any entities")
	}
}

func TestApply_Reset(t *testing.T) {
	initWS := initial()
	s := New(initWS)
	// mutate state
	_, _ = s.Apply(ev("e1", 10, contracts.EventBridgeClosed, map[string]string{"bridgeId": "vora"}))
	if s.Version() != 1 {
		t.Fatalf("version=%d, want 1", s.Version())
	}
	if s.Snapshot().Bridges["vora"].Status != contracts.BridgeClosed {
		t.Fatalf("bridge status=%s, want closed", s.Snapshot().Bridges["vora"].Status)
	}

	// reset
	s.Reset(initWS)
	if s.Version() != 0 {
		t.Fatalf("after reset version=%d, want 0", s.Version())
	}
	if s.Snapshot().Bridges["vora"].Status != contracts.BridgeOpen {
		t.Fatalf("after reset bridge status=%s, want open", s.Snapshot().Bridges["vora"].Status)
	}

	// check duplicate map reset: e1 should be acceptable again
	v, err := s.Apply(ev("e1", 10, contracts.EventBridgeClosed, map[string]string{"bridgeId": "vora"}))
	if err != nil {
		t.Fatalf("after reset apply e1 failed: %v", err)
	}
	if v != 1 {
		t.Fatalf("after reset version=%d, want 1", v)
	}
}

// versionsAdvanced checks that version bumped by delta and time set to ts.
func versionsAdvanced(before, after contracts.WorldState, verDelta int, ts contracts.SimTime) bool {
	return after.Version == before.Version+contracts.StateVersion(verDelta) && after.Time == ts
}

// entitiesUnchanged normalizes version/time then does a full structural compare.
// Stronger than spot-checking a few fields.
func entitiesUnchanged(a, b contracts.WorldState) bool {
	a.Version, a.Time = 0, 0
	b.Version, b.Time = 0, 0
	return reflect.DeepEqual(a, b)
}

func TestSnapshotIsolation_Roads(t *testing.T) {
	s := New(initial())
	snap := s.Snapshot()
	snap.Roads["r-main"] = contracts.Road{ID: "r-main", Status: contracts.RoadBlocked}
	if s.Snapshot().Roads["r-main"].Status != contracts.RoadOpen {
		t.Fatal("mutating roads snapshot leaked into live state")
	}
}
