# Cerebro EOC — AI Emergency Operations Center

An API-first, event-driven, multi-agent command platform where specialist AI **Cells** fire **simultaneously** the instant an anomaly is detected. Built on Cerebras wafer-scale inference (Gemma 4 31B) to process six reasoning loops in parallel, updating the command center faster than the disaster evolves.

## ▶ Live Demo

**[https://eoc.nryn.dev](https://eoc.nryn.dev)** — open in any modern browser. No install or sign-in required.

The site loads directly into the **Common Operational Picture (COP)** dashboard for *Cerebro*, a fictional city undergoing a cascading disaster (M6.8 mainshock, aftershocks, fire, and a levee breach).

### 📺 Watch the Demo

**[▶ Demo video](#)** <!-- TODO: replace # with the published video URL -->

The video runs the full `cerebro-cascade` scenario on both backends. On **Cerebras**, the six Cells reason and the Commander synthesizes a complete COP in **seconds**. On a **conventional GPU provider (OpenRouter)**, the *same model on the same prompts* takes **minutes** — so that run is cut, and we show the final screenshot to confirm the answer is equivalent. The difference you see is purely the inference backend.

### Navigate the Site in 3 Steps

1. **Initialize, then play** — On load, the **Initialize Command Center** overlay prompts you to pick a scenario (**Cerebro Earthquake Cascade**) and click **Initialize**. The dashboard arms at `09:00:00`, paused. Click **Play** on the **Playback Controller** (left sidebar) to begin — speed up to `10×`, **Step** through event-by-event, or **All Clear** to reset.
2. **Watch the parallel fan-out** — When an anomaly hits (e.g., the M6.8 mainshock at `+6s`), the five **Specialist Cells** (bottom — Intel, Infra, Medical, Population, Comms) all flash `Analyzing…` at once, the **HUD** (top) shows latency and token throughput spiking, and the **Commander Cell** (top center) synthesizes a unified COP with a prioritized task list. Track unfolding events in the **System Event Log** and hazards on the **Cerebro Map**.
3. **Inject your own events** — In the **Perception Ingest** box, upload a drone/satellite photo or click a preset (e.g., *Vora Bridge Collapse*). The system interprets it, logs it, recolors the map, and triggers the Cells to re-reason the response plan.

### See Cerebras vs. OpenRouter on the *Same* Model

The HUD (top bar) has an **LLM Provider** dropdown that switches inference between **Cerebras** and **OpenRouter** — both running the *identical* model, **Gemma 4 31B** (`gemma-4-31b` on Cerebras, `google/gemma-4-31b-it` on OpenRouter). This isolates the hardware variable: same prompts, same weights, only the inference backend changes.

To compare:

1. Run a scenario on **Cerebras** and watch the **Decision Latency** and **Total Decision Time** cards (in **Simulation Telemetry**, left sidebar) plus the HUD's tok/s as the Cells fan out.
2. Switch the dropdown to **OpenRouter**, hit **All Clear**, and re-run.
3. Compare the same cards — **Cerebras resolves the full fan-out in seconds; the conventional provider takes minutes** for the identical work. The wafer-scale advantage is a step-change in time-to-first-token and sustained throughput, with little to no change to the model's actual answers.

> Both telemetry cards remember the last run **per provider**, so you can hover either one to see the Cerebras-vs-OpenRouter numbers side by side with the speed ratio — the comparison survives the toggle.

> This seconds-vs-minutes gap is the whole point: in a real emergency, the COP has to update *faster than the disaster evolves*. The model's reasoning is the same on both backends — only Cerebras delivers it inside the decision window.

> Full panel-by-panel layout is in [How to Use the Interface](#how-to-use-the-interface) below. To run it locally instead, see [Prerequisites & Installation](#prerequisites--installation).

---

## AI for Human Survival

AI is increasingly criticized for generating misinformation, displacing jobs, or consuming massive energy budgets. **Cerebro EOC** demonstrates the opposite: how wafer-scale, concurrent AI reasoning can directly save human lives during catastrophic events when communication channels break down and seconds decide between life and death.

### The Scale of the Crisis

Disasters are scaling faster than our traditional human-in-the-loop response systems. Consider the hard global data:

| Metric                                        | Value              | Why it Matters                                                                |
| :-------------------------------------------- | :----------------- | :---------------------------------------------------------------------------- |
| **Weather/climate disasters (1970–2021)**     | 11,778 disasters   | The staggering global scale of the threat.                                    |
| **Deaths**                                    | ~2 million         | The direct human cost of inadequate warning and response.                     |
| **Economic losses**                           | US$4.3 trillion    | Direct financial toll that cripples post-disaster recovery.                   |
| **Population without adequate early warning** | ~1 in 3 people     | A massive opportunity for systemic improvement.                               |
| **Africa lacking warning coverage**           | 60%                | The largest underserved region requiring low-cost digital resilience.         |
| **Disaster mortality**                        | **6× higher**      | The difference in countries with poor vs. comprehensive warning coverage.     |
| **24-hour early warning**                     | **~30% reduction** | The damage reduction achieved by giving responders just a 24-hour head start. |
| **System Investment ROI**                     | **4–20× return**   | Investing US$800M in warning systems avoids US$3–16B/year in losses.          |
| **Infrastructure resilience benefit**         | US$4.2 trillion    | The net benefit of investing in resilient infrastructure (World Bank).        |
| **Mitigation ROI**                            | **1 : 6**          | Every US$1 invested in disaster mitigation saves US$6 in recovery costs.      |

### Case Studies: The Cost of Delays

Historically, the bottleneck of disaster response has not been a lack of courage, but a **lack of coordinated, timely information**.

*   **2004 Indian Ocean Tsunami (~230,000 deaths, 14 countries, >$10B losses):** At the time, there was no tsunami warning system in the Indian Ocean. This disaster directly led to the creation of one, proving that active warning infrastructure is a prerequisite for survival.
*   **2011 Japan Tōhoku Earthquake & Tsunami (~20,000 dead/missing, ~$235B losses):** The costliest natural disaster in history showed that even the most advanced structural engineering requires instant coordination to mitigate cascading coastal floods.
*   **2005 Hurricane Katrina (1,800+ deaths, $125+B damage):** Post-disaster audits highlighted massive communication and coordination failures between local, state, and federal agencies as the primary reason casualties spiked post-landfall.
*   **2023 Türkiye–Syria Earthquake (59,000+ deaths, millions displaced, >$100B damage):** Collapsed local networks isolated emergency services, leaving responders blind to where rescue resources were needed most.
*   **2025 Myanmar Earthquake (Thousands killed/injured):** Damaged physical communication lines and bridge failures blocked medical teams. This highlighted the urgent need for rapid situational awareness and automated, coordinated routing.

---

## Why AI Matters: Compressing the OODA Loop

AI does not stop earthquakes, divert floods, or mend broken levees. **It compresses the OODA Loop** (Observe → Orient → Decide → Act). By using wafer-scale inference to reason in parallel, Cerebro EOC collapses tasks that take humans hours or days down to sub-seconds:

| Human Task                             | AI Cell Contribution                                                       |
| :------------------------------------- | :------------------------------------------------------------------------- |
| **Reading 10,000 emergency calls**     | Clusters, dedupes, and prioritizes reports into incidents in seconds.      |
| **Scanning satellite & drone imagery** | Automatically flags collapsed bridges, blocked roads, and spreading fires. |
| **Hospital coordination**              | Monitors bed/ICU capacity and predicts overload before it happens.         |
| **Resource deployment**                | Solves vehicle routing (ambulances, boats, engines) around active hazards. |
| **Information fusion**                 | Correlates seismic data, flood maps, drone feeds, and P25 radio alerts.    |
| **Command decisions**                  | Synthesizes a unified Common Operational Picture (COP) continuously.       |

---

<a id="how-to-use-the-interface"></a>

## How to Use the Interface

The dashboard provides a **Common Operational Picture (COP)** representing the state of Cerebro, a fictional city experiencing a cascading disaster (M6.8 mainshock, aftershocks, fire, and levee breach). The panel layout:

```
+--------------------------------------------------------------------------------+
|  [1] HUD: Latency, Throughput (tok/s), Active Cells, State Version, Provider    |
+------------------------------------+-------------------------------------------+
|                                    |  [4] COMMANDER CELL (COP)                 |
|  [2] PLAYBACK CONTROLLER           |      - Real-time overall risk assessment  |
|      - Play / Step / All Clear     |      - Synthesized priority task list    |
|      - Speed slider (1x - 10x)     +-------------------------------------------+
|      - Total Decision Time         |  [5] CEREBRO MAP                          |
|      - Decision Latency stats      |      - Interactive city sectors           |
|                                    |      - Real-time bridge & road closures   |
|  [3] SYSTEM EVENT LOG              |      - Spreading fire & flood overlays     |
|      - Time-stamped event list     +-------------------------------------------+
|      - Ingest confidence ratings   |  [6] SPECIALIST CELLS                     |
|                                    |      - Intel, Infra, Medical, Pop, Comms |
|  [3.5] PERCEPTION INGEST           |      - Live status & local recommendations|
+------------------------------------+-------------------------------------------+
|  [7] THE "MATRIX" FEED (Live streaming JSON responses & system logs)            |
+--------------------------------------------------------------------------------+
```

Each numbered panel above:

* **[1] HUD** — live fan-out latency, token throughput (tok/s), active Cell count, state version, inference provider, and connection status (`DISCONNECTED` / `COMPLETE` badges).
* **[2] Playback Controller** — Play / Step / All Clear (reset), the 1×–10× speed slider, and the **Simulation Telemetry** grid (Total Decision Time, Decision Latency, events, tokens, inferences). The scenario is chosen once at startup via the **Initialize Command Center** overlay.
* **[3] System Event Log** — time-stamped events with ingest confidence ratings.
* **[3.5] Perception Ingest** — upload imagery or fire a preset to inject a multimodal event.
* **[4] Commander Cell (COP)** — real-time overall risk assessment and synthesized priority task list.
* **[5] Cerebro Map** — interactive city sectors with live bridge/road closures and fire/flood overlays.
* **[6] Specialist Cells** — the five Cells (Intel, Infra, Medical, Population, Comms) with live status and local recommendations.
* **[7] The "Matrix" Feed** — live streaming JSON responses and system logs.

For the step-by-step walkthrough, see [Navigate the Site in 3 Steps](#navigate-the-site-in-3-steps) at the top.

---

## Technical Architecture & Layout

The project follows a decoupled, unidirectional data flow (Sensors → Event Bus → State Manager → Orchestrator → Specialists → Commander → Dashboard).

```
cmd/
  eoc/            # Main server entrypoint (wires packages; runs API/WS)
  scenariogen/    # Offline scenario compiler (Gemma -> validated JSON)
internal/
  contracts/      # Canonical type definitions & interfaces (cross-package)
  state/          # Sole owner and mutator of the in-memory world state
  validation/     # Invariant rules checked before state updates
  events/         # Unidirectional event bus
  anomaly/        # Maps specific events to specialist cell activations
  orchestrator/   # Handles concurrent cell fan-out and Commander synthesis
  agents/         # Prompts/logic for the five specialist Cells + Commander
  llm/            # Cerebras API client + fallback providers
  simulation/     # Deterministic simulation clock & playback controls
  scenario/       # Scenario loader & structures
  timeline/       # Immutable historical ledger of events
  api/            # REST endpoint definitions
  websocket/      # Live state and event synchronization edge
web/              # Astro + Svelte dashboard UI
```

---

## Prerequisites & Installation

### System Requirements

| Tool        | Version  | Pinned by                             |
| :---------- | :------- | :------------------------------------ |
| **Go**      | `1.24.5` | `go.mod` (`toolchain`)                |
| **Node.js** | `25.9.0` | `.nvmrc`                              |
| **Task**    | `3.x`    | `Taskfile.yml` runner (replaces make) |
| **Docker**  | Any      | Containerized Linux build authority   |

### Running the App

1.  Clone the repository and copy the environment file:
    ```sh
    cp .env.example .env
    ```
2.  Open `.env` and add your `CEREBRAS_API_KEY`.
3.  Run the verification pipeline to ensure the build and tests pass:
    ```sh
    task check
    ```
4.  Start the EOC command server:
    ```sh
    task run
    ```
5.  Open your browser and navigate to `http://localhost:8080` (or the port defined in your configuration).

---

## Tech Stack

| Layer               | Technology                                                                                                                                    |
| :------------------ | :-------------------------------------------------------------------------------------------------------------------------------------------- |
| **Inference**       | **Cerebras** wafer-scale (primary) + **OpenRouter** (fallback & benchmark baseline), both serving **Gemma 4 31B**                                                  |
| **Backend**         | **Go 1.24.5** — event-driven core, deterministic simulation clock, REST API + WebSocket (`/stream`)                                           |
| **Frontend**        | **Astro 7** + **Svelte 5** single-page dashboard                                                                                              |
| **Build & Tooling** | **Task** (task runner), **Docker** (Linux build authority)                                                                                    |
| **Hosting**         | Single-origin Go container on **Fly.io**, fronted by **Cloudflare** for DNS, SSL/TLS, and DDoS — live at [eoc.nryn.dev](https://eoc.nryn.dev) |

The architecture is a decoupled, unidirectional pipeline (see [Technical Architecture & Layout](#technical-architecture--layout)).

> [!IMPORTANT]
> Built by multiple AI coding agents (**Builders**) in parallel. **Read [`AGENTS.md`](AGENTS.md) and [`SPEC.md` §0](SPEC.md) before writing any code.**
> A **Builder** writes this repo; a **Cell** is a runtime agent inside the product — never conflate them.

---

## Acknowledgements

This project was built with the help of many AI systems and the teams behind them. Thanks to **Claude**, **Grok**, **Gemini**, **Nemotron Ultra**, **Nemotron Super**, **OpenRouter**, **Sarvam**, **ChatGPT**, **DeepSeek**, **Xiaomi MiMo 2.5 Pro**, and **Poolside Laguna M**.

**Above all, our deepest thanks to Cerebras and the Gemma team** — the wafer-scale inference and the model that make Cerebro EOC possible.
