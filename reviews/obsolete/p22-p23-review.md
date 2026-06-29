# Deep Review: P22 (State Reset) and P23 (Timeline Truncate)

**Commit:** [283b6bb](file:///e:/opsec-control/commit/283b6bb213467622a3b7dc27661f423f1896c5c0) (`feat(state): implement thread-safe Reset for world state (P22)`) and [aaa472e](file:///e:/opsec-control/commit/aaa472ed350466b64348e77c8a22b399f1de61ae)  
**Date:** 2026-06-29  
**Reviewer:** Gemini 3.5 Flash (Builder)  
**Branch:** feat/live-simulation-controls  

---

## Executive Summary of Findings

Both P22 and P23 are **fully and correctly implemented, thread-safe, and well-tested**.

- **P22 (state.Reset)**: The thread-safe [Reset](file:///e:/opsec-control/internal/state/store.go#L65-L72) method has been successfully added to the [Store](file:///e:/opsec-control/internal/state/store.go). It correctly utilizes a write lock, performs a deep clone of the initial WorldState, clears the event deduplication (`seen`) map, and resets `Version` and `Time` back to 0. A robust unit test is added to [store_test.go](file:///e:/opsec-control/internal/state/store_test.go#L238-L267).
- **P23 (timeline.Truncate)**: The thread-safe [Truncate](file:///e:/opsec-control/internal/timeline/timeline.go#L61-L65) method has been added to [Timeline](file:///e:/opsec-control/internal/timeline/timeline.go). It correctly acquires a write lock and empties the list of events. Comprehensive unit tests are included in [timeline_test.go](file:///e:/opsec-control/internal/timeline/timeline_test.go#L222-L239).

This review corrects a previous draft written by Gemma 4 which mistakenly evaluated P25 (web/ frontend) and P26 (verification/tests) under the P22/P23 labels.

---

## Alignment with Design (HANDOFF.md)

### P22 (State Reset)
- **Required**: Expose thread-safe Reset(initial WorldState) (clears seen map).
- **Actual**: Implemented in [store.go](file:///e:/opsec-control/internal/state/store.go#L65-L72). Clones the initial state to prevent mutable leakages, clears the deduplication map, and resets versioning metadata. Fully compliant.

### P23 (Timeline Truncate)
- **Required**: Expose thread-safe Truncate() to clear events.
- **Actual**: Implemented in [timeline.go](file:///e:/opsec-control/internal/timeline/timeline.go#L61-L65). Re-allocates the internally tracked entries slice. Fully compliant.

---

## Compliance with Core Rules (AGENTS.md / SPEC.md §0)

- **Lane Ownership (§16.1)**: Modified files reside strictly within `internal/state` and `internal/timeline` (and their test counterparts). These areas belong in the state & timeline lanes (implemented by Gemma 4).
- **Contract-first**: No contract changes were required as these functions represent package-internal mutations rather than shared public contracts.
- **Thread-Safety**: Correctly uses write locks via `s.mu.Lock()` and `t.mu.Lock()`.
- **Determinism**: The methods execute completely deterministically. Clearing events/deduplication maps does not introduce wall-clock reads, maps-ordering dependencies, or unseeded randomness.
- **Test Integrity**: All tests (`go test ./...`) pass cleanly with no regression.

---

## Detailed Code Walkthrough

### P22 (State Reset) in [store.go](file:///e:/opsec-control/internal/state/store.go)

The [Reset](file:///e:/opsec-control/internal/state/store.go#L65-L72) method has been defined as:
```go
func (s *Store) Reset(initial contracts.WorldState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ws = clone(initial)
	s.seen = make(map[contracts.EventID]struct{})
	s.ws.Version = 0
	s.ws.Time = 0
}
```

#### Key Strengths:
1. **Safety via `clone`**: Using `clone(initial)` ensures that external modifications to the initial WorldState struct do not affect the store's internal state.
2. **Helper Refactoring**: The `clone` helper was refactored in `store.go` to construct a new struct `out` instead of mutating the parameter `ws` directly. This makes it clean and less error-prone.
3. **Map Re-allocation**: Allocating a fresh map for `s.seen` is correct for clearing deduplication records.

#### Test Coverage:
[TestApply_Reset](file:///e:/opsec-control/internal/state/store_test.go#L238-L267) in [store_test.go](file:///e:/opsec-control/internal/state/store_test.go) exercises the full cycle:
1. Applies an event (e.g. closing bridge "vora"), verifying version increases and bridge status updates.
2. Performs `s.Reset(initWS)`.
3. Asserts the version goes back to 0, and the bridge is open again.
4. Confirms the deduplication map reset by applying the same event ID ("e1") again, asserting that it succeeds instead of being rejected as a duplicate.

---

### P23 (Timeline Truncate) in [timeline.go](file:///e:/opsec-control/internal/timeline/timeline.go)

The [Truncate](file:///e:/opsec-control/internal/timeline/timeline.go#L61-L65) method has been defined as:
```go
func (t *Timeline) Truncate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = make([]Entry, 0)
}
```

#### Key Strengths:
1. **Thread-Safe Mutation**: Wrapping the slice clearing in a write lock is correct, as multiple HTTP/WS read operations may query the timeline concurrently.
2. **Explicit Empty Allocation**: Setting `t.entries = make([]Entry, 0)` is a reliable way to discard previous logs.

#### Test Coverage:
[TestTruncate](file:///e:/opsec-control/internal/timeline/timeline_test.go#L222-L239) in [timeline_test.go](file:///e:/opsec-control/internal/timeline/timeline_test.go) exercises this:
1. Appends two distinct events.
2. Asserts timeline length is 2.
3. Invokes `Truncate()`.
4. Asserts timeline length is 0 and `Last()` returns nil.

---

## Recommendations & Verdict

### Verdict: **PASS**

Both P22 and P23 implementations are clean, minimal, robust, and conform to the project standards. They are fully prepared to support the API-level reset endpoint (P24) and downstream Svelte components (P25).

No changes are needed to the codebase for these two parcels.