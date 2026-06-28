package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// cellBase provides common functionality for all Cells.
type cellBase struct {
	kind contracts.CellKind
	llm  contracts.LLMClient
}

func (b *cellBase) Kind() contracts.CellKind {
	return b.kind
}

func (b *cellBase) executeLLM(ctx context.Context, systemPrompt, userPrompt string) (contracts.CellOutput, error) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"summary": { "type": "string" },
			"riskLevel": { "type": "string", "enum": ["Low", "Medium", "High", "Critical"] },
			"confidence": { "type": "number", "minimum": 0, "maximum": 1 },
			"stateVersion": { "type": "integer" },
			"recommendations": { "type": "array", "items": { "type": "string" } },
			"evidence": { "type": "array", "items": { "type": "string" } }
		},
		"required": ["summary", "riskLevel", "confidence", "stateVersion", "recommendations", "evidence"]
	}`)

	resp, err := b.llm.Complete(ctx, contracts.LLMRequest{
		System: systemPrompt,
		User:   userPrompt,
		Schema: schema,
	})
	if err != nil {
		return contracts.CellOutput{}, fmt.Errorf("llm completion failed: %w", err)
	}

	var out contracts.CellOutput
	err = json.Unmarshal([]byte(resp.Content), &out)
	if err != nil {
		return contracts.CellOutput{}, fmt.Errorf("llm output failed schema validation: %w", err)
	}

	if out.Summary == "" || out.RiskLevel == "" {
		return contracts.CellOutput{}, fmt.Errorf("llm output missing required content")
	}

	return out, nil
}

// --- Infrastructure Cell ---

type infrastructureCell struct {
	cellBase
}

func NewInfrastructure(llm contracts.LLMClient) contracts.Cell {
	return &infrastructureCell{
		cellBase: cellBase{kind: contracts.CellInfrastructure, llm: llm},
	}
}

func (c *infrastructureCell) Analyze(ctx context.Context, in contracts.CellInput) (contracts.CellOutput, error) {
	system := "You are the Infrastructure Cell. Analyze roads, bridges, utilities, and logistics. Identify bottlenecks and structural risks."
	user := fmt.Sprintf("Bridges: %+v\nSectors (Power/Gas/Water): %+v\nDam: %+v\nLevee: %+v\nTrigger Event: %+v",
		in.Snapshot.Bridges, in.Snapshot.Sectors, in.Snapshot.Dam, in.Snapshot.Levee, in.Trigger)

	out, err := c.executeLLM(ctx, system, user)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	out.Cell = c.Kind()
	out.StateVersion = in.StateVersion
	return out, nil
}

// --- Medical Cell ---

type medicalCell struct {
	cellBase
}

func NewMedical(llm contracts.LLMClient) contracts.Cell {
	return &medicalCell{
		cellBase: cellBase{kind: contracts.CellMedical, llm: llm},
	}
}

func (c *medicalCell) Analyze(ctx context.Context, in contracts.CellInput) (contracts.CellOutput, error) {
	system := "You are the Medical Cell. Analyze hospital capacity, casualties, and medical logistics. Prioritize life-saving interventions."
	user := fmt.Sprintf("Hospitals: %+v\nResources (Ambulances/Helicopters): %+v\nTrigger Event: %+v",
		in.Snapshot.Hospitals, in.Snapshot.Resources, in.Trigger)

	out, err := c.executeLLM(ctx, system, user)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	out.Cell = c.Kind()
	out.StateVersion = in.StateVersion
	return out, nil
}

// --- Population Cell ---

type populationCell struct {
	cellBase
}

func NewPopulation(llm contracts.LLMClient) contracts.Cell {
	return &populationCell{
		cellBase: cellBase{kind: contracts.CellPopulation, llm: llm},
	}
}

func (c *populationCell) Analyze(ctx context.Context, in contracts.CellInput) (contracts.CellOutput, error) {
	system := "You are the Population Cell. Analyze shelters, evacuations, and population movement. Focus on citizen safety and transit."
	user := fmt.Sprintf("Shelters: %+v\nFlood Extent: %+v\nSectors (Population): %+v\nTrigger Event: %+v",
		in.Snapshot.Shelters, in.Snapshot.Flood, in.Snapshot.Sectors, in.Trigger)

	out, err := c.executeLLM(ctx, system, user)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	out.Cell = c.Kind()
	out.StateVersion = in.StateVersion
	return out, nil
}

// --- Commander Cell ---

type commanderCell struct {
	cellBase
}

func NewCommander(llm contracts.LLMClient) contracts.Cell {
	return &commanderCell{
		cellBase: cellBase{kind: contracts.CellCommander, llm: llm},
	}
}

func (c *commanderCell) Analyze(ctx context.Context, in contracts.CellInput) (contracts.CellOutput, error) {
	system := "You are the Commander Cell. Synthesize world state and specialist reports into a Common Operational Picture (COP). Prioritize resources and define objectives."
	user := fmt.Sprintf("Critical State (Dam/Levee/Flood): %+v\nSpecialist Reports: %+v\nTrigger Event: %+v",
		struct {
			Dam   contracts.Dam
			Levee contracts.Levee
			Flood contracts.Flood
		}{in.Snapshot.Dam, in.Snapshot.Levee, in.Snapshot.Flood}, in.Peers, in.Trigger)

	out, err := c.executeLLM(ctx, system, user)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	out.Cell = c.Kind()
	out.StateVersion = in.StateVersion
	return out, nil
}
