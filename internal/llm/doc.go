// Package llm is the Cerebras client (Gemma 4 31B) behind the contracts.LLMClient
// interface, plus the throughput/latency metrics surfaced on the HUD (SPEC §16.1,
// §15.1).
//
// Owner: Antigravity Builder (llm lane claimed in HANDOFF.md).
// Depends on: contracts/interfaces (LLMClient).
// Must NOT: own domain/operational logic; leak provider-specific types past the
// interface boundary.
//
// Cerebras API Hard Constraints (Measured June 2026):
// - Concurrency ceiling: 4 concurrent in-flight requests.
// - Request Rate limit: 100 Requests Per Minute (RPM).
// - Token Rate limit: 100,000 Tokens Per Minute (TPM).
// - Exceeding limits triggers HTTP 429 with a 'Retry-After' header (typically 60s).
//
// Bounded Concurrency Semaphore:
// The client enforces a default MaxConcurrency cap of 4. Excess requests queue
// internally inside Complete() until a slot opens, preventing 429 rate limit
// exhaustion.
package llm
