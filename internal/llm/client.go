package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Client implements contracts.LLMClient using the Cerebras API.
type Client struct {
	apiKey         string
	baseURL        string
	model          string
	client         *http.Client
	maxRetries     int
	backoff        time.Duration
	maxConcurrency int
	sem            chan struct{}
	rand           *rand.Rand
	randMu         sync.Mutex
}

// Config holds configuration parameters for the Cerebras client.
type Config struct {
	APIKey         string
	BaseURL        string
	Model          string
	HTTPClient     *http.Client
	MaxRetries     int
	Backoff        time.Duration
	MaxConcurrency int
	// Seed seeds the backoff-jitter PRNG. Determinism is law (SPEC §0.2 r5):
	// the client never reads the wall clock to seed rand. When 0, a fixed
	// default seed is used; callers wanting reproducible-yet-varied jitter can
	// derive this from the scenario seed.
	Seed int64
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

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	backoff := cfg.Backoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}

	maxConcurrency := cfg.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 4 // Measured concurrency ceiling
	}

	sem := make(chan struct{}, maxConcurrency)

	// Determinism (SPEC §0.2 r5): seed from Config, never the wall clock. A
	// fixed default keeps backoff jitter reproducible across runs; jitter only
	// affects retry timing, never state or event output.
	seed := cfg.Seed
	if seed == 0 {
		seed = 1
	}
	r := rand.New(rand.NewSource(seed))

	return &Client{
		apiKey:         apiKey,
		baseURL:        baseURL,
		model:          model,
		client:         httpClient,
		maxRetries:     maxRetries,
		backoff:        backoff,
		maxConcurrency: maxConcurrency,
		sem:            sem,
		rand:           r,
	}
}

// Complete executes a prompt completion against Cerebras (or runs Mock Mode if no key is present or LLM_MOCK=true).
func (c *Client) Complete(ctx context.Context, req contracts.LLMRequest) (contracts.LLMResponse, error) {
	// Trigger Mock Mode if requested or if no API key is configured
	if os.Getenv("LLM_MOCK") == "true" || c.apiKey == "" {
		return c.completeMock(ctx, req)
	}

	return c.completeReal(ctx, req)
}

// ensureAdditionalPropertiesFalse recursively adds "additionalProperties": false to all object definitions
// in a JSON schema. Cerebras requires this parameter to be strictly set for all objects in response_format.
//
// Note: Unmarshaling the raw bytes into a fresh 'any' structure creates a completely separate in-memory
// copy of the JSON schema, so mutating 'data' does not side-effect the caller's raw slice or any other shared memory.
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

func (c *Client) getBackoff(attempt int, retryAfterHeader string) time.Duration {
	if retryAfterHeader != "" {
		if d, ok := parseRetryAfter(retryAfterHeader, time.Now()); ok {
			return d
		}
	}

	// Exponential backoff: BaseBackoff * 2^attempt
	temp := float64(c.backoff) * math.Pow(2, float64(attempt))
	maxBackoff := 10 * time.Second
	if temp > float64(maxBackoff) {
		temp = float64(maxBackoff)
	}

	c.randMu.Lock()
	defer c.randMu.Unlock()

	// Jitter: +/- 50%
	half := temp / 2
	jitter := c.rand.Float64() * half
	return time.Duration(half + jitter)
}

func parseRetryAfter(header string, now time.Time) (time.Duration, bool) {
	if header == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := t.Sub(now); d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
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

	var httpResp *http.Response
	var bodyBytes []byte
	var duration time.Duration

	totalAttempts := c.maxRetries + 1

	for attempt := range totalAttempts {
		if err := ctx.Err(); err != nil {
			return contracts.LLMResponse{}, err
		}

		// Acquire concurrency semaphore with context
		select {
		case c.sem <- struct{}{}:
			// Acquired
		case <-ctx.Done():
			return contracts.LLMResponse{}, ctx.Err()
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
		if err != nil {
			<-c.sem // Release semaphore
			return contracts.LLMResponse{}, fmt.Errorf("create HTTP request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

		startTime := time.Now()
		httpResp, err = c.client.Do(httpReq)
		duration = time.Since(startTime)

		if err != nil {
			<-c.sem // Release semaphore

			if ctx.Err() != nil {
				return contracts.LLMResponse{}, ctx.Err()
			}

			if attempt == totalAttempts-1 {
				return contracts.LLMResponse{}, fmt.Errorf("HTTP request failed (after %d attempts): %w", totalAttempts, err)
			}

			sleepDur := c.getBackoff(attempt, "")
			select {
			case <-ctx.Done():
				return contracts.LLMResponse{}, ctx.Err()
			case <-time.After(sleepDur):
			}
			continue
		}

		bodyBytes, err = io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		<-c.sem // Release semaphore

		if err != nil {
			if attempt == totalAttempts-1 {
				return contracts.LLMResponse{}, fmt.Errorf("read response body (after %d attempts): %w", totalAttempts, err)
			}
			sleepDur := c.getBackoff(attempt, "")
			select {
			case <-ctx.Done():
				return contracts.LLMResponse{}, ctx.Err()
			case <-time.After(sleepDur):
			}
			continue
		}

		// Retry on transient status codes (429 Too Many Requests, or 5xx Server Errors)
		if httpResp.StatusCode == http.StatusTooManyRequests || httpResp.StatusCode >= 500 {
			if attempt == totalAttempts-1 {
				return contracts.LLMResponse{}, fmt.Errorf("API error (status %d) after %d attempts: %s", httpResp.StatusCode, totalAttempts, string(bodyBytes))
			}

			retryAfter := httpResp.Header.Get("Retry-After")
			sleepDur := c.getBackoff(attempt, retryAfter)
			select {
			case <-ctx.Done():
				return contracts.LLMResponse{}, ctx.Err()
			case <-time.After(sleepDur):
			}
			continue
		}

		// Fail fast on terminal 4xx errors
		if httpResp.StatusCode >= 400 && httpResp.StatusCode < 500 {
			return contracts.LLMResponse{}, fmt.Errorf("API terminal error (status %d): %s", httpResp.StatusCode, string(bodyBytes))
		}

		if httpResp.StatusCode != http.StatusOK {
			return contracts.LLMResponse{}, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(bodyBytes))
		}
		break
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
		LatencyMS:    duration.Milliseconds(),
	}, nil
}

// completeMock simulates the Cerebras completion API by returning high-fidelity mock JSON responses
// tailored to the respective EOC specialist and commander cells.
//
// It honors ctx cancellation during the simulated inference latency: if the
// orchestrator's fan-out is cancelled (e.g. a timeout), the mock returns
// promptly rather than sleeping out the full delay, so a cancelled fan-out
// frees the concurrency budget immediately — matching the real client.
func (c *Client) completeMock(ctx context.Context, req contracts.LLMRequest) (contracts.LLMResponse, error) {
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
			"agent": "Commander",
			"summary": "Cerebro earthquake cascade. Two bridges closed, Highgate heavily damaged, Central General hospital at critical capacity.",
			"riskLevel": "Critical",
			"confidence": 0.95,
			"stateVersion": %d,
			"recommendations": [
				"Airlift critical casualties from Westbank Clinic",
				"Deploy USAR search teams to Highgate",
				"Inspect Vora and Iron bridges for structural integrity"
			],
			"evidence": [
				"Multiple utility dropouts and bridge closures",
				"Hospital ER occupancy reports straining"
			]
		}`, stateVersion)

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
	select {
	case <-ctx.Done():
		return contracts.LLMResponse{}, ctx.Err()
	case <-time.After(simulatedDuration):
	}

	return contracts.LLMResponse{
		Content:      content,
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		TokensPerSec: tokensPerSec,
		LatencyMS:    simulatedDuration.Milliseconds(),
	}, nil
}
