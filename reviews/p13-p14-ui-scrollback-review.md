# P13 & P14 Deep Review: UI Scrollback Histories + Smaller Map

**Commit:** `314b656` feat(web): resize map layout and implement scrollable history feeds (P13, P14)  
**Follow-up:** `55bf945` fix(web): add undefined check guards for agent outputs in Dashboard  
**Date:** 2026-06-29  
**Lane:** web/ only (as scoped)  
**Reviewer:** Grok Builder  
**References:** TASK-ui-scrollback.md, HANDOFF.md (P13/P14), previous P9-P12 reviews

## Executive Summary

P13 (smaller map + enlarged data areas) and P14 (accumulating scrollback histories for feeds, cells, and COP with proper ordering and smart auto-scroll) have landed together in a single focused change to the web frontend.

The implementation largely follows the locked decisions in TASK-ui-scrollback.md:
- Everything accumulates (no more overwrite-in-place).
- Ring buffers (~500 lines) for bounded memory.
- Newest-at-top for Cells and Commander COP.
- Bottom-tail for Matrix Feed and System Event log (append + smart scroll only when near bottom).
- Map fixed to 360px height; specialists/COP areas expand.
- Demo triggers and live handlers updated.

**Build:** ✅ `npm run build` succeeds cleanly.

**Overall Verdict: Strong pass with minor inconsistencies and polish items.** Core functionality matches spec. The follow-up fix addressed a real runtime issue (undefined agent outputs). A few carry-over bugs from the append/prepend switch and template details remain. No backend/contracts changes. Good isolation.

## Scope Confirmation
Per TASK-ui-scrollback.md and commit:
- Files touched: `Dashboard.svelte`, `CellPanel.svelte`, `MatrixFeed.svelte`, `dashboard.css` (plus HANDOFF update).
- No changes outside web/.
- Covers P13 + P14 + scrolling slice of P15.
- Independence from provider work (P12) respected in code (currentProvider still flows through).

## Detailed Implementation Review

### 1. Data Model Changes (Dashboard.svelte)
- `copHistory = []`
- `cellHistory = { "Intelligence": [], ... }` (replaces single `cellData`)
- `timelineEvents = []` (System Event log)
- `matrixLogs = []` (Matrix Feed)
- Reset in `loadNominalState()` and on demo loop.

**Live path (handleIncomingData):**
- COP: `cop = payload; copHistory = [payload, ...copHistory]` (dedup on summary+overallRisk)
- cellOutputs / cell_output: `cellHistory[agent] = [out, ...cellHistory[agent]]` (dedup)
- Events: append `timelineEvents = [...timelineEvents, payload]`, ring slice last 500
- Matrix: `addLog` append + ring 500

**Demo triggers (triggerAct1/2/3):**
- Append to `timelineEvents`
- Prepend to `cellHistory[xxx]`
- Prepend to `copHistory`
- Still use `addLog("CEREBRAS", ...)` (transformed)

**addLog:**
```js
matrixLogs = [...matrixLogs, {prefix: final, content}];
if (length > 500) slice last 500
```
(Previously capped at 100.)

**Guard fix (55bf945):**
- Added `if (cellStatuses[out.agent] !== undefined)` and same for `cellHistory`
- Prevents crashes on unexpected agents in cellOutputs.

**Evaluation:** Matches "everything accumulates", prepend for cells/COP (newest top), append for feeds. Dedup is sensible to avoid duplicates on re-renders/WS. Ring buffer good. The undefined guards were necessary post-change.

### 2. Scrolling & Auto-Scroll (Smart, not Force)
- `afterUpdate` on `timelineElement`:
  ```js
  var isNearBottom = ... >= scrollHeight - 30;
  if (isNearBottom || small) scrollTop = scrollHeight;
  ```
- MatrixFeed.svelte updated identically with `feedElement` and `afterUpdate`.
- Binds: `<div class="timeline-list" bind:this={timelineElement}>`
- Matrix feed: `bind:this={feedElement}`

**Evaluation:** Directly implements "Smart auto-scroll: only pin to the bottom when the user is *already* at the bottom". Prevents yanking when reading history. Threshold 30px reasonable. Good.

### 3. History Rendering
**CellPanel.svelte (major update):**
- Prop: `history = []` (was `data`)
- Renders `<div class="agent-history-list">` with `{#each history as data}`
- Each entry: v{stateVersion}, risk badge, summary, top 2 recommendations.
- Analyzing skeleton pinned at top of list when status=analyzing.
- Header risk from `history[0]`
- Idle state when empty.

**COP in Dashboard (commander-panel):**
- `<div class="cop-history-list">`
- `{#each copHistory as item, idx}`
  - Numbering: `#{copHistory.length - idx} COP`
  - Shows summary, risk, prioritizedActions (limited)
- No separate "current summary" above the list (history *is* the list, latest first).

**Evaluation:** Follows "scrollable mini-log inside each card" and "scrollable COP-history list beneath the current summary" (though current is now the first history item). Newest-at-top via prepend. Good detail per entry.

### 4. Map Resize (P13)
**dashboard.css:**
- `.main-area { grid-template-rows: 360px 1fr; ... }` (was 1fr 220px)
- `.cerebro-svg { width: 100%; height: 100%; }` (relaxed previous max-height:520px)
- `.specialists-panel`, sidebars, etc. benefit from extra vertical space.

**Dashboard template:**
- `<Map ... activeEvent={timelineEvents[timelineEvents.length - 1]} />` (uses last)
- `<PlaybackControl ... activeEvent={timelineEvents[0]} />` (still [0])

**Evaluation:** Map is now fixed modest height. Data areas (specialists + commander + feeds) are larger. Matches "One layout flip shrinks the map and enlarges the data areas".

**Inconsistency noted:** Playback uses [0] (oldest after append), Map uses last (newest). Previously both relied on prepend making [0] newest. PlaybackControl itself doesn't appear to heavily depend on the prop for active highlighting (it mostly drives scenario controls), but it's a carry-over.

### 5. Ordering & Feeds
- Feeds (matrix + timeline): append + tail at bottom.
- Cells/COP: prepend ([newest, ...]) → rendered top is latest.
- Matches locked decision exactly.

### 6. CSS Additions
- `.agent-history-list`, `.cop-history-list`, `.cop-actions` { overflow-y: auto; ... max-heights, padding }
- Custom scrollbars.
- Retained/updated agent-card, commander-panel, matrix-feed, etc.
- No breaking changes to existing styles.

### 7. Other / Cross-Cutting
- Provider (P12) still wired: currentProvider passed to HUD/Matrix, change handlers intact.
- Demo loop reset calls loadNominalState() which clears histories.
- Live WS path: unchanged structure + history pushes.
- No contract or backend touches.
- `timelineEvents[0]` in Playback vs length-1 in Map (see above).
- In some legacy addLog calls inside triggers: still "CEREBRAS" (transformed by addLog).
- MatrixFeed empty state uses currentProvider (good).

## Verification
- **Build:** ✅ Successful (astro build complete, 1 page).
- **Spec alignment:** High. Accumulate, ordering, smart scroll, map size, ring ~500, updated write sites, renderers all present.
- **Follow-up fix:** Necessary and correct (guards prevent runtime errors on cell outputs).
- **Runtime behavior (inferred from code):**
  - Scrolling up to read history does not get yanked (smart check).
  - New analyses appear at top of cell cards / COP list.
  - Feeds behave like terminals (new at bottom, auto only if near).
  - Bounded growth.
- **No obvious perf issues:** Limited history per cell (one per anomaly beat), 500 for logs, scroll containers.

## Issues / Findings

### Bugs / Inconsistencies
1. **timelineEvents indexing (Dashboard.svelte:607, 633)**
   - `PlaybackControl activeEvent={timelineEvents[0]}`
   - `Map activeEvent={timelineEvents[timelineEvents.length - 1]}`
   - After switch from prepend → append, [0] is now the *oldest*. This is a regression for any code expecting the "current" event in PlaybackControl.
   - **Impact:** Low-medium (PlaybackControl script doesn't seem to use the prop for critical UI state, but inconsistent and future-proofing risk).
   - **Suggestion:** Standardize on `timelineEvents[timelineEvents.length-1]` (or keep a separate `latestEvent`).

2. **COP display structure**
   - Spec asked for "scrollable COP-history list *beneath the current summary*".
   - Current code puts everything in `.cop-history-list` (latest first in the list).
   - No top-level `{cop.summary}` outside the list anymore.
   - **Suggestion:** Consider keeping a prominent current COP summary + history list below, or document the consolidated approach.

3. **Potential duplicate prevention fragility**
   - Dedup uses `some(...)` on summary + stateVersion/risk. Works for demo but could miss slightly varied outputs in real LLM responses.
   - Fine for now.

### Suggestions / Polish (non-blocking)
- Add ring-buffer cap display or "X more" indicator for very long histories (nice-to-have).
- In CellPanel / cop-history, the "pinned analyzing skeleton" is good but could be more prominent.
- Consider extracting history push logic (Dashboard) to reduce duplication between live handlers and demo.
- Update PlaybackControl / Map callers for consistency (see bug #1).
- The copHistory numbering `#{length - idx}` works but can look odd; consider reversing render or using timestamps.
- Memory: 500 is reasonable; if logs grow very fast in real use, could make configurable.
- Test scrolling UX manually in browser (user scroll up should stick until near bottom).

### Nits
- Still some "CEREBRAS" strings in demo addLog calls (harmless due to transform).
- In cop-history-item: hard-coded styles mixed with classes.
- No explicit test for the new scroll/history behavior (web has no automated tests here).

### Positives
- Faithful to TASK decisions and locked choices (ordering, smart scroll, accumulate, map size).
- Clean separation: data model in Dashboard, presentation in CellPanel/Matrix.
- Smart scroll prevents the "yank while reading" problem called out in the task.
- Ring buffers prevent unbounded growth.
- The fix commit shows responsiveness to runtime issues.
- Demo fully updated (no more single-cell overwrite in triggers).
- CSS changes are targeted and preserve existing look/feel.
- Build clean; no dependency on provider changes.

## Comparison to TASK-ui-scrollback.md "Done When" Criteria
- ✅ Matrix + Event log accumulate (500), tail when at bottom, smart scroll.
- ✅ Specialist Cells: scrollable history (newest top), append not overwrite.
- ✅ Commander: accumulating scrollable COP history (newest top).
- ✅ Map visibly smaller; data areas larger.
- ✅ `npm run build` clean.
- Partial: Current COP is in the history list (not strictly "beneath" a separate summary).

## Recommendations
1. Fix the timeline[0] vs length-1 inconsistency before more work on playback/map.
2. Consider restoring a distinct "current COP" summary + history beneath it to better match spec language (or update TASK if intentional).
3. Add a small manual test note or Storybook-like example for scrolling behavior.
4. If histories become very long, add virtual scrolling for cells (overkill today).
5. Keep the undefined guards; they were a good catch.

P13 and P14 successfully deliver the UI scrollback + layout refresh. The changes are focused, match the detailed task spec, and improve usability for reviewing history (key for the "re-reasoned" demo story). Minor cleanups recommended.

**Files primarily reviewed:**
- web/src/components/{Dashboard.svelte, CellPanel.svelte, MatrixFeed.svelte, PlaybackControl.svelte, Map.svelte}
- web/src/styles/dashboard.css
- TASK-ui-scrollback.md, commit diffs
- Build output

**Review location:** reviews/p13-p14-ui-scrollback-review.md (this file)  
**Status:** Ready for HANDOFF update / next parcels if issues addressed.