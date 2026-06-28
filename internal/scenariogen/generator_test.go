package scenariogen

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/scenario"
	"github.com/nrynss/opsec-control/internal/state"
)

// mockLLM returns a canned JSON array of candidate beats per Compile call,
// advancing through the three acts. No network, fully deterministic.
type mockLLM struct{ calls int }

func (m *mockLLM) Complete(_ context.Context, _ contracts.LLMRequest) (contracts.LLMResponse, error) {
	acts := []string{
		// Act 1 — mainshock: collapses, bridges close, power off.
		`[
			{"type":"MainshockOccurred","confidence":1.0,"payload":{}},
			{"type":"BridgeClosed","confidence":0.95,"payload":{"bridgeId":"B-VORA"}},
			{"type":"BridgeClosed","confidence":0.93,"payload":{"bridgeId":"B-IRON"}},
			{"type":"PowerFailure","confidence":0.9,"payload":{"sector":"S-HIGHGATE"}},
			{"type":"HospitalCapacityChanged","confidence":0.88,"payload":{"hospitalId":"H-CENTRAL","occupancy":480}}
		]`,
		// Act 2 — aftershock + fire: last bridge closes, Ironworks ignites, dam stress.
		`[
			{"type":"BridgeClosed","confidence":0.9,"payload":{"bridgeId":"B-SOUTH"}},
			{"type":"FireIgnited","confidence":0.85,"payload":{"fireZoneId":"FZ-IRON","sector":"S-IRONWORKS"}},
			{"type":"DamStressElevated","confidence":0.8,"payload":{}}
		]`,
		// Act 3 — levee breach + flood + shelters fill.
		`[
			{"type":"LeveeBreached","confidence":0.92,"payload":{}},
			{"type":"FloodExtentUpdated","confidence":0.9,"payload":{"polygons":[{"sector":"S-SOUTHPORT","depthM":1.5,"points":[]}]}},
			{"type":"ShelterOccupancyChanged","confidence":0.87,"payload":{"shelterId":"SH-GREENFIELD-1","occupancy":2000}}
		]`,
	}
	out := "[]"
	if m.calls < len(acts) {
		out = acts[m.calls]
	}
	m.calls++
	return contracts.LLMResponse{Content: out}, nil
}

func TestCompile_ProducesValidatedReplayableScenario(t *testing.T) {
	gen := NewGenerator(&mockLLM{})
	scn, err := gen.Compile(context.Background(), 1729)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if scn.Seed != 1729 {
		t.Errorf("seed not propagated: got %d", scn.Seed)
	}
	if len(scn.Events) < 6 {
		t.Fatalf("expected a multi-act scenario, got %d events", len(scn.Events))
	}

	// Every emitted event must replay cleanly through the §14.2 gatekeeper
	// from the scenario's own Initial state — the core guarantee of the tool.
	st := state.New(scn.Initial)
	prev := contracts.SimTime(-1)
	for _, ev := range scn.Events {
		if ev.Timestamp < prev {
			t.Fatalf("events not monotonic: %d after %d", ev.Timestamp, prev)
		}
		prev = ev.Timestamp
		if _, err := st.Apply(ev); err != nil {
			t.Fatalf("emitted event %s (%s) was rejected on replay: %v", ev.ID, ev.Type, err)
		}
	}

	// The frozen artifact must load through the scenario loader (ordering gate).
	data, err := json.Marshal(scn)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := scenario.LoadJSON(data); err != nil {
		t.Fatalf("scenario.LoadJSON rejected the compiled scenario: %v", err)
	}
}

func TestCompile_DeterministicForSameInputs(t *testing.T) {
	a, err := NewGenerator(&mockLLM{}).Compile(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewGenerator(&mockLLM{}).Compile(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("Compile is not deterministic for identical inputs")
	}
}

func TestDraftEvents_MalformedJSONDropped(t *testing.T) {
	// DraftEvents must not crash on non-JSON LLM output — it returns no events.
	bad := &badLLM{}
	evs, err := NewGenerator(bad).DraftEvents(context.Background(), "anything", 0)
	if err != nil {
		t.Fatalf("DraftEvents should swallow malformed output, got err: %v", err)
	}
	if len(evs) != 0 {
		t.Fatalf("expected 0 events from malformed output, got %d", len(evs))
	}
}

type badLLM struct{}

func (badLLM) Complete(_ context.Context, _ contracts.LLMRequest) (contracts.LLMResponse, error) {
	return contracts.LLMResponse{Content: "not json at all"}, nil
}
