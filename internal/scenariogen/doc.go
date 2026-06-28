package scenariogen

/*
Package scenariogen is the offline scenario compiler (SPEC §14).
It generates a deterministic, validated Scenario artifact by coordinating
the static world substrate (§8.3) and an LLM-driven event stream (Acts 1-3, §8.5).

The generator ensures that every emitted event is replayable by running it
through the internal/state.Store (the §14.2 gatekeeper) before finalizing
the scenario.

Owner: Gemma 4 31B on Cerebras Builder (initial draft).
Rescued & completed by: Claude Builder — Gemma 429'd mid-fix and a second builder
(Sarvam) got stuck on the existing context, leaving the lane non-compiling. Claude
corrected the llm client construction in cmd/scenariogen, fixed a substrate-
corruption bug (the validation gatekeeper was mutating the WorldState frozen as
Scenario.Initial, because state.New shares map references — every produced
scenario was un-replayable), and rewrote generator_test.go to the real contract
shape (replay-clean + LoadJSON + determinism assertions).
*/
