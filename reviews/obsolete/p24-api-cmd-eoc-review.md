# Deep Review: P24 (api + cmd/eoc — eocSimController, epoch guard, reset, /scenario/stats)

**Date:** 2026-06-29  
**Reviewer:** Grok (Builder, lane owner for internal/api + cmd/eoc)  
**Branch:** feat/live-simulation-controls  
**Context:** Follow-up deep review (multiple prior requests). P19 (contracts), P20 (simulation), P21 (llm counters), P22 (state), P23 (timeline) precede it. P24 marked ✅ in HANDOFF. This review covers final landed state, with fixes applied for issues discovered.

---

## Executive Summary

P24 is the integration of the simulation controls/stats surface:

- `internal/api`: delegation for controls, `GET /scenario/stats` composing `SimulationStats` from `SimulationController` + `TokenStatsProvider`.
- `cmd/eoc`: `eocSimController` (implements the controller iface), epoch guard on reasoning loop (`runLoop`/`startLoop`), full coordinated `Reset()` (pause+reset sim, state.Reset, timeline.Truncate, SystemReset synthetic, COP low, WS `{"kind":"reset"}` broadcast), wiring of `llmClient` both as perception/provider + tokenStats.

**Current status (post this review):** **PASS** (with targeted fixes applied during review). All core paths work, tests/build/contracttest green. Minor gaps remain for P26 verification.

Key accomplishments:
- Epoch guard prevents stale fan-outs/COPs after live reset (critical for single-instance live prod).
- Stats now fully wired (events from timeline len, tokens/requests from llm, wall/sim times/speed/status from engine).
- Reset fully clears state + logs + cop + tokens + broadcasts for P25 "All Clear".
- Delegation keeps api thin (no logic).
- Determinism firewall respected (wall stats isolated in P20 sim).

---

## Alignment with HANDOFF.md Design (P24 section)

**Required (exact from HANDOFF):**
- `internal/api/api.go`: Server holds `SimulationController` + `TokenStatsProvider`; `/scenario/reset|pause|...|speed` + new `GET /scenario/stats`; delegation; `handleScenarioStats` builds `SimulationStats` (uses Info/Status/Current/Wall/Speed + Total* ; Events=len(log)).
- `cmd/eoc/main.go`: `eocSimController {sim, store, tl, initial, copStore, bcast, ...}` implementing the iface.
  - Reset epoch guard: atomic epoch, cancel reasoning ctx, unsubscribe/resub to bus, new runLoop with expectedEpoch + discard check.
  - Reset actions: sim.Pause/Reset, store.Reset, tl.Truncate, SystemReset event via tl.Append, copStore Low, WS kind:"reset" broadcast.
  - Delegates for other controls.
- Start/restart loop on boot + reset.

**Actual vs spec:** Matches closely.
- All listed actions present.
- Epoch guard implemented in `runLoop` (discard if `atomic.LoadInt32(&a.epoch) != expectedEpoch`) + `startLoop`.
- Wiring correct: `api.New(..., ctrl, llmClient)`.
- Stats handler populates `EventsReplayed: len(s.log.All())`, `Inferences: reqs`, Elapsed from Current-Start, etc.
- Post-review fixes: removed double-epoch inc; added `llm.ResetStats()` call on reset path.

Minor deviations (acceptable):
- Status strings ("idle"/"running"...) cast to `SimulationStatus`; "idle" not in contract consts (no impact).
- No explicit re-launch of `sim.Run` goroutine (relies on engine's internal resetCh + paused=false for in-progress runs).

Design for live reset safety + display stats is honored.

---

## Compliance with Core Rules (AGENTS.md / SPEC.md §0)

- **Lane ownership (§16.1)**: Strictly `internal/api` + `cmd/eoc/main.go` (and minor test updates historically). No edits to state/timeline/llm/simulation/web. Good.
- **Contract-first**: Pure use of `contracts.SimulationController`, `TokenStatsProvider`, `SimulationInfo/Stats`, `SimStatus*`. No invented shapes. Cross-package only via the seams.
- **Depend on interfaces**: api uses the ifaces; cmd/eoc owns the concrete coordinator. No imports of sim/state/tl impl details from other pkgs in api.
- **No shared mutable global state**: Reset coordination uses the sanctioned `store.Reset`; epoch is atomic on app; no new globals.
- **Determinism is law**: Wall stopwatch (P20) isolated; no wall reads in event ordering, apply, fanout, or stats affecting replay. Epoch/guard is control-plane only. Engine tests assert no wall leakage into logic.
- **Mock dependencies / isolation**: api tests use nils + mocks; cmd/eoc tests cover runLoop/epoch with fakes. Full tree builds against contracts.
- **Tests live with code**: Existing coverage in api_test (stubs/delegation), main_test (runLoop + handle with epochs), engine/llm/timeline/state tests. P26 will add more (noted below).
- **One package / hygiene**: Changes scoped. (gofmt adjustment on owned file during review.)
- **No operational logic in api**: Handlers are pure delegation + response. All reset sequence lives in cmd/eoc coordinator (correct per design + SPEC "api only serializes").
- **Live/prod note**: Epoch + ctx cancel + fresh sub is the right mechanism to safely abort in-flight reasoning on a live single-instance server. WS "reset" enables P25 client clear without full reconnect.

Pre-flight checklist satisfied.

---

## Detailed Code Walkthrough

### 1. eocSimController + epoch guard (cmd/eoc/main.go)

```go
type eocSimController struct {
    ...
    app *app
    bus contracts.EventBus
    ...
    llm *llm.Client   // added for ResetStats during review
    mu  sync.Mutex
    reasoningCancel context.CancelFunc
    subCancel       func()
}

func (c *eocSimController) startLoop() {
    c.mu.Lock()
    epoch := atomic.AddInt32(&c.app.epoch, 1)  // single source of epoch bumps
    ... cancel old ...
    ch, subCancel := c.bus.Subscribe()
    ...
    go c.app.runLoop(reasoningCtx, ch, epoch)
}

func (a *app) runLoop(ctx context.Context, ch <-chan contracts.Event, expectedEpoch int32) {
    for {
        ...
        case ev, ok := <-ch:
            if atomic.LoadInt32(&a.epoch) != expectedEpoch {
                continue // discard stale post-reset
            }
            a.handle(ctx, ev)
```

`Reset()`:
- Cancels prior reasoning ctx + unsubs (aborts in-flight fanouts via ctx).
- sim.Pause + sim.Reset + store.Reset(initial) + tl.Truncate.
- llm.ResetStats() (post-review fix).
- Append SystemReset (Timestamp=0, direct to tl for UI log).
- copStore Low + "All clear" summary.
- WS broadcast `{"kind":"reset"}`.
- startLoop() (establishes next epoch + fresh sub).

Epoch starts at 0; first startLoop -> 1. Each Reset causes exactly one inc (via startLoop).

Old runLoop goroutines exit via ctx.Done() and/or epoch mismatch on any buffered events.

### 2. api delegation + stats (internal/api/api.go)

Server holds the two new deps (added in P19/P24).

Handlers (`handleScenarioReset` etc.):
```go
if s.simCtrl != nil { s.simCtrl.Reset() }
s.respondWithScenarioStub(w, "reset")
```
(similar for pause/resume/step/speed; speed parses body).

`handleScenarioStats`:
```go
if s.simCtrl == nil || s.tokenStats == nil {
    ... "not_wired"
}
info := s.simCtrl.Info()
in, out := s.tokenStats.TotalTokens()
...
stats := contracts.SimulationStats{
    Status:         contracts.SimulationStatus(s.simCtrl.Status()),
    CurrentTime:    ...,
    ElapsedTime:    s.simCtrl.CurrentTime() - info.StartTime,
    WallElapsed:    s.simCtrl.WallElapsedMS(),
    EventsReplayed: len(s.log.All()),  // post-trunc + re-accum
    ...
    Speed: s.simCtrl.Speed(),
}
```
Correct composition. Events cleared on reset via tl.Truncate + repopulated by replay listener.

respondWithScenarioStub kept for backward/partial cases (legacy note updated in spirit).

### 3. Wiring (cmd/eoc/main.go main())

- llmClient created (supports ResetStats via P21).
- sim := simulation.New(bus)
- ctrl := &eocSimController{..., llm: llmClient, ...}
- api.New(..., ctrl, llmClient)
- ctrl.startLoop()
- sim.Load + go sim.Run

Timeline listener remains attached across resets (Truncate is the clear).

### 4. Supporting pieces (P20/P21/P22/P23)
- simulation: wall helpers under lock (start/update), Status/Info/WallElapsed* , resetCh interrupt, paused=false in Reset etc.
- llm: atomics + Total*/ResetStats + record on complete/interpret paths (mock + real).
- state: Reset clears ws + seen + version/time.
- timeline: Truncate + Append (used for synthetic).

All thread-safe where required.

---

## Issues / Findings (by severity)

### Fixed During Review (High → now resolved)
1. **Epoch double-increment (cmd/eoc/main.go:193 originally)**
   - Reset did `AddInt32` then `startLoop` also did `Add`.
   - Result: epochs advanced by 2 per reset; intermediate value never used by a live loop.
   - Impact: harmless for discard (Load eventually mismatches old expected) but incorrect vs design/history ("single Add").
   - **Fix**: Removed the Add from Reset. startLoop's bump now provides the single fresh epoch on restart. Old goroutines still see Load change via ctx+check.
   - Post-fix: clean monotonic bumps (1 initial, +1 per reset).

2. **LLM stats not cleared on All Clear (missing token reset)**
   - eocSimController had no access to token provider.
   - `/scenario/stats` (and P25 widget) would keep growing tokens/inferences after reset.
   - **Fix**: Added `llm *llm.Client` field (internal to cmd/eoc), wired at construction, call `c.llm.ResetStats()` inside Reset() after Truncate.
   - Matches P21 intent ("ResetStats for All Clear") and "true All Clear" UX. No contract change needed.

### Medium (Observations / Polish)
3. **Status string vs typed const**
   - engine.Status() returns "idle" | "running" | "paused" | "complete".
   - Contract defines `SimStatusRunning` etc (no "idle").
   - Cast in api is safe (string alias) but not type-checked.
   - Low risk; "idle" only pre-Load. Consider making engine return typed const or api normalize. (P26 can assert values.)

4. **Test coverage for coordinator**
   - api_test covers only nil-ctrl stub paths + basic delegation.
   - main_test covers app.runLoop/handle with explicit epoch=0 and fakeLLM, plus bus path + ambient/reject.
   - **No direct test** for `eocSimController.Reset`, full action sequence (trunc + cop + synthetic + token reset + bcast + epoch), or stats shape with wired ctrl+tokens.
   - Epoch discard is indirectly exercised in runLoop tests.
   - **For P26**: add table tests or integration exercising ctrl.Reset + post-reset assertions on stats (events~0/1, tokens==0, status, etc.).

5. **Replay goroutine lifecycle after complete + reset**
   - sim.Run goroutine is spawned once in main; exits on scenario end (`idx >= len`).
   - Reset clears engine state (idx=0, paused=false, wall=0) and interrupts via resetCh (if goroutine still alive).
   - If called *after* natural completion, no active Run → no auto-replay of events on bus after reset (state/tl/cop cleared but clock stays "running" with no events emitted until external action).
   - Design focus was "reset on live/running sim". Acceptable for current MVD; if full "replay on All Clear" wanted, controller could manage a runCtx or expose a "play" op that (re)launches Run. Not a P24 blocker.

6. **gofmt**
   - Landed code required formatting (struct field alignment). gofmt -w applied during review (owned file).
   - No other drive-by cleans.

### Low (Nits)
- Serving log mentions GET /scenario/stats but not the control POSTs.
- Synthetic SystemReset has no Payload; direct tl.Append bypasses bus (intentional, non-triggering display marker).
- In stats: relies on log.All() after listener has appended; synthetic + first post-reset events appear promptly.
- No error returns surfaced from ctrl methods in handlers (Step error ignored; matches stub style).

No security, race, or determinism violations found. No cross-lane edits.

---

## Impact on Other Parcels / Live System

- **Unblocks P25 (web)**: /scenario/stats now returns full data; WS "reset" is broadcast; controls are live. Dashboard can poll + react to clear local feeds on kind:"reset".
- **P26**: Unit tests + manual All Clear validation on running + complete scenarios. Determinism firewall already has supporting test in engine.
- **Live (Fly single-instance)**: Epoch+cancel is exactly the mechanism to safely drop in-flight Cells/orchestrator work without leaking stale COPs or state mutations. Reset is the sanctioned exception to single-mutator.
- **Future**: If more resetters appear, could factor a `ResetStatsProvider` or call via interface; current is contained to cmd/eoc.
- No effect on existing event/fanout/COP paths.

---

## Verification Performed

- **Builds**: `go build ./...` ✅; targeted `./cmd/eoc ./internal/api` ✅ (pre + post fixes).
- **Tests**:
  - `go test ./internal/api -count=1` ✅ (controls + stubs + provider).
  - `go test ./cmd/eoc -count=1` ✅ (replay, bus path, ambient, runLoop epochs).
  - `go test ./internal/simulation -count=1 -run 'Wall|Reset|Determinism|Pause|Run'` ✅ (wall stopwatch, reset interrupt, no-wall-in-logic, paced/pause).
  - `go test ./internal/llm -count=1 -run 'Stats|Reset|Token'` ✅ (counters, mock accumulation, ResetStats).
  - `go test ./internal/contracts/contracttest -count=1` ✅ (SimulationInfo + SimulationStats roundtrips).
  - Full `go test ./... -count=1` (all packages) ✅.
- **Static**: `go vet ./internal/api ./cmd/eoc ./internal/simulation` ✅; `gofmt -l` clean after format.
- **Code reads**: Full main.go (controller + run/start/reset + wiring), api.go (New/Register/handlers/stats), engine (wall + status + reset paths), llm (atomics + ResetStats), contracts, main_test, api_test, contracttest roundtrips, timeline Append/Truncate.
- **Cross-checks**: vs HANDOFF P19–P26 design, prior reviews (p19, "p21-api-cmd", p22-p23), AGENTS/SPEC §0 rules, live single-instance constraints.
- **Behavioral notes** (inspection + engine tests): resetCh interrupts sleeps; epoch discards correctly; tl len drops to include only synthetic then re-grows; tokens zeroed.

No failing paths, no panics on reset, stats shape matches contract.

---

## Recommendations

1. **P26 priority**: Add tests that construct a real eocSimController (or test helper), exercise Reset(), assert post-reset: store version=0, tl empty or only system-reset, cop Low, token totals=0, sim CurrentTime=0, epoch advanced, and (if running) new events flow to new epoch only.
2. **Optional UX for complete→reset**: If desired, have the coordinator (or a small wrapper) (re)spawn the sim.Run goroutine after reset when no active player. Document the current "reset aborts in-progress replay" semantics.
3. **Status typing**: Make engine.Status() return `contracts.SimulationStatus` (or api map the string) for stricter checks. Small additive win.
4. **Consider future**: If TokenStatsProvider should be resettable by callers outside cmd, propose small §0.5 additive method on the iface (after agreement).
5. **Observability**: The current poll + WS reset is fine; later could push a "stats" WS update after reset/replay progress.
6. **Keep the guard**: The combination of ctx cancel + fresh subscribe + epoch is robust for the live reset use-case.

---

## Verdict

**PASS (solid after review fixes).**

P24 delivers exactly the coordinator + epoch guard + wiring + stats endpoint required to make simulation clock/stats/All Clear work on the live product. The implementation is clean, rule-compliant, minimal, and correctly layered (api thin delegation; all coordination + guard in the cmd/eoc root).

The two functional issues found (double epoch bump, missing token reset on All Clear) were real but low-risk and have been fixed in-place within the owned lane. No other packages touched. Build + full test suite + contracttest all green. Determinism, thread-safety, and live-safety properties hold.

Ready for P25 (web widgets + WS "reset" handling + polling) and P26 (tests + manual validation).

Review artifacts: this file + fixes in `cmd/eoc/main.go`.

(Deep review followed project protocol: read SPEC §0/AGENTS/HANDOFF/contracts first, lane discipline, contract fidelity, determinism checks, verification runs, structured reporting.)