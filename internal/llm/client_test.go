package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func TestMockMode(t *testing.T) {
	// Force mock mode
	os.Setenv("LLM_MOCK", "true")
	defer os.Unsetenv("LLM_MOCK")

	client := NewClient(Config{})

	tests := []struct {
		name       string
		req        contracts.LLMRequest
		expectType string // e.g. "Infrastructure" or "Medical" or "summary"
	}{
		{
			name: "Infrastructure Cell request",
			req: contracts.LLMRequest{
				System: "You are the Infrastructure cell",
				User:   "Identify damage to bridges for stateVersion: 12",
			},
			expectType: `"agent": "Infrastructure"`,
		},
		{
			name: "Medical Cell request",
			req: contracts.LLMRequest{
				System: "Identify medical needs",
				User:   "Triage details for stateVersion: 42",
			},
			expectType: `"agent": "Medical"`,
		},
		{
			name: "Commander request",
			req: contracts.LLMRequest{
				System: "You are the Commander cell",
				User:   "Synthesize the COP for version: 3",
			},
			expectType: `"prioritizedActions"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Complete(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Complete failed: %v", err)
			}

			if resp.TokensPerSec != 1500.0 {
				t.Errorf("expected mock tokensPerSec to be 1500, got %f", resp.TokensPerSec)
			}

			if !strings.Contains(resp.Content, tt.expectType) {
				t.Errorf("expected response to contain %q, got: %s", tt.expectType, resp.Content)
			}

			// Validate that the output parses as JSON
			var parsed map[string]any
			if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
				t.Errorf("content is not valid JSON: %v", err)
			}
		})
	}
}

func TestEnsureAdditionalPropertiesFalse(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"details": {
				"type": "object",
				"properties": {
					"count": {"type": "integer"}
				}
			}
		}
	}`)

	modified := ensureAdditionalPropertiesFalse(schema)

	var parsed map[string]any
	if err := json.Unmarshal(modified, &parsed); err != nil {
		t.Fatalf("unmarshal modified schema: %v", err)
	}

	if addProps, ok := parsed["additionalProperties"].(bool); !ok || addProps {
		t.Errorf("expected additionalProperties: false at root")
	}

	properties, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatalf("missing properties")
	}

	details, ok := properties["details"].(map[string]any)
	if !ok {
		t.Fatalf("missing details property")
	}

	if addProps, ok := details["additionalProperties"].(bool); !ok || addProps {
		t.Errorf("expected additionalProperties: false inside nested details object")
	}
}

func TestRealClientSuccess(t *testing.T) {
	// Turn off mock mode
	os.Unsetenv("LLM_MOCK")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key-123" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var reqPayload chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		if reqPayload.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", reqPayload.Model)
		}

		// Return a mock Cerebras completion response with time_info
		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: `{"status": "ok"}`,
					},
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 30,
				TotalTokens:      40,
			},
			TimeInfo: struct {
				CompletionTime float64 `json:"completion_time"`
				PromptTime     float64 `json:"prompt_time"`
				QueueTime      float64 `json:"queue_time"`
				TotalTime      float64 `json:"total_time"`
			}{
				CompletionTime: 0.02, // 30 tokens in 0.02s = 1500 tokens/sec
				PromptTime:     0.01,
				TotalTime:      0.03,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:  "test-key-123",
		BaseURL: server.URL,
		Model:   "test-model",
	})

	resp, err := client.Complete(context.Background(), contracts.LLMRequest{
		System: "System prompt",
		User:   "User prompt",
		Schema: []byte(`{"type": "object"}`),
	})

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if resp.Content != `{"status": "ok"}` {
		t.Errorf("unexpected content: %s", resp.Content)
	}

	if resp.TokensIn != 10 {
		t.Errorf("expected 10 prompt tokens, got %d", resp.TokensIn)
	}

	if resp.TokensOut != 30 {
		t.Errorf("expected 30 completion tokens, got %d", resp.TokensOut)
	}

	// 30 tokens / 0.02s = 1500 tokens/sec
	if resp.TokensPerSec != 1500.0 {
		t.Errorf("expected tokensPerSec to be 1500.0, got %f", resp.TokensPerSec)
	}
}

func TestRealClientAPIError(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Cerebras is overloaded"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	_, err := client.Complete(context.Background(), contracts.LLMRequest{
		User: "hello",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "status 500") || !strings.Contains(err.Error(), "Cerebras is overloaded") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRealClientFallbackDuration(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Response has no time_info
		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "hello",
					},
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 10,
				TotalTokens:      15,
			},
		}

		time.Sleep(10 * time.Millisecond) // Ensure duration is > 0
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	resp, err := client.Complete(context.Background(), contracts.LLMRequest{
		User: "hello",
	})

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Should calculate based on fallback duration
	if resp.TokensPerSec <= 0 {
		t.Errorf("expected positive tokensPerSec using fallback duration, got %f", resp.TokensPerSec)
	}
}
