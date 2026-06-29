package contracttest

import (
	"context"
	"testing"

	"github.com/nrynss/opsec-control/internal/anomaly"
	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/orchestrator"
)

const commanderSummary = "cop-synthesized"

// stubCell is a minimal contracts.Cell. The Commander stub returns a sentinel
// summary so we can prove it actually ran.
type stubCell struct{ kind contracts.CellKind }

func (s stubCell) Kind() contracts.CellKind { return s.kind }

func (s stubCell) Analyze(_ context.Context, in contracts.CellInput) (contracts.CellOutput, error) {
	summary := string(s.kind) + " analysis"
	if s.kind == contracts.CellCommander {
		summary = commanderSummary
	}
	return contracts.CellOutput{
		Cell:         s.kind,
		Summary:      summary,
		RiskLevel:    contracts.RiskMedium,
		Confidence:   1,
		StateVersion: in.StateVersion,
	}, nil
}

func allCells() map[contracts.CellKind]contracts.Cell {
	kinds := []contracts.CellKind{
		contracts.CellIntelligence, contracts.CellInfrastructure, contracts.CellMedical,
		contracts.CellPopulation, contracts.CellCommunications, contracts.CellCommander,
	}
	m := make(map[contracts.CellKind]contracts.Cell, len(kinds))
	for _, k := range kinds {
		m[k] = stubCell{kind: k}
	}
	return m
}

// every event type in the §7 taxonomy that can drive a fan-out (event_rejected
// is bookkeeping, never fed to the classifier).
var seamEventTypes = []contracts.EventType{
	contracts.EventMainshockOccurred, contracts.EventAftershockOccurred, contracts.EventAftershockForecastUpdated,
	contracts.EventBuildingCollapsed, contracts.EventBridgeDamaged, contracts.EventBridgeClosed, contracts.EventBridgeCollapsed,
	contracts.EventRoadBlocked, contracts.EventTunnelClosed, contracts.EventDamStressElevated, contracts.EventLeveeBreached,
	contracts.EventPowerFailure, contracts.EventPowerDegraded, contracts.EventGasLeakDetected, contracts.EventWaterMainBreak, contracts.EventCommsOutage,
	contracts.EventFireIgnited, contracts.EventFireSpread, contracts.EventFireContained,
	contracts.EventFloodExtentUpdated,
	contracts.EventCasualtyReportUpdated, contracts.EventMassCasualtyIncident, contracts.EventHospitalCapacityChanged,
	contracts.EventCitizenDistressCall, contracts.EventPersonsTrapped, contracts.EventEvacuationOrdered,
	contracts.EventShelterOccupancyChanged, contracts.EventShelterFull,
	contracts.EventSatelliteImageReceived, contracts.EventDroneImageReceived,
	contracts.EventResourceDeployed, contracts.EventResourceDepleted,
}

// TestSeam_EveryEventYieldsCommanderCOP drives each event type through the REAL
// anomaly Classifier and orchestrator FanOut and asserts the Commander always
// synthesizes a COP. The Classifier contract states the Commander is an
// unconditional phase-2 step (SPEC §6/§7) — including when the specialist wake
// set is empty (e.g. resource events). This guards the anomaly→orchestrator seam.
func TestSeam_EveryEventYieldsCommanderCOP(t *testing.T) {
	// Compile-time conformance: both impls satisfy their contracts.
	var clf contracts.Classifier = anomaly.New()
	var orch contracts.Orchestrator = orchestrator.NewEngine(allCells())

	snap := contracts.WorldState{Version: 1}

	for _, et := range seamEventTypes {
		t.Run(string(et), func(t *testing.T) {
			ev := contracts.Event{
				ID:         "evt-" + contracts.EventID(et),
				Timestamp:  1,
				Type:       et,
				Confidence: 1,
			}
			wake := clf.Classify(snap, ev)

			cop, err := orch.FanOut(context.Background(), snap, ev, wake)
			if err != nil {
				t.Fatalf("FanOut error for %s: %v", et, err)
			}
			if cop.Summary != commanderSummary {
				t.Fatalf("event %s never reached Commander synthesis (COP summary %q); "+
					"anomaly wake=%v, orchestrator skipped the Commander on this path",
					et, cop.Summary, wake)
			}
			if cop.StateVersion != snap.Version {
				t.Errorf("event %s: COP stateVersion=%d, want %d", et, cop.StateVersion, snap.Version)
			}
		})
	}
}
