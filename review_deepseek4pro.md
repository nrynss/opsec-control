# Bug Report — Full End-to-End Codebase Review

**Date:** 2026-06-29  
**Reviewer:** DeepSeek 4Pro Builder (glm47)  
**Repo:** `github.com/nrynss/opsec-control`  
**Spec reference:** [SPEC.md](SPEC.md) v0.2  
**Scope:** Full codebase — 52 Go source files, Astro+Svelte web frontend, embedded scenario, CI, Docker

---

## Executive Summary

The codebase is in **solid MVD shape**. All 74 tests pass in 57 named test cases (0 failures), `go vet` is clean, `gofmt` is clean, and the build succeeds. The contracts-first architecture (§0.2 rule 1) is properly respected throughout. The parallel fan-out (§1, §6) works correctly. Recent commits have fixed several issues (the Commander mock response shape, the event-loop subscription ordering bug, WebSocket done-channels removal).

I found **9 bugs** and **4 design observations** worth addressing. The most severe is a goroutine leak in the orchestrator on context cancellation (BUG-1). The most subtle is negative-timestamp acceptance (BUG-2). Several are contract/implementation gaps where event types exist but have no state handlers.

---

## BUGS FOUND

### BUG-1 [HIGH] — Goroutine leak in orchestrator on context cancellation

**File:** `internal/orchestrator/engine.go`, lines ~148-156

```go
done := make(chan struct{})
go func() {
    wg.Wait()
    close(done)
}()

select {
case <-done:
    // ...
case <-ctx.Done():
    return contracts.CommonOperationalPicture{}, ctx.Err()
}
```

**Description:** When `ctx.Done()` fires first while specialist cells are still running, `FanOut()` returns an error promptly — but the goroutine running `wg.Wait()` continues blocking until **all** background cells finish. These cells use `time.Sleep(delay)` (in the mock) or HTTP calls with a 45s timeout (in real LLM). The goroutine leaks for the full duration.

**Impact:** In the demo, if an LLM timeout occurs during a fan-out, the goroutine holds resources and, more critically, the cell's `mockCell.Analyze()` still runs `time.Sleep(5*time.Second)` because the context is not checked during sleep. With a real Cerebras client, the HTTP request may or may not propagate cancellation depending on network state. The leaked goroutine won't crash the process but contributes to the concurrency budget being consumed unnecessarily.

**Fix:**
```go
select {
case <-done:
    // all finished
case <-ctx.Done():
    go func() { wg.Wait() }() // drain in background
    return contracts.CommonOperationalPicture{}, ctx.Err()
}
```

**Severity:** High. Could waste the 4-concurrent Cerebras ceiling (see HANDOFF §6) during a live demo if any cell times out.

---

### BUG-2 [MEDIUM] — `validation.Envelope` accepts negative timestamps

**File:** `internal/validation/validate.go`, lines ~32-40

```go
func Envelope(ev contracts.Event) *contracts.RejectionError {
    switch {
    case ev.ID == "":
        // ...
    case !KnownType(ev.Type):
        // ...
    case ev.Confidence < 0 || ev.Confidence > 1:
        // ...
    }
    return nil
}
```

**Description:** `contracts.SimTime` is `int64` (signed). The envelope validator checks ID, type, and confidence — but **not** timestamp validity. A negative timestamp passes the envelope check. The only guard is `ev.Timestamp < s.ws.Time` in `Store.Apply()`, which is a monotonicity check, not a range check. If the initial state `ws.Time = 0` and the first event has `Timestamp = -500`, it passes both checks.

**Impact:** A malformed scenario or buggy scenariogen output could set `ws.Time` negative, violating the `t=0` starting assumption. The simulation engine's delta calculation (`nextEv.Timestamp - e.current`) would produce negative deltas when replaying subsequent non-negative events, causing `Run()` to never sleep — effectively running at infinite speed.

**Fix:** Add to `Envelope()`:
```go
case ev.Timestamp < 0:
    return &contracts.RejectionError{EventID: ev.ID, Reason: contracts.RejectRangeSanity, Detail: "negative timestamp"}
```

**Severity:** Medium. Unlikely with hand-authored scenarios, but a latent validation gap.

---

### BUG-3 [MEDIUM] — LLM client determinism violation: unseeded `math/rand`

**File:** `internal/llm/client.go`, line ~97

```go
r := rand.New(rand.NewSource(time.Now().UnixNano()))
```

**Description:** This violates SPEC §0.2 Rule 5: "No wall-clock reads, no `rand` without an injected seed." The `rand` instance is used for exponential backoff jitter in `getBackoff()`. While this doesn't affect state transitions or event output (jitter is only in retry timing), it means retry behavior is non-deterministic across runs, which could cause flaky CI tests under load or near rate limits.

**HANDOFF §6 explicitly warns:** "Determinism is law: no wall-clock reads, no unseeded rand, no map-iteration-order in logic that affects state/output."

**Fix:** Accept a `Seed int64` in `Config` and use it. For production, derive it from the scenario seed. For backoff jitter, a deterministic PRNG is entirely adequate.

**Severity:** Medium. Directly contradicts the determinism contract. Does not affect the demo in mock mode.

---

### BUG-4 [MEDIUM] — `EventBuildingCollapsed` in taxonomy but has zero state effect

**File:** `internal/contracts/events.go` line 32 (defined), `internal/state/store.go` (not in mutate switch)

**Description:** `EventBuildingCollapsed` is defined in the event taxonomy, appears in `validation.knownTypes`, and is recognized by the anomaly detector (wakes Intelligence + Infrastructure). **But it is not handled in `store.mutate()`**. It falls through to the default `return nil` — accepted without modifying any entity.

Meanwhile, the embedded scenario (`cmd/eoc/scenario.json`, `evt-2`) uses `"type": "BuildingCollapsed"` with payload `{"sector": "S-HIGHGATE"}`. This event:
1. Passes validation (envelope check: known type ✓)
2. Has no clone in the store's mutate switch
3. Is accepted with no mutation
4. But still bumps `ws.Version` and `ws.Time`

The anomaly detector *does* wake Infrastructure and Intelligence cells for this event, but the world state they analyze reflects **no building-collapse-related change** — the sectors, bridges, and hospitals are unchanged.

**Impact:** Cells analyzing the post-event snapshot see no structural change. The demo still works visually because the mock LLM responses are canned text, not derived from actual state. With real LLM inference, the cells would see "nothing changed" and produce inaccurate outputs.

**Fix:** Either add building/structural-damage entities to `WorldState` and handle the event in `mutate()`, or remove the event type from the embedded scenario for MVD.

**Severity:** Medium. Functional gap between contract types and state-machine implementation.

---

### BUG-5 [MEDIUM] — `EventRoadBlocked`, `EventTunnelClosed` accepted with no state mutation

**File:** `internal/contracts/events.go` (defined), `internal/state/store.go` (not in mutate switch)

**Description:** Same pattern as BUG-4. These event types exist in the taxonomy, pass validation, and wake cells via the anomaly detector — but they have zero effect on world state. No Road or Tunnel entity exists in `WorldState`.

The anomaly detector wakes Intelligence + Infrastructure for these events, but the snapshot cells analyze contains no road/tunnel data.

**Impact:** Same as BUG-4. Cells receive a trigger event about "road blocked" but see no corresponding change in world state.

**Fix:** Add Road/Tunnel entities to contracts + handle in state, or defer these event types to post-MVD.

**Severity:** Medium. Spec/implementation gap.

---

### BUG-6 [MEDIUM] — `BridgeCollapsed` status is unreachable through any event handler

**File:** `internal/contracts/state.go` (status defined), `internal/state/store.go` (no path to set `BridgeCollapsed`)

**Description:** `BridgeCollapsed` is defined in the bridge status enum with rank 3 (after `BridgeClosed: 2`). The event handlers in `store.mutate()` handle:
- `EventBridgeDamaged` → `BridgeRestricted`
- `EventBridgeClosed` → `BridgeClosed`

There is **no** `EventBridgeCollapsed` event type in the contracts, and no handler transitions a bridge beyond `BridgeClosed`. The `BridgeCollapsed` status is part of the contract but completely unreachable.

The anomaly detector checks for `BridgeCollapsed` in state-based thresholds:
```go
if b.Status == contracts.BridgeClosed || b.Status == contracts.BridgeCollapsed {
```
This is dead code for `BridgeCollapsed`.

**Fix:** Either add `EventBridgeCollapsed` to the taxonomy + a handler that transitions `BridgeClosed` → `BridgeCollapsed`, or remove `BridgeCollapsed` as post-MVD scope.

**Severity:** Medium. Dead contract code + unreachable state value.

---

### BUG-7 [LOW] — Anomaly detector wakes Intel+Infra for unregistered cells

**File:** `internal/anomaly/detector.go`

**Description:** The anomaly detector returns `CellIntelligence` and `CellCommunications` in the wake list. However, in `cmd/eoc/main.go`, only 4 cells are registered:
```go
cells := map[contracts.CellKind]contracts.Cell{
    contracts.CellInfrastructure: agents.NewInfrastructure(llmClient),
    contracts.CellMedical:        agents.NewMedical(llmClient),
    contracts.CellPopulation:     agents.NewPopulation(llmClient),
    contracts.CellCommander:      agents.NewCommander(llmClient),
}
```

`CellIntelligence` and `CellCommunications` are **not registered**. The orchestrator handles this gracefully (logs `"cell %q not registered"` and continues), so no crash occurs. But logging shows the anomaly detector produces wake lists of 5 cells for mainshock events, of which 2 are always unregistered — generating noise in `orchestrator.FanOut`.

From the test run output:
```
[eoc] v1 MainshockOccurred → woke [Intelligence Infrastructure Medical Population Communications] → COP risk=High (1 actions)
```

The orchestrator's `specialistOutputs` contains Infrastructure, Medical, Population (3), skipping Intelligence and Communications because they fail with `"cell %q not registered"`. The Commander gets 3 specialist outputs, not 5. This silently degrades the fan-out.

**Impact:** The anomaly detector's `Classify()` is used as-is by `cmd/eoc` (not registered). The demo still works because registered cells are a superset of what produces meaningful outputs. But Intelligence (Intelligence) and Communications cells never analyze despite being woken.

**Severity:** Low. Expected MVD behavior (only 4 cells registered), but the mismatch creates misleading logs.

---

### BUG-8 [LOW] — Commander wrapped in `capturingCell` but captures input after mutation

**File:** `internal/orchestrator/engine_test.go`, `TestFanOut_CommanderReceivesPeers`

**Description:** The `capturingCell` wrapper fires `onAnalyze()` **before** calling `inner.Analyze()`. Since `inner` is `mockCell.Analyze()` which calls `m.callCount.Add(1)`, the callback fires synchronously before the cell logic. This works correctly for this test, but if someone extends `onAnalyze` to inspect the cell's *output* (not input), it would happen before the output is produced.

This is minor — `onAnalyze` is test-only and the comment says "capture the input it receives," which it does correctly.

**Severity:** Low. Test-only code with no production impact.

---

### BUG-9 [LOW] — `PowerPartial` status has no event handler

**File:** `internal/contracts/state.go` (status defined), `internal/state/store.go` (handler only transitions to `PowerOff`)

**Description:** `PowerPartial` is an intermediate power status between `PowerOn` and `PowerOff`. The event handler only handles `EventPowerFailure` → `PowerOff` (skipping `PowerPartial`). There is no event type to partially degrade power, and no handler for setting `PowerPartial`.

The `LegalPower(from, to)` validation allows `PowerOn → PowerPartial → PowerOff`, but there's no way to reach `PowerPartial` through any event. The `PowerPartial` status is unused.

**Severity:** Low. Similar to BUG-6 but with less impact since skipping an intermediate state is acceptable.

---

## DESIGN OBSERVATIONS (Non-Bugs)

### O-1: Web frontend uses hardcoded sector IDs that differ from the Go backend

**Backend (`cmd/eoc/scenario.json`):** `S-HIGHGATE`, `S-CENTRAL`, `S-IRONWORKS`, `S-HARBORSIDE`, `S-WESTBANK`, `S-SOUTHPORT`, `S-GREENFIELD`, `S-MAINOR`

**Frontend (`Dashboard.svelte`, `loadNominalState()`):** `westbank`, `greenfield`, `harborside`, `central`, `highgate`, `southport`, `ironworks`

The frontend has its own hardcoded fallback state with different ID conventions (no `S-` prefix, different naming). When the WS is connected and `kind: "state"` messages arrive from the backend, the Dashboard uses backend-provided IDs for `state.bridges` and map rendering. But the fallback demo mode (used when WS is disconnected) uses the frontend's own inconsistent IDs.

**Impact:** The frontend's hardcoded bridge IDs (`vora`, `iron`, `south-span`) don't match the backend's (`B-VORA`, `B-IRON`, `B-SOUTH`). If the Map.svelte or CellPanel.svelte components look up bridges by ID, the mismatch would cause display bugs in demo mode.

**Recommendation:** Align the frontend's fallback IDs with the backend scenario, or make the demo mode IDs a superset that matches both.

### O-2: Timeline `All()` returns shallow-copied entries with aliased Payload

**File:** `internal/timeline/timeline.go`

```go
func (t *Timeline) All() []Entry {
    t.mu.RLock()
    defer t.mu.RUnlock()
    out := make([]Entry, len(t.entries))
    copy(out, t.entries)
    return out
}
```

The comment warns: "Payload fields in returned entries are still aliases to cloned storage; callers should treat them as read-only." This is documented but fragile — a consumer mutating `entry.Event.Payload` could corrupt the timeline. The `api.toFlatEvents()` extracts `entry.Event` by copy (value type), which is safe, but direct consumers of `All()` are at risk.

**Recommendation:** Consider returning a deep copy or making `Entry.Event` immutable via interface.

### O-3: `api.Server` has unreachable `orch` field

**File:** `internal/api/api.go`

The `Server` struct fields:
```go
type Server struct {
    store contracts.StateStore
    bus   contracts.EventBus
    orch  contracts.Orchestrator  // <-- never used
    log   EventLog
    cop   COPProvider
}
```

The `orch` field is stored but never accessed. No handler calls the orchestrator. It was wired in the constructor and passed to `Register()` but never consumed. This is harmless — `cmd/eoc` calls `FanOut` directly in its own loop, not through the API — but the unused field is dead code.

### O-4: `scenariogen` generates 30-second spacing, but the demo scenario has varied spacing

**File:** `internal/scenariogen/generator.go`, line ~80

```go
currentSimTime += 30 // 30s between beats
ev.Timestamp = currentSimTime
```

The demo scenario (`cmd/eoc/scenario.json`) has timestamps at 0, 10, 20, 30, 40, 50, 70, 80, 90, 100, 110, 130, 140, 150, 160, 170 — with gaps between 10-30 seconds, not strictly 30. The real scenario was hand-edited after generation or produced by a different run. Re-running the generator would produce a different (but valid) scenario.

**Recommendation:** Document that the embedded scenario is a hand-curated artifact, not the generator's direct output.

---

## TEST COVERAGE GAPS

| Area | Gap |
|---|---|
| `Envelope` negative timestamp | No test for `Timestamp = -1` |
| `EventDamStressElevated` no-op on already-stressed dam | Not tested |
| `EventLeveeBreached` no-op on already-breached levee | Not tested |
| `EventPowerFailure` from `PowerPartial` to `PowerOff` (skip rank) | Not tested |
| `EventFireContained` → `EventFireIgnited` (backward) | Not tested |
| `EventBridgeDamaged` on already-closed bridge (backward) | Not tested (correctly rejected, but untested path) |
| `EventFloodExtentUpdated` with decreasing depth (monotonicity) | Not tested |
| `store.mutate` panic on concurrent Apply calls | Not tested (the mutex protects, but there's no concurrent Apply test) |
| WebSocket `Broadcast` to a client whose write channel is full | Not tested |
| Scenario generator with empty LLM output (`[]`) | Not tested (only malformed JSON tested) |

---

## CROSS-LANE CONTRACT COMPLIANCE

All packages correctly depend on `contracts/*` interfaces and types. No package reaches into another package's internals. The ownership table from SPEC §16.1 is followed.

Verified:
- ✅ `state` is the sole world-state mutator
- ✅ `orchestrator` is the only Cell invoker; fan-out is concurrent
- ✅ `api` contains no operational logic, only serialization
- ✅ `events` owns no domain state
- ✅ `anomaly` does not mutate state or invoke cells
- ✅ `timeline` is append-only
- ✅ `scenariogen` is offline-only (no live-path invocation)

---

## DETERMINISM CHECK

| Check | Status |
|---|---|
| No wall-clock reads in state logic | ✅ All time is `SimTime` |
| No unseeded `rand` in state/anomaly/orchestrator | ✅ (except BUG-3 in llm) |
| Map iteration in anomaly detector | ⚠️ Uses `map` then fixed-order slice — determined |
| Map iteration in state snapshots | ✅ `maps.Copy` + `encoding/json` sorted output |
| Simulation engine | ✅ Pure replay from ordered events |
| Timeline | ✅ Append-only in event-bus order |

---

## SUMMARY TABLE

| ID | Severity | Package | Issue |
|---|---|---|---|
| BUG-1 | **HIGH** | orchestrator | Goroutine leak on context cancellation |
| BUG-2 | MEDIUM | validation | Negative timestamps accepted by envelope check |
| BUG-3 | MEDIUM | llm | Determinism violation: unseeded random for backoff jitter |
| BUG-4 | MEDIUM | state | `EventBuildingCollapsed` has zero state effect |
| BUG-5 | MEDIUM | state | `EventRoadBlocked`/`EventTunnelClosed` have zero state effect |
| BUG-6 | MEDIUM | state | `BridgeCollapsed` status is unreachable |
| BUG-7 | LOW | anomaly | Classifier wakes unregistered cells (Intel/Comms) |
| BUG-8 | LOW | orchestrator/test | Capturing cell callback ordering (test-only) |
| BUG-9 | LOW | state | `PowerPartial` status has no event handler |

**Observations:**
| O-1 | LOW | web | Frontend fallback state uses different entity IDs than backend |
| O-2 | LOW | timeline | `All()` returns aliased payload references |
| O-3 | LOW | api | Unused `orch` field |
| O-4 | INFO | scenariogen | Generator spacing doesn't match demo scenario |

---

## OVERALL ASSESSMENT

The codebase is **demo-ready** with 5-minute fixes for BUG-1 (goroutine leak) and BUG-2 (negative timestamp). The remaining issues are contract completeness gaps that don't affect the MVD video but should be addressed before expanding the cell count or using real Cerebras inference. All 74 tests pass, contracts are clean, and the architecture faithfully implements the SPEC.