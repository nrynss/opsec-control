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
| `internal/state` + `validation` | stub `doc.go` | world state + §14.2 gate |
| `internal/events` | stub | event bus |
| `internal/anomaly` | stub | fan-out triggers |
| `internal/orchestrator` | stub | concurrent fan-out + Commander |
| `internal/agents` | stub | the six Cells |
| `internal/llm` | stub | Cerebras client |
| `internal/simulation` + `scenario` | stub | sim clock + replay |
| `internal/scenariogen` (+`cmd`) | stub | offline compiler |
| `internal/timeline` | stub | event log |
| `internal/sensors` | stub | ingest adapters |
| `internal/api` + `websocket` | stub | HTTP/WS edge |
| `web/` | README only | Astro+Svelte dashboard |

**MVD build order (SPEC §13):** scenario+sim → events → state → anomaly →
orchestrator fan-out of 2–3 Cells → Commander → dashboard.

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

## 7. Definition of done (SPEC §0 / §19.3)

Your package builds in isolation, ships unit tests, **passes the full
`contracttest` suite**, is `gofmt`-clean, and touched **no files outside your
lane** (except a coordinated §0.5 contract commit). Green Linux CI = green
everywhere.
