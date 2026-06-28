// Package websocket is the streaming transport edge paired with internal/api
// (WS /stream; SPEC §12, §10): it streams state ripples and agent output
// (token-by-token) to the dashboard.
//
// Owner: Grok Builder (api + websocket lane claimed in HANDOFF.md).
// Depends on: contracts/* (DTOs) and the StateStore/EventBus/Orchestrator ifaces.
// Must NOT: hold state; contain operational logic — it only serializes contract
// types over the wire.
package websocket
