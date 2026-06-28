# opsec-control — AI Emergency Operations Center (EOC)

An API-first, event-driven, multi-agent command platform where specialist AI
**Cells** fire **simultaneously** the instant an anomaly is detected — built on
Cerebras wafer-scale inference (Gemma 4 31B). One quake in the city of Cerebro,
six Cells reasoning at once, the command center re-reasoning faster than the
disaster evolves. See **[`SPEC.md`](SPEC.md)** for the full design.

> Built by multiple AI coding agents (**Builders**) in parallel. **Read
> [`AGENTS.md`](AGENTS.md) and [`SPEC.md` §0](SPEC.md) before writing any code.**
> A **Builder** writes this repo; a **Cell** is a runtime agent inside the
> product — never conflate them.

## Prerequisites

| Tool | Version | Pinned by |
|---|---|---|
| Go | 1.24.5 | `go.mod` (`toolchain`) |
| Node | 25.9.0 | `.nvmrc` |
| [Task](https://taskfile.dev) | 3.x | task runner (no `make` — SPEC §19.1) |
| Docker | any | Linux build authority (SPEC §19.3) |

Copy `.env.example` to `.env` and add your `CEREBRAS_API_KEY`.

## Quick start

```sh
task            # list tasks
task check      # build + vet + test (the local gate before "done")
task run        # run the EOC server
task docker:build
```

## Layout (SPEC §16)

```
cmd/
  eoc/            # server entrypoint (wires interfaces; owns no logic)
  scenariogen/    # offline scenario compiler (Gemma -> validated JSON)
internal/
  contracts/      # ★ canonical types & interfaces — change only via §0.5
    contracttest/ #   shared contract test suite (§0.6)
    schemas/      #   JSON Schemas mirrored for frontend + validation
  state/          # sole world-state owner & mutator + §14.2 gatekeeper
  validation/     # event-validation rules (shared with scenariogen)
  events/         # event bus (pub/sub; owns no state)
  anomaly/        # which Cells wake per event (fan-out trigger)
  orchestrator/   # concurrent fan-out + Commander synthesis
  agents/         # the Cells (each independently ownable)
  llm/            # Cerebras client + throughput metrics
  simulation/     # deterministic sim clock + replay
  scenario/       # scenario loading
  scenariogen/    # offline generator logic
  timeline/       # immutable event log / replay index
  sensors/        # ingest adapters
  api/ websocket/ # HTTP/WS edge (the only thing the frontend talks to)
web/              # Astro + Svelte dashboard (talks ONLY to internal/api)
pkg/              # small dependency-free helpers
```

## The rules that keep parallel Builders from colliding (SPEC §0)

1. **Contract-first** — cross-package shapes live in `internal/contracts/`.
2. **Stay in your lane** — one owner per package (§16.1); never edit another's.
3. **Depend on interfaces, not implementations** (`contracts/interfaces.go`).
4. **No shared mutable global state** — only `internal/state` owns world state.
5. **Determinism is law** — no wall-clock, no unseeded `rand`, no map-order logic.
6. **Mock your dependencies** — build/test your package in isolation.
7. **Tests live with the code you own** — and pass `contracttest` (§0.6).

Changing a `contracts/` file is the **only** cross-lane action and is gated by
the §0.5 coordinated step — its own isolated `contract(...)` commit.
