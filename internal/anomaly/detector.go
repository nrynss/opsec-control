package anomaly

import (
	"github.com/nrynss/opsec-control/internal/contracts"
)

// Detector implements contracts.Classifier.
//
// It classifies accepted events and decides which specialist Cells to wake
// for the parallel fan-out (SPEC §6). The returned slice contains only
// specialist CellKinds — the orchestrator unconditionally invokes the
// Commander as a phase-2 synthesis step after all specialists return.
type Detector struct{}

// compile-time interface check
var _ contracts.Classifier = (*Detector)(nil)

// New returns a new Detector with default classification rules.
func New() *Detector {
	return &Detector{}
}

// Classify returns the distinct Cells that should be woken to analyze the
// situation after this event. The order is deterministic (Intelligence,
// Infrastructure, Medical, Population, Communications, Commander).
//
// The returned slice contains only specialist CellKinds — the orchestrator
// unconditionally invokes the Commander as a phase-2 synthesis step after
// all specialists return (§6).
//
// Rules are based on:
//   - Event type (per the taxonomy in contracts/events.go)
//   - Current entity status/thresholds in the snapshot (e.g. hospital band,
//     bridge status, flood presence, fire spreading, dam/levee stress, shelter full)
//   - SPEC §6 examples: status changes, capacity bands (>~85% critical),
//     aftershocks, power/utility failures, etc.
//
// See TODOs in the implementation for known §6 simplifications (confidence,
// deltas, exact 85% threshold).
func (d *Detector) Classify(snapshot contracts.WorldState, event contracts.Event) []contracts.CellKind {
	wake := make(map[contracts.CellKind]struct{})

	// Always consider the event type first (primary signals)
	switch event.Type {
	case contracts.EventMainshockOccurred, contracts.EventAftershockOccurred:
		// Major seismic: everyone cares (one anomaly → all relevant cells)
		wake[contracts.CellIntelligence] = struct{}{}
		wake[contracts.CellInfrastructure] = struct{}{}
		wake[contracts.CellMedical] = struct{}{}
		wake[contracts.CellPopulation] = struct{}{}
		wake[contracts.CellCommunications] = struct{}{}

	case contracts.EventAftershockForecastUpdated:
		wake[contracts.CellIntelligence] = struct{}{}

	case contracts.EventBuildingCollapsed,
		contracts.EventBridgeDamaged, contracts.EventBridgeClosed,
		contracts.EventRoadBlocked, contracts.EventTunnelClosed:
		wake[contracts.CellInfrastructure] = struct{}{}
		wake[contracts.CellIntelligence] = struct{}{}

	case contracts.EventDamStressElevated, contracts.EventLeveeBreached:
		wake[contracts.CellInfrastructure] = struct{}{}
		wake[contracts.CellIntelligence] = struct{}{}

	case contracts.EventPowerFailure, contracts.EventGasLeakDetected,
		contracts.EventWaterMainBreak, contracts.EventCommsOutage:
		wake[contracts.CellInfrastructure] = struct{}{}
		wake[contracts.CellIntelligence] = struct{}{}

	case contracts.EventFireIgnited, contracts.EventFireSpread:
		wake[contracts.CellInfrastructure] = struct{}{}
		wake[contracts.CellPopulation] = struct{}{}

	case contracts.EventFireContained:
		wake[contracts.CellInfrastructure] = struct{}{}

	case contracts.EventFloodExtentUpdated:
		wake[contracts.CellIntelligence] = struct{}{}
		wake[contracts.CellPopulation] = struct{}{}

	case contracts.EventCasualtyReportUpdated, contracts.EventMassCasualtyIncident:
		wake[contracts.CellMedical] = struct{}{}
		wake[contracts.CellPopulation] = struct{}{}

	case contracts.EventHospitalCapacityChanged:
		wake[contracts.CellMedical] = struct{}{}

	case contracts.EventCitizenDistressCall, contracts.EventPersonsTrapped,
		contracts.EventEvacuationOrdered:
		wake[contracts.CellPopulation] = struct{}{}
		wake[contracts.CellMedical] = struct{}{}

	case contracts.EventShelterOccupancyChanged, contracts.EventShelterFull:
		wake[contracts.CellPopulation] = struct{}{}

	case contracts.EventSatelliteImageReceived, contracts.EventDroneImageReceived:
		wake[contracts.CellIntelligence] = struct{}{}
	}

	// Additional state-based thresholds (even if the triggering event was
	// something else, the current situation may require attention).
	// These catch ongoing critical conditions.

	// Bridges / structural
	for _, b := range snapshot.Bridges {
		if b.Status == contracts.BridgeClosed || b.Status == contracts.BridgeCollapsed {
			wake[contracts.CellInfrastructure] = struct{}{}
			wake[contracts.CellIntelligence] = struct{}{}
		}
	}

	// Dam / levee
	if snapshot.Dam.Status == contracts.DamStressed ||
		snapshot.Dam.Status == contracts.DamReleasing ||
		snapshot.Dam.Status == contracts.DamBreached {
		wake[contracts.CellInfrastructure] = struct{}{}
		wake[contracts.CellIntelligence] = struct{}{}
	}
	if snapshot.Levee.Status == contracts.LeveeOvertopping ||
		snapshot.Levee.Status == contracts.LeveeBreached {
		wake[contracts.CellInfrastructure] = struct{}{}
		wake[contracts.CellPopulation] = struct{}{}
	}

	// Fires
	for _, f := range snapshot.FireZones {
		if f.Status == contracts.FireStatusIgnited || f.Status == contracts.FireStatusSpreading {
			wake[contracts.CellInfrastructure] = struct{}{}
			wake[contracts.CellPopulation] = struct{}{}
		}
	}

	// Flood present
	if len(snapshot.Flood.Polygons) > 0 {
		wake[contracts.CellIntelligence] = struct{}{}
		wake[contracts.CellPopulation] = struct{}{}
	}

	// Hospitals critical
	for _, h := range snapshot.Hospitals {
		if isMedicalCritical(h.Band) {
			wake[contracts.CellMedical] = struct{}{}
		}
	}

	// Shelters full
	for _, s := range snapshot.Shelters {
		if s.Full {
			wake[contracts.CellPopulation] = struct{}{}
		}
	}

	// Power out anywhere
	for _, sec := range snapshot.Sectors {
		if sec.Power == contracts.PowerOff {
			wake[contracts.CellInfrastructure] = struct{}{}
			wake[contracts.CellIntelligence] = struct{}{}
			break
		}
	}

	// TODO §6 (MVD simplification): event.Confidence is not used for
	// "confidence-weighted clustering of citizen reports".
	// TODO §6: delta-based thresholds (e.g. "flood extent delta beyond threshold")
	// are presence-based only for now.
	// TODO §6: hospital example threshold ">85%" would include strained band;
	// we currently trigger Medical only on critical/over_capacity (>=90%).

	// Convert map to deterministic slice (fixed order) — specialists only.
	// Commander is always run by the orchestrator as phase-2 (§6).
	order := []contracts.CellKind{
		contracts.CellIntelligence,
		contracts.CellInfrastructure,
		contracts.CellMedical,
		contracts.CellPopulation,
		contracts.CellCommunications,
	}

	var result []contracts.CellKind
	for _, k := range order {
		if _, ok := wake[k]; ok {
			result = append(result, k)
		}
	}
	return result
}

func isMedicalCritical(band contracts.HospitalBand) bool {
	return band == contracts.HospitalCritical || band == contracts.HospitalOverCapacity
}
