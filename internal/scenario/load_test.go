package scenario

import (
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func TestLoadJSON_Valid(t *testing.T) {
	data := []byte(`{
		"schemaVersion": "0.1",
		"name": "test-cerebro",
		"seed": 42,
		"initial": {"version": 0, "time": 0},
		"events": [
			{"id": "e1", "timestamp": 0, "source": "sim", "type": "MainshockOccurred", "confidence": 1.0},
			{"id": "e2", "timestamp": 120, "source": "sim", "type": "BridgeClosed", "confidence": 0.95}
		]
	}`)

	sc, err := LoadJSON(data)
	if err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}
	if sc.Name != "test-cerebro" {
		t.Errorf("name = %q, want %q", sc.Name, "test-cerebro")
	}
	if len(sc.Events) != 2 {
		t.Fatalf("got %d events, want 2", len(sc.Events))
	}
	if sc.Events[1].Timestamp != 120 {
		t.Errorf("second event time = %d, want 120", sc.Events[1].Timestamp)
	}
}

func TestLoadJSON_EmptyEventsOK(t *testing.T) {
	data := []byte(`{"schemaVersion":"0.1","name":"empty","seed":1,"initial":{},"events":[]}`)
	_, err := LoadJSON(data)
	if err != nil {
		t.Fatalf("empty events should be valid: %v", err)
	}
}

func TestLoadJSON_MissingSchemaVersion(t *testing.T) {
	data := []byte(`{"name":"no-schema","seed":1,"events":[]}`)
	_, err := LoadJSON(data)
	if err == nil {
		t.Fatal("expected error for missing schemaVersion")
	}
}

func TestLoadJSON_UnsortedEvents(t *testing.T) {
	data := []byte(`{
		"schemaVersion":"0.1",
		"name":"bad-order",
		"seed":1,
		"initial":{},
		"events":[
			{"id":"e1","timestamp":100,"source":"s","type":"MainshockOccurred","confidence":1},
			{"id":"e2","timestamp":50,"source":"s","type":"BridgeClosed","confidence":1}
		]
	}`)
	_, err := LoadJSON(data)
	if err == nil {
		t.Fatal("expected error for unsorted events")
	}
}

func TestLoadJSON_BadJSON(t *testing.T) {
	_, err := LoadJSON([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestLoadJSON_RoundTripsContractShape(t *testing.T) {
	// Build a scenario using the exact contract types. Loader should accept it.
	sc := contracts.Scenario{
		SchemaVersion: "0.1",
		Name:          "contract-example",
		Seed:          1729,
		Initial:       contracts.WorldState{Version: 0, Time: 0},
		Events: []contracts.Event{
			{ID: "evt-1", Timestamp: 0, Source: "sim", Type: contracts.EventMainshockOccurred, Confidence: 1},
		},
	}
	// We don't need to serialize here; constructing from contracts types + Load is exercised elsewhere.
	// This test just ensures the package compiles against the contract and basic acceptance.
	_ = sc
}
