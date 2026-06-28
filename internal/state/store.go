package state

import (
	"encoding/json"
	"maps"
	"sync"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/validation"
)

// Store is the sole owner and single mutator of the world state (§8). It is the
// §14.2 gatekeeper: every event passes validation before it touches state, and
// every accepted event increments Version.
type Store struct {
	mu   sync.RWMutex
	ws   contracts.WorldState
	seen map[contracts.EventID]struct{}
}

// New builds a Store from the scenario's t=0 substrate. Maps are ensured
// non-nil so mutation is safe.
func New(initial contracts.WorldState) *Store {
	if initial.Sectors == nil {
		initial.Sectors = map[contracts.SectorID]contracts.Sector{}
	}
	if initial.Bridges == nil {
		initial.Bridges = map[contracts.BridgeID]contracts.Bridge{}
	}
	if initial.Hospitals == nil {
		initial.Hospitals = map[contracts.HospitalID]contracts.Hospital{}
	}
	if initial.Shelters == nil {
		initial.Shelters = map[contracts.ShelterID]contracts.Shelter{}
	}
	if initial.FireZones == nil {
		initial.FireZones = map[contracts.FireZoneID]contracts.FireZone{}
	}
	if initial.Resources == nil {
		initial.Resources = map[contracts.ResourceID]contracts.Resource{}
	}
	return &Store{ws: initial, seen: map[contracts.EventID]struct{}{}}
}

// Version returns the current world version.
func (s *Store) Version() contracts.StateVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ws.Version
}

// Snapshot returns a deep copy so callers cannot mutate live state.
func (s *Store) Snapshot() contracts.WorldState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.ws)
}

// Apply runs the §14.2 contract and, if accepted, mutates state and returns the
// new version. A rejected event returns a *contracts.RejectionError and leaves
// state unchanged.
func (s *Store) Apply(ev contracts.Event) (contracts.StateVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if re := validation.Envelope(ev); re != nil {
		return s.ws.Version, re
	}
	if _, dup := s.seen[ev.ID]; dup {
		return s.ws.Version, rej(ev, contracts.RejectDuplicate, "duplicate event id")
	}
	if ev.Timestamp < s.ws.Time {
		return s.ws.Version, rej(ev, contracts.RejectTemporalMonotonicity, "timestamp before last applied")
	}
	if re := s.mutate(ev); re != nil {
		return s.ws.Version, re
	}

	s.seen[ev.ID] = struct{}{}
	s.ws.Version++
	s.ws.Time = ev.Timestamp
	return s.ws.Version, nil
}

func rej(ev contracts.Event, r contracts.RejectionReason, d string) *contracts.RejectionError {
	return &contracts.RejectionError{EventID: ev.ID, Reason: r, Detail: d}
}

func parse[T any](raw json.RawMessage) (T, bool) {
	var v T
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		return v, false
	}
	return v, true
}

type (
	bridgeRef struct {
		BridgeID contracts.BridgeID `json:"bridgeId"`
	}
	sectorRef struct {
		Sector contracts.SectorID `json:"sector"`
	}
	fireRef struct {
		FireZoneID contracts.FireZoneID `json:"fireZoneId"`
		Sector     contracts.SectorID   `json:"sector"`
	}
	hospRef struct {
		HospitalID contracts.HospitalID `json:"hospitalId"`
		Occupancy  int                  `json:"occupancy"`
	}
	shelRef struct {
		ShelterID contracts.ShelterID `json:"shelterId"`
		Occupancy int                 `json:"occupancy"`
	}
	floodRef struct {
		Polygons []contracts.FloodPolygon `json:"polygons"`
	}
	resRef struct {
		ResourceID contracts.ResourceID `json:"resourceId"`
		Count      int                  `json:"count"`
	}
)

var fireTo = map[contracts.EventType]contracts.FireStatus{
	contracts.EventFireIgnited:   contracts.FireStatusIgnited,
	contracts.EventFireSpread:    contracts.FireStatusSpreading,
	contracts.EventFireContained: contracts.FireStatusContained,
}

// mutate applies the event's effect after referential/transition/range checks.
// Events with no tracked state effect (seismic, perception, citizen reports,
// evacuations, casualties) are accepted without mutation; they still bump the
// version via Apply (§8).
func (s *Store) mutate(ev contracts.Event) *contracts.RejectionError {
	switch ev.Type {
	case contracts.EventBridgeDamaged, contracts.EventBridgeClosed:
		p, ok := parse[bridgeRef](ev.Payload)
		if !ok {
			return rej(ev, contracts.RejectSchema, "bad bridge payload")
		}
		b, ok := s.ws.Bridges[p.BridgeID]
		if !ok {
			return rej(ev, contracts.RejectReferentialIntegrity, "unknown bridge")
		}
		to := contracts.BridgeRestricted
		if ev.Type == contracts.EventBridgeClosed {
			to = contracts.BridgeClosed
		}
		if !validation.LegalBridge(b.Status, to) {
			return rej(ev, contracts.RejectIllegalTransition, "bridge")
		}
		b.Status = to
		s.ws.Bridges[p.BridgeID] = b

	case contracts.EventPowerFailure:
		sec, re := s.sector(ev)
		if re != nil {
			return re
		}
		if !validation.LegalPower(sec.Power, contracts.PowerOff) {
			return rej(ev, contracts.RejectIllegalTransition, "power")
		}
		sec.Power = contracts.PowerOff
		s.ws.Sectors[sec.ID] = sec

	case contracts.EventGasLeakDetected, contracts.EventWaterMainBreak, contracts.EventCommsOutage:
		sec, re := s.sector(ev)
		if re != nil {
			return re
		}
		cur := sec.Gas
		if ev.Type == contracts.EventWaterMainBreak {
			cur = sec.Water
		} else if ev.Type == contracts.EventCommsOutage {
			cur = sec.Comms
		}
		if !validation.LegalUtility(cur, contracts.UtilityDown) {
			return rej(ev, contracts.RejectIllegalTransition, "utility")
		}
		switch ev.Type {
		case contracts.EventGasLeakDetected:
			sec.Gas = contracts.UtilityDown
		case contracts.EventWaterMainBreak:
			sec.Water = contracts.UtilityDown
		case contracts.EventCommsOutage:
			sec.Comms = contracts.UtilityDown
		}
		s.ws.Sectors[sec.ID] = sec

	case contracts.EventDamStressElevated:
		if !validation.LegalDam(s.ws.Dam.Status, contracts.DamStressed) {
			return rej(ev, contracts.RejectIllegalTransition, "dam")
		}
		s.ws.Dam.Status = contracts.DamStressed

	case contracts.EventLeveeBreached:
		if !validation.LegalLevee(s.ws.Levee.Status, contracts.LeveeBreached) {
			return rej(ev, contracts.RejectIllegalTransition, "levee")
		}
		s.ws.Levee.Status = contracts.LeveeBreached

	case contracts.EventFireIgnited, contracts.EventFireSpread, contracts.EventFireContained:
		p, ok := parse[fireRef](ev.Payload)
		if !ok || p.FireZoneID == "" {
			return rej(ev, contracts.RejectSchema, "bad fire payload")
		}
		to := fireTo[ev.Type]
		if z, exists := s.ws.FireZones[p.FireZoneID]; exists {
			if !validation.LegalFire(z.Status, to) {
				return rej(ev, contracts.RejectIllegalTransition, "fire")
			}
			z.Status = to
			s.ws.FireZones[p.FireZoneID] = z
		} else {
			if ev.Type != contracts.EventFireIgnited {
				return rej(ev, contracts.RejectReferentialIntegrity, "unknown fire zone")
			}
			s.ws.FireZones[p.FireZoneID] = contracts.FireZone{ID: p.FireZoneID, Sector: p.Sector, Status: contracts.FireStatusIgnited}
		}

	case contracts.EventHospitalCapacityChanged:
		p, ok := parse[hospRef](ev.Payload)
		if !ok {
			return rej(ev, contracts.RejectSchema, "bad hospital payload")
		}
		h, ok := s.ws.Hospitals[p.HospitalID]
		if !ok {
			return rej(ev, contracts.RejectReferentialIntegrity, "unknown hospital")
		}
		if p.Occupancy < 0 {
			return rej(ev, contracts.RejectRangeSanity, "negative occupancy")
		}
		h.Occupancy = p.Occupancy
		h.Band = hospitalBand(p.Occupancy, h.Beds)
		s.ws.Hospitals[p.HospitalID] = h

	case contracts.EventShelterOccupancyChanged, contracts.EventShelterFull:
		p, ok := parse[shelRef](ev.Payload)
		if !ok {
			return rej(ev, contracts.RejectSchema, "bad shelter payload")
		}
		sh, ok := s.ws.Shelters[p.ShelterID]
		if !ok {
			return rej(ev, contracts.RejectReferentialIntegrity, "unknown shelter")
		}
		if ev.Type == contracts.EventShelterFull {
			sh.Occupancy = sh.Capacity
		} else {
			if p.Occupancy < 0 {
				return rej(ev, contracts.RejectRangeSanity, "negative occupancy")
			}
			sh.Occupancy = p.Occupancy
			if sh.Capacity > 0 && sh.Occupancy > sh.Capacity {
				sh.Occupancy = sh.Capacity // clamp (§14.2 range sanity)
			}
		}
		sh.Full = sh.Capacity > 0 && sh.Occupancy >= sh.Capacity
		s.ws.Shelters[p.ShelterID] = sh

	case contracts.EventFloodExtentUpdated:
		p, ok := parse[floodRef](ev.Payload)
		if !ok {
			return rej(ev, contracts.RejectSchema, "bad flood payload")
		}
		for _, pg := range p.Polygons {
			if pg.DepthM < 0 {
				return rej(ev, contracts.RejectRangeSanity, "negative flood depth")
			}
		}
		s.ws.Flood.Polygons = p.Polygons

	case contracts.EventResourceDeployed, contracts.EventResourceDepleted:
		p, ok := parse[resRef](ev.Payload)
		if !ok {
			return rej(ev, contracts.RejectSchema, "bad resource payload")
		}
		r, ok := s.ws.Resources[p.ResourceID]
		if !ok {
			return rej(ev, contracts.RejectReferentialIntegrity, "unknown resource")
		}
		if ev.Type == contracts.EventResourceDeployed {
			r.Deployed += p.Count
		} else {
			r.Deployed -= p.Count
		}
		if r.Deployed < 0 {
			r.Deployed = 0
		}
		if r.Deployed > r.Count {
			r.Deployed = r.Count
		}
		s.ws.Resources[p.ResourceID] = r
	}
	return nil
}

// sector resolves the sectorRef payload to a tracked sector.
func (s *Store) sector(ev contracts.Event) (contracts.Sector, *contracts.RejectionError) {
	p, ok := parse[sectorRef](ev.Payload)
	if !ok {
		return contracts.Sector{}, rej(ev, contracts.RejectSchema, "bad sector payload")
	}
	sec, ok := s.ws.Sectors[p.Sector]
	if !ok {
		return contracts.Sector{}, rej(ev, contracts.RejectReferentialIntegrity, "unknown sector")
	}
	return sec, nil
}

// hospitalBand maps occupancy/beds to the §8.4 band.
func hospitalBand(occ, beds int) contracts.HospitalBand {
	if beds <= 0 {
		return contracts.HospitalNormal
	}
	switch r := float64(occ) / float64(beds); {
	case r < 0.7:
		return contracts.HospitalNormal
	case r < 0.9:
		return contracts.HospitalStrained
	case r <= 1.0:
		return contracts.HospitalCritical
	default:
		return contracts.HospitalOverCapacity
	}
}

func clone(ws contracts.WorldState) contracts.WorldState {
	ws.Sectors = copyMap(ws.Sectors)
	ws.Bridges = copyMap(ws.Bridges)
	ws.Hospitals = copyMap(ws.Hospitals)
	ws.Shelters = copyMap(ws.Shelters)
	ws.FireZones = copyMap(ws.FireZones)
	ws.Resources = copyMap(ws.Resources)
	pg := make([]contracts.FloodPolygon, len(ws.Flood.Polygons))
	copy(pg, ws.Flood.Polygons)
	for i := range pg {
		pts := make([]contracts.Point, len(pg[i].Points))
		copy(pts, pg[i].Points)
		pg[i].Points = pts
	}
	ws.Flood.Polygons = pg
	return ws
}

func copyMap[K comparable, V any](m map[K]V) map[K]V {
	out := make(map[K]V, len(m))
	maps.Copy(out, m)
	return out
}
