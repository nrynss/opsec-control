# P12 Deep Review: Web Provider Dropdown & Dynamic Logs

**Commit:** `4da9212` feat(web): add global provider dropdown and dynamic logs (P12)  
**Author:** Antigravity Builder (2026-06-29)  
**Date reviewed:** 2026-06-29  
**Reviewer:** Grok Builder  
**Scope:** P12 per HANDOFF: Add global provider dropdown (HUD/controls). Call /provider on change. Update all "CEREBRAS"/logs to reflect current provider. (Depends on P10/P11)

## Executive Summary

P12 successfully implements the frontend half of the multi-provider feature. It adds a runtime-switchable LLM provider dropdown in the HUD, wires it to the backend `/provider` API (GET for current, POST for change), propagates the change via props and WebSocket broadcasts, and dynamically updates log prefixes and UI labels (replacing hardcoded "CEREBRAS" references with the active provider).

**Overall verdict: Solid pass with minor UX/polish suggestions.** The implementation is clean, reactive, and follows Svelte patterns. It integrates well with P10 (api surface) and P11 (wiring + broadcast). Build succeeds. No major bugs or rule violations.

## Changes in P12

From commit:
- `web/src/components/HUD.svelte`: New provider selector dropdown + dispatch.
- `web/src/components/Dashboard.svelte`: State management (`currentProvider`), `fetchProvider()`, `changeProvider()`, WS "provider" handling, log prefix transform, prop passing to children.
- `web/src/components/MatrixFeed.svelte`: Uses `currentProvider` for empty-state message.
- `.env.example`, `HANDOFF.md`, `hosting.md`: Doc updates for dual providers.
- No new deps or breaking layout changes.

Files touched are exclusively `web/` + coordination/docs (appropriate for web lane).

## Detailed Code Review

### 1. HUD.svelte (Provider UI)
- Dropdown placed in HUD metrics strip as a new "LLM Provider" metric.
- `<select value={currentProvider} on:change={handleProviderChange}>` with options for "cerebras" / "openrouter".
- Dispatches custom event `'changeProvider'` with the value (Svelte `createEventDispatcher`).
- Inline `<style>` for `.provider-select` (themed dark select with custom arrow, hover/focus states using cyan accents). Consistent with HUD neon/cyber theme.
- `currentProvider` is a prop (`export var`).

**Strengths:**
- Non-intrusive addition to existing HUD grid.
- Good visual design, accessible basic (id, native select).
- Label "LLM Provider" clear.

**Observations:**
- Select is controlled by parent prop (good for reactivity).
- No `disabled` state during pending switch.
- Styles scoped to component (good).

### 2. Dashboard.svelte (Core State & Integration)
- `currentProvider = "cerebras"` default (matches backend default).
- `onMount`: calls `fetchProvider()` + `fetchInitialState()`.
- `fetchProvider()`: GET /provider, sets if `data.provider`.
- `changeProvider(e)`: POST /provider with `{ provider: newProvider }`, optimistic? update on `res.ok`, logs success/err.
- Passes `{currentProvider}` to `<HUD ... on:changeProvider={changeProvider} />` and `<MatrixFeed ... />`.
- WS handling in `handleIncomingData`:
  - Special case: `if (kind === "provider") { currentProvider = payload.provider; addLog... }`
- `addLog(prefix, content)`:
  ```js
  var finalPrefix = prefix === "CEREBRAS" ? currentProvider.toUpperCase() : prefix;
  ```
  - Transforms demo/legacy "CEREBRAS" prefixes.
- In `handleIncomingData`: fallback logs use "CEREBRAS" when no kind (replaced dynamically).
- Demo `trigger*` functions still call `addLog("CEREBRAS", JSON...)` — correctly transformed at runtime.
- Also updates some initial matrix log and WS labels.

**Integration points (P10/P11):**
- Uses exact shapes: GET returns `{provider: "..." }`, POST `{provider: "..."}`.
- Handles WS envelope `{"kind": "provider", "payload": {...}}` from backend broadcast.
- Single-origin fetches (`/provider`, `/stream`) consistent with hosting decision.

**Strengths:**
- Central source of truth for `currentProvider`.
- Reactive prop drilling to children.
- Handles both user-initiated change + server broadcast (idempotent).
- Graceful: catches fetch errors, falls back to demo/default.
- Log transform is a clever, minimal way to fulfill "update all CEREBRAS/logs" without rewriting every demo string.

**Potential issues / nits:**
- **Controlled select behavior (line ~551 in HUD usage):** User selects in dropdown → `on:change` fires dispatch → parent does async POST → *then* sets `currentProvider`. During the request, the `<select>` may not reflect the new value visually until re-render from parent. No optimistic update before `await`.
  - Suggestion: Set `currentProvider = newProvider` immediately on dispatch (optimistic), revert on error.
- No pending/loading state on the select or HUD during switch.
- `addLog` replace is exact string match (`=== "CEREBRAS"`), case-sensitive. Real WS uses `WS:PROVIDER` etc. — fine.
- Demo logs inside `setTimeout` chains still reference old strings but get rewritten.
- Matrix log cap at 100 (related to later P14 scroll history work).
- In live mode without provider key, still defaults correctly via fetch.

### 3. MatrixFeed.svelte
- Receives `currentProvider` prop.
- Empty state: `Listening for {capitalize(currentProvider)} telemetry stream...`
- Uses `afterUpdate` to auto-scroll (existing).
- Simple, focused.

**Good:** Updates message dynamically when provider changes (e.g., "Listening for OPENROUTER...").

### 4. Styles (dashboard.css + inline)
- No changes to main CSS for provider (all in HUD `<style>`). Good scoping.
- Theming consistent (cyan accents, dark bg).

### 5. Other updates (docs)
- `.env.example`: Added comments explaining provider boot logic, mock mode, runtime switch via POST.
- `HANDOFF.md`: Marked P12 ✅ (Antigravity), P16 claimed.
- `hosting.md`: Updated architecture diagram, deploy examples to include OPENROUTER secrets/vars, notes on dual-provider model, vision support on both, etc. Helpful for deploy.

## Compliance & Architecture

- **Follows web lane:** Only web/src + docs. No backend changes.
- **Single-origin / same-origin:** Uses relative `/provider`, `/stream` — consistent with prior decisions.
- **No operational logic in frontend:** Delegates to backend API. State is view model.
- **Reactivity & Svelte idiomatic:** Event dispatch + prop updates, no mutable DOM hacks.
- **Error resilience:** Demo fallback, try/catch on network calls.
- **Backward compat:** Defaults to "cerebras", demo mode unchanged behavior.
- **P9–P11 integration:** Perfect match to `/provider` contract and WS broadcast shape.
- **Determinism/UI:** Provider choice is user/env config, does not affect sim replay.

## Build & Runtime Verification
- `npm run build` (astro): ✅ Success. "1 page(s) built in 1.10s. Complete!"
- No TS/Svelte compile errors, no missing imports.
- Assumes backend (P10/P11) running for live /provider + WS; falls back gracefully.

## Issues & Recommendations

### Bugs / Correctness
- None critical. The optimistic update gap is the closest (UX, not crash).

### Suggestions (improvements)
1. **Optimistic UI for provider change** (HUD + Dashboard): Update `currentProvider` immediately on dispatch. Revert + error log on failure. Improves perceived latency.
2. **Add loading/disabled state**: Disable `<select>` or show spinner while POST in flight.
3. **Initial provider sync robustness**: If fetch fails in live mode, perhaps poll or rely on first WS "state"/"cop". Current keeps default.
4. **Expose current provider more broadly?** (future) Could pass to CellPanel or Map for provider-specific badges if desired.
5. **Test coverage**: No unit tests in web/ (Svelte/Astro). Manual + build is current. Consider Playwright/Cypress for E2E on provider switch in future.
6. **Log history**: The 100-line cap + slice is temporary; aligns with TASK-ui-scrollback.md for P14.

### Nits
- Some demo `addLog("CEREBRAS", ...)` remain in trigger functions (by design, transformed).
- "CEREBRO EOC" logo still uses the project name (correct, not the provider).
- Minor: `capitalize` helper only used in MatrixFeed empty state.
- In non-kind WS fallback: still defaults to dynamic "CEREBRAS" → good.

## Positive Highlights
- Minimal, surgical changes.
- Excellent reuse of existing patterns (addLog transform, WS envelope handling, prop passing).
- Dynamic labels fulfill the "Update all CEREBRAS/logs" requirement elegantly.
- Dropdown fits HUD aesthetics perfectly.
- Docs updates (hosting + env) make the feature usable for deployers.
- Fully functional with backend: GET on load, POST on change, WS broadcast reflection.

## Verdict

**Pass (strong for UI feature).** P12 completes the end-to-end provider switching story on the frontend side. The code is maintainable, reactive, and well-integrated. Minor UX improvements (optimistic updates, feedback) would polish it further, but nothing blocks use or P13–P16.

P12 is ready. Unblocks nothing hard, but enables user-visible provider switching + correct telemetry attribution.

**Review file location:** `reviews/p12-web-provider-dropdown-review.md`

**Files inspected:**
- web/src/components/{Dashboard.svelte, HUD.svelte, MatrixFeed.svelte}
- web/src/styles/dashboard.css (grep)
- web/src/pages/index.astro
- .env.example, HANDOFF.md, hosting.md diffs
- Build output + commit diff

No other web files modified for P12. All changes confined to web lane + docs.