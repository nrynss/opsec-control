# TASK — State-machine completeness (BUG-4 / BUG-5)

**Status:** Open — ready for an implementing agent
**Origin:** Consolidated from four LLM code reviews (`review_deepseek4.md`, `review_deepseek4pro.md`, `review_grokbuild.md`, `review_mimov25pro.md`), verified against live code on 2026-06-29.
**Type:** Contract change (SPEC §0.5) + state/validation/anomaly implementation.
**Severity:** Medium. Not a demo blocker in mock mode; becomes real once cells run on live inference, because cells reason over a snapshot that does not reflect the triggering event.

---

## 0. TL;DR for the implementing agent

There are event types and status enum values that are defined but have **no effect on world state**:

| # | Symbol | Defined in | Problem | Recommended fix |
|---|--------|-----------|---------|-----------------|
| 4a | `EventRoadBlocked` | `contracts/events.go:32` | SPEC §8.4 defines a **Road** entity, but `WorldState` has no `Roads` map and `mutate()` has no handler. Accepted as a silent no-op. | **Implement** — add `Road` entity + handler (Part A1). |
| 4b | `EventBuildingCollapsed` | `contracts/events.go:29` | No "building"/structural entity exists. Used by the demo scenario (`cmd/eoc/scenario.json:44`, `evt-2`). Accepted as no-op. | **Decide** — Part B (document as trigger-only *or* add sector structural-damage). |
| 4c | `EventTunnelClosed` | `contracts/events.go:33` | No Tunnel entity in SPEC §8.4 or `WorldState`. Accepted as no-op. | **Decide** — Part B (document as trigger-only *or* add Tunnel entity). |
| 5a | `BridgeCollapsed` status | `contracts/state.go:79` | Valid forward state (rank 3) but **no event ever reaches it**. The anomaly detector branch at `anomaly/detector.go:107` is dead. | **Implement** — add `EventBridgeCollapsed` + handler (Part A2). |
| 5b | `PowerPartial` status | `contracts/state.go:46` | Valid intermediate state (rank 1) but **no event ever reaches it**; `EventPowerFailure` jumps straight to `PowerOff`. | **Implement** — add `EventPowerDegraded` + handler (Part A3). |

This is **NOT** a malfunction today — these events are accepted and bump `Version`/`Time` (see the documented behavior at `internal/state/store.go:131-134`). The fix closes the gap between the contract taxonomy / SPEC §8.4 and the state machine so that an event which *says* "a road is blocked" actually *changes* the world the cells observe.

---

## 1. Hard constraints — READ BEFORE EDITING

This repo is built by multiple agents in parallel under a strict protocol. See `AGENTS.md` and `SPEC.md` §0.

1. **`internal/contracts/` is change-controlled (SPEC §0.5).** You may not casually edit it.
   - Contract changes are **additive by default**. Everything in Part A is additive (new event-type constants, a new entity type, a new map field) — no existing field changes type or meaning.
   - The contract change lands as **its own isolated commit touching only `contracts/`** (SPEC §0.7: "never mix a contract change with an implementation in the same commit").
   - Commit message prefix: `contract(events):` / `contract(state):`.
2. **`internal/state` is the sole world-state mutator; `internal/validation` owns the §14.2 rules.** All `mutate()` and `Legal*()` changes go here. (SPEC §16 ownership, boundary note at `SPEC.md:241`.)
3. **`internal/anomaly` may not mutate state or invoke cells** — it only classifies. Adding a wake-rule for a new event is fine; it already handles `RoadBlocked`/`TunnelClosed`/`BuildingCollapsed` at `detector.go:56-58`.
4. **Determinism is law (SPEC §0.2 r5).** No wall-clock, no unseeded `rand`, no map-iteration-order affecting output. Snapshot cloning must stay deep — if you add a `Roads` map, extend `clone()` / `copyMap` in `internal/state/store.go:328`.
5. **Tests live with the code, and the shared contract suite must stay green** (`internal/contracts/contracttest/`, SPEC §0.6). Add round-trip cases for any new contract type/enum.

**Suggested commit sequence (one concern per commit):**
1. `contract(events): add EventBridgeCollapsed, EventPowerDegraded, EventRoadBlocked entity`
   *(plus `contract(state): add Road entity + Roads map` — may be the same isolated contracts-only commit since both are contracts/)*
2. `feat(state): handle bridge-collapse, power-degrade, road-blocked transitions`
3. `feat(validation): add LegalRoad (bidirectional) + knownTypes for new events`
4. `feat(anomaly): wake rules for new events` *(if any new event types are added)*
5. `test(contracttest): round-trip Road + new event types`
6. `docs(scenario): exercise new events in demo scenario` *(optional, see Part C)*

---

## 2. Current behavior (verified facts)

- `internal/state/store.go` `mutate()` (switch starts line 135) has cases for: bridge damaged/closed, power failure, gas/water/comms, dam stress, levee breach, fire, hospital, shelter, flood, resource. It has **no** case for `RoadBlocked`, `TunnelClosed`, `BuildingCollapsed`, so they hit the implicit `return nil` (accepted, no mutation).
- `internal/validation/validate.go`:
  - `bridgeRank` (line 44) already includes `BridgeCollapsed: 3`. So `LegalBridge(BridgeClosed, BridgeCollapsed)` is **already true** — only the event + handler are missing.
  - `powerRank` (line 47) already includes `PowerPartial: 1`. So `LegalPower(PowerOn, PowerPartial)` is **already true** — only the event + handler are missing.
  - The generic `forward[T]()` (line 41) is strict-forward (`rank[to] > rank[from]`). It is **not** suitable for Road, which SPEC §8.4 marks **bidirectional** (`open ↔ congested ↔ blocked`). Road needs its own rule.
- SPEC §8.4 table (`SPEC.md:277-288`) authoritatively defines: **Road** = `open ↔ congested ↔ blocked` (bidirectional). It does **not** define Tunnel or Building entities. §8.5 narrative mentions building collapses but assigns them no tracked entity.
- `cmd/eoc/scenario.json:44` (`evt-2`) emits `BuildingCollapsed` with payload `{"sector": "S-HIGHGATE"}` — so 4b is exercised by the shipped demo.

---

## 3. Part A — Implement (clear wins, do all three)

### A1. Road entity + `EventRoadBlocked` handler

**Contract (`contracts/state.go`) — additive:**
```go
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
```
Add to `WorldState`:
```go
Roads map[RoadID]Road `json:"roads"`
```

**Validation (`internal/validation/validate.go`):** Road is bidirectional, so do **not** use `forward()`. Add:
```go
// LegalRoad allows any transition between distinct road states (bidirectional,
// §8.4); a no-op (to == from) is rejected for consistency with the other
// "must actually change" transitions.
func LegalRoad(from, to RoadStatus) bool { return from != to && validRoad(to) }
```
(Define a small set/validity check for `to`. Match the existing file's style.)

**State (`internal/state/store.go`):**
- Add a `roadRef` payload struct (mirror `bridgeRef`): `{ RoadID RoadID `json:"roadId"` }`.
- Add a `mutate()` case:
```go
case contracts.EventRoadBlocked:
	p, ok := parse[roadRef](ev.Payload)
	if !ok {
		return rej(ev, contracts.RejectSchema, "bad road payload")
	}
	r, ok := s.ws.Roads[p.RoadID]
	if !ok {
		return rej(ev, contracts.RejectReferentialIntegrity, "unknown road")
	}
	if !validation.LegalRoad(r.Status, contracts.RoadBlocked) {
		return rej(ev, contracts.RejectIllegalTransition, "road")
	}
	r.Status = contracts.RoadBlocked
	s.ws.Roads[p.RoadID] = r
```
- Ensure `New()` initializes `Roads` non-nil (mirror the other maps at `store.go:24-41`).
- Extend `clone()` (`store.go:328`) to deep-copy `Roads` via `copyMap`.

> Note: only `EventRoadBlocked` exists in the taxonomy today. The bidirectional `↔ congested ↔ open` half of the SPEC model has no events yet. Adding `EventRoadCongested` / `EventRoadCleared` is optional and out of scope unless you also add them to the taxonomy; `LegalRoad` is written bidirectionally so they slot in later without a validation change.

### A2. `EventBridgeCollapsed` → make `BridgeCollapsed` reachable

**Contract (`contracts/events.go`)** — additive, add next to the other structural events (line ~31):
```go
EventBridgeCollapsed EventType = "BridgeCollapsed"
```

**State (`internal/state/store.go`)** — extend the existing bridge case (line 137). `bridgeRank` already ranks `collapsed: 3`, so `LegalBridge` already permits `closed → collapsed`:
```go
case contracts.EventBridgeDamaged, contracts.EventBridgeClosed, contracts.EventBridgeCollapsed:
	// ... existing parse + lookup ...
	to := contracts.BridgeRestricted
	switch ev.Type {
	case contracts.EventBridgeClosed:
		to = contracts.BridgeClosed
	case contracts.EventBridgeCollapsed:
		to = contracts.BridgeCollapsed
	}
	// ... existing LegalBridge check + assignment ...
```

**Validation (`internal/validation/validate.go`):** add `EventBridgeCollapsed` to the `knownTypes` map (line 9). No rank change needed.

**Anomaly (`internal/anomaly/detector.go`):** add `EventBridgeCollapsed` to the structural wake case (line 56). The state-threshold branch at line 107 already checks `BridgeCollapsed` and stops being dead code once the state is reachable.

### A3. `EventPowerDegraded` → make `PowerPartial` reachable

Mirror the bridge pattern: a dedicated event for the intermediate state, parallel to `EventPowerFailure` for the terminal state.

**Contract (`contracts/events.go`)** — additive, add near `EventPowerFailure` (line 38):
```go
EventPowerDegraded EventType = "PowerDegraded"
```

**State (`internal/state/store.go`)** — add a case (or generalize the existing `EventPowerFailure` case at line 156). `powerRank` already ranks `partial: 1`, so `LegalPower(on, partial)` already holds:
```go
case contracts.EventPowerDegraded:
	sec, re := s.sector(ev)
	if re != nil {
		return re
	}
	if !validation.LegalPower(sec.Power, contracts.PowerPartial) {
		return rej(ev, contracts.RejectIllegalTransition, "power")
	}
	sec.Power = contracts.PowerPartial
	s.ws.Sectors[sec.ID] = sec
```

**Validation:** add `EventPowerDegraded` to `knownTypes`.
**Anomaly:** add `EventPowerDegraded` to the utility wake case (`detector.go:66`).

---

## 4. Part B — Decision required: Building & Tunnel

These two have **no SPEC §8.4 entity**. Pick one option per event and apply consistently.

### Option B1 (recommended for MVD) — document as anomaly-trigger-only
Keep them as accepted-no-mutation, but make that **intentional and explicit** instead of incidental:
- Add a short comment block above `mutate()` (extend the note at `store.go:131-134`) listing `BuildingCollapsed` and `TunnelClosed` as deliberate trigger-only events (they wake cells via the anomaly detector but track no entity in the MVD world model).
- Add a one-line note in `SPEC.md` §8.4 (this is documentation, not a contract type change) stating Tunnel/Building are event-only signals for the MVD.
- Add a state test asserting they are **accepted with no entity change** (so the behavior is pinned and can't silently regress into an error). See §5.

### Option B2 (full fidelity) — add tracked state
- **Building:** add a per-sector indicator to `Sector` (e.g. `StructuralDamage bool` or an `int` count). `EventBuildingCollapsed` payload already carries `{"sector": ...}`, so reuse `sectorRef`. This is a contract change to `Sector` (still additive).
- **Tunnel:** add a `Tunnel` entity mirroring `Road` (states + `Tunnels` map + handler + `clone()` + `New()` init). Bigger surface; only do this if the demo will visibly use it.

> Recommendation: **B1.** It closes the "silent no-op" concern with the least surface area, keeps the demo scenario valid, and leaves B2 as a clean follow-up. If you choose B1, `BuildingCollapsed` staying a no-op is now a *documented contract*, not a bug.

---

## 5. Tests to add (required)

- `internal/contracts/contracttest/`: round-trip marshal/unmarshal for `Road`/`RoadStatus` (and `Tunnel`/`Sector.StructuralDamage` if B2). Add the new `EventType` constants to the event-type uniqueness test.
- `internal/validation/validate_test.go`: `LegalRoad` cases — `open→blocked` legal, `blocked→blocked` illegal (no-op), `blocked→open` legal (bidirectional). Confirm new events pass `KnownType`.
- `internal/state/store_test.go`:
  - `EventRoadBlocked` on a known road → status becomes `blocked`, version++; unknown road → `RejectReferentialIntegrity`; bad payload → `RejectSchema`.
  - `EventBridgeCollapsed`: `closed → collapsed` accepted; second `EventBridgeCollapsed` (collapsed→collapsed) rejected `RejectIllegalTransition`.
  - `EventPowerDegraded`: `on → partial` accepted; then `EventPowerFailure` `partial → off` accepted (confirms the full `on → partial → off` chain is now reachable).
  - If B1: `EventBuildingCollapsed` / `EventTunnelClosed` accepted, `Version` increments, **no entity mutated**.
- `internal/anomaly/detector_test.go`: new events wake the expected specialist cells.
- Snapshot isolation: if `Roads` (or `Tunnels`) map added, extend the existing snapshot-isolation test to prove `clone()` deep-copies it.

---

## 6. Part C — Optional: exercise in the demo scenario

`cmd/eoc/scenario.json` is a hand-curated artifact (its timestamps are not the generator's 30s spacing). If you want the new transitions visible in the demo:
- Add `S-*`-style road entities to `scenario.json`'s `initial.roads` and an `evt-*` `RoadBlocked` beat in Act 2 (the "Westbank/Southport isolated" moment, SPEC §8.5).
- This is **not** required for the bug fix; the end-to-end test (`cmd/eoc` `TestEmbeddedScenarioReplaysCleanly`) must continue to pass either way. If you add `initial.roads`, confirm `scenario.LoadJSON` and the embedded replay still produce zero rejections.

---

## 7. Acceptance criteria

- [ ] `BridgeCollapsed` and `PowerPartial` are each reachable through at least one event, proven by a state test.
- [ ] `EventRoadBlocked` mutates a `Road` entity (Part A1).
- [ ] `BuildingCollapsed` / `TunnelClosed` are either implemented (B2) or **documented as deliberate trigger-only** with a pinning test (B1).
- [ ] The anomaly detector's `BridgeCollapsed` branch (`detector.go:107`) is no longer dead.
- [ ] Contract change is an isolated `contracts/`-only commit (SPEC §0.5 / §0.7).
- [ ] All packages: `go build ./...`, `go vet ./...`, `gofmt -l .` (clean), `go test ./...` green, including `internal/contracts/contracttest`.
- [ ] `go test -race ./...` green (run in CI / a cgo-enabled env; CI has a Linux race job).
- [ ] `gopls check -severity=hint <files>` introduces no new modernize hints.

---

## 8. Verification commands

```bash
go build ./...
go vet ./...
gofmt -l .                       # expect no output
go test ./...
go test ./internal/contracts/contracttest/   # the cross-lane seam
# race (needs CGO_ENABLED=1 + a C toolchain; runs in CI on Linux):
CGO_ENABLED=1 go test -race ./internal/state/ ./internal/validation/ ./internal/anomaly/ ./cmd/eoc/
```

---

## 9. Explicitly out of scope

- The other review findings are already fixed on this branch: unseeded `rand` in `internal/llm` (now `Config.Seed`), negative-timestamp validation in `Envelope`, mock LLM honoring `ctx` cancellation, and removal of the dead `orch` field in `internal/api`. Do **not** redo these.
- Do not change the meaning of any existing contract field (additive only).
- Do not add Road `congested`/`cleared` *events* unless you also extend the taxonomy and tests; `LegalRoad` is already written to accommodate them later.
