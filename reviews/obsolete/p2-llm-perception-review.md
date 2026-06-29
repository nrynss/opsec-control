# Review: P2 — LLM Client (LatencyMS + Multimodal Perception) (Antigravity Builder)

**Parcel ID:** P2  
**Status (HANDOFF):** ✅ Done — Antigravity Builder (2026-06-29)  
**Date of review:** 2026-06-29  
**Reviewer:** Grok  

## Summary

P2 delivered the population of telemetry metrics (especially `LatencyMS`) from the Cerebras client and the new live multimodal perception capability (`POST /perception` path via `Perception.Interpret`).

### Changes delivered
- `internal/llm/client.go`
  - `completeReal`: Now populates `TokensIn`, `TokensOut`, `TokensPerSec` (prefers `time_info.completion_time`, falls back to client-side duration), and `LatencyMS: duration.Milliseconds()`.
  - `completeMock`: Fixed `TokensPerSec: 1500.0`, computes realistic simulated duration, sets `LatencyMS`.
  - Semaphore (`maxConcurrency=4`) and full ctx cancellation handling preserved.
- New `internal/llm/perception.go`
  - `Interpret(ctx, ImageInput)` — dispatches to mock or real.
  - **Mock mode** (`LLM_MOCK=true` or no key): 300ms simulated delay (ctx-aware), string-based trigger matching for demo images, returns structured `[]Event`.
  - **Real mode**: Uses `gemma-4-31b`, base64 data-URI (`data:image/...;base64,...`), sends vision request with JSON schema, parses into `Event`s with content-hash IDs.
- Tests:
  - `client_test.go`: asserts mock `TokensPerSec == 1500`, fallback duration path, retries.
  - New `perception_test.go`: covers all demo trigger cases for mock perception.

## Protocol Compliance

- Stayed strictly in `internal/llm` lane.
- `maxConcurrency=4` was explicitly kept (as required in the parcel description and "reality check" section of HANDOFF).
- Both paths honor `context.Context` cancellation.
- No domain logic leaked into contracts.
- Provider-specific types (`chatCompletionResponse`, etc.) stay inside `llm/`.
- Determinism: mock path uses no unseeded rand; real path uses only response data.

## Positives

- Exactly matches the P2 spec:
  > Populate `LatencyMS` (real + mock); implement `Interpret` in new `perception.go` (mock + Cerebras vision `gemma-4-31b`, base64 data-URI, structured `[]Event`); keep `maxConcurrency=4`
- Excellent ctx handling in both `Complete` and `Interpret` (important for concurrent fan-out).
- Mock perception is high-fidelity enough for the demo (including the string triggers used in `scenario.json`).
- Real vision path correctly builds data URIs and uses structured output.
- Metrics population now feeds the `CellMetrics` we added in P3.
- Semaphore is applied to perception calls as well (correct for overall concurrency budget).

## Minor Nits / Observations

1. **Source string inconsistency**  
   Mock uses `"Gemma4-Perception-..."` while real uses `"Cerebras-Perception-..."`.  
   Minor cosmetic issue; tests expect the Gemma4 form in mock mode.

2. **Mock perception trigger matching**  
   Uses `strings.Contains(string(input.Data), ...)` on raw bytes. Works for the hand-curated demo payloads but is fragile for real images. Acceptable for MVD.

3. **No metrics returned from `Interpret`**  
   Correct by design (`Perception` produces events, not `LLMResponse`). The metrics are only for the cell reasoning calls.

4. **Perception calls also consume the concurrency semaphore**  
   This is good (prevents exceeding the 4-in-flight limit), but means a live perception call + fan-out could queue. Worth documenting in the HUD story.

5. **Test coverage for `LatencyMS`**  
   The fallback-duration test only asserts `TokensPerSec > 0`. A direct assertion on `LatencyMS > 0` would be stronger.

6. **Real vision path error messages**  
   Good detail, but the schema sent to the vision model is slightly looser than the cell schema (payload not strictly required). Intentional and fine.

## Test & Verification Status

- `go test ./internal/llm -run 'Mock|Perception|fallback'` — all pass.
- `TestPerceptionMock` covers the four demo cases.
- Full `./...` green.
- No new gopls hints.
- `maxConcurrency=4` remains the default.

## Verdict

**Strong and complete.** P2 delivered both halves of the requirement (metrics population + the new perception layer) while respecting the hard concurrency constraints emphasized throughout the project.

The implementation enables:
- Real per-cell telemetry (P3)
- Live image → event flow (P5/P6/P7)

**Score:** 9 / 10  
(Docked one point for the small source-string and test-assertion nits.)

**Recommendation:** Ready. Minor cleanups (source naming, one extra test assertion) can be done as drive-bys if desired, but nothing blocking.

The live "drop an image and watch cells re-reason" demo story now has its backend.
