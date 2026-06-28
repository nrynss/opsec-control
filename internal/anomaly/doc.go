// Package anomaly classifies each accepted event and decides which Cells to
// wake — the trigger for the parallel fan-out (SPEC §6). Thresholds/status
// changes (bridge/road, flood delta, hospital capacity bands, aftershocks,
// clustered citizen reports) decide who fires.
//
// Owner: anomaly Builder.
// Depends on: contracts/{state,events,agentio}.
// Must NOT: mutate world state; invoke Cells (it only decides who wakes — the
// orchestrator does the firing).
package anomaly
