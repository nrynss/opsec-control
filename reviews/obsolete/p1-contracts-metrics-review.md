# Review: P1 — Contracts Telemetry / Metrics (Claude Builder)

**Parcel ID:** P1  
**Status (HANDOFF):** ✅ Done — Claude Builder (commit `47a4809`)  
**Date of review:** 2026-06-29  
**Reviewer:** Grok  

## Summary

P1 was the required isolated §0.5 contract change to add telemetry fields so that real Cerebras throughput numbers (tokens/sec, latency, etc.) can flow from the LLM client through the agents to the COP and ultimately the HUD.

### Changes delivered
- `internal/contracts/agentio.go`
  - New `CellMetrics` struct (exact fields: tokensIn, tokensOut, tokensPerSec, latencyMs)
  - `Metrics CellMetrics` field added to `CellOutput`
  - New `COPMetrics` struct (fanOutLatencyMs, totalTokensIn, totalTokensOut, peakTokensPerSec, aggregateTokensPerSec, cellCount)
  - `Metrics COPMetrics` field added to `CommonOperationalPicture`
- `internal/contracts/interfaces.go`
  - `LatencyMS int64` added to `LLMResponse` (with clear comment about real vs. mock semantics)
- `internal/contracts/contracttest/roundtrip_test.go`
  - `TestCOPRoundTrip` extended with realistic `CellMetrics` and `COPMetrics` values

All changes were additive.

## Protocol Compliance

- **§0.5 followed perfectly**: Contract change landed as its own isolated commit touching only `contracts/` (plus the collectively-owned contracttest).
- **Stay in lane**: No implementation logic added. Only type definitions and test data.
- **Additive by default**: No existing fields or semantics were altered.
- **Contracttest updated**: Round-trip coverage for the new shapes was added in the same parcel (as expected).
- No other packages were touched.

## Positives

- Exact match to the spec in HANDOFF.md §8:
  > Add `CellMetrics{...}` + `CellOutput.Metrics`; `COPMetrics{...}` + `COP.Metrics`; `LLMResponse.LatencyMS`
- Excellent comments in the code explaining:
  - That metrics are computed by the LLM client (not the model)
  - That multi-turn cells (plan→critique) must aggregate
  - The meaning of `AggregateTokensPerSec` as the "headline wafer-scale" number
- Roundtrip test now exercises the new types with realistic numbers.
- Fields are placed logically and use consistent naming (`LatencyMS`, camelCase in JSON tags).
- No breakage to existing `CellOutput` or `COP` usage (additive only).

## Minor Nits / Observations

1. **Aggregation logic is intentionally deferred**  
   The actual population of `COPMetrics` (summing across cells, computing peaks and aggregate rate) lives in P4 (`orchestrator`). This is the correct sequencing — P1 only provided the containers.

2. **No new JSON schemas in `contracts/schemas/`**  
   The schemas directory appears to be lightly used / not yet comprehensive. Since P1 was purely additive and the existing roundtrips pass, this is acceptable, but future frontend work (P7) will eventually need them.

3. **Field ordering in structs**  
   `CellMetrics` and `COPMetrics` are appended at the end of their parent structs. This is fine for Go/JSON (tag-keyed), but if a strict "append new fields" convention is desired, it was already followed here.

## Test & Verification Status

- `go test ./internal/contracts/contracttest -run COP` — passes (roundtrip of new metrics shapes)
- Full `./...` and contracttest suite remain green after P1.
- No gopls modernize hints introduced.

## Verdict

**Clean, minimal, and correct.** P1 is a textbook example of a well-executed §0.5 contract change. It delivered exactly what was asked, updated the shared test seam, and left the door open for P2 (population) and P4 (aggregation) without any future friction.

**Score:** 9.5 / 10  
(Only docked half a point for the missing schema files, which may be out of scope for this parcel.)

**Recommendation:** Ready for P2 / P3 / P4 consumers. No rework required.
