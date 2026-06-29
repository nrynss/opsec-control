# TASK — UI scrollback histories + smaller map

**Lane:** `web/` only. **No contract changes, no backend.**
**Covers HANDOFF parcels:** P13 (map too big / data areas too small) + P14
(timeline/matrix/logs refresh instead of hold+scroll) + the scrolling slice of P15.
**Status:** ⬜ Unclaimed — scoped, ready to build.

> **Independence note:** this work does **not** depend on the provider switch
> (P9–P12). It only touches `web/` presentation and can land in parallel with the
> backend multi-provider parcels. (HANDOFF lists P13 "depends on P12" / P14
> "depends on P13" — that ordering was for the original UI sweep; the scrolling +
> map work has no real dependency on the provider dropdown and can go first.)

## Decisions (locked)
- **Everything accumulates.** Feeds, Commander COP, *and* the specialist Cell
  outputs all become scrolling histories rather than overwrite-in-place.
- **Map smaller.** Give the map a fixed modest height; let the (now scrolling)
  data areas take the remaining space.
- **History ordering.** **Newest-at-top for Cells and Commander** (latest analysis
  visible without scrolling), **bottom-tail for the two feeds** (Matrix + Event log
  read like a terminal, auto-scrolling down to the newest line).

---

## Change 1 — Scrolling histories

### 1a. Matrix Feed + System Event log  *(low risk, do first)*
- Remove the 100-line cap at `Dashboard.svelte` `addLog` (currently slices to 100)
  → **ring buffer (~500 lines)**: bounded memory, deep history.
- Make both feeds tail consistently: switch the System Event log from prepend
  (`timelineEvents = [payload, ...timelineEvents]`) to **append** so it tails at
  the bottom like the Matrix feed.
- **Smart auto-scroll**: only pin to the bottom when the user is *already* at the
  bottom — track `scrollTop + clientHeight >= scrollHeight - threshold` before
  setting `scrollTop = scrollHeight`. `MatrixFeed.svelte`'s `afterUpdate` currently
  force-scrolls unconditionally, which yanks the view down while reading back.
  Apply the same guarded pattern to the Event log container.

**Files:** `Dashboard.svelte` (cap + event-log append), `MatrixFeed.svelte`
(guarded auto-scroll), `dashboard.css` (Event log scroll container if needed).

### 1b. Specialist Cells  *(the heavy piece)*
- **Data shape:** replace `cellData[name]` (single latest) with
  `cellHistory[name] = []`; **push** each completed `CellOutput` instead of
  overwriting. Update every write site:
  - decl in `Dashboard.svelte`,
  - the three demo `triggerAct*` functions (`cellData.X = {...}`),
  - the live handlers (`payload.cellOutputs.forEach` and the `cell_output` branch).
- `CellPanel.svelte`: render the history as a **scrollable mini-log inside each
  card** (per entry: state version / time, risk badge, summary, top recs), with
  the analyzing skeleton pinned at the tail. `cellStatuses` dot logic unchanged.
- **CSS:** `.agent-card` needs an inner scroll region (`overflow-y:auto` + a
  bounded body via `max-height`/`flex`) since each card now holds N entries.

**Files:** `Dashboard.svelte`, `CellPanel.svelte`, `dashboard.css`.

### 1c. Commander COP  *(small)*
- Add `copHistory = []`; **push** each synthesis. Render a scrollable COP-history
  list beneath the current summary.
- **CSS:** `.commander-panel` / `.cop-summary` get a scroll container.

**Files:** `Dashboard.svelte`, `dashboard.css`.

---

## Change 2 — Smaller map  *(covers P13)*
- `dashboard.css` `.main-area`: `grid-template-rows: 1fr 220px` → **`360px 1fr`**
  (map fixed-modest; the now-scrolling specialist area gets the remaining space).
- `dashboard.css` `.cerebro-svg`: drop/relax `max-height: 520px`.
- One layout flip shrinks the map **and** enlarges the data areas — both P13
  complaints — and gives 1b the vertical room it needs.

**Files:** `dashboard.css`.

---

## Files touched (whole task)
`web/src/components/Dashboard.svelte`, `web/src/components/CellPanel.svelte`,
`web/src/components/MatrixFeed.svelte`, `web/src/styles/dashboard.css`.
**No backend, no `contracts/`.**

## Done when
- Matrix feed + Event log accumulate (ring-buffered) and tail only when at bottom;
  scrolling up to read history is not interrupted.
- Each specialist Cell shows a scrollable history of its analyses; new analyses
  append rather than wipe the previous.
- Commander shows an accumulating, scrollable COP history.
- Map is visibly smaller; HUD/panels/timeline/matrix/commander areas are larger.
- `npm run build` (astro build) in `web/` is clean.

## Already landed (out of scope here)
- **Westbank → Westside** display rename (keep IDs) — done across `web/`,
  `cmd/eoc/scenario.json`, `internal/llm`, `internal/scenariogen`. IDs
  (`S-WESTBANK`, `westbank`, etc.) intentionally unchanged.
