// Package simulation is the deterministic simulation clock and replay engine
// (Replay/Pause/Fast-Forward/Reset/branch; SPEC §11). It emits events onto the
// bus like any other sensor — the clock advances the scenario; anomalies (not
// ticks) drive inference.
//
// Owner: simulation + scenario Builder.
// Depends on: contracts/{scenario,events,interfaces} (EventBus).
// Must NOT: write world state directly; read the wall-clock for logic (use the
// injected sim clock — determinism).
package simulation
