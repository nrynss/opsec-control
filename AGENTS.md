# AGENTS.md — entry rules for every AI coding agent (Builder)

> This file is the **on-entry hook**. It is intentionally short. The binding rules live in
> [`SPEC.md` §0 — Multi-Agent Development Protocol](SPEC.md). Read §0 in full before writing any code.

You are a **Builder**: an AI coding agent (Claude, Codex, Grok, Pi, …) working on this repo
**in parallel with other Builders**. Discipline is what keeps you from overwriting each other.

## The word that matters
- **Builder** = you, an AI writing this code.
- **Cell** = a runtime AI agent *inside the product* (Intelligence, Infrastructure, Medical, Population, Communications, Commander).

Never conflate them. The spec says "Builder" for code authors, "Cell" for runtime agents.

## Before you write a single line
1. Read [`SPEC.md` §0](SPEC.md), then the section(s) governing your package, then the **§16 ownership table**.
2. Confirm your task touches **only the package(s) you own**. If it doesn't, **stop and flag** — do not proceed.
3. Read the relevant files in `internal/contracts/` — they are your **only** source of truth for any cross-package shape.

## The seven rules (full text in §0.2)
1. **Contract-first.** Shared types/interfaces/schemas live in `internal/contracts/`. Code *to* them, never to another package's internals.
2. **Stay in your lane.** One owner per package (§16). Never edit another package's files to make yours work.
3. **Depend on interfaces, not implementations.** Cross-package access only via `internal/contracts/interfaces.go`.
4. **No shared mutable global state.** World state has exactly one owner (`internal/state`) and one mutator path.
5. **Determinism is law.** No wall-clock reads, no unseeded `rand`, no map-order-dependent logic. Everything must replay identically.
6. **Mock your dependencies.** Build and test your package in isolation against contract interfaces.
7. **Tests live with the code you own.** Ship unit tests; satisfy the shared contract tests. Don't edit another package's tests.

## Changing a contract — the ONLY cross-lane action (§0.5)
You may **not** unilaterally edit `internal/contracts/`. Propose the change, get agreement, land it as its
**own isolated commit** touching only `contracts/`, then each affected owner updates their package.
Contract changes are **additive by default**.

## Hard "do nots"
- ❌ Don't mutate world state outside `internal/state`.
- ❌ Don't invoke Cells anywhere but `internal/orchestrator`, and never **sequentially** — fan-out is concurrent (the whole point; see SPEC §1).
- ❌ Don't put operational logic in `internal/api` or the `web/` frontend (they only serialize/visualize).
- ❌ Don't run the scenario generator on the live request path (it's an offline tool).
- ❌ Don't reformat or "drive-by clean" files you don't own. Found a bug outside your lane? **Report it.**

## Commit hygiene (§0.7)
- One package (or one contract change) per commit. Never mix a contract change with an implementation.
- Scoped messages: `feat(state): …`, `fix(orchestrator): …`, `contract(agentio): …`.

## Cross-platform (macOS + Windows → Linux deploy) — see SPEC §19
- Lowercase package dirs/filenames (Linux is case-sensitive; it'll build on Mac/Windows and fail in Docker).
- Paths via `filepath.Join`; bundle assets with `//go:embed`. No hardcoded `/` or `\`.
- No shell one-liners in build scripts (`rm -rf`, `&&`, `VAR=x cmd` break on Windows).
- Line endings are LF (enforced by `.gitattributes`). Green Linux CI = green everywhere.

## Definition of done
Your owned package builds in isolation, ships unit tests, **passes the full `internal/contracts/contracttest` suite**,
and touched no files outside your lane (except a coordinated §0.5 contract commit).

---
*If anything here conflicts with `SPEC.md`, the SPEC wins — and flag the conflict.*
