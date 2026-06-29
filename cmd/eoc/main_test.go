package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nrynss/opsec-control/internal/agents"
	"github.com/nrynss/opsec-control/internal/anomaly"
	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/events"
	"github.com/nrynss/opsec-control/internal/orchestrator"
	"github.com/nrynss/opsec-control/internal/scenario"
	"github.com/nrynss/opsec-control/internal/state"
)

// fakeLLM returns a fixed, schema-valid CellOutput so the Cells/orchestrator are
// deterministic and offline (no Cerebras, no llm-package mock content dependency).
type fakeLLM struct{}

func (fakeLLM) Complete(_ context.Context, _ contracts.LLMRequest) (contracts.LLMResponse, error) {
	return contracts.LLMResponse{
		Content: `{"summary":"mock analysis","riskLevel":"High","confidence":0.9,"recommendations":["act now"],"evidence":["telemetry"]}`,
	}, nil
}

func newTestApp(t *testing.T) (*app, *contracts.Scenario) {
	t.Helper()
	scn, err := scenario.LoadJSON(embeddedScenario)
	if err != nil {
		t.Fatalf("embedded scenario failed to load: %v", err)
	}
	llm := fakeLLM{}
	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: agents.NewInfrastructure(llm),
		contracts.CellMedical:        agents.NewMedical(llm),
		contracts.CellPopulation:     agents.NewPopulation(llm),
		contracts.CellCommander:      agents.NewCommander(llm),
	}
	a := &app{
		store:      state.New(scn.Initial),
		classifier: anomaly.New(),
		orch:       orchestrator.NewEngine(cells),
		cop:        &copStore{},
		ws:         nil, // broadcast is a no-op in tests
		epoch:      0,
	}
	return a, scn
}

// TestEmbeddedScenarioReplaysCleanly is the end-to-end flow check: every event in
// the shipped scenario must validate+apply (zero rejections) and the cascade must
// produce a COP. If this fails, the demo scenario is broken.
func TestEmbeddedScenarioReplaysCleanly(t *testing.T) {
	a, scn := newTestApp(t)
	ctx := context.Background()

	fanouts := 0
	for _, ev := range scn.Events {
		before := a.store.Version()
		if a.handle(ctx, ev) {
			fanouts++
		}
		if a.store.Version() != before+1 {
			t.Fatalf("event %s (%s) did not apply — likely rejected by the §14.2 gatekeeper", ev.ID, ev.Type)
		}
	}

	if got := int(a.store.Version()); got != len(scn.Events) {
		t.Fatalf("final version %d, want %d (one per event)", got, len(scn.Events))
	}
	if fanouts == 0 {
		t.Fatal("no fan-outs occurred over the whole scenario")
	}
	cop := a.cop.Current()
	if cop.OverallRisk == "" || cop.Summary == "" {
		t.Fatalf("no COP produced after replay: %+v", cop)
	}
	if len(cop.CellOutputs) == 0 {
		t.Fatal("COP has no specialist cell outputs")
	}
	t.Logf("replayed %d events, %d fan-outs, final COP risk=%s", len(scn.Events), fanouts, cop.OverallRisk)
}

// TestBusPathAppliesAllEvents exercises the real subscribe→loop→state path (not
// just direct handle). Subscribing before publishing guards the ordering bug
// where the state loop could miss the simulator's first (t=0) event.
func TestBusPathAppliesAllEvents(t *testing.T) {
	a, scn := newTestApp(t)
	bus := events.New(128)

	ctx := t.Context()

	ch, unsub := bus.Subscribe() // subscribe BEFORE publishing
	defer unsub()
	go a.runLoop(ctx, ch, 0)

	for _, e := range scn.Events {
		bus.Publish(e)
	}

	deadline := time.After(5 * time.Second)
	want := contracts.StateVersion(len(scn.Events))
	for a.store.Version() < want {
		select {
		case <-deadline:
			t.Fatalf("only %d/%d events reached state", a.store.Version(), want)
		default:
			time.Sleep(2 * time.Millisecond)
		}
	}
}

func ev(id string, ts int, src string, typ contracts.EventType, payload string) contracts.Event {
	return contracts.Event{
		ID:         contracts.EventID(id),
		Timestamp:  contracts.SimTime(ts),
		Source:     src,
		Type:       typ,
		Confidence: 1,
		Payload:    json.RawMessage(payload),
	}
}

// TestAmbientEventSkipsFanOut proves the volume/signal split: an ambient-sourced
// event is applied to state but never triggers the fan-out (Cerebras budget).
func TestAmbientEventSkipsFanOut(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()

	// A real anomaly fans out.
	if !a.handle(ctx, ev("a1", 1, "sensor", contracts.EventBridgeClosed, `{"bridgeId":"B-VORA"}`)) {
		t.Fatal("bridge-closed should have fanned out")
	}
	verAfterAnomaly := a.store.Version()

	// An ambient event applies to state but must NOT fan out.
	if a.handle(ctx, ev("amb1", 2, "ambient", contracts.EventCitizenDistressCall, `{}`)) {
		t.Fatal("ambient event must not trigger fan-out")
	}
	if a.store.Version() != verAfterAnomaly+1 {
		t.Fatal("ambient event should still have been applied to state")
	}
}

// TestRejectedEventDoesNotBumpVersionOrFanOut guards the gatekeeper path.
func TestRejectedEventDoesNotBumpVersionOrFanOut(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()

	before := a.store.Version()
	// Unknown bridge → referential-integrity rejection.
	if a.handle(ctx, ev("bad", 1, "sensor", contracts.EventBridgeClosed, `{"bridgeId":"NOPE"}`)) {
		t.Fatal("rejected event must not fan out")
	}
	if a.store.Version() != before {
		t.Fatal("rejected event must not bump the world version")
	}
}
