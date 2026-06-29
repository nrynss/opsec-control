# Codebase Review — opsec-control (Grok Build, 2026-06-29)

**Reviewer:** Grok 4.3 (Builder)  
**Date:** 2026-06-29  
**Scope:** Full end-to-end codebase review (backend Go packages, contracts, cmd, simulation, web frontend, build artifacts, adherence to SPEC.md v0.2 + AGENTS.md). Inspected all internal packages, key tests, scenario, wiring in cmd/eoc, frontend components, cross-cutting concerns. Verified current state via `go test ./...`, `go vet ./...`, `go build ./...` (all clean).  
**Prior reviews referenced for delta:** review_deepseek4pro.md, review_mimov25pro.md, reviews/*.md (issues re-checked against live code).

---

## Executive Summary / Verdict

**PASS (MVD-complete, production-grade skeleton).** All 15 test packages green. `go test ./...`, `go vet`, `go build` clean. The reasoning spine (scenario → simulation → EventBus → state.Apply (§14.2) → anomaly.Classify → orchestrator.FanOut (concurrent) → Cells (mock LLM) → Commander COP synthesis → WS/API) works end-to-end. 16/16 embedded scenario events apply cleanly and drive fan-outs + COPs.

The project demonstrates strong adherence to the **Multi-Agent Development Protocol** (SPEC §0). Contract-first, one-mutator state, interfaces, concurrent-only Cell invocation, determinism in core paths, and lane discipline are largely respected. Previous seam issues (e.g., anomaly/orchestrator empty-wake + Commander phase-2) appear resolved in current code.

**Notable remaining issues (mostly known MVD simplifications or low-severity):**
- 1 HIGH (goroutine leak on ctx cancel in orchestrator).
- 2 MEDIUM (determinism violation in llm; validation gap on negative timestamps).
- Several MEDIUM/LOW contract-vs-impl gaps (no-op events, unreachable statuses).
- Frontend/backend ID drift in demo mode.
- Schemas/ directory is placeholder-only.

The demo is **ready for the video** (recorded replay mitigates live LLM variance). Expanding beyond MVD (real Cerebras, all 6 cells, live perception) will surface the remaining gaps.

**Overall grade: A-** (strong architecture + tests; minor hygiene + completeness debts).

---

## 1. Test & Build Results

```
ok  cmd/eoc
ok  internal/agents
ok  internal/anomaly
ok  internal/api
ok  internal/contracts/contracttest
ok  internal/events
ok  internal/llm
ok  internal/orchestrator
ok  internal/scenario
ok  internal/scenariogen
ok  internal/simulation
ok  internal/state
ok  internal/timeline
ok  internal/validation
ok  internal/websocket
```

- `go vet ./...`: clean.
- `go build ./...`: clean.
- Contract roundtrips + uniqueness + rejection `errors.As` pass.
- Critical E2E: `TestEmbeddedScenarioReplaysCleanly` (cmd/eoc) replays all 16 events with zero rejections, produces fan-outs + non-empty COP.
- `internal/llm` tests take longest (retries/concurrency simulation) but pass.

---

## 2. SPEC §0 Protocol & Ownership Compliance

| Rule | Status | Notes |
|------|--------|-------|
| §0.2 r1 Contract-first | ✅ | All cross-boundary types in `internal/contracts/*`. No package defines its own Event/State/CellOutput shapes. |
| §0.2 r2 Stay in lane | ✅ | `internal/state` sole mutator. `orchestrator` sole Cell invoker. `api`/`websocket` thin. `scenariogen` offline. |
| §0.2 r3 Depend on interfaces | ✅ (mostly) | Core paths use `contracts.StateStore`, `EventBus`, `Cell`, `Orchestrator`, `Classifier`, `LLMClient`. Minor: `api` uses `timeline.Entry` in its local `EventLog` iface (see §7). |
| §0.2 r4 No shared mutable global state | ✅ | Only `state.Store` holds live `WorldState`. No package-level vars for world state elsewhere. `copStore` in cmd/eoc is narrow (latest COP only). |
| §0.2 r5 Determinism | ✅ core / ⚠️ llm | SimTime, scenario Seed, clone() + maps.Copy, fixed-order wake lists, sorted JSON. **Violation in llm/client.go** (see BUG-3). |
| §0.2 r6 Mock deps | ✅ | Every package testable with fakes (see cmd/eoc main_test fakeLLM, orchestrator/engine_test mockCell, etc.). |
| §0.2 r7 Tests live with code + contracttest | ✅ | All packages have *_test.go. Full `contracttest` suite passes. |

**§16.1 Ownership table** followed. No cross-lane edits observed. `contracts/` untouched since v0 (no unilateral changes).

**cmd/eoc** correctly acts as composition root (wires, owns no domain logic).

---

## 3. Architecture Fidelity (The Spine)

```
embedded scenario.json → scenario.Load → simulation.Engine (deterministic replay)
  → EventBus → state.Store.Apply (validation gate + mutate + version++)
    → anomaly.Detector.Classify (event + state thresholds)
      → orchestrator.Engine.FanOut (concurrent specialists → phase-2 Commander)
        → agents.*Cell (LLM or mock) → COP
          → copStore + WS Broadcast + API
```

- **Parallel fan-out**: Correct use of goroutines + WaitGroup. Commander always runs as phase-2 (even on 0 specialists) **when registered** (fixed from prior seam report).
- **State**: Sole mutator path, deep clone on Snapshot, dedup, monotonic time, legal transitions via validation. forward() rank checks good.
- **Anomaly**: Comprehensive switch on every EventType + state-based thresholds (bridges closed, dam stressed, hospitals critical, shelters full, fire spreading, power off, flood present). Returns specialists-only; fixed order slice for determinism.
- **Cells**: 4 implemented and registered (Infra/Medical/Population/Commander). Intelligence/Communications defined in contracts but not wired (see BUG-7).
- **LLM client**: Bounded semaphore (default 4), retry with backoff + Retry-After, strict schema cleaning (`additionalProperties: false`), mock mode when no key.
- **Simulation**: Pure replay on SimTime deltas. Run() pacing uses wall time only for sleep (documented). Reset interrupts cleanly.
- **Timeline**: Append-only, Payload cloned on append for immutability. Listens via bus.
- **API/WS**: Thin (serialize + forward). POST /events just publishes (validation deferred to state). 501s on scenario load/reset (MVD).
- **Web**: Self-contained Astro/Svelte demo + live WS mode. Visualizes state/COP. No operational logic.

---

## 4. Bugs & Issues (Current Code State)

### BUG-1 [HIGH] — Goroutine leak on context cancellation in FanOut

**File:** [internal/orchestrator/engine.go](/internal/orchestrator/engine.go) ~95-105

```go
done := make(chan struct{})
go func() { wg.Wait(); close(done) }()

select {
case <-done:
case <-ctx.Done():
	return contracts.CommonOperationalPicture{}, ctx.Err()
	// Leaked: the wg waiter goroutine keeps running until cells finish
}
```

Same pattern reported previously. No background drain. Cells may do `time.Sleep` (mocks) or long HTTP (real client 45s timeout). Wastes Cerebras concurrency budget on timeout paths.

**Fix sketch (prior review):** On ctx.Done, `go func(){ wg.Wait() }()` then return.

### BUG-2 [MEDIUM] — validation.Envelope accepts negative timestamps

**File:** [internal/validation/validate.go](/internal/validation/validate.go) ~30-40

Only checks ID, KnownType, confidence ∈[0,1]. No `Timestamp < 0`. Monotonicity guard in Apply (`< s.ws.Time`) allows first event `ts=-500` when ws.Time==0 → negative world time, negative deltas in sim Run().

**Recommendation:** Add `case ev.Timestamp < 0:` rejection (RejectRangeSanity).

### BUG-3 [MEDIUM] — Determinism violation: unseeded rand + wall time in LLM client

**File:** [internal/llm/client.go](/internal/llm/client.go) ~97, ~210-230

```go
r := rand.New(rand.NewSource(time.Now().UnixNano()))
...
jitter := c.rand.Float64() * half
...
if d, ok := parseRetryAfter(..., time.Now())
```

Violates SPEC §0.2 r5 and HANDOFF §6 explicitly. Jitter/retry timing non-deterministic. Affects only backoff, not state/output, but still a rule violation. CI flakiness possible near rate limits.

**Fix:** Accept seed in Config (derive from scenario seed), or use deterministic source for jitter.

### BUG-4 [MEDIUM] — Multiple event types accepted with zero state mutation

**Files:** [internal/contracts/events.go](/internal/contracts/events.go), [internal/state/store.go](/internal/state/store.go) `mutate()`

`EventBuildingCollapsed` (used in embedded scenario evt-2), `EventRoadBlocked`, `EventTunnelClosed`, `EventAftershockForecastUpdated`, most perception/casualty/citizen/evac events, some utility/fire have **no case** in mutate switch → accepted as no-op (version++ and time advance only).

Anomaly still wakes cells (correctly). Cells see no structural change in snapshot.

**Impact:** For MVD/demo with canned mocks this is invisible. With real LLM inference, cells reason over stale substrate.

**Status:** Documented MVD simplification ("seismic, perception, citizen reports... are accepted without mutation").

### BUG-5 [MEDIUM/LOW] — Unreachable statuses in contracts/state

- `BridgeCollapsed` defined (rank 3) but no `EventBridgeCollapsed` type and no mutate path beyond Closed.
- `PowerPartial` defined (rank 1) but only `EventPowerFailure` → Off; no event or path to Partial.
- Anomaly checks `BridgeCollapsed` (dead code path).

### BUG-6 [LOW] — Anomaly wakes unregistered cells

**Files:** [internal/anomaly/detector.go](/internal/anomaly/detector.go), [cmd/eoc/main.go](/cmd/eoc/main.go) ~155

Mainshock/aftershock + state thresholds wake `CellIntelligence` + `CellCommunications`. Only 4 cells registered. Orchestrator logs `"cell %q not registered"` and skips. Result: Commander receives 3 (or fewer) specialist outputs instead of 5. Logs are noisy.

**MVD expected**, but detector produces superset of reality.

### BUG-7 [LOW] — Unused field + 501 surface

- `internal/api/api.go: Server.orch` stored in ctor but never read (O-3 prior).
- `/scenario/load`, `/scenario/reset` return 501 (expected for MVD).

### BUG-8 [LOW] — Context handling in llm + retry uses wall time

`parseRetryAfter` + `time.Now()` calls. Acceptable for I/O timing but contributes to "no wall-clock" spirit violation.

---

## 5. Design Observations & Gaps vs SPEC

| ID | Area | Observation |
|----|------|-------------|
| O-1 | Frontend demo | Hardcoded fallback state uses `westbank`/`highgate` + `vora`/`iron` (no S-/B- prefixes) vs backend `S-HIGHGATE`/`B-VORA`. Demo mode works standalone; live WS path uses real data. Risk of lookup bugs if components key by ID in demo. |
| O-2 | Timeline | `All()`/`Since()` return shallow copies of `Entry`; Payload alias warning exists but callers must be careful. |
| O-3 | Schemas | `internal/contracts/schemas/` contains only README. No actual *.json. Frontend + validation rely on runtime Go shapes or ad-hoc. SPEC §0.4 and web/README expect mirrored schemas. |
| O-4 | Scenariogen | Generator produces fixed 30s spacing; shipped `scenario.json` has variable (10-30s). Curated artifact, not direct generator output. |
| O-5 | Cells | Mock responses in `llm.completeMock` are string-contains heuristics on prompts + hardcoded JSON. Fine for demo; real inference will be richer. |
| O-6 | MVD simplifications (documented) | Only 4/6 cells; no Perception impl (sim is source); hospital triggers only on Critical+ (not >=85%); no delta clustering or confidence weighting; sensors stub; no live image→event. |
| O-7 | API EventLog | Uses `timeline.Entry` directly instead of a pure contracts shape. Minor boundary smell but practical. |
| O-8 | Goroutine in sim Run | Documented re-check after sleep for concurrent Reset; tests exist. |

---

## 6. Determinism Audit

- **Core path**: Excellent. `SimTime` everywhere that matters. No `rand` in state/anomaly/orchestrator/simulation/agents (except test timing). Map clones via `maps.Copy`. JSON roundtrips deterministic.
- **llm**: Jitter + `time.Now()` (BUG-3).
- **Tests**: Some use `time.Now()`/`time.Sleep` for concurrency timing assertions (orchestrator/engine_test, simulation). Acceptable.
- **Scenario replay**: Fully deterministic given same JSON + seed.

---

## 7. Cross-Package & Lane Hygiene

- No package mutates world state except via `state.Store`.
- No sequential Cell calls.
- `scenariogen` is offline (uses throwaway state for validation replay).
- `api`/`web` contain zero operational logic.
- Imports: state package only touched by cmd/eoc (wiring), internal/scenariogen (validation), and itself. All others go through `contracts.StateStore`.
- cmd/eoc and tests correctly use fakes for isolation.

**One minor note:** `api` test and impl import `internal/timeline` for the `EventLog` adapter interface. This is not a state violation, but if strict "contracts only" is desired, a small contracts addition or `[]contracts.Event` projection could be considered.

---

## 8. Test Coverage Notes

Strong coverage of:
- Every EventType in anomaly.
- Legal/illegal transitions, rejections, hospital bands, snapshot isolation in state.
- Concurrent fan-out timing, Commander peers, fallbacks, unregistered cells, error surfacing in orchestrator.
- Bus pub/sub, slow consumers, FIFO, cancel.
- Sim step/run/reset/pacing/determinism.
- LLM mock, schema cleaning, retries, concurrency cap, ctx cancel.
- Timeline immutability (clone on append).
- Contract roundtrips + error types.

**Gaps** (non-blocking for MVD):
- No explicit test for `Envelope` negative timestamp.
- No concurrent Apply stress test (mutex protects, but untested).
- No test of WS Broadcast under full write-buffer or client disconnect mid-broadcast.
- Scenariogen empty LLM output path lightly covered.
- No test exercising BuildingCollapsed et al. causing meaningful state (by design).
- Frontend has no automated tests in repo (demo-driven).

---

## 9. Build, Cross-Platform, Hygiene

- **Taskfile.yml**: Cross-platform, uses `{{exeExt}}`, no dangerous one-liners.
- **Dockerfile**: Multi-stage, distroless, non-root.
- **.gitattributes**: Full LF enforcement (including *.go *.json *.svelte etc.). Good.
- **go.mod**: toolchain 1.24.5 pinned.
- **//go:embed**: Used for scenario.json. No runtime `os.ReadFile` on assets in core path.
- **No hardcoded `/` or `\`** observed in Go logic (uses maps/ids).
- **web/**: Has dist/ (committed for demo?), node_modules present in tree (should be gitignored in practice). package.json scripts not deeply inspected but README acknowledges cross-platform rules.
- CI (implied): go + race + docker jobs mentioned in prior reviews.

---

## 10. Recommendations (Priority Order)

1. **Fix BUG-1** (orchestrator ctx leak) before any live-Cerebras or long-running use.
2. **Fix BUG-2** (negative timestamp in Envelope) + add test.
3. **Address BUG-3** (seed the llm rand or make jitter deterministic) to fully satisfy §0.2 r5.
4. Decide on "no-op events": either add minimal state entities (Road, Building damage counters, etc.) or document that certain EventTypes exist purely for anomaly triggering.
5. Align frontend demo fallback IDs with backend scenario (or make demo data a superset projection of contracts).
6. Populate `contracts/schemas/*.json` (or generate from Go) for frontend type safety.
7. Wire/register Intelligence + Communications (even as additional mocks) or prune them from anomaly for MVD to reduce log noise.
8. Consider removing dead `orch` field from api.Server or document its future use.
9. Add a seam-level test in `contracttest/` that drives the full Classify→FanOut→COP path for *every* EventType (prevents future drift).
10. (Stretch) Make `EventLog` interface use `[]contracts.Event` projection to keep api purely on contracts.

---

## 11. Summary Table

| ID | Severity | Package | Issue |
|----|----------|---------|-------|
| BUG-1 | HIGH | orchestrator | Goroutine leak on ctx cancel (wg waiter) |
| BUG-2 | MEDIUM | validation | Negative timestamps pass Envelope |
| BUG-3 | MEDIUM | llm | Unseeded rand + time.Now for jitter/retry |
| BUG-4 | MEDIUM | state/contracts | BuildingCollapsed / RoadBlocked / TunnelClosed etc. = no-op |
| BUG-5 | MEDIUM/LOW | state/contracts | BridgeCollapsed, PowerPartial unreachable |
| BUG-6 | LOW | anomaly + cmd | Wakes unregistered Intel/Comms |
| BUG-7 | LOW | api | Unused `orch` field; 501 endpoints |
| O-1..O-8 | LOW/INFO | various | Frontend ID drift, missing schemas, MVD gaps, timeline alias note |

**Prior high issues resolved or mitigated:**
- Concurrent WS write (now uses per-client mu + write()).
- API owning private event buffer (now uses injected timeline via EventLog iface).
- Anomaly/orchestrator seam on empty wake + Commander phase-2 (current logic always runs registered Commander).

---

## Final Assessment

This is a **disciplined, well-tested MVD** that successfully demonstrates the core thesis (one anomaly → parallel specialist fan-out + Commander synthesis) under the constraints of parallel AI Builders. The contract/seam protocol largely worked. The Go core is clean, deterministic where it matters, and correctly layered.

Remaining work is mostly **scope + polish** rather than architectural debt. With the three bugs above addressed, it would be an unqualified A for a hackathon deliverable.

**Demo video will look excellent.** The repeated visible fan-outs at speed are the product.

*Review written by Grok (Builder) after full source + execution inspection on 2026-06-29.*
