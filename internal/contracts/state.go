package contracts

// state.go — World State types, entity types, and their status enums
// (SPEC §8.2–§8.4). Change only via the §0.5 coordinated step.
//
// Note on determinism: maps are keyed by entity ID for lookup. encoding/json
// marshals map keys in sorted order, so serialization is deterministic; logic
// that iterates these maps must not depend on iteration order (§0.2 rule 5).

// StateVersion increments on every accepted event (SPEC §8): v41 → v42.
type StateVersion uint64

// Entity identifiers.
type (
	SectorID   string
	BridgeID   string
	HospitalID string
	ShelterID  string
	FireZoneID string
	ResourceID string
)

// WorldState is the single authoritative in-memory snapshot (SPEC §8). Only
// internal/state mutates it; consumers receive a read-only copy.
type WorldState struct {
	Version   StateVersion            `json:"version"`
	Time      SimTime                 `json:"time"`
	Sectors   map[SectorID]Sector     `json:"sectors"`
	Bridges   map[BridgeID]Bridge     `json:"bridges"`
	Dam       Dam                     `json:"dam"`
	Levee     Levee                   `json:"levee"`
	Hospitals map[HospitalID]Hospital `json:"hospitals"`
	Shelters  map[ShelterID]Shelter   `json:"shelters"`
	FireZones map[FireZoneID]FireZone `json:"fireZones"`
	Flood     Flood                   `json:"flood"`
	Resources map[ResourceID]Resource `json:"resources"`
	Roads     map[RoadID]Road         `json:"roads"`
}

// --- Sectors & utilities (§8.2, §8.4) ---

// PowerStatus: on → partial → off (forward; → on is a stretch: restoration).
type PowerStatus string

const (
	PowerOn      PowerStatus = "on"
	PowerPartial PowerStatus = "partial"
	PowerOff     PowerStatus = "off"
)

// UtilityStatus models comms / water / gas: up → degraded → down (forward;
// gas "down" can mean a deliberate shutoff).
type UtilityStatus string

const (
	UtilityUp       UtilityStatus = "up"
	UtilityDegraded UtilityStatus = "degraded"
	UtilityDown     UtilityStatus = "down"
)

type Sector struct {
	ID         SectorID      `json:"id"`
	Name       string        `json:"name"`
	Power      PowerStatus   `json:"power"`
	Comms      UtilityStatus `json:"comms"`
	Water      UtilityStatus `json:"water"`
	Gas        UtilityStatus `json:"gas"`
	Population int           `json:"population"`
}

// --- Bridges (§8.3, §8.4) ---

// BridgeStatus: open → restricted → closed → collapsed (forward only).
type BridgeStatus string

const (
	BridgeOpen       BridgeStatus = "open"
	BridgeRestricted BridgeStatus = "restricted"
	BridgeClosed     BridgeStatus = "closed"
	BridgeCollapsed  BridgeStatus = "collapsed"
)

type Bridge struct {
	ID     BridgeID     `json:"id"`
	Name   string       `json:"name"`
	Status BridgeStatus `json:"status"`
}

// --- Roads (§8.3, §8.4) ---

// RoadStatus: open ↔ congested ↔ blocked (bidirectional, §8.4).
type RoadStatus string

const (
	RoadOpen      RoadStatus = "open"
	RoadCongested RoadStatus = "congested"
	RoadBlocked   RoadStatus = "blocked"
)

type RoadID string

type Road struct {
	ID     RoadID     `json:"id"`
	Name   string     `json:"name"`
	Status RoadStatus `json:"status"`
}

// --- Dam & levee (§8.3, §8.4) ---

// DamStatus: normal → stressed → releasing → breached (forward only).
type DamStatus string

const (
	DamNormal    DamStatus = "normal"
	DamStressed  DamStatus = "stressed"
	DamReleasing DamStatus = "releasing"
	DamBreached  DamStatus = "breached"
)

type Dam struct {
	ID           string    `json:"id"`
	Status       DamStatus `json:"status"`
	ReservoirPct float64   `json:"reservoirPct"` // ∈ [0,1]
	StressRating float64   `json:"stressRating"` // ∈ [0,1]
}

// LeveeStatus: intact → overtopping → breached (forward only).
type LeveeStatus string

const (
	LeveeIntact      LeveeStatus = "intact"
	LeveeOvertopping LeveeStatus = "overtopping"
	LeveeBreached    LeveeStatus = "breached"
)

type Levee struct {
	ID        string      `json:"id"`
	Status    LeveeStatus `json:"status"`
	Height    float64     `json:"height"`
	Integrity float64     `json:"integrity"` // ∈ [0,1]
}

// --- Hospitals & shelters (§8.3, §8.4) ---

// HospitalBand tracks occupancy: normal <70% / strained 70–90% /
// critical 90–100% / over-capacity.
type HospitalBand string

const (
	HospitalNormal       HospitalBand = "normal"
	HospitalStrained     HospitalBand = "strained"
	HospitalCritical     HospitalBand = "critical"
	HospitalOverCapacity HospitalBand = "over_capacity"
)

type Hospital struct {
	ID          HospitalID   `json:"id"`
	Name        string       `json:"name"`
	Sector      SectorID     `json:"sector"`
	Beds        int          `json:"beds"`
	ICU         int          `json:"icu"`
	ER          int          `json:"er"`
	Occupancy   int          `json:"occupancy"` // current patient load
	Band        HospitalBand `json:"band"`
	OnGenerator bool         `json:"onGenerator"`
}

type Shelter struct {
	ID        ShelterID `json:"id"`
	Name      string    `json:"name"`
	Sector    SectorID  `json:"sector"`
	Capacity  int       `json:"capacity"`
	Occupancy int       `json:"occupancy"`
	Full      bool      `json:"full"` // true when Occupancy ≥ Capacity
}

// --- Fire & flood (§8.4) ---

// FireStatus: ignited → spreading → contained → out.
type FireStatus string

const (
	FireStatusIgnited   FireStatus = "ignited"
	FireStatusSpreading FireStatus = "spreading"
	FireStatusContained FireStatus = "contained"
	FireStatusOut       FireStatus = "out"
)

type FireZone struct {
	ID     FireZoneID `json:"id"`
	Sector SectorID   `json:"sector"`
	Status FireStatus `json:"status"`
}

// Flood extent is monotonically increasing within an episode unless an explicit
// recession event (§8.4).
type Flood struct {
	Polygons []FloodPolygon `json:"polygons"`
}

type FloodPolygon struct {
	Sector SectorID `json:"sector"`
	DepthM float64  `json:"depthM"`
	Points []Point  `json:"points"`
}

// Point is a coordinate on the stylized Cerebro map board.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// --- Resources (§8.3) ---

type ResourceKind string

const (
	ResourceAmbulance   ResourceKind = "ambulance"
	ResourceFireEngine  ResourceKind = "fire_engine"
	ResourceUSARTeam    ResourceKind = "usar_team"
	ResourceHelicopter  ResourceKind = "helicopter"
	ResourceEvacBus     ResourceKind = "evac_bus"
	ResourceSupplyCache ResourceKind = "supply_cache"
)

type Resource struct {
	ID       ResourceID   `json:"id"`
	Kind     ResourceKind `json:"kind"`
	HomeBase SectorID     `json:"homeBase"`
	Count    int          `json:"count"`
	Deployed int          `json:"deployed"`
}
