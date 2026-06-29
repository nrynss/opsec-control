# Deep Review: P21 (api + cmd/eoc for simulation controls/stats/reset wiring)

**Context:** P21 (renumbered P24 in latest table) for "EOC coordinator and WS broadcast of 'reset' kind + actual API endpoint wiring". P19 (contracts) and P20 (simulation) completed; P21 partially landed per code. Review focuses on landed changes in internal/api and cmd/eoc/main.go. Compared to design in HANDOFF.md, AGENTS.md/SPEC.md rules, previous reviews, and live product constraints.

**Date of review:** 2026-06-29 (on feat/live-simulation-controls)
**Reviewer:** Grok
**Status:** P21 claimed by Grok Builder in HANDOFF.

## Summary of Changes (Landed in P21)
- **internal/api/api.go**:
  - Server struct extended with `simCtrl contracts.SimulationController` and `tokenStats contracts.TokenStatsProvider`.
  - `New()` signature updated to accept them (9 params total).
  - Register() now includes `GET /scenario/stats` and the scenario control routes (load/reset/pause/resume/step/speed).
  - Scenario control handlers (`handleScenarioReset`, `handleScenarioPause`, etc.) now delegate to `simCtrl` if wired (e.g., `s.simCtrl.Reset()`), then fall back to `respondWithScenarioStub`.
  - New `handleScenarioStats()`: Returns `SimulationStats` JSON using simCtrl (Info, Status, CurrentTime, WallElapsedMS) and tokenStats (TotalTokens, TotalRequests) if wired; else `{"status": "not_wired"}`.
  - `respondWithScenarioStub` remains (with outdated "MVD stub" note referencing P6).
  - `handlePostEvent` comment still references old P11.

- **cmd/eoc/main.go**:
  - Added comment: "// Simulation engine (for stats/clock in P19+)".
  - `sim := simulation.New(bus)` created.
  - Wiring: `api.New(..., wsSrv, sim, nil)` (passes sim as simCtrl; nil for tokenStats).
  - No `eocSimController` struct, no epoch guard, no full reset coordination (sim.Pause/Reset, tl.Truncate, cop reset, WS "reset" broadcast, SystemReset event, etc.).
  - Serving log not updated to mention /scenario/stats or controls.
  - No changes to app.handle or copStore for reset.

- Tests: `internal/api/api_test.go` updated New() calls to pass extra nils (9 args). `TestScenarioControlStubs` still tests stub path (nils).

No WS "kind: reset" broadcast, no full coordinator in cmd, partial stats data.

## Alignment with HANDOFF Design (P24 section)
**Design requires:**
- api.go: Add SimulationController + TokenStatsProvider to Server. Update scenario control endpoints. Add GET /scenario/stats returning SimulationStats JSON.
- cmd/eoc/main.go: Define eocSimController wrapping sim, store, tl, initial, copStore, bcast. Implement SimulationController.
  - Reset Epoch Guard: epoch tracking, cancel reasoning ctx, resub/unsub EventBus, new runLoop. Discard old callbacks/WS.
  - Reset Actions: sim.Pause/Reset, store.Reset, tl.Truncate, append SystemReset event, reset copStore, broadcast `kind: "reset"`.

**Current vs Design:**
- api side: Mostly matches (interfaces added, New/handlers/stats endpoint updated, delegation works).
- cmd side: Does not match. No eocSimController, no epoch guard, no broadcast/reset actions, no full wiring (nil tokens). Just passes raw sim.
- Stats: Returns partial JSON (EventsReplayed=0, Speed=1.0, relies on P20 methods). No full composition from tl/cop etc.
- WS/reset: Not implemented.
- **Gaps:** ~50% complete. Api exposure/delegation done; coordinator + full reset + WS + complete stats data missing.

## Compliance with Rules (AGENTS.md, SPEC.md §0)
- **Contract-first:** Good. Uses contracts.SimulationController/TokenStatsProvider (from P19). No local invention.
- **Lane ownership (§16.1):** api + websocket owned by Grok; cmd/eoc is "single owner of main.go" (DeepSeek/Claude per history). Edits to main.go for wiring are necessary for api work but touch another package—borderline (past P6/P11 did similar; document as coordination).
- **No operational logic in api:** Handlers delegate to ctrl (good). respondWithScenarioStub is legacy but now conditional.
- **Determinism:** Not affected here (P20 firewall). Stats are display-only.
- **Single mutator / no shared state:** Relies on P20/P22/P23 resets (state.Reset is sanctioned exception).
- **Build isolation:** api changes isolated; tests updated. But wiring in main means full tree build requires P20 sim (done).
- **Other:** respondWithScenarioStub comment outdated (ref P6/MVD). No new globals/mutables.
- **Live/prod:** Adding /scenario/stats is positive for observability. But incomplete data + nil wiring means "not_wired" in practice. Single-instance rule still holds (no HA changes).

**Strengths:**
- Clean delegation pattern (consistent with COPProvider, ProviderSwitcher).
- Api now properly surfaces controls/stats via contracts (enables P25 web).
- Backward-compatible for tests (nils keep stubs).
- Build/tests green (see verification).
- Supports live reset/stats without breaking existing event flow.

## Issues / Findings (by severity)

### Medium (Functional / Completeness)
1. **Incomplete wiring and stats data (api.go:254, main.go:213)**
   - tokenStats passed as nil (even though llmClient now has Total*/ResetStats from P21 llm).
   - handleScenarioStats hardcodes `EventsReplayed: 0`, `Speed: 1.0`.
   - No population of full stats (e.g., from tl.Len() or sim events count).
   - **Why:** P21 "in flight" partial. Handler returns "not_wired" currently.
   - **Suggestion:** Wire `llmClient` (implements TokenStatsProvider) in main. Extend handler to pull events from s.log if available, speed from simCtrl if added. Compose full SimulationStats.
   - **File:** internal/api/api.go:243, cmd/eoc/main.go:213

2. **No eocSimController / reset coordination / WS broadcast (design vs code)**
   - Design specifies full struct in main.go with epoch guard, ctx cancel, bus resub, tl.Truncate, cop reset, SystemReset event, WS `kind: "reset"`.
   - None present. Only raw sim passed for basic controls.
   - Reset handlers just call simCtrl.Reset() + stub response (no full actions).
   - **Impact:** All Clear / reset won't work end-to-end (no WS notification for P25, no epoch to prevent stale fanouts, partial state clear).
   - **Suggestion:** Implement eocSimController per design (or simplify if overkill). Add `broadcast("reset", ...)` and SystemReset event. Update app to support reset epoch.
   - **Files:** cmd/eoc/main.go (missing), internal/api/api.go (delegation only)

3. **Outdated comments/stubs (api.go:164, cmd/eoc/main.go:231)**
   - respondWithScenarioStub still says "MVD stub - no-op until wired in P6".
   - Serving log omits /scenario/stats and controls.
   - handlePostEvent comment refs old P11.
   - **Suggestion:** Update notes to reference P19-P26. Extend log: "... /scenario/stats /scenario/{reset,...}".
   - **Severity:** Nit, but confusing for maintainers.

### Low (Nits / Polish)
1. **Test coverage gaps for new paths (api_test.go)**
   - TestScenarioControlStubs only tests nil/simCtrl==nil path (stubs).
   - No test for handleScenarioStats (with/without wiring), delegation success, stats JSON shape.
   - **Suggestion:** Add cases with mock simCtrl/tokenStats. Verify delegation calls, stats fields, 202/OK responses.
   - **File:** internal/api/api_test.go:217+

2. **Type safety / casting (api.go:254)**
   - `contracts.SimulationStatus(s.simCtrl.Status())` (string to alias).
   - Assumes simCtrl.Status() returns valid const value.
   - **Suggestion:** Add validation or make simCtrl.Status() return SimulationStatus directly (update iface if possible, or keep).
   - Minor since additive.

3. **No error handling in delegation**
   - Ctrl calls (e.g. Step()) ignore return values/errors.
   - Stats doesn't handle cases where simCtrl.Info() etc. fail.
   - **Suggestion:** Log errors or propagate (e.g., 500 if ctrl fails).

4. **Other**
   - In stats: ElapsedTime computation assumes StartTime <= CurrentTime (from P20).
   - No update to cop or other on controls (e.g., after reset).
   - Design mentions "WS broadcast of 'reset' kind" — not in code (P25 web expects it).

## Impact on Other Parcels / Live System
- Enables P25 (web) once implemented (can now poll /scenario/stats, call controls).
- P26 tests can now cover api paths (with mocks).
- Live: /scenario/stats available (but incomplete). Reset via /scenario/reset now calls sim (from P20), but no full cleanup/WS yet — risk of stale state/fanouts on live (in-memory).
- Determinism: Unaffected (delegation only).
- Since live on Fly (single instance): Good for observability, but ensure reset doesn't break ongoing WS clients (epoch guard missing).

## Verification Performed
- Code reads: api.go (handlers, New, stats), main.go (wiring), tests.
- `go build ./internal/api ./cmd/eoc` ✅
- `go test ./internal/api -count=1` ✅ (stubs + updates pass)
- Cross-checked vs HANDOFF design, P19 contracts, P20 sim methods.
- No premature / breaking changes.
- Greps for "reset", stats, simCtrl confirmed scope.

## Recommendations
1. Complete cmd/eoc side of P21: Implement eocSimController with full reset (per design), wire llmClient for tokenStats (e.g., `api.New(..., sim, llmClient)`), update log message.
2. Flesh out handleScenarioStats: Populate EventsReplayed (e.g., from s.log), Speed (from sim if exposed), handle errors.
3. Add tests: For stats handler, delegation, with real mocks.
4. Remove outdated stubs/comments.
5. Once done, enable P25/P26 (frontend + full tests).
6. Consider pushing stats via WS (like "cop") for live efficiency, not just poll.
7. For live: Test reset on running sim (epoch to avoid races).

**Overall Verdict:** **Partial / In-progress (api side mostly done; cmd incomplete).** Good foundation for exposing controls/stats via contracts. Matches api exposure part of design but misses coordinator/WS/broadcast/full data. Clean delegation, no rules violations. Ready to finish P21 to unblock P25. No critical bugs, but TODOs and gaps mean not production-ready for full reset/stats feature.

See also prior reviews (p19, p20 simulation, p22-p23) for context.

(Review based on direct code inspection, design cross-reference, and project rules. No P21 files beyond api/cmd were checked.)