// Package validation implements the §14.2 event-validation rules shared by
// internal/state (the live gatekeeper) and internal/scenariogen.
//
// Owner: state + validation lane.
package validation

import "github.com/nrynss/opsec-control/internal/contracts"

var knownTypes = map[contracts.EventType]struct{}{
	contracts.EventMainshockOccurred: {}, contracts.EventAftershockOccurred: {}, contracts.EventAftershockForecastUpdated: {},
	contracts.EventBuildingCollapsed: {}, contracts.EventBridgeDamaged: {}, contracts.EventBridgeClosed: {}, contracts.EventBridgeCollapsed: {},
	contracts.EventRoadBlocked: {}, contracts.EventTunnelClosed: {}, contracts.EventDamStressElevated: {}, contracts.EventLeveeBreached: {},
	contracts.EventPowerFailure: {}, contracts.EventPowerDegraded: {}, contracts.EventGasLeakDetected: {}, contracts.EventWaterMainBreak: {}, contracts.EventCommsOutage: {},
	contracts.EventFireIgnited: {}, contracts.EventFireSpread: {}, contracts.EventFireContained: {},
	contracts.EventFloodExtentUpdated:    {},
	contracts.EventCasualtyReportUpdated: {}, contracts.EventMassCasualtyIncident: {}, contracts.EventHospitalCapacityChanged: {},
	contracts.EventCitizenDistressCall: {}, contracts.EventPersonsTrapped: {}, contracts.EventEvacuationOrdered: {},
	contracts.EventShelterOccupancyChanged: {}, contracts.EventShelterFull: {},
	contracts.EventSatelliteImageReceived: {}, contracts.EventDroneImageReceived: {},
	contracts.EventResourceDeployed: {}, contracts.EventResourceDepleted: {}, contracts.EventRejected: {},
}

// KnownType reports whether t is in the §7 taxonomy.
func KnownType(t contracts.EventType) bool { _, ok := knownTypes[t]; return ok }

// Envelope checks schema-level invariants independent of world state (§14.2).
func Envelope(ev contracts.Event) *contracts.RejectionError {
	switch {
	case ev.ID == "":
		return &contracts.RejectionError{EventID: ev.ID, Reason: contracts.RejectSchema, Detail: "missing id"}
	case !KnownType(ev.Type):
		return &contracts.RejectionError{EventID: ev.ID, Reason: contracts.RejectSchema, Detail: "unknown event type"}
	case ev.Confidence < 0 || ev.Confidence > 1:
		return &contracts.RejectionError{EventID: ev.ID, Reason: contracts.RejectSchema, Detail: "confidence out of [0,1]"}
	case ev.Timestamp < 0:
		// SimTime is signed; t=0 is the scenario start (§8.5). A negative
		// timestamp is out of range — reject it at the envelope so it never
		// reaches the monotonicity check or sets world time negative.
		return &contracts.RejectionError{EventID: ev.ID, Reason: contracts.RejectRangeSanity, Detail: "negative timestamp"}
	}
	return nil
}

// forward reports whether from→to is a legal forward-only transition (strictly
// advancing rank — a no-op or backward move is illegal, §8.4).
func forward[T comparable](rank map[T]int, from, to T) bool { return rank[to] > rank[from] }

var (
	bridgeRank  = map[contracts.BridgeStatus]int{contracts.BridgeOpen: 0, contracts.BridgeRestricted: 1, contracts.BridgeClosed: 2, contracts.BridgeCollapsed: 3}
	damRank     = map[contracts.DamStatus]int{contracts.DamNormal: 0, contracts.DamStressed: 1, contracts.DamReleasing: 2, contracts.DamBreached: 3}
	leveeRank   = map[contracts.LeveeStatus]int{contracts.LeveeIntact: 0, contracts.LeveeOvertopping: 1, contracts.LeveeBreached: 2}
	powerRank   = map[contracts.PowerStatus]int{contracts.PowerOn: 0, contracts.PowerPartial: 1, contracts.PowerOff: 2}
	utilityRank = map[contracts.UtilityStatus]int{contracts.UtilityUp: 0, contracts.UtilityDegraded: 1, contracts.UtilityDown: 2}
	fireRank    = map[contracts.FireStatus]int{contracts.FireStatusIgnited: 0, contracts.FireStatusSpreading: 1, contracts.FireStatusContained: 2, contracts.FireStatusOut: 3}
)

func LegalBridge(from, to contracts.BridgeStatus) bool   { return forward(bridgeRank, from, to) }
func LegalDam(from, to contracts.DamStatus) bool         { return forward(damRank, from, to) }
func LegalLevee(from, to contracts.LeveeStatus) bool     { return forward(leveeRank, from, to) }
func LegalPower(from, to contracts.PowerStatus) bool     { return forward(powerRank, from, to) }
func LegalUtility(from, to contracts.UtilityStatus) bool { return forward(utilityRank, from, to) }
func LegalFire(from, to contracts.FireStatus) bool       { return forward(fireRank, from, to) }

// Road is bidirectional per SPEC §8.4 (open ↔ congested ↔ blocked).
// LegalRoad allows any transition between *distinct* valid states; no-op is rejected
// for consistency with other "must actually change" rules.
// Both 'from' and 'to' are validated for robustness (symmetry); in practice 'from'
// comes from tracked state but callers should not be able to sneak in invalid values.
func LegalRoad(from, to contracts.RoadStatus) bool {
	return from != to && validRoad(from) && validRoad(to)
}

func validRoad(s contracts.RoadStatus) bool {
	switch s {
	case contracts.RoadOpen, contracts.RoadCongested, contracts.RoadBlocked:
		return true
	}
	return false
}
