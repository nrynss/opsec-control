// Package contracttest holds the shared, contract-level tests that prove any
// implementation satisfies the interface seams in internal/contracts and
// round-trips the canonical schemas (SPEC §0.6).
//
// Every Builder runs the full suite before declaring done. The suite is
// collectively owned: a Builder may add cases, but changing it is a §0.5 action.
package contracttest
