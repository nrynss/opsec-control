// Package anomaly classifies each accepted event and decides which Cells to
// wake — the trigger for the parallel fan-out (SPEC §6). Thresholds/status
// changes (bridge/road, flood delta, hospital capacity bands, aftershocks,
// clustered citizen reports) decide who fires.
//
// Owner: Grok Builder (anomaly lane implemented; see HANDOFF.md).
// Depends on: contracts/{state,events,agentio}.
// Must NOT: mutate world state; invoke Cells (it only decides who wakes — the
// orchestrator does the firing).
//
// Detector implements contracts.Classifier. The returned list contains
// specialist CellKinds only. The orchestrator unconditionally runs
// Commander as phase-2 synthesis after specialists (§6).
package anomaly
