package agents

import (
	"context"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// mockLLM implements contracts.LLMClient for testing.
type mockLLM struct {
	response contracts.LLMResponse
	err      error
}

func (m *mockLLM) Complete(ctx context.Context, req contracts.LLMRequest) (contracts.LLMResponse, error) {
	if m.err != nil {
		return contracts.LLMResponse{}, m.err
	}
	return m.response, nil
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
				response: contracts.LLMResponse{
					Content: validJSON,
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
		response: contracts.LLMResponse{
			Content: `{"invalid": "json"}`,
		},
	}

	cell := NewInfrastructure(mock)
	in := contracts.CellInput{StateVersion: 1}
	_, err := cell.Analyze(context.Background(), in)

	if err == nil {
		t.Error("expected error for malformed LLM response, got nil")
	}
}
