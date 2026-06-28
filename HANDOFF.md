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
| `internal/scenariogen` (+`cmd`) | **claimed by Gemma/Pi Builder** — offline authoring tool | Gemma → validated, frozen `scenario.json` |
| `internal/timeline` | **implemented** (Poolside Laguna M) | event log |
| `internal/sensors` | stub | ingest adapters |
| `internal/api` + `websocket` | **claimed by Grok Builder**; implementation starting | HTTP/WS edge |
| `cmd/eoc` (server wiring) | **stub** | integration root: wires all pkgs + runs the anomaly→fan-out loop, serves api/ws |
| `web/` | README only | Astro+Svelte dashboard |

**MVD build order (SPEC §13):** scenario+sim → events → state → anomaly →
orchestrator fan-out of 2–3 Cells → Commander → dashboard.

### 3.1 Remaining work & parallelism (post-spine)

The reasoning spine (sim → events → state → anomaly → orchestrator → Cells →
Commander) is **complete and green**. What's left, and what can run concurrently:

- **`internal/api` + `websocket`** — **independent, startable now.** Its deps
  (`StateStore`/`EventBus`/`Orchestrator` interfaces) are all done. Codes to
  `contracts/*`; imports neither `cmd/eoc` nor `web`.
- **`web/`** — **build-independent** (separate Astro/Svelte toolchain); can be
  scaffolded now against `contracts/schemas`. Needs a running `api` only for
  live data, not to start.
- **`cmd/eoc`** — the integration root; imports everything. Splits in two:
  - **headless reasoning loop** (sim → bus → `state.Apply` → `anomaly.Classify`
    → `orchestrator.FanOut` → log the COP) depends only on already-done
    packages → buildable + smoke-testable **now**, no api/web needed. Fastest
    proof the spine works end-to-end.
  - **HTTP serving** wiring depends on `api`+`websocket` landing first.

Dependency shape:

```
api+websocket ┐
              ├─> cmd/eoc  (wires all packages; serves the HTTP/WS edge)
web/ ─────────┘   (web → api at runtime, for live data)
```

So **`api`+`websocket` and `web/` can proceed in parallel**, and **`cmd/eoc`'s
headless loop can start in parallel too** — only cmd/eoc's *serving* wiring must
wait for `api`. Mind the measured Cerebras ceiling (4 concurrent, 100 RPM/TPM —
see §6) when the loop fans out live.

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
