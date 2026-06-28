// Package events is the event bus: it distributes events (pub/sub) and never
// owns state. Flow: Sensors -> Event Bus -> State Manager -> Cells -> Commander
// -> Dashboard (SPEC §7).
//
// Owner: events Builder.
// Depends on: contracts/{events,interfaces} (EventBus).
// Must NOT: own state; call Cells or the LLM; add event types (that's a §0.5
// change to contracts/events.go, not here).
package events
