 Comprehensive Codebase Review

 ### Overall Assessment: A- (Strong MVD, demo-ready with known gaps)

 The codebase is well-architected, contract-first, thoroughly tested, and correctly implements the core thesis (parallel fan-out on anomaly). All 74 tests pass. The SPEC's multi-agent development protocol (§0) is faithfully followed.

 ────────────────────────────────────────────────────────────────────────────────

 ### What Works Well (Strengths)

 ┌────────────────────┬───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ Area               │ Assessment                                                                                                            │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Contracts layer    │ Excellent — canonical types/interfaces in internal/contracts/; round-trip tests; additive change protocol enforced    │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ State management   │ Correct — single owner (internal/state), versioned snapshots, §14.2 validation gatekeeper, deep-copy isolation        │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Parallel fan-out   │ Correct — orchestrator.Engine fans out specialists concurrently, Commander as phase-2, context cancellation respected │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Anomaly classifier │ Complete — event-type + state-threshold rules, deterministic order, wakes correct cell subsets                        │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Simulation engine  │ Solid — deterministic replay, pause/resume/speed, reset/load interrupt handling                                       │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ LLM client         │ Production-grade — retries, backoff, concurrency cap, Retry-After parsing, deterministic seed config, mock mode       │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Event bus          │ Clean — per-subscriber queues, no slow-consumer blocking, idempotent subscribe/cancel                                 │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ API/WS edge        │ Thin serialization layer — no logic, only contract DTOs, WebSocket with per-conn mutex                                │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Scenario compiler  │ Offline-only — validates against throwaway state, emits replayable JSON                                               │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Frontend           │ Functional Astro+Svelte dashboard with map, cell panels, HUD, matrix feed, playback controls                          │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Determinism        │ Largely upheld — SimTime, seeded RNG (except BUG-3), sorted JSON, no wall-clock in logic                              │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Cross-platform     │ .gitattributes LF, pinned toolchains, filepath.Join, //go:embed, no shell one-liners                                  │
 ├────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ Testing            │ 74 tests across all packages; contract tests verify seams; integration tests for full flows                           │
 └────────────────────┴───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 ────────────────────────────────────────────────────────────────────────────────

 ### Critical Blockers (Must Fix Before Live Demo)

 ┌───────┬──────────┬─────────────────────────────────────────┬───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ ID    │ Severity │ Location                                │ Issue                                                                                                                                                                                                                 │
 ├───────┼──────────┼─────────────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-1 │ HIGH     │ internal/orchestrator/engine.go:148-156 │ Goroutine leak on context cancellation — when ctx.Done() fires before wg.Wait(), the waiter goroutine blocks until all cells finish (mock: 5s sleep; real: 45s HTTP timeout). Wastes Cerebras concurrency budget (max │
 │       │          │                                         │ 4).                                                                                                                                                                                                                   │
 └───────┴──────────┴─────────────────────────────────────────┴───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 Fix (5 min):

 ```go
   select {
   case <-done:
       // all finished
   case <-ctx.Done():
       go func() { wg.Wait() }() // drain in background
       return contracts.CommonOperationalPicture{}, ctx.Err()
   }
 ```

 ────────────────────────────────────────────────────────────────────────────────

 ### Significant Bugs (Should Fix)

 ┌───────┬──────────┬─────────────────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ ID    │ Severity │ Location                            │ Issue                                                                                                                                                │
 ├───────┼──────────┼─────────────────────────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-2 │ MEDIUM   │ internal/validation/validate.go     │ Negative timestamps accepted by Envelope() — only monotonicity checked in Store.Apply(). First event at t=-500 passes, sets negative world time.     │
 ├───────┼──────────┼─────────────────────────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-3 │ MEDIUM   │ internal/llm/client.go:97           │ Determinism violation — rand.New(rand.NewSource(time.Now().UnixNano())) for backoff jitter. Violates SPEC §0.2 Rule 5.                               │
 ├───────┼──────────┼─────────────────────────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-4 │ MEDIUM   │ internal/state/store.go             │ EventBuildingCollapsed in taxonomy + anomaly detector → wakes cells, but no state mutation (no Building entity in MVD). Cells see "nothing changed." │
 ├───────┼──────────┼─────────────────────────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-5 │ MEDIUM   │ internal/state/store.go             │ EventRoadBlocked, EventTunnelClosed — same: accepted, wake cells, zero state effect.                                                                 │
 ├───────┼──────────┼─────────────────────────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-6 │ MEDIUM   │ internal/state/store.go + contracts │ BridgeCollapsed status defined (rank 3) but unreachable — no event type/handler transitions to it.                                                   │
 └───────┴──────────┴─────────────────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 ────────────────────────────────────────────────────────────────────────────────

 ### Contract/Implementation Gaps (MVD Scope Decisions)

 These are deliberate MVD simplifications per SPEC §13, but create silent no-op events:

 ┌───────────────────┬───────────────┬────────────────┬─────────────────┬────────┐
 │ Event Type        │ In Taxonomy?  │ Anomaly Wakes? │ State Mutation? │ Status │
 ├───────────────────┼───────────────┼────────────────┼─────────────────┼────────┤
 │ BuildingCollapsed │ ✅            │ Intel + Infra  │ ❌              │ BUG-4  │
 ├───────────────────┼───────────────┼────────────────┼─────────────────┼────────┤
 │ RoadBlocked       │ ✅            │ Intel + Infra  │ ❌              │ BUG-5  │
 ├───────────────────┼───────────────┼────────────────┼─────────────────┼────────┤
 │ TunnelClosed      │ ✅            │ Intel + Infra  │ ❌              │ BUG-5  │
 ├───────────────────┼───────────────┼────────────────┼─────────────────┼────────┤
 │ BridgeCollapsed   │ ❌ (no event) │ —              │ ❌              │ BUG-6  │
 ├───────────────────┼───────────────┼────────────────┼─────────────────┼────────┤
 │ PowerPartial      │ ✅ (status)   │ —              │ ❌ (no handler) │ O-4    │
 └───────────────────┴───────────────┴────────────────┴─────────────────┴────────┘

 Recommendation: Either (a) add minimal entities (Road, Building damage counter) + handlers, or (b) remove these event types from the MVD scenario/taxonomy and document as post-MVD.

 ────────────────────────────────────────────────────────────────────────────────

 ### Low-Priority / Cosmetic Issues

 ┌───────┬───────────────────┬────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
 │ ID    │ Area              │ Issue                                                                                                                                                                                          │
 ├───────┼───────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-7 │ anomaly           │ Classifier wakes Intelligence + Communications cells, but only 4 cells registered in cmd/eoc (Infra, Medical, Population, Commander). Orchestrator logs "not registered" — noisy but harmless. │
 ├───────┼───────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ BUG-8 │ orchestrator test │ Test helper capturingCell fires callback before inner call (test-only, no prod impact).                                                                                                        │
 ├───────┼───────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ O-1   │ web               │ Frontend fallback IDs (westbank) ≠ backend IDs (S-WESTBANK). Bridge IDs mismatch (vora vs B-VORA). Demo mode diverges from live.                                                               │
 ├───────┼───────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ O-2   │ timeline          │ All() returns shallow copy with aliased Payload byte slice — documented but fragile.                                                                                                           │
 ├───────┼───────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ O-3   │ api               │ Unused orch field in Server struct.                                                                                                                                                            │
 ├───────┼───────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
 │ O-4   │ scenariogen       │ Generator uses fixed 30s spacing; embedded scenario is hand-curated with varied timing.                                                                                                        │
 └───────┴───────────────────┴────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 ────────────────────────────────────────────────────────────────────────────────

 ### Architecture Compliance (SPEC §0 / §16)

 ┌──────────────────────┬────────┬──────────────────────────────────────────────┐
 │ Rule                 │ Status │ Evidence                                     │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ Contract-first       │ ✅     │ All cross-pkg types in contracts/            │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ Stay in lane         │ ✅     │ No pkg reaches into another's internals      │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ Interfaces not impls │ ✅     │ contracts/interfaces.go used everywhere      │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ Single state owner   │ ✅     │ Only internal/state mutates WorldState       │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ Determinism          │ ⚠️     │ BUG-3 (llm backoff) is the only violation    │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ Mock deps            │ ✅     │ All packages testable with fakes             │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ Tests with code      │ ✅     │ Each pkg has _test.go; contract tests shared │
 ├──────────────────────┼────────┼──────────────────────────────────────────────┤
 │ One pkg per commit   │ ✅     │ History shows clean separation               │
 └──────────────────────┴────────┴──────────────────────────────────────────────┘

 ────────────────────────────────────────────────────────────────────────────────

 ### Test Coverage Gaps (from DeepSeek review)

 ┌──────────────────────────────────────────────────────────────┐
 │ Missing Test                                                 │
 ├──────────────────────────────────────────────────────────────┤
 │ Envelope negative timestamp rejection                        │
 ├──────────────────────────────────────────────────────────────┤
 │ EventDamStressElevated on already-stressed dam (no-op)       │
 ├──────────────────────────────────────────────────────────────┤
 │ EventLeveeBreached on already-breached levee                 │
 ├──────────────────────────────────────────────────────────────┤
 │ EventPowerFailure from PowerPartial → PowerOff (skip rank)   │
 ├──────────────────────────────────────────────────────────────┤
 │ EventFireContained → EventFireIgnited (backward)             │
 ├──────────────────────────────────────────────────────────────┤
 │ EventFloodExtentUpdated with decreasing depth (monotonicity) │
 ├──────────────────────────────────────────────────────────────┤
 │ Concurrent Store.Apply calls (mutex protection)              │
 ├──────────────────────────────────────────────────────────────┤
 │ WS Broadcast with full client write buffer                   │
 ├──────────────────────────────────────────────────────────────┤
 │ Scenariogen with empty LLM output                            │
 └──────────────────────────────────────────────────────────────┘

 ────────────────────────────────────────────────────────────────────────────────

 ### Frontend Readiness

 The Svelte dashboard is functional for the demo video — it has:
 - Live HUD (tokens/sec, latency, active cells, state version)
 - Tactical map with sectors, bridges, dam, levee, flood overlay, fire indicators
 - 5 cell panels showing status (idle/analyzing/done) + structured output
 - Commander COP panel with prioritized actions
 - Matrix feed (raw JSON stream)
 - Playback controls (speed, pause, reset)

 Gap: Demo mode uses hardcoded IDs that don't match the embedded scenario. When WS connects, it switches to backend data — so the live demo works, but offline preview is mismatched.

 ────────────────────────────────────────────────────────────────────────────────

 ### Build & Deploy

 ```bash
   # Go build
   go build -o bin/eoc ./cmd/eoc        # ✅ succeeds

   # Scenario generator
   go build -o bin/scenariogen ./cmd/scenariogen  # ✅ succeeds

   # Docker
   docker build -t eoc .                # ✅ multi-stage, distroless, non-root

   # Frontend (separate)
   cd web && npm run build              # ✅ outputs to dist/
 ```

 ────────────────────────────────────────────────────────────────────────────────

 ### Priority Fix List for Demo

 ┌──────────┬──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┬───────────┐
 │ Priority │ Fix                                                                                                                                              │ Est. Time │
 ├──────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┼───────────┤
 │ P0       │ Fix BUG-1 (orchestrator goroutine leak)                                                                                                          │ 5 min     │
 ├──────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┼───────────┤
 │ P0       │ Fix BUG-2 (validation: reject negative timestamp)                                                                                                │ 3 min     │
 ├──────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┼───────────┤
 │ P0       │ Fix BUG-3 (llm: seed backoff RNG from Config)                                                                                                    │ 5 min     │
 ├──────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┼───────────┤
 │ P1       │ Decide on BUG-4/5/6: either add minimal Road/Building entities + handlers, OR prune no-op events from scenario + taxonomy (contract change §0.5) │ 30-60 min │
 ├──────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┼───────────┤
 │ P2       │ Align frontend fallback IDs with backend (or make demo mode a projection)                                                                        │ 15 min    │
 ├──────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┼───────────┤
 │ P3       │ Add missing contract tests (negative timestamp, flood monotonicity, etc.)                                                                        │ 20 min    │
 └──────────┴──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┴───────────┘

 ────────────────────────────────────────────────────────────────────────────────

 ### Verdict

 The codebase is a strong, demo-ready MVD. The core thesis (parallel fan-out on anomaly, Cerebras-speed) is implemented correctly and tested. The three P0 bugs are trivial fixes. The contract gaps (BUG-4/5/6) are known MVD scope decisions — they don't break the demo because mock
 LLM responses are canned, but with real Cerebras inference they would produce "nothing changed" analyses.

 Recommendation: Fix P0 items today, decide on P1 (add entities vs. prune events), and you have a winning hackathon submission. The 60-second video choreography (§15) will work beautifully — the repeated visible fan-outs at ~500ms are the money shot.