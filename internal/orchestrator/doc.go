// Package orchestrator runs the concurrent fan-out: on an anomaly it fans the
// state snapshot out to the woken Cells as simultaneous goroutines, gathers
// their parallel outputs, and invokes the Commander to synthesize the COP
// (SPEC §1, §6). It is the ONLY place Cells are invoked.
//
// Owner: orchestrator Builder.
// Depends on: contracts/{interfaces,agentio}.
// Must NOT: mutate world state (it reads a snapshot, Cells return data, only
// internal/state writes); invoke Cells SEQUENTIALLY — that's a spec violation.
package orchestrator
