// Package api is the HTTP edge (SPEC §12): GET /state /agents /timeline /events,
// POST /events /perception /scenario/load /scenario/reset. It serializes contract DTOs and
// forwards events to the bus. It is the ONLY package the frontend talks to.
//
// Owner: Grok Builder (api + websocket lane implemented; see HANDOFF.md).
// Depends on: contracts/* (DTOs) and the StateStore/EventBus/Orchestrator ifaces.
// Must NOT: hold state; contain operational logic; transform or derive state —
// it only serializes contract types over HTTP and forwards events.
package api
