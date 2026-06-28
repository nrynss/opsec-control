// Command eoc is the EOC server entrypoint. It wires the interfaces together
// (EventBus, StateStore, Orchestrator, API/WebSocket, simulation) and owns no
// operational logic itself (SPEC §5, §16).
package main

func main() {
	// TODO: construct internal/* implementations against their contract
	// interfaces and serve the API/WebSocket edge. See SPEC §5, §12.
}
