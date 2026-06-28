// Package scenariogen is the offline scenario-compiler logic: Gemma proposes raw
// candidate events, the §14.2 validator rejects/repairs invariant violations,
// and the result is a deterministic, versioned, replayable scenario JSON file
// (SPEC §14.1). It is an OFFLINE tool, driven by cmd/scenariogen.
//
// Owner: scenariogen Builder.
// Depends on: contracts/{scenario,events,interfaces} (LLMClient), internal/validation.
// Must NOT: run on the live request path; touch runtime world state; emit
// unvalidated output.
package scenariogen
