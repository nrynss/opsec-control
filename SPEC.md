# AI Emergency Operations Center (EOC) — Spec v0.2

> Status: **draft, iterating**
> Target: Cerebras Hackathon celebrating the **Gemma 4 31B** launch
> One-line: An API-first, event-driven, multi-agent command platform where specialist AI cells fire **simultaneously** the instant an anomaly is detected — made possible by Cerebras wafer-scale inference (~1,500 tok/s, native 16-bit).
>
> ⚠️ **Built by multiple AI coding agents (Builders) in parallel. Every Builder must read §0 (Multi-Agent Development Protocol) before writing any code. Separation of concerns is enforced, not optional.**

---

## 0. Multi-Agent Development Protocol — READ FIRST (binding)

This codebase is written by **multiple AI coding agents in parallel** (Claude, Codex, Grok, Pi, …). Without strict separation of concerns they will overwrite each other's work, redefine shared types, and produce a pile of merge conflicts. These rules are **binding on every coding agent**. If a task would require breaking one, stop and flag it instead.

### 0.1 Terminology (avoid the word collision)
Two different "agents" exist in this project. Never conflate them:
- **Builder** = an AI *coding agent* writing this repo (Claude/Codex/Grok/Pi). "You."
- **Cell** = a runtime *AI operational agent* inside the product (Intelligence, Infrastructure, Medical, Population, Communications, Commander).

This spec says **Builder** for the code authors and **Cell** for the runtime agents, everywhere.

### 0.2 The Prime Directives
1. **Contract-first, always.** Shared types, interfaces, event schemas, and agent I/O schemas are defined in **canonical contract files** (§0.4) *before* any implementation. Builders code **to** contracts, never to another package's internals.
2. **Stay in your lane.** Each package has **exactly one owner** (§16 ownership table). A Builder edits only the package(s) assigned to its task. **Never** edit another package's files to make your code work — fix it through the contract, or raise a contract-change request (§0.5).
3. **Depend on interfaces, not implementations.** Cross-package access goes through the interfaces declared in `internal/contracts/`. No reaching into another package's concrete structs, unexported helpers, or globals.
4. **No shared mutable global state.** The world state has exactly one owner (`internal/state`) and one mutator path (§8, §14.2). No package keeps its own copy or its own package-level vars for live state.
5. **Determinism is law.** No wall-clock reads, no `rand` without an injected seed, no map-iteration-order dependence in logic that affects state or output. Everything must replay identically (Principle 7).
6. **Mock your dependencies.** A Builder must be able to build and test its package in isolation against the contract interfaces (fakes/stubs), without any other Builder's implementation existing yet.
7. **Tests live with the code you own.** Every owned package ships unit tests + satisfies the shared **contract tests** (§0.6). You do not edit another package's tests.

### 0.3 What a Builder must do before writing any code
1. Read **this §0**, the section(s) governing your package, and the **§16 ownership table**.
2. Read the relevant files in `internal/contracts/` — these are your only source of truth for cross-package shapes.
3. Confirm your task touches **only** files you own. If not, stop and flag.
4. Write to the contract. If the contract is missing something you need, do **not** invent it locally — see §0.5.

### 0.4 Canonical contract files (the single source of truth)
These define every cross-boundary shape. They are **append-/change-by-coordination-only** (§0.5). Implementations depend on them; they depend on nothing.

```
internal/contracts/
    events.go        # Event struct, the event-type enum, payload shapes (§7)
    state.go         # World State types, entity types + status enums (§8.2–8.4)
    agentio.go       # Cell input envelope + per-Cell output schemas (§9)
    interfaces.go    # EventBus, StateStore, Cell, Orchestrator, LLMClient, Perception ifaces
    scenario.go      # Scenario file format the compiler emits / sim replays (§14, §scenario)
    errors.go        # shared error/rejection types (event_rejected reasons, §14.2)
```

JSON schemas mirrored under `contracts/schemas/*.json` for the frontend + scenario validation.

### 0.5 Changing a contract (the only coordinated step)
A contract change is the **one** action that crosses lanes, so it is gated:
- A Builder may **not** unilaterally edit a `contracts/` file to suit its package.
- Propose the change (what field, why, who's affected), get it agreed, land the contract change as its **own isolated commit** touching only `contracts/`, then each affected owner updates their package.
- Contract changes are **additive by default**. Breaking changes require updating every consumer in the same coordinated pass.

### 0.6 Contract tests (the seam that proves lanes line up)
Shared, contract-level tests live in `internal/contracts/contracttest/`. They assert that any implementation satisfies the interface and round-trips the canonical schemas. Every Builder runs the full suite before declaring done; a Builder may add cases but the suite is collectively owned and changed only via §0.5.

### 0.7 Commit / file hygiene for parallel Builders
- One package (or one contract change) **per commit**; never mix a contract change with an implementation in the same commit.
- Conventional, scoped commit messages: `feat(state): …`, `fix(orchestrator): …`, `contract(agentio): …`.
- Never reformat or "drive-by clean" files you don't own.
- If you find a bug outside your lane, **report it** (note in the spec/issue), don't silently fix it.

> **Every section below that defines a boundary carries a `🔒 Agent boundary` callout** naming the owner, the contract it depends on, and what it must not touch.

---

## 1. The Thesis (why this is a *Cerebras* project, not a generic AI project)

The defining constraint of multi-agent systems is the **latency cascade**: agents run one after another, each waiting on the previous one's tokens, and the UX stalls. On GPU-bound inference you hide this with spinners and batching.

Cerebras removes the constraint. At ~1,500 tok/s with native 16-bit weights, a single specialist cell turns an incoming event + sector state into a ~300-token structured JSON recommendation in **~200 ms**.

That changes the architecture, not just the speed:

> **When an anomaly is detected, every relevant cell fires at once across its own dataset — not in sequence.** Go fans the world-state out to all cells as concurrent goroutines; Cerebras returns all their analyses in parallel; the Commander synthesizes. The entire specialist phase completes in **under ~500 ms**.

This is the core demo claim and the thing judges should *feel*: **the command center re-reasons faster than the disaster evolves.** It behaves like a real-time strategy (RTS) command engine, not a batch tool.

### Design consequences of the thesis
- **Parallel-by-default fan-out.** Sequential agent chaining is an anti-pattern here. Default execution = simultaneous fan-out across cells; chaining is allowed only *within* a cell (plan → self-critique → refine), where Cerebras speed makes multi-turn loops still sub-second.
- **Anomaly-triggered, not tick-triggered.** Cells react to events as they land, not to a slow simulation clock. The clock drives the *scenario*; anomalies drive *inference*.
- **Speed is a first-class UI element.** Token throughput, agents-fired-per-event, and end-to-end latency are shown on the dashboard, not hidden.

---

## 2. Vision

Model a real Emergency Operations Center where multiple autonomous AI agents continuously analyze information from different operational domains and collaborate to maintain a shared **Common Operational Picture (COP)**.

- The **backend owns operational state.**
- The **frontend visualizes that state.**
- The **AI provides perception and reasoning.**

Not a chatbot. A miniature AI-powered emergency command-and-control platform.

---

## 3. Technology Stack

| Layer | Technology |
|---|---|
| Backend / Orchestration | Go |
| Frontend | Astro + Svelte |
| Reasoning Inference | **Gemma 4 31B on Cerebras** (~1,500 tok/s, native 16-bit) |
| Vision / Sensor Interpretation | Gemma 4 multimodal (Cerebras) for image → structured events |
| Communication | REST, WebSockets, Server-Sent Events |
| Data Storage | In-memory World State + immutable Event Log |
| Scenario Data | JSON |
| Deployment | Containerized Go services |

> Note: keep the headline compute on Cerebras. Vision-to-event ideally runs on Gemma 4 multimodal on Cerebras so the whole reasoning + perception story is "powered by Cerebras." Any external vision API is a fallback only.

---

## 4. Design Principles

1. API-first
2. Event-driven
3. Single Source of Truth (one authoritative world state)
4. Autonomous AI agents
5. Streaming updates
6. **Stateless AI, stateful backend**
7. Reproducible / deterministic simulations
8. **Parallel-on-anomaly** (the Cerebras principle)

Backend = orchestration. AI = reasoning. Backend does no ML.

---

## 5. High-Level Architecture

```
                 Astro + Svelte Dashboard
                           │
               REST / WebSocket / SSE
                           │
────────────────────────────────────────────────────
                     Go API Layer
────────────────────────────────────────────────────
                  Command & Orchestration
────────────────────────────────────────────────────
                       Event Bus
────────────────────────────────────────────────────
                  Shared World State
────────────────────────────────────────────────────
        Simulation Engine • Timeline • Scenario Loader
────────────────────────────────────────────────────
        Gemma 4 multimodal (Perception) → Structured Events
────────────────────────────────────────────────────
   Gemma 4 31B on Cerebras — PARALLEL Agent Inference (fan-out)
```

---

## 6. The Anomaly → Parallel Fan-Out Loop (the heart of the system)

```
Sensor / image / report
        ↓
Perception (Gemma 4 multimodal)  → structured event
        ↓
Event Bus → State Manager        → world version N → N+1
        ↓
ANOMALY DETECTED
        ↓
┌──────── Go fans state out as concurrent goroutines ────────┐
│   Intelligence   Infrastructure   Medical   Population ...  │   ← all fire AT ONCE on Cerebras
└──────── all structured outputs return in parallel ─────────┘
        ↓
Commander synthesizes → COP + prioritized actions
        ↓
Dashboard ripple update (map, panels, timeline)   [target: < 800 ms end-to-end]
```

### Anomaly detection (what triggers the fan-out)
The State Manager classifies each accepted event. Fan-out triggers when an event crosses a threshold or changes a tracked entity's status, e.g.:
- bridge/road status change, flood extent delta beyond threshold
- hospital/shelter capacity crossing a band (e.g. >85%)
- new aftershock / power failure / mass distress signal
- confidence-weighted clustering of citizen reports

(Exact rules live in `internal/anomaly/` — see open questions.)

> 🔒 **Agent boundary** — Owners: `internal/anomaly` (classification → which Cells to wake) + `internal/orchestrator` (the goroutine fan-out, gathering parallel Cell outputs, invoking the Commander). Depend on: `contracts/interfaces.go` (`Cell`, `StateStore`, `LLMClient`), `contracts/agentio.go`. Must NOT: define event/state/agent-output types (they're imported from `contracts/`); mutate world state (orchestrator reads a snapshot, Cells return data, only `internal/state` writes). The orchestrator is the **only** place allowed to invoke Cells, and it does so concurrently — sequential invocation is a spec violation (§1).

---

## 7. Event-Driven Architecture

Everything is an immutable event = a fact about the evolving scenario.

```json
{
  "id": "evt-10042",
  "timestamp": "...",
  "source": "Gemma4-Perception",
  "type": "bridge_closed",
  "confidence": 0.96,
  "payload": { "bridge_id": "BR-12", "reason": "Structural failure" }
}
```

Event taxonomy (Cerebro earthquake cascade), tagged by the cell(s) it wakes:

- **Seismic (→ Intelligence + all):** `MainshockOccurred`, `AftershockOccurred`, `AftershockForecastUpdated`
- **Structural (→ Infrastructure):** `BuildingCollapsed`, `BridgeDamaged`, `BridgeClosed`, `RoadBlocked`, `TunnelClosed`, `DamStressElevated`, `LeveeBreached`
- **Utility (→ Intelligence/Infrastructure):** `PowerFailure`, `GasLeakDetected`, `WaterMainBreak`, `CommsOutage`
- **Fire (→ Infrastructure + Population):** `FireIgnited`, `FireSpread`, `FireContained`
- **Flood (→ Intelligence + Population):** `FloodExtentUpdated`
- **Medical (→ Medical):** `CasualtyReportUpdated`, `MassCasualtyIncident`, `HospitalCapacityChanged`
- **Population (→ Population):** `CitizenDistressCall`, `PersonsTrapped`, `EvacuationOrdered`, `ShelterOccupancyChanged`, `ShelterFull`
- **Perception (→ generates the above):** `SatelliteImageReceived`, `DroneImageReceived`
- **Resource (→ all, via Commander):** `ResourceDeployed`, `ResourceDepleted`

**Event Bus** distributes; it never owns state. Flow: Sensors → Event Bus → State Manager → Cells → Commander → Dashboard.

> 🔒 **Agent boundary** — Owner: `internal/events`. Depends on: `contracts/events.go`, `contracts/interfaces.go` (`EventBus`). Must NOT: mutate world state, call Cells/LLM, or add new event types without a §0.5 contract change. The `Event` struct and event-type enum live in `contracts/`, not here.

---

## 8. Shared World State + Versioning

Single authoritative in-memory state: weather, seismic, flood extent, fire, roads, bridges, dam, levee, hospitals, shelters, emergency calls, utilities, casualties, population, resources, active incidents, timeline.

- **Only the State Manager mutates state.**
- Every accepted event increments the world version (`v41 → bridge_closed → v42`).
- Every agent records the `stateVersion` it analyzed:

```json
{ "agent": "Infrastructure", "stateVersion": 42 }
```

Guarantees deterministic execution and clean debugging/replay.

The concrete world being simulated is the city of **Cerebro** (§8.1–§8.4); the disaster that drives it is the three-act earthquake cascade (§8.5).

> 🔒 **Agent boundary** — Owner: `internal/state` (sole world-state owner & mutator) + `internal/validation` (the §14.2 contract). Depends on: `contracts/state.go`, `contracts/events.go`, `contracts/errors.go`. Must NOT: be bypassed — no other package may hold or mutate live world state. All entity types and status enums live in `contracts/state.go`; changing them is a §0.5 action.

### 8.1 The Scenario — Earthquake cascade in Cerebro

**Why an earthquake (not a flood):** the thesis is *one anomaly → every relevant cell fires at once*. A flood is gradual and only one or two cells care at any instant — it hides the Cerebras advantage. An earthquake does the opposite: a single shock simultaneously means new collapses (Infrastructure), a casualty surge (Medical), trapped people and shelter shifts (Population), gas/power failures (Intelligence/utilities), and a forecast of the *next* shock (Intelligence). One event, six cells lit at once — a literal screenshot of the thesis. Aftershocks give a **repeatable fan-out beat** for pacing.

**Cerebro** — a coastal river-delta city of ~1.2M on the **Cerebro Fault**. The **Vora River** bisects it, fed by the upstream **Mainor Dam / reservoir**. The low-lying delta district sits behind a **levee**. Geography is purpose-built to cascade: one quake naturally produces collapses → bridge failures → gas-main fires → dam stress → levee breach → flooding, so the simulation keeps generating fresh multi-domain anomalies without scripting unrelated disasters.

### 8.2 Sectors (8) — the map board

River runs roughly N→S; **North/East bank**: Highgate, Central, Ironworks, Harborside. **South/West bank** (isolated when bridges fall): Westbank, Southport, Greenfield. **Upstream**: Mainor Heights.

| Sector | Character | Elevation | Seismic vuln | Demo role |
|---|---|---|---|---|
| **Highgate** | Old masonry, hillside | High | **Very high** | Building collapses (Act 1) |
| **Central** | Dense downtown core; EOC + Central General Hospital | Mid | High | Casualty surge; command hub |
| **Ironworks** | Riverfront industrial, gas mains | Low | Med | Gas-main fire (Act 2) |
| **Harborside** | Port, logistics/resource base | Low | Med | Resource staging |
| **Westbank** | Residential, across river; Westbank Clinic | Mid | Med | **Cut off** when bridges fall → medical logistics crisis |
| **Southport** | Low delta behind the levee | **Very low** | Med | **Flooding** (Act 3) |
| **Greenfield** | University + open space | Mid | Low | Designated shelters / evac centers |
| **Mainor Heights** | Upstream, at the dam/reservoir | High | Med | Dam stress → release/breach |

### 8.3 Static substrate (fixed before t=0)

- **Bridges (3)** crossing the Vora — `Vora Bridge` (main arterial), `Iron Bridge`, `South Span`. When ≥2 fail, Westbank + Southport are isolated.
- **Dam** (Mainor) — reservoir level, stress rating. **Levee** (Southport) — height, integrity.
- **Roads/arterials** between sectors; **power** substations → sector mapping; **gas & water mains** (dense in Ironworks); **comms/cell** coverage.
- **Hospitals** — `Central General` (Central, main, has generator) and `Westbank Clinic` (small, across river). Beds, ICU, ER capacity per hospital.
- **Shelters** — primarily in Greenfield + schools; id, location, capacity.
- **Resources** — ambulances, fire engines, USAR teams, helicopters (river-crossing matters once bridges drop), evac buses, supply caches — counts + home base (mostly Harborside/Central).

### 8.4 Entities & legal state transitions (enforced by §14.2)

Degradation is largely one-way within an episode (no magic repairs mid-demo); restoration is a stretch goal.

| Entity | States | Legal transitions |
|---|---|---|
| **Bridge** | open → restricted → closed → collapsed | forward only |
| **Road** | open ↔ congested ↔ blocked | bidirectional |
| **Dam** | normal → stressed → releasing → breached | forward only |
| **Levee** | intact → overtopping → breached | forward only |
| **Power (sector)** | on → partial → off | forward (→ on = stretch: restoration) |
| **Comms/Water/Gas (sector)** | up → degraded → down | forward (gas: down can mean shutoff) |
| **Hospital** | tracked by occupancy band `normal <70% / strained 70–90% / critical 90–100% / over-capacity` + `on_generator` flag | band follows load |
| **Shelter** | occupancy `0…capacity`; `full` when ≥capacity | numeric |
| **Fire zone** | ignited → spreading → contained → out | forward + contain/out |
| **Flood** | extent polygons + depth; monotonic ↑ within episode unless explicit recession | numeric |

Note (BUG-4/5 B1): BuildingCollapsed and TunnelClosed are accepted trigger-only events for anomaly classification (Intelligence/Infrastructure wake) but track no entity in the MVD WorldState model (no Building/Tunnel per §8.4 table; collapses are narrative in §8.5 only). Road/Bridge/Power have full state entities and handlers.

### 8.5 The three-act cascade

One disaster, five hazard types, all six cells busy throughout:

1. **Act 1 — Mainshock (M6.8).** Collapses concentrated in Highgate; `Vora Bridge` + `Iron Bridge` damaged→closed; power off across Highgate/Central; casualty surge at Central General. Full fan-out; Commander issues first COP.
2. **Act 2 — Aftershock (M5.9) + ignition.** `South Span` closes → Westbank/Southport **isolated**; gas main ruptures in Ironworks → **fire ignites and spreads**; Westbank Clinic overwhelmed but cut off from Central General → medical-logistics problem; **dam stress rises** at Mainor. Second fan-out — the "re-reason instantly" beat.
3. **Act 3 — Levee breach → flood.** Mainor releases; **levee breaches** in Southport → flooding spreads (the map-animation payoff). Evacuation routing must avoid downed bridges *and* the flood; shelters in Greenfield fill. Commander re-prioritizes; final COP.

---

## 9. AI Agent Architecture

Six autonomous specialist cells (scope down to 2–3 for the MVD, see §13):

- **Intelligence Cell** — weather, flood modelling, satellite/drone interpretation, damage assessment, hazard prediction. Inputs: weather, seismic, satellite, drone imagery.
- **Infrastructure Cell** — roads, bridges, utilities, power grid, logistics.
- **Medical Cell** — hospital capacity, ambulances, casualties, medical logistics.
- **Population Cell** — shelter occupancy, evacuations, population movement, citizen safety.
- **Communications Cell** — consumes all specialist outputs → public advisories, internal briefings, situation summaries.
- **Commander** — consumes entire world state + all specialist outputs → COP, prioritized actions, executive recommendations, mission objectives.

### Structured outputs (never free-form prose)

```json
{
  "summary": "Hospital nearing capacity.",
  "riskLevel": "High",
  "confidence": 0.94,
  "stateVersion": 42,
  "recommendations": ["Deploy field hospital", "Redirect ambulances"],
  "evidence": ["Hospital feed", "Emergency calls"]
}
```

Within a cell, Cerebras speed allows a sub-second **plan → self-critique → refine** loop before emitting the final JSON.

> 🔒 **Agent boundary** — Owner: `internal/agents` (the Cells). Depends on: `contracts/agentio.go` (input envelope + per-Cell output schema), `contracts/interfaces.go` (`Cell`, `LLMClient`), `contracts/state.go` (read-only view). Must NOT: mutate world state, call the EventBus directly, talk to other Cells, or read wall-clock/rand. A Cell is a **pure function of (state snapshot + triggering event) → structured output**; it receives a read-only snapshot and the `LLMClient`, nothing else. Each Cell is independently ownable by a different Builder *because* they share no state and only the schema in `agentio.go`. The Commander consumes other Cells' outputs **as data passed in by the orchestrator**, never by calling them.

---

## 10. Streaming Architecture

Sources: Gemma 4 perception, simulated sensors, weather feeds, drone/satellite imagery, human operators. Every update enters via REST/WS/SSE and becomes an event. Agent outputs stream to the dashboard token-by-token so judges watch JSON pour in live.

---

## 11. Simulation Engine

Deterministic simulation clock (09:00 → 09:05 → …) supporting Replay, Pause, Fast-Forward, Reset, Scenario branching. Pre-recorded event streams make the live demo bulletproof. The clock advances the *scenario*; anomalies (not ticks) drive *inference*.

> 🔒 **Agent boundary** — Owner: `internal/simulation` (+ `internal/scenario` for loading). Depends on: `contracts/scenario.go`, `contracts/events.go`, `contracts/interfaces.go` (`EventBus`). Must NOT: write world state directly (it emits events onto the bus like any sensor); read wall-clock for logic (uses the injected sim clock — determinism). Replays validated scenario files only.

---

## 12. API Surface (API-first; dashboard is just another client)

```
GET    /state
GET    /agents
GET    /timeline
GET    /events

POST   /events
POST   /scenario/load
POST   /scenario/reset

WS     /stream
```

> 🔒 **Agent boundary** — Owner: `internal/api` + `internal/websocket`. Depends on: `contracts/*` (DTOs) + the `StateStore`/`EventBus`/`Orchestrator` interfaces. Must NOT: contain operational logic, transform/derive state, or own any state — it serializes contract types over HTTP/WS and forwards events to the bus. This is the *only* package the frontend talks to; the frontend talks to nothing else.

---

## 13. Scoping — Minimum Viable Demo (MVD)

Hackathon = short. Ship the spine that proves the thesis; defer the rest.

**Build first (the spine):**
1. Scenario JSON + simulation engine that emits events on a timeline.
2. Event bus → versioned in-memory world state.
3. Anomaly detector that triggers fan-out.
4. **Parallel fan-out of 2–3 cells** (Infrastructure, Medical/Population) on Cerebras — the money shot.
5. Commander synthesis → COP.
6. Svelte dashboard: map + live agent panels + timeline + **speed/throughput HUD**.

**Nice-to-have (time permitting):** all 6 cells, real multimodal perception on live images, scenario branching, Communications cell.

**Defer:** auth/multi-session, production observability, complex hazard physics, triple REST+WS+SSE parity (pick one transport for v1).

---

## 14. Synthetic Data Generation — Scenario Compiler & Event Validation

In production, the event stream is **real telemetry**: sensors, 911/citizen feeds, weather APIs, drone/satellite imagery. For the hackathon we **simulate that stream**. Gemma stands in for the world's data source — it does *not* replace the perception or reasoning layers. Two clearly separated roles:

- **Generator = "the world"** (ground-truth reality + sensors). Produces the raw event stream.
- **Agents = "the responders"** (reason over derived operational state). Consume it.

This is a flight-sim feeding synthetic ADS-B, not a model talking to itself.

### 14.1 Pre-generation, not live generation
The generator runs **offline as a scenario compiler**, ahead of the demo — never on the live Cerebras request path.

Why offline:
- **Determinism preserved.** Output is a fixed, replayable scenario JSON file. Same run every time → rehearsable, reproducible (honors Principle 7).
- **Full request budget for agents.** The generator must not compete with the parallel fan-out for the concurrent-request ceiling — the exact resource the money-shot depends on. Generate first, replay at demo time.
- **Reviewable.** A human can read/edit the compiled scenario before trusting it on stage.

```
Gemma (offline scenario compiler)
        ↓
raw candidate events
        ↓
Event Validator  → reject/repair invariant violations
        ↓
scenario JSON (deterministic, versioned, replayable)
        ↓   [demo time]
Simulation Engine replays → Event Bus → State Manager → fan-out
```

> 🔒 **Agent boundary** — Owner: `internal/scenariogen` + `cmd/scenariogen`. Depends on: `contracts/scenario.go`, `contracts/events.go`, `internal/validation` (reuses the §14.2 contract), `contracts/interfaces.go` (`LLMClient`). Must NOT: run on the live request path, touch runtime world state, or emit unvalidated output. It is an **offline tool**; its only product is a validated scenario file.

### 14.2 Event Validation Contract (enforced by the State Manager)
The State Manager is the only mutator and the gatekeeper. Every event — generated or live — must pass before it touches world state. Reject (or quarantine) on any violation:

- **Schema:** required fields present, correct types; `confidence ∈ [0,1]`; `type` in the known event enum.
- **Referential integrity:** referenced entity exists (`bridge_id`, `hospital_id`, `sector_id` known to the world).
- **Temporal monotonicity:** `timestamp` ≥ last applied event's timestamp; no time travel.
- **State-transition legality:** transition is valid for the entity's current status (e.g. can't `bridge_closed` an already-closed bridge; can't reopen what the scenario never closed).
- **Range / physical sanity:** capacities clamped to `[0, max]`; flood extent monotonic within an episode unless an explicit recession event; population conserved.
- **Idempotency:** duplicate `id` is dropped.

Rejections are logged to the event log as `event_rejected` (with reason) so the pipeline is debuggable and the validator's own coverage is visible. This makes the offline generator's mistakes **harmless** — bad events never corrupt state.

### 14.3 Runtime ambient noise (optional, non-LLM)
For visual liveness, a constrained **templated** generator (NOT an LLM, no Cerebras cost) can emit low-stakes ambient chatter (routine citizen reports, minor weather ticks). Keeps the feed busy and the dashboard alive without touching the request budget or the deterministic spine.

### 14.4 Live perception moment (optional climax)
If you want live inference visible on stage, do it as the **perception layer**, not event fabrication: feed a mock satellite/drone image to Gemma 4 multimodal live → watch it emit a structured event in real time. More honest and more impressive than generating events, and it has a recorded fallback behind a keypress.

---

## 15. Demo Choreography — the 1-minute video

**Deliverable: a single ~60-second video**, not a live stage demo. This de-risks everything: we run the deterministic Cerebro scenario in real time, **screen-record it, and re-take if inference hiccups**. The only hard requirement is that the parallel fan-out *looks* instant on screen. No live-failure risk.

### 15.1 Always-on-screen elements (so the speed story needs no narration)
- **HUD strip:** live token throughput (~1,500 tok/s), **# cells firing in parallel**, end-to-end fan-out latency (ms), tokens this tick, world `stateVersion`.
- **The "Matrix" feed:** terminal sidebar streaming the raw JSON payloads from Cerebras as they generate.
- **Cerebro map:** 8 sectors, 3 bridges, river, dam, levee — the stage for every ripple.
- **Six cell panels** that flash `Analyzing…` **simultaneously** on each fan-out, then fill with structured output.

### 15.2 Beat sheet (compressed — all three acts in ~60s)

| Time | Beat | What the viewer sees |
|---|---|---|
| **0–6s** | Cold open: Cerebro calm, HUD idle | Map of Cerebro, sectors labeled, "SYSTEM NOMINAL". One line of title. |
| **6–18s** | **Act 1 — Mainshock M6.8** | Map jolts; Highgate flashes red (collapses); Vora + Iron Bridge → closed; power off across Highgate/Central. **All 6 cell panels fire at once**; HUD spikes to "6 cells • ~1,500 tok/s • 480 ms". Commander emits first COP. |
| **18–34s** | **Act 2 — Aftershock M5.9 + fire** | South Span closes → Westbank/Southport greyed out (isolated); Ironworks ignites (spreading fire overlay); Central General hits `critical`; dam stress bar rises. **Second full fan-out** — the "it re-reasoned instantly" beat. |
| **34–52s** | **Act 3 — Levee breach → flood** | Mainor releases; levee breaches; **flood polygon animates across Southport**; evac routes re-draw around downed bridges *and* flood; Greenfield shelters fill. **Third fan-out**; Commander re-prioritizes. |
| **52–60s** | Resolution + the number | Final COP + prioritized actions on screen; closing card: *"6 agents · 3 cascading hazards · re-reasoned in under 1 second each — on Cerebras Gemma 4 31B."* |

### 15.3 The point the video must make
Three times in 60 seconds the whole command center **re-reasons across six domains simultaneously**, each in under a second. The repeated, visible fan-out *is* the argument. Optional A/B insert: a 2-second split showing the same beat sequential-vs-parallel to dramatize what Cerebras buys.

---

## 16. Backend Layout & Ownership

```
cmd/
    eoc/            # main server (wires interfaces; owns no logic)
    scenariogen/    # offline scenario compiler (Gemma → validated JSON)

internal/
    contracts/      # ★ CANONICAL TYPES & INTERFACES — change only via §0.5
        contracttest/   # shared contract tests (§0.6)
    api/
    agents/         # the Cells (each Cell independently ownable)
    orchestrator/   # parallel fan-out + Commander invocation
    anomaly/        # anomaly classification + fan-out triggers
    events/
    state/          # sole world-state owner & mutator (§8, §14.2)
    validation/     # event validation rules, shared by state + scenariogen
    simulation/
    scenario/       # scenario loading
    scenariogen/    # generator logic (offline data path)
    timeline/
    llm/            # Cerebras client (Gemma 4 31B)
    sensors/
    websocket/

web/                # Astro + Svelte dashboard (talks ONLY to internal/api)

pkg/
```

Single responsibility per package, minimal coupling. **One owner per package** (§0.2).

### 16.1 Ownership table (assign one Builder per row)

| Package | Owns / responsible for | Depends on (contracts) | Must NOT |
|---|---|---|---|
| `contracts/` | All cross-boundary types & interfaces | nothing | depend on any impl; change without §0.5 |
| `state` + `validation` | World state, versioning, §14.2 gatekeeping | events, state, errors | let any other pkg mutate state |
| `events` | Event bus / pub-sub | events, interfaces | own state; call Cells/LLM |
| `anomaly` | Decide which Cells wake per event | state, events, agentio | mutate state; invoke Cells |
| `orchestrator` | Concurrent fan-out, gather, Commander | interfaces, agentio | mutate state; invoke Cells sequentially |
| `agents` (per-Cell) | One Cell's reasoning + prompt | agentio, interfaces, state(RO) | mutate state; call bus/other Cells |
| `llm` | Cerebras client, throughput metrics | interfaces (`LLMClient`) | own domain logic; leak provider types past iface |
| `simulation` + `scenario` | Sim clock, replay, scenario load | scenario, events, interfaces | write state directly; read wall-clock |
| `scenariogen` (+cmd) | Offline data compiler | scenario, events, interfaces, validation | run on live path; emit unvalidated data |
| `api` + `websocket` | HTTP/WS edge, DTO serialization | all contracts + ifaces | hold state; contain operational logic |
| `web/` (frontend) | Astro+Svelte visualization | `contracts/schemas/*.json` via the API | contain operational logic; call anything but `api` |
| `timeline` | Immutable event log / replay index | events | mutate events |
| `sensors` | Sensor/ingest adapters | events, interfaces | bypass validation |

> Two Builders may safely work in parallel **iff** their rows share no package and any shared shape already exists in `contracts/`. If a task spans two rows, split it or sequence it.

---

## 17. Why Go

Orchestration engine: massive concurrency (goroutines/channels) for the parallel fan-out, native streaming APIs (REST/WS/SSE), deterministic state management, simplicity, reliability. Behaves like a real-time simulation engine, not a CRUD web app. Goroutines + Cerebras parallel inference are the exact pairing that makes simultaneous multi-cell reasoning viable.

---

## 18. Open Questions (to iterate)

1. **Hackathon logistics:** exact date, duration, team size, judging criteria?
2. **Anomaly rules:** fixed thresholds, or a lightweight Gemma "triage" pass that decides which cells to wake? (A triage call is itself cheap on Cerebras.)
3. **Perception in the video:** include one real image→event multimodal beat (§14.4), or keep imagery as eye candy and rely on the compiled event stream? (Video means we can re-take, so live is lower-risk than on stage.)
4. **Transport for v1:** SSE (simplest for one-way streaming) vs WebSockets (bidirectional)?
5. **Cerebras client specifics:** confirmed model ID, endpoint, rate limits, max parallel in-flight requests during the event? (Thesis depends on true concurrent fan-out — measure first.)
6. **Map tech:** Leaflet/OSM (free, fast) vs Mapbox (prettier, key needed) vs a stylized hand-drawn Cerebro map (most control for a 60s video).
7. **Map render:** since Cerebro is fictional, do we need real geo at all, or a custom SVG/canvas board of the 8 sectors? (Likely the latter — full control of every ripple/animation for the video.)

---

## 19. Cross-Platform Development (macOS + Windows → Linux deploy)

Builders and humans work across **macOS and Windows**; deployment is **containerized Linux** (§3). Go and the Astro/Svelte stack are fully cross-platform, so this is a config problem, not an architecture problem. **Linux CI/Docker is the great equalizer — it is the authority on "does it build."** Cross-platform discipline is the same as determinism discipline (§0.2 rule 5): OS-dependent behavior breaks reproducible replay.

### 19.1 Set-once configuration (binding on the repo)
- **`.gitattributes`** — `* text=auto eol=lf`, and `*.sh text eol=lf`. Normalizes line endings so Windows (CRLF) and Mac (LF) Builders don't generate phantom diffs / merge noise. **This is the #1 cross-platform risk; fix it before the first commit.**
- **Pin toolchains** — `toolchain` directive in `go.mod`; `.nvmrc` + `engines` for Node. All machines/Builders use identical versions (reproducibility + determinism). Commit the lockfile.
- **No `make`** — Windows lacks it by default. Use a cross-platform task runner (`mage` (Go) or `Taskfile`/`just`) so both machines run identical commands.

### 19.2 Code rules (extend §0.2; enforced in review)
- **Lowercase package dirs & filenames.** macOS/Windows are case-insensitive; Linux is not — a case mismatch builds locally and **fails only in Docker**.
- **Paths via `filepath.Join`**, never hardcoded `/` or `\`. Bundle scenario/asset files with `//go:embed` (always forward-slash, OS-independent) rather than runtime filesystem reads.
- **No shell one-liners in `package.json`/build scripts** (`rm -rf`, `cp`, `VAR=x cmd`, `&&` break on Windows). Use `rimraf`/`cross-env` or Node/Go programs.
- **No wall-clock, no unseeded `rand`, no map/dir-iteration-order dependence** (already §0.2 rule 5) — these are also the OS-variance sources.

### 19.3 The safety net
CI builds and runs the full test suite (incl. `contracttest`, §0.6) **on Linux** for every change. If it's green there, it's green everywhere. Builders should run the Docker build before declaring done if they touched paths, embeds, or filenames.

---

## 20. Verdict / Why this wins

A stateful Go backend + wafer-scale Cerebras inference = a live, breathing RTS command engine for disasters. It directly demonstrates the *only-possible-with-Cerebras* capability the hackathon is celebrating: **many agents reasoning simultaneously across many datasets the instant an anomaly appears.** Modular, reproducible, API-first, and built to make a judge say *"wait — it did all that at once, instantly?"*
