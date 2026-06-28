# CLAUDE.md

This project is built by **multiple AI coding agents (Builders) in parallel**.

👉 **Read [`AGENTS.md`](AGENTS.md) first** — it is the on-entry rule set for every Builder.
The binding protocol is [`SPEC.md` §0](SPEC.md); the full design is the rest of `SPEC.md`.

Quick reminders (full text in `AGENTS.md` / `SPEC.md` §0):
- You are a **Builder** (AI writing code). A **Cell** is a runtime AI agent inside the product. Don't conflate them.
- **Stay in your lane** — one owner per package (`SPEC.md` §16). Don't edit packages you don't own.
- **Contract-first** — cross-package shapes live in `internal/contracts/`; change them only via the §0.5 coordinated step.
- **Determinism is law**; **no shared mutable global state**; world state is owned solely by `internal/state`.
- Before coding: confirm your task touches only your package(s). If not, **stop and flag**.

If this file and `SPEC.md` ever disagree, `SPEC.md` wins.
