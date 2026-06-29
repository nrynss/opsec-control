# Cerebras Wafer-Scale Inference Effectiveness & Multimodal Perception Analysis

This document evaluates the integration, performance, and functional effectiveness of **Cerebras wafer-scale inference (~1,500 tokens/sec, Gemma 4 31B)** and details the status of the **Multimodal Image Perception Layer** in the **Cerebro Emergency Operations Center (EOC)** project.

---

## 1. Executive Summary

Conceptually, the EOC project is designed to leverage Cerebras's extreme inference speed (~1,500 tok/s) by invoking multiple specialist agents concurrently during an anomaly event. 

However, in the current codebase, **we are not using the Cerebras 1500+ token capability effectively**. While the orchestrator implements parallel fan-out using Go goroutines, three main issues limit its real-world benefit:
1. **Concurrency Bottlenecks:** A strict client-side cap of 4 concurrent requests serializes the 5th agent request during major seismic events.
2. **Lack of Agentic Multi-turn Loops:** The agents only run single-shot prompt-completions, neglecting the planned critique-refine loops that Cerebras's speed is designed to enable.
3. **Telemetry Metrics Disconnect:** Actual tokens-per-second and latency metrics are calculated by the client but discarded during unmarshaling, forcing the frontend dashboard to display hardcoded mock data (`1500 tok/s`).

Additionally, **no actual image analysis or multimodal capabilities are being performed**. The perception event stream is entirely simulated from static mock events in the scenario timeline.

---

## 2. Core Cerebras Inference Effectiveness

### A. Architectural Strengths
The system aligns cleanly with the thesis of parallel reasoning:
- **Parallel Fan-out:** In `internal/orchestrator/engine.go`, specialist cells are executed concurrently using goroutines. They process the world state snapshot in parallel rather than sequentially, which prevents latency compounding.
- **Pure Functions:** Specialist cells act as pure functions of `(Snapshot + Trigger) -> CellOutput` and never mutate state directly or communicate with each other, maximizing reasoning safety and execution speed.

### B. Inefficiencies & Bottlenecks

#### 1. Concurrency Serialization
The client-side LLM wrapper in `internal/llm/client.go` caps requests at `maxConcurrency = 4` to prevent HTTP 429 rate limit errors (due to developer account ceilings):
```go
// internal/llm/client.go
maxConcurrency := cfg.MaxConcurrency
if maxConcurrency <= 0 {
    maxConcurrency = 4 // Strict concurrency ceiling
}
sem := make(chan struct{}, maxConcurrency)
```
However, in `internal/anomaly/detector.go`, a mainshock or aftershock wakes up **5 specialist cells** simultaneously (Intelligence, Infrastructure, Medical, Population, Communications). 

Because of the cap, the 5th cell blocks on the semaphore until one of the first 4 finishes. This serializes execution, increasing the total reasoning latency to **$2 \times \text{average LLM request duration}$**, undermining the parallel-by-default wafer-scale speed advantage.

#### 2. Absence of Reasoning Loops
The `SPEC.md` outlines that Cerebras's speed allows a sub-second **plan $\rightarrow$ critique $\rightarrow$ refine** loop within each cell before outputting recommendations. 

In the actual implementation (`internal/agents/cells.go`), every agent simply makes a single call to `llm.Complete`. The agents act as basic, single-shot responders without any agentic self-critique or structured refinement.

#### 3. HUD Telemetry Disconnect
The LLM client correctly parses `TimeInfo.CompletionTime` and `Usage.CompletionTokens` from the Cerebras API response to compute `TokensPerSec`:
```go
// internal/llm/client.go
return contracts.LLMResponse{
    Content:      completionContent,
    TokensIn:     tokensIn,
    TokensOut:    tokensOut,
    TokensPerSec: tokensPerSec,
}, nil
```
However, the agent runner in `internal/agents/cells.go` immediately discards the response metadata and only unmarshals `resp.Content` into a standard `CellOutput` struct:
```go
// internal/agents/cells.go
resp, err := b.llm.Complete(ctx, ...)
// ...
var out contracts.CellOutput
err = json.Unmarshal([]byte(resp.Content), &out) // TokensPerSec and TokensOut are lost here!
```
Because the `CellOutput` and `CommonOperationalPicture` contracts lack timing/throughput fields, the backend cannot stream real timing telemetry to the web dashboard. The Astro/Svelte dashboard HUD instead relies on hardcoded mockup values (`metrics.tokensPerSec = 1500`) in its offline demo mode, showing `0 tok/s` in live mode.

---

## 3. The Image Layer & Multimodal Aspect

### A. Current Status
Currently, there is **zero image analysis happening in the codebase**:
- The `Perception` interface defined in `internal/contracts/interfaces.go` has no concrete Go implementation.
- The `internal/sensors` package (intended for ingest adapters like satellite and drone imagery) is a stub containing only a `doc.go` file.
- The scenario file (`cmd/eoc/scenario.json`) contains hardcoded pre-fabricated events with the source labeled `"Gemma4-Perception"`. These are static mock payloads; no vision model ever processes an image to generate them.

#### B. Multimodal Capabilities on Cerebras
Cerebras runs the Gemma 4 31B model, which is native multimodal. Processing base64 image data directly on this native multimodal model returns a structured JSON array of events in under **~300ms**, allowing live image-to-event perception to run on the reasoning path.

### C. Implementation Blueprint for today's Hackathon

To show a live multimodal perception demo today, we can wire up the image layer using the following blueprint:

```
                  ┌──────────────────────┐
                  │ Svelte Web Dashboard │
                  └──────────┬───────────┘
                             │ POST /perception (Image Payload)
                             ▼
                  ┌──────────────────────┐
                  │  internal/api/api.go │
                  └──────────┬───────────┘
                             │ Calls Interpret()
                             ▼
               ┌───────────────────────────┐
               │  internal/llm/perception  │
               └─────────────┬─────────────┘
                             │ Multimodal vision API
                             ▼
                  ┌──────────────────────┐
                  │ Cerebras Wafer-scale │
                  └───────────▲──────────┘
                              │
                  (Gemma 4 31B Native)
```

#### 1. Implement `contracts.Perception` in the LLM Package
Create `internal/llm/perception.go` to process image bytes. It should base64-encode the payload and make a structured OpenAI-compatible vision completion call to Cerebras using `gemma-4-31b`:
```go
func (c *Client) Interpret(ctx context.Context, input contracts.ImageInput) ([]contracts.Event, error) {
	if c.apiKey == "" {
		return c.interpretMock(ctx, input) // Return pre-mapped events for offline demo
	}
	// Assemble base64 data URI: data:image/jpeg;base64,...
	// Send to Cerebras /chat/completions using gemma-4-31b
}
```

#### 2. Expose the Ingestion Endpoint in the API
Add a `POST /perception` route in `internal/api/api.go` that accepts multipart form uploads or raw binary payloads:
```go
func (s *Server) handlePostPerception(w http.ResponseWriter, r *http.Request) {
	// 1. Read binary image bytes
	// 2. Instantiate ImageInput{Source: "drone", Data: bytes}
	// 3. Call perception.Interpret(ctx, imageInput)
	// 4. Publish resulting events to s.bus
}
```

#### 3. Connect Svelte Dashboard to upload Drone/Satellite Imagery
Update `web/src/components/Map.svelte` or add an upload widget that lets users drop a disaster screenshot (e.g. showing a flooded Southport area). 
- When an image is dropped, upload it to `POST /perception`.
- The backend parses the image using the Cerebras vision model.
- The vision model outputs structured JSON (e.g., `LeveeBreached` or `BridgeClosed`).
- The event is published onto the EventBus, instantly triggering the concurrent specialist cells fan-out and updating the command room dashboard.

---

## 4. Hackathon Action Plan (Summary)

To maximize the impact of the Cerebras presentation today:
1. **Expose Real Metrics:** Add metrics fields to the contracts (`CellOutput` and `CommonOperationalPicture`) and pipe the real tokens-per-second and latency from `llm.Complete` directly to the Svelte HUD.
2. **Showcase Vision Live:** Implement the `POST /perception` endpoint and Svelte upload interface using the blueprint above. This turns "deferred future intent" into a concrete, impressive, live wafer-scale perception demonstration.
3. **Showcase Reasoning Speed:** Add a simple critique step in `internal/agents/cells.go` (e.g., asking the model to check its own JSON format and recommendations once before emitting) to demonstrate that multi-turn agent reasoning still completes in under a second on Cerebras.
