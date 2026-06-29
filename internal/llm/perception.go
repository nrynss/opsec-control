package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Interpret implements contracts.Perception. It processes satellite or drone
// images to detect disaster anomalies and returns structured EOC events.
func (c *Client) Interpret(ctx context.Context, input contracts.ImageInput) ([]contracts.Event, error) {
	// If LLM_MOCK is true or API Key is missing, run high-fidelity mock perception
	// Mock mode: LLM_MOCK=true or no API key for the active provider.
	// Snapshot avoids a data race with SetProvider.
	_, activeKey, _, _ := c.configSnapshot()
	if os.Getenv("LLM_MOCK") == "true" || activeKey == "" {
		return c.interpretMock(ctx, input)
	}

	return c.interpretReal(ctx, input)
}

func (c *Client) interpretMock(ctx context.Context, input contracts.ImageInput) ([]contracts.Event, error) {
	// Simulate minor vision latency (multimodal is slightly slower than pure text, e.g. ~300ms on Cerebras)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(300 * time.Millisecond):
	}

	// Read content as string to see if we match key mock triggers
	dataStr := string(input.Data)

	var events []contracts.Event

	switch {
	case strings.Contains(dataStr, "vora") || strings.Contains(dataStr, "bridge_collapsed"):
		events = append(events, contracts.Event{
			ID:         contracts.EventID("evt-perc-vora"),
			Timestamp:  0, // Will be set by EOC runner
			Source:     fmt.Sprintf("Cerebras-Perception-%s", input.Source),
			Type:       contracts.EventBridgeCollapsed,
			Confidence: 0.98,
			Payload:    json.RawMessage(`{"bridgeId": "vora"}`),
		})
	case strings.Contains(dataStr, "highgate") || strings.Contains(dataStr, "building_collapsed"):
		events = append(events, contracts.Event{
			ID:         contracts.EventID("evt-perc-highgate"),
			Timestamp:  0,
			Source:     fmt.Sprintf("Cerebras-Perception-%s", input.Source),
			Type:       contracts.EventBuildingCollapsed,
			Confidence: 0.92,
			Payload:    json.RawMessage(`{"sector": "highgate"}`),
		})
	case strings.Contains(dataStr, "southport") || strings.Contains(dataStr, "levee_breach"):
		events = append(events, contracts.Event{
			ID:         contracts.EventID("evt-perc-levee"),
			Timestamp:  0,
			Source:     fmt.Sprintf("Cerebras-Perception-%s", input.Source),
			Type:       contracts.EventLeveeBreached,
			Confidence: 0.95,
			Payload:    json.RawMessage(`{"sector": "southport"}`),
		})
	default:
		// Default generic detection based on source
		if input.Source == "satellite" {
			events = append(events, contracts.Event{
				ID:         contracts.EventID("evt-perc-gen-sat"),
				Timestamp:  0,
				Source:     "Cerebras-Perception-satellite",
				Type:       contracts.EventBuildingCollapsed,
				Confidence: 0.88,
				Payload:    json.RawMessage(`{"sector": "central"}`),
			})
		} else {
			events = append(events, contracts.Event{
				ID:         contracts.EventID("evt-perc-gen-drone"),
				Timestamp:  0,
				Source:     "Cerebras-Perception-drone",
				Type:       contracts.EventRoadBlocked,
				Confidence: 0.91,
				Payload:    json.RawMessage(`{"roadId": "R-WEST-1"}`),
			})
		}
	}

	return events, nil
}

func (c *Client) interpretReal(ctx context.Context, input contracts.ImageInput) ([]contracts.Event, error) {
	systemPrompt := "You are an EOC Multimodal Perception Agent. Analyze the aerial drone/satellite image and identify any structural collapses, bridge blockages, fires, or flooding. Output a JSON array of events."

	// Snapshot active provider config atomically to avoid data races with
	// concurrent SetProvider calls.
	provider, apiKey, baseURL, visionModel := c.configSnapshot()

	// Base64 encode the image data
	base64Data := base64.StdEncoding.EncodeToString(input.Data)
	mediaType := "image/jpeg"
	if len(input.Data) > 4 && string(input.Data[:4]) == "\x89PNG" {
		mediaType = "image/png"
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", mediaType, base64Data)

	schema := json.RawMessage(`{
		"type": "array",
		"items": {
			"type": "object",
			"properties": {
				"type": { "type": "string" },
				"confidence": { "type": "number", "minimum": 0, "maximum": 1 },
				"payload": { "type": "object" }
			},
			"required": ["type", "confidence"]
		}
	}`)

	type contentPart struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url,omitempty"`
	}

	type visionMessage struct {
		Role    string        `json:"role"`
		Content []contentPart `json:"content"`
	}

	type visionRequest struct {
		Model          string          `json:"model"`
		Messages       []visionMessage `json:"messages"`
		ResponseFormat *responseFormat `json:"response_format,omitempty"`
	}

	apiReqPayload := visionRequest{
		Model:    visionModel,
		Messages: []visionMessage{
			{
				Role: "user",
				Content: []contentPart{
					{Type: "text", Text: systemPrompt},
					{
						Type: "image_url",
						ImageURL: &struct {
							URL string `json:"url"`
						}{URL: dataURI},
					},
				},
			},
		},
	}

	// response_format with json_schema + strict is Cerebras-specific.
	// OpenRouter proxies many models — some don't support it.
	if provider == ProviderCerebras {
		cleanedSchema := ensureAdditionalPropertiesFalse(schema)
		apiReqPayload.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &responseFormatSchema{
				Name:   "perception_output",
				Strict: true,
				Schema: cleanedSchema,
			},
		}
	}

	reqBytes, err := json.Marshal(apiReqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(baseURL, "/"))

	// Acquire semaphore
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("perception API error (status %d): %s", httpResp.StatusCode, string(bodyBytes))
	}

	var apiResp chatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in API response")
	}

	var rawEvents []struct {
		Type       string          `json:"type"`
		Confidence float64         `json:"confidence"`
		Payload    json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &rawEvents); err != nil {
		return nil, fmt.Errorf("unmarshal perception output: %w", err)
	}

	h := sha256.New()
	h.Write(input.Data)
	hashStr := fmt.Sprintf("%x", h.Sum(nil))[:16]

	var events []contracts.Event
	for i, re := range rawEvents {
		events = append(events, contracts.Event{
			ID:         contracts.EventID(fmt.Sprintf("evt-perc-%s-%d", hashStr, i)),
			Timestamp:  0, // Set by the runner
			Source:     fmt.Sprintf("Cerebras-Perception-%s", input.Source),
			Type:       normalizeEventType(re.Type),
			Confidence: re.Confidence,
			Payload:    re.Payload,
		})
	}

	return events, nil
}

var canonicalEventTypes = []contracts.EventType{
	contracts.EventMainshockOccurred,
	contracts.EventAftershockOccurred,
	contracts.EventAftershockForecastUpdated,
	contracts.EventBuildingCollapsed,
	contracts.EventBridgeDamaged,
	contracts.EventBridgeClosed,
	contracts.EventBridgeCollapsed,
	contracts.EventRoadBlocked,
	contracts.EventTunnelClosed,
	contracts.EventDamStressElevated,
	contracts.EventLeveeBreached,
	contracts.EventPowerFailure,
	contracts.EventPowerDegraded,
	contracts.EventGasLeakDetected,
	contracts.EventWaterMainBreak,
	contracts.EventCommsOutage,
	contracts.EventFireIgnited,
	contracts.EventFireSpread,
	contracts.EventFireContained,
	contracts.EventFloodExtentUpdated,
	contracts.EventCasualtyReportUpdated,
	contracts.EventMassCasualtyIncident,
	contracts.EventHospitalCapacityChanged,
	contracts.EventCitizenDistressCall,
	contracts.EventPersonsTrapped,
	contracts.EventEvacuationOrdered,
	contracts.EventShelterOccupancyChanged,
	contracts.EventShelterFull,
	contracts.EventSatelliteImageReceived,
	contracts.EventDroneImageReceived,
	contracts.EventResourceDeployed,
	contracts.EventResourceDepleted,
	contracts.EventRejected,
}

var eventTypeNormalizationMap = func() map[string]contracts.EventType {
	m := make(map[string]contracts.EventType)
	for _, et := range canonicalEventTypes {
		normalized := normalizeString(string(et))
		m[normalized] = et
	}
	// Add specific common aliases/abbreviations
	m["leveebreach"] = contracts.EventLeveeBreached
	m["bridgecollapse"] = contracts.EventBridgeCollapsed
	m["buildingcollapse"] = contracts.EventBuildingCollapsed
	m["roadblock"] = contracts.EventRoadBlocked
	m["gasleak"] = contracts.EventGasLeakDetected
	m["watermainbreakage"] = contracts.EventWaterMainBreak
	m["waterbreak"] = contracts.EventWaterMainBreak
	m["commsfailure"] = contracts.EventCommsOutage
	m["poweroutage"] = contracts.EventPowerFailure
	return m
}()

func normalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func normalizeEventType(input string) contracts.EventType {
	norm := normalizeString(input)
	if et, ok := eventTypeNormalizationMap[norm]; ok {
		return et
	}
	return contracts.EventType(input)
}
