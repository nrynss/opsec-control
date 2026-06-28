package scenariogen

/*
Package scenariogen is the offline scenario compiler (SPEC §14).
It generates a deterministic, validated Scenario artifact by coordinating
the static world substrate (§8.3) and an LLM-driven event stream (Acts 1-3, §8.5).

The generator ensures that every emitted event is replayable by running it
through the internal/state.Store (the §14.2 gatekeeper) before finalizing
the scenario.

Owner: Gemma 4 31B on Cerebras Builder
*/
