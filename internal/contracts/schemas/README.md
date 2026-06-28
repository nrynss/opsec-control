# contracts/schemas

JSON Schemas mirroring the Go contract types (SPEC §0.4). They are the shared
shape for two consumers that don't import Go:

- the **frontend** (`web/`) — typed against these via the API.
- **scenario validation** — the §14.2 validator and the offline compiler.

One `.json` file per canonical shape (e.g. `event.schema.json`,
`scenario.schema.json`). Keep them in lockstep with the Go types in
`internal/contracts/` — both change together via the §0.5 coordinated step.
