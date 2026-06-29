package agents

import (
	"context"
	"fmt"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// mockLLM implements contracts.LLMClient for testing.
// Supports a sequence of responses for multi-turn (e.g. critique) scenarios.
type mockLLM struct {
	responses []contracts.LLMResponse
	errs      []error
	callCount int
}

func (m *mockLLM) Complete(ctx context.Context, req contracts.LLMRequest) (contracts.LLMResponse, error) {
	m.callCount++
	idx := m.callCount - 1
	if idx < len(m.errs) && m.errs[idx] != nil {
		return contracts.LLMResponse{}, m.errs[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	// fallback to last response or empty
	if len(m.responses) > 0 {
		return m.responses[len(m.responses)-1], nil
	}
	return contracts.LLMResponse{}, nil
}

func TestCells(t *testing.T) {
	tests := []struct {
		name string
		kind contracts.CellKind
		cell func(contracts.LLMClient) contracts.Cell
	}{
		{"Infrastructure", contracts.CellInfrastructure, NewInfrastructure},
		{"Medical", contracts.CellMedical, NewMedical},
		{"Population", contracts.CellPopulation, NewPopulation},
		{"Commander", contracts.CellCommander, NewCommander},
		{"Intelligence", contracts.CellIntelligence, NewIntelligence},
		{"Communications", contracts.CellCommunications, NewCommunications},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare valid JSON response matching CellOutput
			validJSON := `{
				"summary": "Test summary",
				"riskLevel": "High",
				"confidence": 0.9,
				"stateVersion": 1,
				"recommendations": ["Rec 1"],
				"evidence": ["Ev 1"]
			}`
			mock := &mockLLM{
				responses: []contracts.LLMResponse{
					{Content: validJSON},
				},
			}

			cell := tt.cell(mock)
			if cell.Kind() != tt.kind {
				t.Errorf("expected kind %s, got %s", tt.kind, cell.Kind())
			}

			in := contracts.CellInput{
				StateVersion: 1,
				Snapshot:     contracts.WorldState{},
				Trigger:      contracts.Event{},
			}

			out, err := cell.Analyze(context.Background(), in)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if out.Cell != tt.kind {
				t.Errorf("output cell %s does not match kind %s", out.Cell, tt.kind)
			}
			if out.StateVersion != in.StateVersion {
				t.Errorf("expected state version %d, got %d", in.StateVersion, out.StateVersion)
			}
			if out.Summary == "" || out.RiskLevel == "" {
				t.Errorf("output fields are empty: %+v", out)
			}
		})
	}
}

func TestMalformedLLMResponse(t *testing.T) {
	mock := &mockLLM{
		responses: []contracts.LLMResponse{
			{Content: `{"invalid": "json"}`},
		},
	}

	cell := NewInfrastructure(mock)
	in := contracts.CellInput{StateVersion: 1}
	_, err := cell.Analyze(context.Background(), in)

	if err == nil {
		t.Error("expected error for malformed LLM response, got nil")
	}
}

// TestCellMetrics verifies that executeLLM populates CellOutput.Metrics from the
// LLMResponse (this is the P3 requirement for real telemetry).
func TestCellMetrics(t *testing.T) {
	validJSON := `{"summary":"ok","riskLevel":"High","confidence":0.9,"stateVersion":42,"recommendations":["do it"],"evidence":["data"]}`
	mock := &mockLLM{
		responses: []contracts.LLMResponse{{
			Content:      validJSON,
			TokensIn:     12,
			TokensOut:    34,
			TokensPerSec: 567.8,
			LatencyMS:    90,
		}},
	}

	cell := NewIntelligence(mock)
	out, err := cell.Analyze(context.Background(), contracts.CellInput{StateVersion: 42})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if out.Metrics.TokensIn != 12 || out.Metrics.TokensOut != 34 {
		t.Errorf("unexpected tokens: %+v", out.Metrics)
	}
	if out.Metrics.TokensPerSec != 567.8 || out.Metrics.LatencyMS != 90 {
		t.Errorf("unexpected rate/latency: %+v", out.Metrics)
	}
}

// TestCritiquePath exercises the env-gated sequential critique behavior.
func TestCritiquePath(t *testing.T) {
	draftJSON := `{"summary":"draft","riskLevel":"Medium","confidence":0.6,"stateVersion":1,"recommendations":["r1"],"evidence":["e1"]}`
	refinedJSON := `{"summary":"refined better","riskLevel":"High","confidence":0.95,"stateVersion":1,"recommendations":["r1","r2"],"evidence":["e1","e2"]}`

	t.Run("critique disabled by default", func(t *testing.T) {
		mock := &mockLLM{
			responses: []contracts.LLMResponse{{Content: draftJSON, TokensIn: 5, TokensOut: 10, LatencyMS: 100}},
		}
		cell := NewCommunications(mock)
		_, err := cell.Analyze(context.Background(), contracts.CellInput{StateVersion: 1})
		if err != nil {
			t.Fatal(err)
		}
		if mock.callCount != 1 {
			t.Errorf("expected 1 LLM call without critique env, got %d", mock.callCount)
		}
	})

	t.Run("critique enabled aggregates metrics", func(t *testing.T) {
		t.Setenv("LLM_CRITIQUE", "1")
		mock := &mockLLM{
			responses: []contracts.LLMResponse{
				{Content: draftJSON, TokensIn: 5, TokensOut: 10, LatencyMS: 100},
				{Content: refinedJSON, TokensIn: 3, TokensOut: 7, LatencyMS: 40},
			},
		}
		cell := NewIntelligence(mock)
		out, err := cell.Analyze(context.Background(), contracts.CellInput{StateVersion: 1})
		if err != nil {
			t.Fatal(err)
		}
		if mock.callCount != 2 {
			t.Errorf("expected 2 calls with critique, got %d", mock.callCount)
		}
		// aggregated
		if out.Metrics.TokensIn != 8 || out.Metrics.TokensOut != 17 {
			t.Errorf("bad token aggregation: %+v", out.Metrics)
		}
		if out.Metrics.LatencyMS != 140 {
			t.Errorf("bad latency aggregation: %+v", out.Metrics)
		}
		// derived rate
		expectedRate := float64(17) / (140.0 / 1000.0)
		if out.Metrics.TokensPerSec < expectedRate-0.1 || out.Metrics.TokensPerSec > expectedRate+0.1 {
			t.Errorf("unexpected aggregate rate: %f", out.Metrics.TokensPerSec)
		}
		// refined content used
		if out.Summary != "refined better" {
			t.Errorf("expected refined summary, got %q", out.Summary)
		}
	})

	t.Run("critique failure falls back gracefully", func(t *testing.T) {
		t.Setenv("LLM_CRITIQUE", "1")
		mock := &mockLLM{
			responses: []contracts.LLMResponse{
				{Content: draftJSON, TokensIn: 5, TokensOut: 10, LatencyMS: 100},
			},
			errs: []error{nil, fmt.Errorf("critique llm failed")},
		}
		cell := NewPopulation(mock)
		out, err := cell.Analyze(context.Background(), contracts.CellInput{StateVersion: 1})
		if err != nil {
			t.Fatal(err)
		}
		if mock.callCount != 2 {
			t.Errorf("expected 2 calls (first success, second fail), got %d", mock.callCount)
		}
		// should have fallen back to draft content + first metrics
		if out.Summary != "draft" {
			t.Errorf("expected fallback to draft summary, got %q", out.Summary)
		}
		if out.Metrics.TokensIn != 5 {
			t.Errorf("expected original metrics on fallback, got %+v", out.Metrics)
		}
	})
}
