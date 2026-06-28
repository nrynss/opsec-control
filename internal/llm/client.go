package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Client implements contracts.LLMClient using the Cerebras API.
type Client struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// Config holds configuration parameters for the Cerebras client.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// NewClient creates a new Cerebras LLM client.
// It loads settings from environment variables if Config fields are empty:
// - CEREBRAS_API_KEY
// - CEREBRAS_BASE_URL (defaults to https://api.cerebras.ai/v1)
// - CEREBRAS_MODEL (defaults to gemma-4-31b)
func NewClient(cfg Config) *Client {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("CEREBRAS_API_KEY")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("CEREBRAS_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.cerebras.ai/v1"
	}

	model := cfg.Model
	if model == "" {
		model = os.Getenv("CEREBRAS_MODEL")
	}
	if model == "" {
		model = "gemma-4-31b"
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 45 * time.Second,
		}
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  httpClient,
	}
}

// Complete executes a prompt completion against Cerebras (or runs Mock Mode if no key is present or LLM_MOCK=true).
func (c *Client) Complete(ctx context.Context, req contracts.LLMRequest) (contracts.LLMResponse, error) {
	// Trigger Mock Mode if requested or if no API key is configured
	if os.Getenv("LLM_MOCK") == "true" || c.apiKey == "" {
		return c.completeMock(req)
	}

	return c.completeReal(ctx, req)
}

// ensureAdditionalPropertiesFalse recursively adds "additionalProperties": false to all object definitions
// in a JSON schema. Cerebras requires this parameter to be strictly set for all objects in response_format.
func ensureAdditionalPropertiesFalse(raw []byte) []byte {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return raw
	}
	data = setAdditionalProperties(data)
	if res, err := json.Marshal(data); err == nil {
		return res
	}
	return raw
}

func setAdditionalProperties(val any) any {
	m, ok := val.(map[string]any)
	if !ok {
		if arr, ok := val.([]any); ok {
			for i, v := range arr {
				arr[i] = setAdditionalProperties(v)
			}
			return arr
		}
		return val
	}
	if t, ok := m["type"].(string); ok && t == "object" {
		m["additionalProperties"] = false
	}
	for k, v := range m {
		m[k] = setAdditionalProperties(v)
	}
	return m
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormatSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type responseFormat struct {
	Type       string                `json:"type"`
	JSONSchema *responseFormatSchema `json:"json_schema,omitempty"`
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	TimeInfo struct {
		CompletionTime float64 `json:"completion_time"`
		PromptTime     float64 `json:"prompt_time"`
		QueueTime      float64 `json:"queue_time"`
		TotalTime      float64 `json:"total_time"`
	} `json:"time_info"`
}

func (c *Client) completeReal(ctx context.Context, req contracts.LLMRequest) (contracts.LLMResponse, error) {
	messages := make([]chatMessage, 0, 2)
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.User})

	apiReqPayload := chatCompletionRequest{
		Model:    c.model,
		Messages: messages,
	}

	if len(req.Schema) > 0 {
		cleanedSchema := ensureAdditionalPropertiesFalse(req.Schema)
		apiReqPayload.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &responseFormatSchema{
				Name:   "structured_output",
				Strict: true,
				Schema: cleanedSchema,
			},
		}
	}

	reqBytes, err := json.Marshal(apiReqPayload)
	if err != nil {
		return contracts.LLMResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(c.baseURL, "/"))
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return contracts.LLMResponse{}, fmt.Errorf("create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	startTime := time.Now()
	httpResp, err := c.client.Do(httpReq)
	duration := time.Since(startTime)
	if err != nil {
		return contracts.LLMResponse{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return contracts.LLMResponse{}, fmt.Errorf("read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return contracts.LLMResponse{}, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(bodyBytes))
	}

	var apiResp chatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return contracts.LLMResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return contracts.LLMResponse{}, errors.New("empty choices in API response")
	}

	completionContent := apiResp.Choices[0].Message.Content
	tokensIn := apiResp.Usage.PromptTokens
	tokensOut := apiResp.Usage.CompletionTokens

	// Calculate tokens per second. Use time_info.completion_time from Cerebras API if present.
	// Fall back to wall-clock duration if completion_time is missing or 0.
	var tokensPerSec float64
	if apiResp.TimeInfo.CompletionTime > 0 {
		tokensPerSec = float64(tokensOut) / apiResp.TimeInfo.CompletionTime
	} else if duration.Seconds() > 0 {
		tokensPerSec = float64(tokensOut) / duration.Seconds()
	}

	return contracts.LLMResponse{
		Content:      completionContent,
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		TokensPerSec: tokensPerSec,
	}, nil
}

// completeMock simulates the Cerebras completion API by returning high-fidelity mock JSON responses
// tailored to the respective EOC specialist and commander cells.
func (c *Client) completeMock(req contracts.LLMRequest) (contracts.LLMResponse, error) {
	// Parse stateVersion if passed in user prompt to stay in lockstep
	stateVersion := uint64(1)
	versionRegex := regexp.MustCompile(`"stateVersion":\s*(\d+)`)
	if matches := versionRegex.FindStringSubmatch(req.User); len(matches) > 1 {
		if val, err := strconv.ParseUint(matches[1], 10, 64); err == nil {
			stateVersion = val
		}
	}

	content := ""
	cell := strings.ToLower(req.System + req.User)

	switch {
	case strings.Contains(cell, "commander"):
		content = fmt.Sprintf(`{
			"summary": "Cerebro earthquake cascade. Two bridges closed, Highgate heavily damaged, Central General hospital at critical capacity.",
			"stateVersion": %d,
			"overallRisk": "Critical",
			"prioritizedActions": [
				{"priority": 1, "action": "Airlift critical casualties from Westbank Clinic", "owner": "Medical"},
				{"priority": 2, "action": "Deploy USAR search teams to Highgate", "owner": "Population"},
				{"priority": 3, "action": "Inspect Vora and Iron bridges for structural integrity", "owner": "Infrastructure"}
			],
			"cellOutputs": [
				{
					"agent": "Infrastructure",
					"summary": "Vora Bridge and Iron Bridge closed due to structural damage.",
					"riskLevel": "High",
					"confidence": 0.95,
					"stateVersion": %d,
					"recommendations": ["Assess Vora Bridge", "Establish detours via South Span"],
					"evidence": ["Bridge structural sensors showing deflection"]
				},
				{
					"agent": "Medical",
					"summary": "Casualty surge at Central General. Westbank Clinic cut off.",
					"riskLevel": "Critical",
					"confidence": 0.90,
					"stateVersion": %d,
					"recommendations": ["Airlift patients from Westbank Clinic"],
					"evidence": ["Hospital ER occupancy above 90%%"]
				}
			]
		}`, stateVersion, stateVersion, stateVersion)

	case strings.Contains(cell, "infrastructure"):
		content = fmt.Sprintf(`{
			"agent": "Infrastructure",
			"summary": "Vora Bridge and Iron Bridge closed due to structural damage. Highgate masonry collapses blocking roads.",
			"riskLevel": "High",
			"confidence": 0.95,
			"stateVersion": %d,
			"recommendations": [
				"Initiate structural assessment of Vora Bridge",
				"Establish detours via South Span",
				"Clear arterial blockages in Highgate sector"
			],
			"evidence": [
				"Bridge structural sensors showing deflection",
				"Citizen reports of arterial blockage on main boulevard"
			]
		}`, stateVersion)

	case strings.Contains(cell, "medical"):
		content = fmt.Sprintf(`{
			"agent": "Medical",
			"summary": "Casualty surge at Central General. Westbank Clinic cut off from transport loop, occupancy straining.",
			"riskLevel": "Critical",
			"confidence": 0.92,
			"stateVersion": %d,
			"recommendations": [
				"Airlift critical patients from Westbank Clinic",
				"Establish triage tent at Greenfield evacuation center"
			],
			"evidence": [
				"Central General Hospital ER occupancy at 92%%",
				"Vora Bridge closure prevents standard ambulance transport across river"
			]
		}`, stateVersion)

	case strings.Contains(cell, "population"):
		content = fmt.Sprintf(`{
			"agent": "Population",
			"summary": "Trapped citizens reported in Highgate old masonry area. Greenfield shelter occupancy approaching capacity.",
			"riskLevel": "High",
			"confidence": 0.94,
			"stateVersion": %d,
			"recommendations": [
				"Deploy Urban Search & Rescue (USAR) teams to Highgate sector",
				"Redirect evacuees to secondary Greenfield shelters"
			],
			"evidence": [
				"12 active citizen distress calls in Highgate",
				"Greenfield Arena shelter occupancy at 95%%"
			]
		}`, stateVersion)

	case strings.Contains(cell, "intelligence"):
		content = fmt.Sprintf(`{
			"agent": "Intelligence",
			"summary": "Aftershock hazard remains elevated. Mainor Dam reporting elevated stress telemetry. Power grid failed in Highgate and Central.",
			"riskLevel": "Medium",
			"confidence": 0.88,
			"stateVersion": %d,
			"recommendations": [
				"Monitor Mainor Dam stress telemetry continuously",
				"Prepare backup power generators for key infrastructure nodes"
			],
			"evidence": [
				"Seismic sensors reporting micro-shocks",
				"Power substation offline indicators"
			]
		}`, stateVersion)

	case strings.Contains(cell, "communications"):
		content = fmt.Sprintf(`{
			"agent": "Communications",
			"summary": "Comms outage across Highgate and Southport. Emergency broadcast system online.",
			"riskLevel": "Medium",
			"confidence": 0.91,
			"stateVersion": %d,
			"recommendations": [
				"Enable mesh network protocol in Southport sector",
				"Publish alert notifications via backup radio channel"
			],
			"evidence": [
				"Cell tower signal dropouts",
				"Emergency broadcast transmission confirmation"
			]
		}`, stateVersion)

	default:
		// Generic JSON output conforming to CellOutput as fallback
		content = fmt.Sprintf(`{
			"agent": "Intelligence",
			"summary": "General scenario update processed.",
			"riskLevel": "Low",
			"confidence": 0.85,
			"stateVersion": %d,
			"recommendations": ["Continue standard operations"],
			"evidence": ["System tick received"]
		}`, stateVersion)
	}

	// Compute simulated token counts (approx. 4 chars per token)
	tokensIn := (len(req.System) + len(req.User)) / 4
	tokensOut := len(content) / 4

	// Mock high performance wafer-scale inference rate (SPEC: ~1,500 tokens/sec)
	tokensPerSec := 1500.0

	// Simulating duration based on tokens generated at 1500 tokens/sec
	simulatedDuration := time.Duration(float64(tokensOut)/tokensPerSec*1000) * time.Millisecond
	time.Sleep(simulatedDuration)

	return contracts.LLMResponse{
		Content:      content,
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		TokensPerSec: tokensPerSec,
	}, nil
}
