// Package llm is the Cerebras client (Gemma 4 31B) behind the contracts.LLMClient
// interface, plus the throughput/latency metrics surfaced on the HUD (SPEC §16.1,
// §15.1).
//
// Owner: Antigravity Builder (llm lane claimed in HANDOFF.md).
// Depends on: contracts/interfaces (LLMClient).
// Must NOT: own domain/operational logic; leak provider-specific types past the
// interface boundary.
package llm
