package validation

import (
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func TestEnvelope(t *testing.T) {
	ok := contracts.Event{ID: "e1", Type: contracts.EventBridgeClosed, Confidence: 0.5}
	if re := Envelope(ok); re != nil {
		t.Fatalf("valid event rejected: %v", re)
	}
	cases := []struct {
		name string
		ev   contracts.Event
		want contracts.RejectionReason
	}{
		{"no id", contracts.Event{Type: contracts.EventBridgeClosed, Confidence: 1}, contracts.RejectSchema},
		{"unknown type", contracts.Event{ID: "x", Type: "Nope", Confidence: 1}, contracts.RejectSchema},
		{"bad confidence", contracts.Event{ID: "x", Type: contracts.EventBridgeClosed, Confidence: 1.5}, contracts.RejectSchema},
		{"negative timestamp", contracts.Event{ID: "x", Type: contracts.EventBridgeClosed, Confidence: 1, Timestamp: -1}, contracts.RejectRangeSanity},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			re := Envelope(c.ev)
			if re == nil || re.Reason != c.want {
				t.Fatalf("got %v, want reason %s", re, c.want)
			}
		})
	}
}

func TestLegalTransitions(t *testing.T) {
	if !LegalBridge(contracts.BridgeOpen, contracts.BridgeClosed) {
		t.Error("open→closed should be legal")
	}
	if LegalBridge(contracts.BridgeClosed, contracts.BridgeClosed) {
		t.Error("closed→closed must be illegal (no-op)")
	}
	if LegalBridge(contracts.BridgeClosed, contracts.BridgeOpen) {
		t.Error("closed→open must be illegal (backward)")
	}
	if !LegalDam(contracts.DamNormal, contracts.DamStressed) {
		t.Error("dam normal→stressed legal")
	}
	if !LegalLevee(contracts.LeveeIntact, contracts.LeveeBreached) {
		t.Error("levee intact→breached legal")
	}
	if LegalPower(contracts.PowerOff, contracts.PowerOff) {
		t.Error("power off→off illegal")
	}

	// LegalRoad is bidirectional (§8.4)
	if !LegalRoad(contracts.RoadOpen, contracts.RoadBlocked) {
		t.Error("open→blocked should be legal for road")
	}
	if LegalRoad(contracts.RoadBlocked, contracts.RoadBlocked) {
		t.Error("blocked→blocked must be illegal (no-op)")
	}
	if !LegalRoad(contracts.RoadBlocked, contracts.RoadOpen) {
		t.Error("blocked→open must be legal (bidirectional)")
	}
	if !LegalRoad(contracts.RoadCongested, contracts.RoadBlocked) {
		t.Error("congested→blocked legal")
	}
	// validate both sides (robustness)
	if LegalRoad(contracts.RoadStatus("bogus"), contracts.RoadBlocked) {
		t.Error("invalid from must be illegal")
	}
	if LegalRoad(contracts.RoadOpen, contracts.RoadStatus("bogus")) {
		t.Error("invalid to must be illegal")
	}
}

func TestKnownType_NewEvents(t *testing.T) {
	for _, et := range []contracts.EventType{
		contracts.EventBridgeCollapsed,
		contracts.EventPowerDegraded,
		contracts.EventRoadBlocked, // already present but exercise
	} {
		if !KnownType(et) {
			t.Errorf("KnownType(%s) = false, want true", et)
		}
	}
}
