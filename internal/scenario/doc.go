// Package scenario loads validated scenario files for the simulation engine to
// replay (SPEC §11, §14). It reads only validated, versioned scenario JSON;
// bundle files with //go:embed rather than runtime filesystem reads (SPEC §19.2).
//
// Owner: simulation + scenario Builder.
// Depends on: contracts/{scenario,events}.
// Must NOT: write world state; replay unvalidated input.
package scenario
