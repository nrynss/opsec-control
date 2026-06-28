// Package contracts is the canonical source of truth for every cross-boundary
// shape in the system: event/state/agent-I/O types, the interface seams between
// packages, the scenario file format, and shared error types (SPEC §0.4).
//
// It depends on no implementation; implementations depend on it. The files in
// this package are append-/change-by-coordination-only: do NOT add or alter a
// type here to suit your package. Propose it, land it as its own isolated
// `contract(...)` commit, then each affected owner updates (SPEC §0.5).
//
// Owner: contracts Builder. Changing anything here is a §0.5 action.
package contracts
