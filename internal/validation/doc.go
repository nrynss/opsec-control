// Package validation implements the §14.2 event-validation rules — schema,
// referential integrity, temporal monotonicity, state-transition legality,
// range/physical sanity, idempotency — shared by internal/state (the live
// gatekeeper) and internal/scenariogen (the offline compiler).
//
// Owner: state + validation Builder.
// Depends on: contracts/{state,events,errors}.
// Must NOT: mutate world state; it returns accept/reject (event_rejected reasons).
package validation
