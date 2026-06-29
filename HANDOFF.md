# HANDOFF.md — Builder onboarding & current repo state

Operational companion to [`AGENTS.md`](AGENTS.md) (the rules) and
[`SPEC.md`](SPEC.md) (the design). Start here when you pick up a task.

> **Builder** = you, an AI writing this code.  **Cell** = a runtime agent inside
> the product. Never conflate them.

## 1. Read order (do not skip)

1. [`AGENTS.md`](AGENTS.md) — the seven rules, in full.
2. [`SPEC.md` §0](SPEC.md) — the binding Multi-Agent Development Protocol.
3. The SPEC section(s) for **your** package, then the **§16.1 ownership table**.
4. [`internal/contracts/`](internal/contracts/) — the only source of truth for
   any cross-package shape.

## 2. Pre-flight checklist (SPEC §0.3) — before writing a line

- [ ] My task touches **only the package(s) I own** (§16.1). If not → **stop and flag**.
- [ ] I've read the contract files my package depends on.
- [ ] Anything I need from another package exists as an **interface** in
      `contracts/interfaces.go`. If a shape is missing, I do **not** invent it
      locally — I raise a contract change (§4 below).
- [ ] I can build & test my package in isolation against contract fakes.

## 3. Current repo state

The **skeleton + v0 contracts** are in place. Every `internal/*` package has a
`doc.go` stating its owner, dependencies, and "must-not" (from §16.1). The two
`cmd` mains are stubs. `web/` is intentionally **not** scaffolded (frontend
Builder's lane — see [`web/README.md`](web/README.md)).

| Area | State | Owner picks up |
|---|---|---|
| `internal/contracts/` | **v0 types defined** (events, state, agentio, interfaces, scenario, errors) | refine via §0.5 only |
| `internal/contracts/contracttest/` | seed round-trip test | extend (collectively owned) |
| `internal/state` + `validation` | **implemented (Claude Builder)** | world state + §14.2 gate |
| `internal/events` | **implemented by Codex Builder** | event bus |
| `internal/anomaly` | **implemented by Grok Builder** (Classifier via §0.5 w/ orchestrator) | fan-out triggers |
| `internal/orchestrator` | **implemented by Antigravity Builder** | concurrent fan-out + Commander |
| `internal/agents` | **implemented (Gemma 4 31B on Cerebras Builder)** | the six Cells |
| `internal/llm` | **implemented by Antigravity Builder** | Cerebras client |
| `internal/simulation` + `scenario` | **implemented by Grok Builder** | sim clock + replay |
| `internal/scenariogen` (+`cmd`) | **implemented** — drafted by Gemma 4 31B on Cerebras Builder, **rescued/completed by Claude Builder** (Gemma 429'd; substrate-corruption bug + llm-ctor + test fixed) | offline authoring tool → validated, frozen `scenario.json` |
| `internal/timeline` | **implemented** (Poolside Laguna M) | event log |
| `internal/sensors` | stub | ingest adapters |
| `internal/api` + `websocket` | **implemented by Grok Builder** | HTTP/WS edge |
| `cmd/eoc` (server wiring) | **implemented (Claude Builder)** — end-to-end flow tested live | integration root: wires all pkgs + runs the anomaly→fan-out loop, serves api/ws |
| `web/` | **implemented by Antigravity Builder** | Astro+Svelte dashboard |

**MVD build order (SPEC §13):** scenario+sim → events → state → anomaly →
orchestrator fan-out of 2–3 Cells → Commander → dashboard.

**Live production note (post-2026-06-29):** The application is now deployed and live on Fly.io (https://cerebro-eoc.fly.dev/). Development has transitioned from the original fully-parallel parcel model to a shared common branch workflow for better coordination and stability on production code.

- Common working branch: `feat/live-simulation-controls` (created off `main`).
- All new feature work (P19+) and non-urgent changes target this branch (or short-lived feature branches off it).
- Main remains the production baseline. Hotfixes may land directly on main when necessary.
- All AGENTS.md / SPEC §0 rules remain strictly in force (lane ownership, contract-first changes, one-package-per-commit, determinism, `task check` + contracttest green before merge).
- Builders continue to respect package ownership (§16.1) even when collaborating on the shared branch.

**Claimed (2026-06-29) — BUG-4/BUG-5 state-completeness** ([`TASK-state-completeness.md`](TASK-state-completeness.md)):
- Full Part A: Road entity + EventRoadBlocked handler; EventBridgeCollapsed + PowerDegraded.
- Part B1: BuildingCollapsed / TunnelClosed documented as deliberate trigger-only (no entity) + pinning tests.
- Contract change per §0.5 (additive only), LegalRoad bidirectional, wake rules, all tests + contracttest, acceptance criteria.

**Claimed by Claude Builder (2026-06-29) — review hardening, P1 contracts, deploy planning:**
- **Review fixes** (from 4 LLM code reviews): BUG-1 mock LLM honors `ctx` cancellation; BUG-2 `Envelope` rejects negative timestamps; BUG-3 seeded LLM backoff `rand` (determinism §0.2 r5); BUG-7 removed dead `orch` field from `api.Server`. Tests added.
- **Repo-wide `modernize` sweep:** `maps.Copy`, range-over-int, `atomic.Int32`, `slices.Contains`, `t.Context`, tagged switch, `any` — zero gopls hints remain.
- **P1 (contracts, §0.5 additive):** HUD telemetry — `CellMetrics`+`CellOutput.Metrics`, `COPMetrics`+`COP.Metrics`, `LLMResponse.LatencyMS`; contracttest extended. **Unblocks P2–P5.**
- **Planning:** authored [`TASK-state-completeness.md`](TASK-state-completeness.md); reviewed the BUG-4/5 impl; parceled the Cerebras-effectiveness work (§8, P1–P8) and rewrote [`hosting.md`](hosting.md) for single-origin deploy.
- Verified: `go build`/`vet`/`gofmt`/`go test ./...` + `contracttest` all green.
- **Next (open on common branch):** P19–P23 (Simulation Clock, Stats, and All Clear — see detailed design below). All prior parcels (P1–P18) landed and the MVD is live on Fly.io. Work for these parcels is happening on the shared branch `feat/live-simulation-controls`.

### 3.1 Post-spine + live production workflow

The reasoning spine (sim → events → state → anomaly → orchestrator → Cells →
Commander) is **complete and green**. The MVD is deployed live.

**Workflow change (2026-06-29+):** Because the product is live, Builders now work from the shared common branch `feat/live-simulation-controls` (branched from main). 

- Feature work happens on this branch (or short-lived sub-branches).
- PRs target `feat/live-simulation-controls`.
- Main is treated as the production baseline.
- Lane ownership, contract-first discipline (§0.5), and one-package-per-commit rules are still mandatory.
- For the current open work (P19–P23), P19 (contracts) must still land as its own isolated commit.

Current open parcels are P19–P23 (see below). Earlier parallelism notes are superseded by the common-branch model.

**`internal/scenariogen` (+`cmd/scenariogen`)** — being built as an **offline
authoring tool**: Gemma drafts the curated 3-act anomaly beats → validated by
replay through `state.New(Initial)` + `Apply` → frozen to a deterministic,
reviewable `scenario.json`. It **never** runs on the live path (§14.1); its only
product is the frozen file the simulation replays.

**Deferrable for the MVD:** `internal/sensors` (the simulation is the event
source). Volume/liveness — the "thousands of data points" — comes from a
**templated, non-LLM** ambient-noise generator (§14.3) layered in later, **not**
from scenariogen or the live LLM path. Keep the signal (curated, LLM-authored,
frozen) separate from the volume (templated, cheap, deterministic).

## 4. Changing a contract — the ONLY cross-lane action (SPEC §0.5)

You may **not** unilaterally edit `internal/contracts/`. Instead:

1. Propose the change: **what** field/type, **why**, **who** it affects.
2. Get agreement.
3. Land it as its **own isolated commit** touching only `contracts/`
   (message `contract(<file>): …`).
4. Each affected owner then updates their package.

Contract changes are **additive by default**; a breaking change updates every
consumer in the same pass.

## 5. Command cheat-sheet (Taskfile — SPEC §19.1)

```sh
task                 # list tasks
task check           # build + vet + test — the local gate before "done"
task test            # unit tests
task contracttest    # the shared contract suite (§0.6) — must pass
task build:eoc       # build the server binary
task run             # run the EOC server
task docker:build    # Linux image — the build authority (§19.3)
```

## 6. The traps that fail only in CI/Docker (not on your machine)

- **Determinism is law (§0.2 r5):** no wall-clock reads, no unseeded `rand`, no
  map-iteration-order in logic that affects state/output. Use the injected sim
  clock and the scenario `Seed`. Event time is `contracts.SimTime` (scenario
  seconds), **not** wall time.
- **Cross-platform (§19.2):** lowercase package dirs/filenames (Linux is
  case-sensitive); paths via `filepath.Join`; bundle assets with `//go:embed`;
  no shell one-liners in build scripts. LF endings are enforced by
  `.gitattributes`.
- **No shared mutable global state (§0.2 r4):** only `internal/state` holds live
  world state, and it has one mutator path (`StateStore.Apply`).
- **Cerebras Concurrency Limit Warning:** The developer account has a strict **concurrency ceiling of 4 concurrent in-flight requests** (100 RPM / 100k TPM). Because the MVD uses 4 Cells (Infrastructure, Medical, Population, Commander), any multi-turn loops (such as plan->critique->refine) executing in parallel *will* hit HTTP 429s. The LLM client handles this gracefully via internal queueing and backoff/retry, but orchestrator/agent builders should be aware of this tight concurrency budget.

## 7. Definition of done (SPEC §0 / §19.3)

Your package builds in isolation, ships unit tests, **passes the full
`contracttest` suite**, is `gofmt`-clean, and touched **no files outside your
lane** (except a coordinated §0.5 contract commit). Green Linux CI = green
everywhere.

## 8. Cerebras-effectiveness work parcels (hackathon, 2026-06-29)

Source: [`CEREBRAS-EFFECTIVENESS.md`](CEREBRAS-EFFECTIVENESS.md). Four gaps between
the "wafer-scale parallel reasoning" thesis and the current build. Parceled by
lane so they run mostly in parallel **after the one contracts change (P1) lands**.

### Reality check vs. the doc (read before picking a parcel)
- **The hard 4-concurrent ceiling (§6) is the real constraint — NOT the client
  cap.** Raising `llm` `maxConcurrency` 4→6 does not speed anything up; it trades
  client-side queueing for server-side HTTP 429 + backoff (i.e. *worse* latency).
  The real lever is *how many cells fan out at once* (cell roster) and/or a higher
  Cerebras account limit.
- **Roster decision (2026-06-29): register all 6 cells.** A mainshock now wakes
  **5 specialists** → 5 concurrent calls against the 4-cap. The `llm` client's
  semaphore (`maxConcurrency=4`) **is the queue**: 4 run immediately, the 5th
  blocks on the channel (Go serves blocked senders FIFO) until a slot frees, then
  runs — so we **never exceed 4 in-flight and never trip a 429**. With near-instant
  Cerebras inference the 5th's wait ≈ one request-duration (negligible).
  **Keep `maxConcurrency=4`; do NOT raise it** — raising removes the queue and
  causes server-side 429s + backoff (worse). This also **resolves BUG-6** (anomaly
  was waking unregistered Intelligence/Communications). Caveat: all-6 ×
  (optional critique) multiplies *total* requests → watch the 100 RPM / 100k TPM
  budget; gate critique (P3) accordingly.
- **Critique loops don't raise *peak* concurrency** (each cell holds one in-flight
  request at a time), but they multiply **total** requests → keep plan→critique
  **sequential within a cell** and watch the 100 RPM / 100k TPM budget.

### Parcels

**Status legend:** ✅ Done · 🔵 Claimed (in progress) · ⬜ Unclaimed (free to pick up).
To claim: change the cell to 🔵 with your builder name + date in the same commit you start.

| ID | Status | Lane / owner | Work | Depends on |
|----|--------|--------------|------|-----------|
| **P1** | ✅ **Done** — Claude Builder (commit `47a4809`) | `internal/contracts/` (§0.5 — isolated commit) | Add `CellMetrics{tokensIn,tokensOut,tokensPerSec,latencyMs}` + `CellOutput.Metrics`; `COPMetrics{fanOutLatencyMs,totalTokensIn,totalTokensOut,peakTokensPerSec,aggregateTokensPerSec,cellCount}` + `COP.Metrics`; `LLMResponse.LatencyMS`. All **additive**. (`Perception`/`ImageInput` already exist.) | — |
| **P2** | ✅ **Done** — Antigravity Builder (2026-06-29) | `internal/llm` | Populate `LatencyMS` (real + mock); implement `Interpret` in new `perception.go` (mock + Cerebras vision `gemma-4-31b`, base64 data-URI, structured `[]Event`); **keep `maxConcurrency=4` (the semaphore IS the FIFO queue — never raise; see reality-check)**; optionally expose queue depth / wait-time as telemetry. | P1 ✅ |
| **P3** | ✅ **Done** — Grok Builder (commit `58d881c`) | `internal/agents` | **Add `NewIntelligence` + `NewCommunications` cell constructors** (mirror the existing 4; seismic/intel + comms prompts; mock responses already exist) so all 6 can be registered. `executeLLM` writes real metrics into `CellOutput.Metrics`; add **sequential** plan→critique pass (env-gated `LLM_CRITIQUE`, graceful fallback to draft on failure), aggregating tokens + latency. | P1 ✅ |
| **P4** | ✅ **Done** — Antigravity Builder (2026-06-29) | `internal/orchestrator` | Time the fan-out (wall clock around phase-1/2); aggregate specialist + Commander metrics into `COP.Metrics`. | P1 ✅, P3 |
| **P5** | ✅ **Done** — Grok Builder (2026-06-29) | `internal/api` | `POST /perception` (multipart or raw bytes → `Perception.Interpret` → publish events to bus). Inject a `Perception` dependency into `Server`. Handler stamps sim time for validity + publishes to bus. Tests + isolation green. | P1 ✅, P2 |
| **P6** | ✅ **Done** — DeepSeek V4 Pro (2026-06-29) | `cmd/eoc` (integration root — **single owner of `main.go`**) | (a) **Serve static `web/dist` at `/`** (dir from `WEB_DIR`, default `web/dist`) alongside the API routes + `/stream`; (b) **honor `$PORT`** (Cloud Run contract, fallback `8080`); (c) wire the perception client into `api.New`; (d) **register all 6 cells** (Intelligence, Infrastructure, Medical, Population, Communications, Commander) — decided 2026-06-29; the `llm` semaphore queues the 5th specialist under the 4-cap. All four sub-tasks landed, `go build` + `go test ./...` + `contracttest` green. | P2, P5, P3(d) |
| **P7** | ✅ **Done** — Antigravity Builder (2026-06-29) | `web/` | HUD reads `cop.metrics` (real tok/s + fan-out latency, replacing the hardcoded `1500`); add a drone/satellite image upload widget → `POST /perception`. **Single-origin (see §8 deploy decision): keep the existing same-origin WS/fetch — do NOT add a `PUBLIC_API_URL`.** | P1 ✅ (shapes), running `api` |
| **P8** | ✅ **Done** — Grok Builder (2026-06-29) | `Dockerfile` + `Taskfile.yml` + `.env.example` (deploy/build lane; `hosting.md` already done) | **Multi-stage image**: Node stage builds `web/dist` → Go stage builds `eoc` → distroless runtime carrying the binary **and** `web/dist`. Add a `docker:build`/deploy Taskfile target; document `PORT`/`WEB_DIR`/`CEREBRAS_*` in `.env.example`. Validate: `docker run -e PORT=9090 -p 9090:9090 <img>` serves the dashboard live. | P6 behavior (serves `WEB_DIR`, honors `$PORT`) + a working `web` build |
| **P9** | ✅ **Done** — DeepSeek V4 Pro (2026-06-29) | `internal/llm` | Add OpenRouter as alternative provider. Extend client for OpenAI-compatible /chat/completions + vision. Support separate OPENROUTER_* env vars (key, baseURL, model). Make provider switchable (global). Update prompt/schema handling if needed. Add mocks for both. 26 tests pass (13 original + 10 new P9 + 3 additional). | P8 |
| **P10** | ✅ **Done** — Grok Builder (2026-06-29) | `internal/api` | Add global provider switch API: GET/POST /provider to read/set current provider (cerebras/openrouter). Wire through to llm client. Broadcast change over WS. | P9 |
| **P11** | ✅ **Done** — DeepSeek V4 Pro (2026-06-29) | `cmd/eoc` | Support multiple LLM clients (or switchable one) for global provider. Update main wiring, app state, broadcast. Initial provider from env or default. | P6, P9, P10 |
| **P12** | ✅ **Done** — Antigravity Builder (2026-06-29) | `web/` | Add global provider dropdown (e.g. HUD or controls). Call /provider on change. Update all "CEREBRAS"/logs to reflect current provider. | P10, P11 |
| **P13** | ✅ **Done** — Antigravity Builder (2026-06-29) | `web/` | UI layout fixes: Make map smaller (too big currently). Enlarge data point areas (HUD metrics, panels, timeline, matrix, commander). | — (independent of P9–P12; see task doc) |
| **P14** | ✅ **Done** — Antigravity Builder (2026-06-29) | `web/` | Make timeline, matrix feed, logs hold full history + proper scrolling (instead of refresh/overwrite). **Decision: everything accumulates** — feeds + Commander COP + specialist Cell outputs all become scrolling histories. | — (independent of P9–P12; see task doc) |
| **P15** | ✅ **Done** — Antigravity Builder (2026-06-29) | `web/` | Additional UI polish: improve controls, badges, perception panel, live vs demo clarity, general layout/responsiveness. | P14 |
| **P16** | ✅ **Done — Claude Builder (2026-06-29)** | deploy/build + docs | Docs ✅ (OPENROUTER_* in .env.example; hosting.md §4.1). **Live validation: 2×2 matrix all-green** (Cerebras + OpenRouter, text + vision) after the **P17** `internal/llm` fixes landed. | P8, P9, **P17** |
| **P17** | ✅ **Done — Claude Builder (2026-06-29)** | `internal/llm` | Fix dual-provider bugs found by P16 validation (see findings below): object-wrapped perception schema, `extractJSON` fence/prose stripping, `response_format` sent for OpenRouter too, vision event-type aliases. All providers green for text + vision. | P9 |
| **P18** | ✅ **Done — Claude Builder (2026-06-29)** | `web/` + `cmd/eoc` + `internal/api` | Preset "scenario" buttons now **inject real events** (POST `/events`) instead of mock-only filename strings (which 400'd the real vision API). Apply-time timestamp stamping in `cmd/eoc` `handle()` keeps injected events monotonic-valid; `/events` no longer stamps. (See "P18" section below.) | P16/P17 |
| **P19** | ✅ **Done** — Grok Builder (2026-06-29) | `internal/contracts` | §0.5 contract change: Add SimulationStats, SimulationInfo, SimulationController, TokenStatsProvider (with SimulationStatus constants + ElapsedTime for P22 metrics). | — |
| **P20** | 🔵 **Claimed (simulation)** — Grok Builder (2026-06-29) | `internal/llm` + `simulation` + `state` + `timeline` | Core engines update: atomic counters, wall stopwatch, state reset, and timeline truncate. | P19 |
| **P21** | 🔵 **Claimed (api)** — Grok Builder (2026-06-29) | `internal/api` + `cmd/eoc` | EOC coordinator and WS broadcast of "reset" kind + actual API endpoint wiring. | P20 |
| **P22** | ⬜ **Unclaimed** | `web/` | Svelte visual updates: All Clear button, timeline clock with limits, stats dashboard widget. | P21 |
| **P23** | ⬜ **Unclaimed** | verification/tests | Add contracts roundtrip + engine + llm stats unit tests. Manual validation of All Clear. | P22 |



### P16 validation findings + P17 fixes (2026-06-29, live keys)
Ran `cmd/eoc` against real Cerebras + OpenRouter keys (scenario replay = text;
`POST /perception` with a real PNG = vision; runtime `POST /provider` switch).
Connectivity sanity-checked per-model with raw `chat/completions` curls (gemma +
deepseek): gemma text/vision both respond, but **wrap JSON in ```json fences /
prose**; Cerebras returns clean JSON.

**Before P17** — only 1/4 quadrants worked:

| | Cerebras | OpenRouter |
|---|---|---|
| **Text** | ✅ | ❌ prose/markdown fails schema validation (`response_format` was dropped for OpenRouter) |
| **Vision** | ❌ `response_format` schema rejected | ❌ markdown-fenced JSON not stripped |

**After P17 — all green:** Cerebras text/vision ✅, OpenRouter text/vision ✅
(OpenRouter ~15s/fan-out, slower than Cerebras but reliable, no timeouts).

**P17 fixes (`internal/llm`, no contract change):**
1. **Cerebras vision schema** ([`perception.go`](internal/llm/perception.go)) — wrapped the
   top-level array in an **object** `{"events":[…]}` (Cerebras rejects top-level
   `items`); gave `payload` explicit properties (Cerebras strict needs object
   `properties`). Parser tolerates both `{"events":[…]}` and a bare `[…]`.
2. **`extractJSON`** ([`client.go`](internal/llm/client.go)) — strips ```` ```json ```` fences
   **and** narrows to the outermost `{…}`/`[…]` span (drops gemma's prose preamble).
   Applied to the cell-text path and perception. Safety net even with (1)/(3).
3. **`response_format` for OpenRouter too** ([`client.go`](internal/llm/client.go)) — removed
   the Cerebras-only gate; without structured output gemma returns prose. OpenRouter
   honors json_schema for the Gemma family, so cells now get clean JSON.
4. **Vision event-type aliases** ([`perception.go`](internal/llm/perception.go)) — `fire`→
   FireIgnited, `flood`/`flooding`→FloodExtentUpdated, `bridgeblockage`→BridgeCollapsed.

Tracked as **P17** (✅ Done). `go test ./...` + `contracttest` green; gofmt/vet clean.

### P18 — preset triggers inject real events (2026-06-29)
**Root cause found during deploy smoke-testing:** the perception-panel preset
buttons ("Vora Bridge Collapse", etc.) POSTed a **filename string** to
`/perception`. That only worked in **mock mode** (`interpretMock` substring-matches
the string); with real keys it hit the vision API with undecodable bytes → 400
`invalid_image`. (Reminder: **image perception is an *optional* ingest — the
simulation datastream drives the demo**, SPEC §14. Presets are a manual nudge, not
a required trigger.)

**Fix — presets now publish real events onto the bus:**
- **`web/` ([`PerceptionUpload.svelte`](web/src/components/PerceptionUpload.svelte))** —
  `triggerPreset` POSTs a real `contracts.Event` to `/events` (timestamp omitted):
  `Aftershock M5.5` (`AftershockOccurred`, wakes all 5), `Vora Bridge Collapse`
  (`BridgeCollapsed{B-VORA}`), `Highgate Building Collapse`
  (`BuildingCollapsed{S-HIGHGATE}`). **Avoid `LeveeBreached`** — the scenario
  already breaches the levee, so a repeat is an illegal transition.
- **`cmd/eoc` ([`main.go`](cmd/eoc/main.go) `handle`)** — events arriving with
  `Timestamp == 0` are stamped to the **current world time at apply time**, so they
  satisfy Apply's temporal-monotonicity rule (`ev.Timestamp < ws.Time` → reject,
  [`state/store.go`](internal/state/store.go)) no matter how far the replay has
  drained. Handler-side stamping was racy against the bus queue and got rejected.
- **`internal/api` ([`api.go`](internal/api/api.go) `/events`)** — dropped the racy
  handler-side stamping; the loop owns it.

**Verified live** (real keys): each preset fires a real fan-out (woke cells + new
COP) from the post-replay end-state. Note: most *entity*-state events (levee, an
already-closed bridge) won't re-fire from the damaged end-state; **seismic events
always do** — they're the most robust manual triggers.

**Also reverted (not committed):** a perception image-transcode experiment +
`golang.org/x/image` dep — it solved a non-issue (the error was the preset strings,
not real uploads) and regressed JPEG. Perception is back to the P17 state: raw
passthrough, **PNG/JPEG** uploads work; GIF/WEBP are not converted.

### Provider switching from the UI (P12, confirmed working)
The HUD **LLM Provider** dropdown ([`HUD.svelte`](web/src/components/HUD.svelte))
flips **Cerebras ↔ OpenRouter** live: `change → POST /provider → llm.SetProvider →
broadcast over /stream`. It switches the **provider/backend**, each pinned to its
env model (`CEREBRAS_MODEL=gemma-4-31b`, `OPENROUTER_MODEL=google/gemma-4-31b-it`) —
there is **no per-model picker**. To demo a different model (e.g.
`deepseek/deepseek-v4-flash`), set `OPENROUTER_MODEL` at deploy. The switch applies
to the **next** fan-out; OpenRouter is ~15s/fan-out vs Cerebras ~1-2s.

### Deploy smoke-test status (2026-06-29, local Docker)
`docker build` + `docker run -e PORT=9090 --env-file .env` (with `-e WEB_DIR=/web/dist`)
serves the dashboard **live** (real metrics, scenario replay, COPs) and the
`/provider` switch works in-container. This is the same image Fly will build — the
remaining deploy steps are mechanical (see [`hosting.md`](hosting.md) §4).

### Live on Fly.io (2026-06-29) — ✅ DEPLOYED & VERIFIED LIVE
App **`cerebro-eoc`** deployed from [`fly.toml`](fly.toml) (region `iad`,
single-origin image). URL: **https://cerebro-eoc.fly.dev/**.

**Verified live** (external curl, no Fly auth): `fly status` = 1 machine, health
check passing; `/state` version 17 / time 170 (full scenario replay); `/agents`
returns real entity-grounded COPs (not the canned mock strings) → Cerebras secrets
set and doing real inference; `/provider` = cerebras. Single-instance rule honored.

Gotchas hit + fixes (record for the Cloud Run migration / re-deploys):
- **Fly creates 2 machines by default** (HA, zero-downtime) — this **violates the
  single-instance rule**. Always `fly scale count 1` immediately after deploy and
  confirm with `fly status` (exactly 1 machine). The 512mb/iad/no-auto-stop config
  is in `fly.toml`, but the machine *count* is not — it must be scaled explicitly.
- **Secrets**: set the two keys from `.env` (do NOT put them in `fly.toml`):
  `fly secrets set CEREBRAS_API_KEY=… OPENROUTER_API_KEY=…` (a placeholder typo
  leaves the app in **mock mode** — no real LLM). Setting secrets rolls the machine.

Post-deploy verification (the demo bar): `fly status` shows 1 machine ✅; `/state`
serves a fully-replayed snapshot ✅; `/agents` shows real (non-mock) COPs ✅.
Confirm in a browser too: page loads **Live / Connected**, provider dropdown flips,
a preset button fires a real fan-out.

**Still pending:** Cloudflare custom domain (CNAME → `cerebro-eoc.fly.dev`,
proxied, Full(strict)) — see [`hosting.md`](hosting.md) §5.

### Deploy decision (2026-06-29): single-origin on Fly.io (Cloud Run later)
Per [`hosting.md`](hosting.md): **one Go container serves both the static
dashboard (`web/dist`) and the API/WS**, fronted by Cloudflare. Single-origin
because the frontend is hard-coded **same-origin** (WS `wss://<page-host>/stream`,
relative `fetch("/state")`); a two-subdomain split would silently drop the live
demo to offline demo mode and would need CORS the API doesn't have.

**Target: Fly.io now** (7-day trial, fastest single-always-on-container path),
**Cloud Run later** — same OCI image, so migration is cheap (move 2 secrets,
repoint the Cloudflare CNAME; Cloud Run recipe kept in `hosting.md` §7). Both
providers shipped switchable (boot Cerebras → live-switch to OpenRouter).

**⚠️ Single-instance rule (every platform):** world state is in-memory
([`internal/state`](internal/state)) + pushed over WS, and the scenario replays
per-process → must run **exactly one instance** (Fly `fly scale count 1` +
`auto_stop_machines=false`; Cloud Run `--max/min-instances 1`). No HA by design.
`.dockerignore` now excludes `.env*` (keys out of build layers). `fly.toml`
hardening (no auto-stop, restart=always, `/state` health check, US region) is in
`hosting.md` §4.1.

### Landed (2026-06-29): Westbank → Westside display rename
Sector/clinic/road **display names** renamed `Westbank` → `Westside` across `web/`,
`cmd/eoc/scenario.json`, `internal/llm` (mock strings), and `internal/scenariogen`.
**IDs intentionally unchanged** (`S-WESTBANK`, `H-WESTBANK`, `R-WEST-1`, lowercase
`westbank` key) — renaming identifiers buys nothing and risks breaking refs.

### Suggested sequence (common branch era)
Work for remaining parcels (P19+) occurs on the shared branch `feat/live-simulation-controls`.

1. **P19** (simulation contracts) — §0.5 isolated commit. Coordinates new interfaces and stats DTO shapes. **Must land before P20+**.
2. **P20** (backend implementation of stopwatch, counters, store reset, and timeline truncate).
3. **P21** (API endpoint implementation and cmd/eoc coordinator).
4. **P22** (Astro/Svelte HUD, PlaybackControl stats and All Clear button).
5. **P23** (Verification of resets, counters, and timeline truncation via tests).

All changes follow the updated live workflow rules above (lanes, contracts, one-package-per-commit). Deployments to Fly still require explicit single-instance scaling and secret management (see live section).

**File ownership note (common branch era):** Each parcel still owns a largely disjoint set of files (see list above). The shared `feat/live-simulation-controls` branch does not relax lane ownership or contract rules — changes crossing lanes still require coordination and (for contracts) isolated §0.5 commits.

### Priority for demo impact
1. **Telemetry** (P1 + P3 + P4 + HUD half of P7) — turns the HUD's fake
   `1500 tok/s` into real wafer-scale numbers. Highest value-per-effort, fully
   verifiable in mock mode.
2. **Perception** (P2 + P5 + P6 + upload half of P7) — the live "drop a disaster
   image → instant fan-out" wow moment.
3. **Critique** (P3) — the "multi-turn still sub-second on Cerebras" beat;
   cheapest to add, mind RPM.
4. **P9–P11** enable provider comparison (global switch to OpenRouter for same model).
5. **P13–P15** address current UI problems (map too big, data areas too small, logs refresh instead of hold+scroll) + general polish.
6. **All Clear, Clock, and Stats** (P19–P23 on common branch) — adds the visual timeline slider, wall/sim telemetry stats, and system-wide reset controls.

---

### Simulation Clock, Stats, & All Clear (P19–P23) — Detailed Design

#### P19 Coordinated Contract Addition
- Add `SimulationStats` DTO, `SimulationInfo` struct, `SimulationController`, and `TokenStatsProvider` to `internal/contracts/interfaces.go`.

#### P20 Core Engines Reset & Stats Support
- **`internal/llm` (`client.go`)**: Add thread-safe atomic counters for `tokensIn`, `tokensOut`, and `requestCount`. Increment on successful model completions and interpretations (both real and mock). Expose `TotalTokens()`, `TotalRequests()`, and `ResetStats()`. Mock mode completions increment these counters with simulated values (~1000 input, ~300 output tokens per call) so the offline/demo mode dashboard shows telemetry movement.
- **`internal/simulation` (`engine.go`)**: Add `wallStart time.Time` and `wallElapsed time.Duration`. Manage the stopwatch in lifecycle methods: start on `Step()` or when entering unpaused `Run()`, accumulate on `Pause()`, freeze on scenario completion, and reset to zero on `Reset()`. Expose `Status()`, `Info() SimulationInfo`, and `WallElapsed()`.
  - **Determinism Firewall**: The stopwatch is strictly for display telemetry and isolated from any state, event generation, COP, or replay logic. It must never affect determinism and is excluded from contracttest assertions.
- **`internal/state` (`store.go`)**: Expose a thread-safe `Reset(initial contracts.WorldState)` method to reload the scenario's t=0 substrate. This method clears the `seen` duplicate event ID map. It is documented as the single sanctioned exception to the single-mutator rule.
- **`internal/timeline` (`timeline.go`)**: Expose a thread-safe `Truncate()` method to clear timeline log entries.

#### P21 API Controllers & cmd/eoc Wiring
- **`internal/api` (`api.go`)**: Add `SimulationController` and `TokenStatsProvider` to `api.Server`. Update `/scenario/reset`, `/scenario/pause`, `/scenario/resume`, `/scenario/step`, `/scenario/speed` endpoints. Add `GET /scenario/stats` endpoint returning the parsed `SimulationStats` JSON.
- **`cmd/eoc` (`main.go`)**: Define `eocSimController` struct:
  ```go
  type eocSimController struct {
      sim      *simulation.Engine
      store    *state.Store
      tl       *timeline.Timeline
      initial  contracts.WorldState
      copStore *copStore
      bcast    api.Broadcaster
  }
  ```
  Implement the `SimulationController` interface. 
  - **Reset Epoch Guard**: Implement `epoch` tracking. On `Reset()`, increment the epoch, cancel the reasoning context (which aborts all in-flight cell/orchestrator calls), unsubscribe and resubscribe to the EventBus to discard queued events, and launch a new `runLoop` goroutine with the new context and epoch. Discard callbacks and WS broadcasts from outdated epochs.
  - **Reset Actions**: Call `sim.Pause()`, `sim.Reset()`, `store.Reset()`, `tl.Truncate()`, append a synthetic `SystemReset` event (`tl.Append(contracts.Event{ID: "system-reset", Timestamp: 0, Source: "system", Type: "SystemReset", Confidence: 1.0})`), reset `copStore` to Low risk, and broadcast `kind: "reset"` message over WS.

#### P22 Frontend Svelte Components
- **`PlaybackControl.svelte`**: Render a green styled **All Clear** button. Render a horizontal **Simulation Progress** timeline bar with Start and End bounds. Render a **Simulation Metrics** grid showing elapsed Sim time, elapsed Wall time, replayed event count, LLM tokens, and inferences run.
- **`Dashboard.svelte`**: Periodically poll `/scenario/stats` every 1s (or fake metrics locally in demo mode). Handle WS `kind: "reset"` to empty the local timeline/COP/cell logs.

#### P23 Verification & Sequencing
- **Sequencing**: All work is on the common branch `feat/live-simulation-controls` (branched from main after the live baseline was frozen). Merge via PRs that respect lanes and contracts.
- **Tests**: Write stats tests in `internal/simulation/engine_test.go` and `internal/llm/client_test.go`. Include a determinism firewall test in `engine_test.go` asserting that varying wall-clock times or stopwatch reads do not change scenario replay events or logical timing. Ensure `task check` compiles and all tests pass.


