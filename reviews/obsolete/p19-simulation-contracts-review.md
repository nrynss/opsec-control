# Deep Review: P19 — Simulation Contracts (§0.5)

**Commit:** `63e41fa` (contract(simulation): P19 §0.5 ...)
**Date:** 2026-06-29
**Reviewer:** Grok
**Branch:** feat/live-simulation-controls (post common-branch transition)

## Summary of Changes

P19 is the isolated contract change to enable the Simulation Clock, Stats, and All Clear features (P19–P23).

**Files touched (exactly as required for §0.5):**
- `internal/contracts/interfaces.go` (+44 lines of new types)
- `HANDOFF.md` (table status update only: ⬜ → ✅ Done)

**New definitions added (additive only):**

```go
// SimulationInfo ...
type SimulationInfo struct {
	Name      string  `json:"name"`
	StartTime SimTime `json:"startTime"`
	EndTime   SimTime `json:"endTime"`
}

// SimulationStats ...
type SimulationStats struct {
	Status         string  `json:"status"`
	CurrentTime    SimTime `json:"currentTime"`
	WallElapsed    int64   `json:"wallElapsed"` // ms (display only)
	EventsReplayed int     `json:"eventsReplayed"`
	TokensIn       int     `json:"tokensIn"`
	TokensOut      int     `json:"tokensOut"`
	Inferences     int     `json:"inferences"`
	Speed          float64 `json:"speed"`
}

type SimulationController interface {
	Reset()
	Pause()
	Resume()
	Step() (bool, error)
	SetSpeed(float64)
	Info() SimulationInfo
}

type TokenStatsProvider interface {
	TotalTokens() (in, out int)
	TotalRequests() int
}
```

## Alignment with Design (HANDOFF.md P19–P23 section)

- **Exact match to P19 spec**: "Add `SimulationStats` DTO, `SimulationInfo` struct, `SimulationController`, and `TokenStatsProvider`".
- **Fields align with downstream needs** (P20/P22/P21):
  - `SimulationInfo`: Name + bounds for "timeline clock with limits" and progress bar.
  - `SimulationStats`: Covers "elapsed Wall time, replayed event count, LLM tokens, inferences", plus current time, speed, status.
  - `SimulationController`: Methods directly support reset/pause/resume/step/speed + Info().
  - `TokenStatsProvider`: Supports token/inference aggregation without leaking LLM impl.
- **Documentation**: Good inline comments referencing purpose, P19, determinism firewall ("display only", "never used for logic").
- **Future P20+**: Simulation will implement `Info()` + `WallElapsed()`; LLM will expose token totals; API will use the interfaces; cmd/eoc will implement the controller.

No mismatches found between contract and the detailed design.

## Compliance with Core Rules

**AGENTS.md / SPEC §0 (Multi-Agent Development Protocol)**:
- ✅ **Contract-first**: Purely additive change in the canonical location. No unilateral edits elsewhere.
- ✅ **Isolated commit**: Only touched `contracts/` + marker in HANDOFF (coordination file). Message starts with `contract(simulation)`.
- ✅ **Stay in lane**: `contracts/` is the designated lane for shared shapes. No other package files modified.
- ✅ **Additive by default**: All new; no breaking changes to existing interfaces.
- ✅ **Determinism firewall**: Explicit notes that WallElapsed is display-only.
- Pre-flight checklist satisfied (read contracts, etc.).

**Other**:
- No new imports, no time.Duration (uses int64 ms — portable and keeps contracts pure).
- Reuses `SimTime` (correct, per events.go).
- JSON tags present for API/WS serialization.
- `contracttest` unaffected (green).

## Strengths

- Minimal and focused — exactly what P19 calls for.
- Clear separation: Controller for *control* (reset/playback), Provider for *read-only stats*.
- Good use of existing primitives (SimTime, basic Go types).
- Comments help future implementers (P20–P23) and reviewers.
- Aligns with live product needs (stats for HUD, reset for All Clear, without affecting running sim determinism).
- Follow-up commits can extend impls in their lanes.

## Issues & Findings

### No Critical Bugs
The types themselves are just data + interfaces — no runtime logic here.

### Suggestions (Medium)

1. **Status field** (`SimulationStats.Status`)
   - Currently a bare `string` with comment listing values.
   - **Suggestion**: Add typed constants in contracts (or a new `SimulationStatus` type) for type safety and to prevent typos in P20/P21 impls.
   - Example:
     ```go
     type SimulationStatus string
     const (
         SimRunning  SimulationStatus = "running"
         SimPaused   SimulationStatus = "paused"
         SimComplete SimulationStatus = "complete"
     )
     ```
     Then use `Status SimulationStatus` in the struct.
   - **File**: `internal/contracts/interfaces.go:104`
   - **Why now**: Prevents drift when P21 wires the real status from simulation.Engine.

2. **Missing "elapsed Sim time" vs CurrentTime**
   - P22 design mentions "elapsed Sim time" in metrics grid.
   - CurrentTime + StartTime can derive it, but explicit field might be cleaner for frontend.
   - **Suggestion**: Consider adding `ElapsedTime SimTime` (or compute in stats provider later). Or clarify in P22 that CurrentTime suffices.
   - **File**: `internal/contracts/interfaces.go:103` (SimulationStats)

3. **Controller scope**
   - Interface has the control methods + Info().
   - Future may want a separate read-only stats iface vs control.
   - Currently fine (matches design), but watch for bloat in P21.

### Nits (Low)

- Comment on `SimulationInfo` says "for the UI simulation clock/progress bar" — good, but could reference SimTime doc for consistency.
- `Inferences` comment: "LLM calls / completions" — clear, but could link to LLMClient usage.
- HANDOFF table update is correct, but the detailed P19 section still says "Add ..." (no need to change yet).

No issues with imports, json, or cross-package leakage (these are the contracts).

## Impact on Future Parcels & Live System

- **P20**: simulation.Engine must grow `Info()`, wall stopwatch (display-only), and later Status(). Reset already exists but will be wrapped.
- **P21**: api.Server will hold the new interfaces; cmd/eoc will implement `eocSimController` (with epoch guard for safe reset on live).
- **P22**: web/ can use the JSON shapes directly for polling /stats and rendering.
- **Live considerations**: Types are small and serializable. No risk to current running instance (additive). Reset semantics (in P21) will be critical because of in-memory state + concurrent fanouts.
- **Determinism**: Well protected by comments and design.
- **Tests (P23)**: Will need to exercise these via contracttest roundtrips and engine tests.

## Verification Performed (as part of review)

- `go build ./internal/contracts` ✅
- `go test ./internal/contracts/contracttest -count=1` ✅ (unaffected)
- `grep` for the new types: only in HANDOFF + this file (no premature usage).
- Diff review: only contracts + handoff marker.
- Cross-check vs HANDOFF design + original plan: fields/methods cover the required metrics (wall, events, tokens, inferences, clock bounds).
- No conflicts with existing code (StateStore, LLMResponse already has per-call tokens, etc.).

## Recommendations

1. **Before P20**: Add the status constants (see suggestion #1) as a small follow-up contract tweak if needed (or do it in P19 if re-landing allowed, but since committed, additive extension ok).
2. **In P21**: When implementing in api/cmd, ensure the controller is injected properly (like current COPProvider, ProviderSwitcher).
3. **Docs**: In P22/P23, add examples of the JSON shapes.
4. **General**: Good job keeping it minimal. Continue the pattern of local seams in api (like ProviderSwitcher) for the impl side.

## Verdict

**Pass — clean, correct, rule-compliant contract addition.**

This is exactly the kind of focused §0.5 change the process is designed for. Types are well-scoped, documented, and directly enable the rest of the feature without over-engineering.

No blockers for P20–P23. The only improvements are small polish items for type safety and clarity.

**Overall confidence in P19**: High. Ready to proceed.

---

Review file location: `reviews/p19-simulation-contracts-review.md`

(Deep review performed per project norms: code inspection, spec cross-check, rule compliance, forward-looking impact analysis, build/test verification.)