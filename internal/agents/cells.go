package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"

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

// callLLM performs a single LLM call with the given prompts and schema, returning
// the parsed CellOutput and the raw LLMResponse (for metrics).
func (b *cellBase) callLLM(ctx context.Context, systemPrompt, userPrompt string) (contracts.CellOutput, contracts.LLMResponse, error) {
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
		return contracts.CellOutput{}, resp, fmt.Errorf("llm completion failed: %w", err)
	}

	var out contracts.CellOutput
	err = json.Unmarshal([]byte(resp.Content), &out)
	if err != nil {
		return contracts.CellOutput{}, resp, fmt.Errorf("llm output failed schema validation: %w", err)
	}

	if out.Summary == "" || out.RiskLevel == "" {
		return contracts.CellOutput{}, resp, fmt.Errorf("llm output missing required content")
	}

	return out, resp, nil
}

// shouldCritique returns true if the LLM_CRITIQUE env var is set to a truthy value.
// Off by default (for tests and to keep single-turn unless explicitly enabled).
func (b *cellBase) shouldCritique() bool {
	switch os.Getenv("LLM_CRITIQUE") {
	case "1", "true", "TRUE", "yes", "YES":
		return true
	}
	return false
}

// aggregateMetrics sums tokens and latency across turns. Computes an
// aggregate tokens/sec from total output tokens over total latency.
func aggregateMetrics(a, b contracts.CellMetrics) contracts.CellMetrics {
	res := contracts.CellMetrics{
		TokensIn:  a.TokensIn + b.TokensIn,
		TokensOut: a.TokensOut + b.TokensOut,
		LatencyMS: a.LatencyMS + b.LatencyMS,
	}
	if res.LatencyMS > 0 {
		res.TokensPerSec = float64(res.TokensOut) / (float64(res.LatencyMS) / 1000.0)
	} else {
		// If both turns had 0 latency, tokensPerSec is technically infinite or undefined.
		// We default to the best available rate to avoid 0.0 or NaN.
		res.TokensPerSec = math.Max(a.TokensPerSec, b.TokensPerSec)
	}
	return res
}

// executeLLM performs the (plan) call and optionally a sequential critique
// pass (when LLM_CRITIQUE env is set). Metrics are populated from the LLM
// response(s) and aggregated for multi-turn. On critique failure, gracefully
// falls back to the initial draft.
func (b *cellBase) executeLLM(ctx context.Context, systemPrompt, userPrompt string) (contracts.CellOutput, error) {
	out, resp, err := b.callLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	metrics := contracts.CellMetrics{
		TokensIn:     resp.TokensIn,
		TokensOut:    resp.TokensOut,
		TokensPerSec: resp.TokensPerSec,
		LatencyMS:    resp.LatencyMS,
	}

	if b.shouldCritique() {
		// Use the Pure type to strip metrics for the prompt
		draft := contracts.CellOutputPure{
			Cell:            out.Cell,
			Summary:         out.Summary,
			RiskLevel:       out.RiskLevel,
			Confidence:      out.Confidence,
			StateVersion:    out.StateVersion,
			Recommendations: out.Recommendations,
			Evidence:        out.Evidence,
		}
		draftJSON, marshalErr := json.Marshal(draft)
		if marshalErr != nil {
			out.Metrics = metrics
			return out, nil
		}

		critiqueUser := fmt.Sprintf(`Original task:
%s

Initial draft:
%s

Critique the draft for accuracy (vs provided data), completeness, actionability, and schema compliance.
Output ONLY one refined JSON object in the exact same schema. No extra text or markdown.`, userPrompt, string(draftJSON))

		refined, refinedResp, cerr := b.callLLM(ctx, systemPrompt, critiqueUser)
		if cerr != nil {
			out.Metrics = metrics
			return out, nil
		}

		refinedMetrics := contracts.CellMetrics{
			TokensIn:     refinedResp.TokensIn,
			TokensOut:    refinedResp.TokensOut,
			TokensPerSec: refinedResp.TokensPerSec,
			LatencyMS:    refinedResp.LatencyMS,
		}
		refined.Metrics = aggregateMetrics(metrics, refinedMetrics)
		return refined, nil
	}

	out.Metrics = metrics
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

// --- Intelligence Cell ---

type intelligenceCell struct {
	cellBase
}

func NewIntelligence(llm contracts.LLMClient) contracts.Cell {
	return &intelligenceCell{
		cellBase: cellBase{kind: contracts.CellIntelligence, llm: llm},
	}
}

func (c *intelligenceCell) Analyze(ctx context.Context, in contracts.CellInput) (contracts.CellOutput, error) {
	system := "You are the Intelligence Cell. Analyze weather, seismic events, flood modelling, satellite/drone imagery, damage assessment, and hazard prediction. Provide clear situational awareness and forecasts."
	user := fmt.Sprintf("Sectors: %+v\nBridges: %+v\nDam: %+v\nLevee: %+v\nFlood: %+v\nTrigger Event: %+v",
		in.Snapshot.Sectors, in.Snapshot.Bridges, in.Snapshot.Dam, in.Snapshot.Levee, in.Snapshot.Flood, in.Trigger)

	out, err := c.executeLLM(ctx, system, user)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	out.Cell = c.Kind()
	out.StateVersion = in.StateVersion
	return out, nil
}

// --- Communications Cell ---

type communicationsCell struct {
	cellBase
}

func NewCommunications(llm contracts.LLMClient) contracts.Cell {
	return &communicationsCell{
		cellBase: cellBase{kind: contracts.CellCommunications, llm: llm},
	}
}

func (c *communicationsCell) Analyze(ctx context.Context, in contracts.CellInput) (contracts.CellOutput, error) {
	system := "You are the Communications Cell. Synthesize world state and all specialist reports into concise public advisories, internal briefings, and situation summaries."
	user := fmt.Sprintf("Key State (Dam/Levee/Flood/Sectors): %+v\nSpecialist Reports: %+v\nTrigger Event: %+v",
		struct {
			Dam     contracts.Dam
			Levee   contracts.Levee
			Flood   contracts.Flood
			Sectors map[contracts.SectorID]contracts.Sector
		}{in.Snapshot.Dam, in.Snapshot.Levee, in.Snapshot.Flood, in.Snapshot.Sectors}, in.Peers, in.Trigger)

	out, err := c.executeLLM(ctx, system, user)
	if err != nil {
		return contracts.CellOutput{}, err
	}

	out.Cell = c.Kind()
	out.StateVersion = in.StateVersion
	return out, nil
}
