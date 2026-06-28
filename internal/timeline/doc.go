// Package timeline is the immutable event log and replay index (SPEC §16.1):
// the append-only record of every accepted event (and event_rejected entries),
// queried for the dashboard timeline and deterministic replay.
//
// Owner: timeline Builder.
// Depends on: contracts/events.
// Must NOT: mutate events — the log is append-only.
package timeline
