package contracts

import "encoding/json"

// events.go — the Event struct, the event-type enum, and per-type payloads
// (SPEC §7). Change only via the §0.5 coordinated step.

// EventID uniquely identifies an event. Duplicate IDs are dropped (idempotency,
// §14.2).
type EventID string

// SimTime is scenario time in seconds since the scenario's t=0 — NOT wall-clock
// time (determinism, §0.2 rule 5). It is totally ordered, so temporal
// monotonicity (§14.2) is a simple comparison and replays render identically
// (e.g. the UI maps it onto the 09:00 → 09:05 clock face).
type SimTime int64

// EventType enumerates every kind of fact the world can assert (SPEC §7),
// tagged in comments by the Cell(s) it wakes.
type EventType string

const (
	// Seismic (→ Intelligence + all).
	EventMainshockOccurred         EventType = "MainshockOccurred"
	EventAftershockOccurred        EventType = "AftershockOccurred"
	EventAftershockForecastUpdated EventType = "AftershockForecastUpdated"

	// Structural (→ Infrastructure).
	EventBuildingCollapsed EventType = "BuildingCollapsed"
	EventBridgeDamaged     EventType = "BridgeDamaged"
	EventBridgeClosed      EventType = "BridgeClosed"
	EventBridgeCollapsed   EventType = "BridgeCollapsed"
	EventRoadBlocked       EventType = "RoadBlocked"
	EventTunnelClosed      EventType = "TunnelClosed"
	EventDamStressElevated EventType = "DamStressElevated"
	EventLeveeBreached     EventType = "LeveeBreached"

	// Utility (→ Intelligence/Infrastructure).
	EventPowerFailure    EventType = "PowerFailure"
	EventPowerDegraded   EventType = "PowerDegraded"
	EventGasLeakDetected EventType = "GasLeakDetected"
	EventWaterMainBreak  EventType = "WaterMainBreak"
	EventCommsOutage     EventType = "CommsOutage"

	// Fire (→ Infrastructure + Population).
	EventFireIgnited   EventType = "FireIgnited"
	EventFireSpread    EventType = "FireSpread"
	EventFireContained EventType = "FireContained"

	// Flood (→ Intelligence + Population).
	EventFloodExtentUpdated EventType = "FloodExtentUpdated"

	// Medical (→ Medical).
	EventCasualtyReportUpdated   EventType = "CasualtyReportUpdated"
	EventMassCasualtyIncident    EventType = "MassCasualtyIncident"
	EventHospitalCapacityChanged EventType = "HospitalCapacityChanged"

	// Population (→ Population).
	EventCitizenDistressCall     EventType = "CitizenDistressCall"
	EventPersonsTrapped          EventType = "PersonsTrapped"
	EventEvacuationOrdered       EventType = "EvacuationOrdered"
	EventShelterOccupancyChanged EventType = "ShelterOccupancyChanged"
	EventShelterFull             EventType = "ShelterFull"

	// Perception (→ generates the above).
	EventSatelliteImageReceived EventType = "SatelliteImageReceived"
	EventDroneImageReceived     EventType = "DroneImageReceived"

	// Resource (→ all, via Commander).
	EventResourceDeployed EventType = "ResourceDeployed"
	EventResourceDepleted EventType = "ResourceDepleted"

	// Validation bookkeeping: logged to the event log when the gatekeeper
	// rejects an event (§14.2).
	EventRejected EventType = "event_rejected"
)

// Event is an immutable fact about the evolving scenario (SPEC §7). Payload is a
// raw JSON object whose shape depends on Type; typed payload accessors live with
// the owning logic, not in contracts.
type Event struct {
	ID         EventID         `json:"id"`
	Timestamp  SimTime         `json:"timestamp"`
	Source     string          `json:"source"`
	Type       EventType       `json:"type"`
	Confidence float64         `json:"confidence"` // schema rule (§14.2): ∈ [0,1]
	Payload    json.RawMessage `json:"payload,omitempty"`
}
