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

// executeLLM handles the plan -> self-critique -> refine loop (SPEC §9)
// and enforces the schema for structured output.
func (b *cellBase) executeLLM(ctx context.Context, systemPrompt, userPrompt string) (contracts.CellOutput, error) {
	// Schema for CellOutput to force the LLM into valid JSON.
	// In a real implementation, this would be a pre-defined JSON schema object.
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

	// Loop for: Plan -> Critique -> Refine (Limited to 2 turns for latency)
	var lastResponse string
	var currentOutput contracts.CellOutput
	var err error

	for turn := 0; turn < 2; turn++ {
		userMsg := userPrompt
		if turn > 0 {
			userMsg = fmt.Sprintf("Critique and refine the previous analysis for better accuracy and specific recommendations:\n\n%s", lastResponse)
		}

		resp, err := b.llm.Complete(ctx, contracts.LLMRequest{
			System: systemPrompt,
			User:   userMsg,
			Schema: schema,
		})
		if err != nil {
			return contracts.CellOutput{}, fmt.Errorf("llm completion failed: %w", err)
		}

		lastResponse = resp.Content
		err = json.Unmarshal([]byte(resp.Content), &currentOutput)
		if err != nil {
			// If the LLM fails the schema on turn 0, we try one more time with the refine loop.
			// If it fails on turn 1, we return an error.
			if turn == 1 {
				return contracts.CellOutput{}, fmt.Errorf("llm output failed schema validation: %w", err)
			}
			continue
		}

		// Post-unmarshal: verify required fields are actually present/non-zero
		// if the LLM returned a valid JSON object but missed crucial fields.
		if currentOutput.Summary == "" || currentOutput.RiskLevel == "" {
			if turn == 1 {
				return contracts.CellOutput{}, fmt.Errorf("llm output missing required content")
			}
			continue
		}

		// If unmarshaled successfully, we have a valid output.
		// For the MVD, we can return the first valid one or continue to refine.
		// Let's refine once if we have time.
		if turn == 0 {
			continue
		}
		break
	}

	return currentOutput, err
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
	user := fmt.Sprintf("World State: %+v\nTrigger Event: %+v", in.Snapshot, in.Trigger)

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
	user := fmt.Sprintf("World State: %+v\nTrigger Event: %+v", in.Snapshot, in.Trigger)

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
	user := fmt.Sprintf("World State: %+v\nTrigger Event: %+v", in.Snapshot, in.Trigger)

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
	user := fmt.Sprintf("World State: %+v\nTrigger Event: %+v\nSpecialist Reports: %+v", in.Snapshot, in.Trigger, in.Peers)

	out, err := c.executeLLM(ctx, system, user)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	out.Cell = c.Kind()
	out.StateVersion = in.StateVersion
	return out, nil
}
