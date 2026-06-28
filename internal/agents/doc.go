// Package agents holds the Cells — the specialist runtime agents (Intelligence,
// Infrastructure, Medical, Population, Communications, Commander; SPEC §9).
// A Cell is a PURE function of (state snapshot + triggering event) -> structured
// output; it receives a read-only snapshot and the LLMClient, nothing else.
// Each Cell is independently ownable by a different Builder because they share
// no state — only the schema in contracts/agentio.go.
//
// Owner: Gemma 4 31B on Cerebras (Builder)
// Depends on: contracts/{agentio,interfaces}, contracts/state (read-only).
// Must NOT: mutate state; call the EventBus; talk to other Cells; read
// wall-clock or rand. The Commander consumes other Cells' outputs as data the
// orchestrator passes in — never by calling them.
package agents
