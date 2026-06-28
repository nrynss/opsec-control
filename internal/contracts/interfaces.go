package contracts

// interfaces.go — the interface seams every package depends on instead of each
// other's implementations (SPEC §0.2 rule 3):
//
//	EventBus      — pub/sub distribution (internal/events)
//	StateStore    — read snapshot + the single mutator path (internal/state)
//	Cell          — one specialist agent: (snapshot, event) -> output (internal/agents)
//	Orchestrator  — concurrent fan-out + Commander synthesis (internal/orchestrator)
//	LLMClient     — Cerebras/Gemma client + throughput metrics (internal/llm)
//	Perception    — image -> structured event
//
// Placeholder: no interfaces are defined yet. Add them via the §0.5 step;
// keep them small and implementation-free.
