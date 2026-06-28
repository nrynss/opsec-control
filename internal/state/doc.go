// Package state is the sole owner and the single mutator of the authoritative
// in-memory World State, and the §14.2 validation gatekeeper: every event —
// generated or live — must pass before it touches state, and every accepted
// event increments the world version (SPEC §8, §14.2).
//
// Owner: state + validation Builder.
// Depends on: contracts/{state,events,errors}.
// Must NOT: let any other package hold or mutate live state; bypass validation.
package state
