// Package timeline is the immutable event log and replay index (SPEC §16.1):
// the append-only record of every accepted event (and event_rejected entries),
// queried for the dashboard timeline and deterministic replay.
//
// Owner: Poolside Laguna M (timeline lane taken in HANDOFF.md §3).
// Depends on: contracts/{events,interfaces}.
// Must NOT: mutate events — the log is append-only; does not own world state.
package timeline
