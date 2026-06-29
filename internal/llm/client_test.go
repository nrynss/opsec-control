package llm

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
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
			expectType: `"agent": "Commander"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Complete(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Complete failed: %v", err)
			}

			const mockTokensPerSec = 1500.0
			if math.Abs(resp.TokensPerSec-mockTokensPerSec) > 1e-9 {
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
	os.Unsetenv("LLM_MOCK")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqPayload chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

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
				CompletionTime: 0.02,
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
		APIKey:     "test-key-123",
		BaseURL:    server.URL,
		Model:      "test-model",
		MaxRetries: 1,
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

	if resp.TokensPerSec != 1500.0 {
		t.Errorf("expected tokensPerSec to be 1500.0, got %f", resp.TokensPerSec)
	}
}

func TestRealClientAPIError(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Cerebras overloaded"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 1,
		Backoff:    1 * time.Millisecond,
	})

	_, err := client.Complete(context.Background(), contracts.LLMRequest{
		User: "hello",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRealClientFallbackDuration(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		time.Sleep(10 * time.Millisecond)
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

	if resp.TokensPerSec <= 0 {
		t.Errorf("expected positive tokensPerSec using fallback duration, got %f", resp.TokensPerSec)
	}
}

func TestRetryOnTransientErrors(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limited"))
			return
		}
		if count == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server error"))
			return
		}

		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "success"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 3,
		Backoff:    1 * time.Millisecond,
	})

	resp, err := client.Complete(context.Background(), contracts.LLMRequest{User: "hi"})
	if err != nil {
		t.Fatalf("Complete should have succeeded after retries: %v", err)
	}

	if resp.Content != "success" {
		t.Errorf("expected success content, got: %s", resp.Content)
	}

	finalAttempts := attempts.Load()
	if finalAttempts != 3 {
		t.Errorf("expected 3 total attempts, got %d", finalAttempts)
	}
}

func TestRetryAfterHeader(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count == 1 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "success"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 2,
		Backoff:    1 * time.Millisecond,
	})

	start := time.Now()
	_, err := client.Complete(context.Background(), contracts.LLMRequest{User: "hi"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Complete should have succeeded: %v", err)
	}

	// Should have slept for ~1 second due to Retry-After
	if elapsed < 800*time.Millisecond {
		t.Errorf("Retry-After header was ignored: elapsed only %v", elapsed)
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count == 1 {
			futureStr := time.Now().Add(3 * time.Second).UTC().Format(http.TimeFormat)
			w.Header().Set("Retry-After", futureStr)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "success"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 2,
		Backoff:    1 * time.Millisecond,
	})

	start := time.Now()
	_, err := client.Complete(context.Background(), contracts.LLMRequest{User: "hi"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Complete should have succeeded: %v", err)
	}

	// Should have slept for ~3 seconds (minus RTT) due to Retry-After HTTP date
	if elapsed < 1500*time.Millisecond {
		t.Errorf("Retry-After HTTP date header was ignored: elapsed only %v", elapsed)
	}
}

func TestTerminal4xxNoRetry(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity) // 422 Unprocessable Entity
		w.Write([]byte("Terminal error detail"))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 3,
		Backoff:    1 * time.Millisecond,
	})

	_, err := client.Complete(context.Background(), contracts.LLMRequest{User: "hi"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts.Load() != 1 {
		t.Errorf("expected only 1 attempt for terminal 4xx, got %d", attempts.Load())
	}

	if !strings.Contains(err.Error(), "Terminal error detail") {
		t.Errorf("expected terminal error message, got: %v", err)
	}
}

func TestConcurrencyCap(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	var activeRequests atomic.Int32
	var maxActiveRequests atomic.Int32
	var hasExceeded atomic.Bool

	maxConcurrency := 3

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentActive := activeRequests.Add(1)
		defer activeRequests.Add(-1)

		for {
			max := maxActiveRequests.Load()
			if currentActive <= max {
				break
			}
			if maxActiveRequests.CompareAndSwap(max, currentActive) {
				break
			}
		}

		if currentActive > int32(maxConcurrency) {
			hasExceeded.Store(true)
		}

		time.Sleep(20 * time.Millisecond)

		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:         "test-key",
		BaseURL:        server.URL,
		MaxConcurrency: maxConcurrency,
	})

	totalCalls := 10
	errChan := make(chan error, totalCalls)

	for range totalCalls {
		go func() {
			_, err := client.Complete(context.Background(), contracts.LLMRequest{User: "hi"})
			errChan <- err
		}()
	}

	for range totalCalls {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent call failed: %v", err)
		}
	}

	if hasExceeded.Load() {
		t.Errorf("max concurrent requests exceeded limit of %d, got max active %d", maxConcurrency, maxActiveRequests.Load())
	}
}

func TestBackoffDeterministic(t *testing.T) {
	// Determinism is law (SPEC §0.2 r5): the backoff PRNG must never be seeded
	// from the wall clock. Two clients with equal config (including the default
	// seed) must produce identical jitter sequences.
	newSeq := func(seed int64) []time.Duration {
		c := NewClient(Config{APIKey: "k", Seed: seed})
		out := make([]time.Duration, 0, 5)
		for attempt := range 5 {
			out = append(out, c.getBackoff(attempt, ""))
		}
		return out
	}

	a, b := newSeq(0), newSeq(0)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("default-seed backoff not reproducible at attempt %d: %v != %v", i, a[i], b[i])
		}
	}

	// An explicit seed is honored and (with overwhelming probability) yields a
	// different sequence than the default — confirming the seed is actually used.
	c := newSeq(99)
	differs := false
	for i := range a {
		if a[i] != c[i] {
			differs = true
			break
		}
	}
	if !differs {
		t.Fatal("explicit seed produced an identical sequence to the default; Config.Seed not applied")
	}
}

func TestMockHonorsContextCancellation(t *testing.T) {
	os.Setenv("LLM_MOCK", "true")
	defer os.Unsetenv("LLM_MOCK")

	client := NewClient(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the call — mock must not sleep out its latency

	_, err := client.Complete(ctx, contracts.LLMRequest{System: "commander", User: "synthesize"})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled from a cancelled mock completion, got: %v", err)
	}
}

func TestContextCancellationMidBackoff(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	// High backoff so we definitely get stuck sleeping
	client := NewClient(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 3,
		Backoff:    5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		_, err := client.Complete(ctx, contracts.LLMRequest{User: "hi"})
		errChan <- err
	}()

	time.Sleep(50 * time.Millisecond) // Let it make the first call and enter backoff sleep
	cancel()                          // Cancel context during backoff sleep

	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancellation did not return promptly during backoff sleep")
	}
}

// --- P9: multi-provider tests ---

func TestNewClientDefaultProvider(t *testing.T) {
	c := NewClient(Config{})
	if c.Provider() != ProviderCerebras {
		t.Errorf("expected default provider cerebras, got %s", c.Provider())
	}
}

func TestNewClientExplicitProvider(t *testing.T) {
	c := NewClient(Config{Provider: ProviderOpenRouter})
	if c.Provider() != ProviderOpenRouter {
		t.Errorf("expected openrouter, got %s", c.Provider())
	}
	// Legacy fields should reflect OpenRouter config.
	if c.baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("expected openrouter baseURL, got %s", c.baseURL)
	}
}

func TestSetProviderSwitchAtRuntime(t *testing.T) {
	c := NewClient(Config{
		APIKey:  "cerebras-key",
		BaseURL: "https://cerebras.test/v1",
		Model:   "gemma-4-31b",
	})
	if c.Provider() != ProviderCerebras {
		t.Fatalf("expected cerebras, got %s", c.Provider())
	}

	// Switch to OpenRouter.
	c.SetProvider(ProviderOpenRouter)
	if c.Provider() != ProviderOpenRouter {
		t.Fatalf("expected openrouter after switch, got %s", c.Provider())
	}
	if c.apiKey != c.openrouterKey {
		t.Errorf("apiKey not switched to openrouter key")
	}
	if c.baseURL != c.openrouterURL {
		t.Errorf("baseURL not switched to openrouter URL")
	}
	if c.model != c.openrouterModel {
		t.Errorf("model not switched to openrouter model")
	}

	// Switch back.
	c.SetProvider(ProviderCerebras)
	if c.Provider() != ProviderCerebras {
		t.Fatalf("expected cerebras after switch-back, got %s", c.Provider())
	}
	if c.apiKey != c.cerebrasKey {
		t.Errorf("apiKey not restored to cerebras key")
	}
}

func TestSetProviderNoop(t *testing.T) {
	c := NewClient(Config{Provider: ProviderCerebras, APIKey: "orig-key"})
	c.SetProvider(ProviderCerebras) // no-op
	if c.apiKey != "orig-key" {
		t.Errorf("apiKey changed on no-op SetProvider")
	}
}

func TestOpenRouterEnvConfig(t *testing.T) {
	os.Setenv("OPENROUTER_API_KEY", "or-key")
	os.Setenv("OPENROUTER_BASE_URL", "https://or.test/v1")
	os.Setenv("OPENROUTER_MODEL", "or-model")
	defer func() {
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("OPENROUTER_BASE_URL")
		os.Unsetenv("OPENROUTER_MODEL")
	}()

	c := NewClient(Config{Provider: ProviderOpenRouter})
	if c.openrouterKey != "or-key" {
		t.Errorf("expected or-key, got %s", c.openrouterKey)
	}
	if c.openrouterURL != "https://or.test/v1" {
		t.Errorf("expected or URL, got %s", c.openrouterURL)
	}
	if c.openrouterModel != "or-model" {
		t.Errorf("expected or-model, got %s", c.openrouterModel)
	}
	// Active fields should match.
	if c.apiKey != "or-key" {
		t.Errorf("active apiKey should be or-key, got %s", c.apiKey)
	}
}

func TestConfigOverrideForProvider(t *testing.T) {
	// When Config APIKey/BaseURL/Model are set, they override env for the active provider.
	c := NewClient(Config{
		Provider: ProviderOpenRouter,
		APIKey:   "cfg-key",
		BaseURL:  "https://cfg.test/v1",
		Model:    "cfg-model",
	})
	if c.apiKey != "cfg-key" {
		t.Errorf("expected cfg-key, got %s", c.apiKey)
	}
	if c.baseURL != "https://cfg.test/v1" {
		t.Errorf("expected cfg URL, got %s", c.baseURL)
	}
	if c.model != "cfg-model" {
		t.Errorf("expected cfg-model, got %s", c.model)
	}
}

func TestCompleteUsesActiveProviderURLAndAuth(t *testing.T) {
	os.Unsetenv("LLM_MOCK")

	var sawAuth, sawURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawURL = r.URL.String()

		resp := chatCompletionResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "ok"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(Config{
		Provider: ProviderOpenRouter,
		APIKey:   "or-test-key",
		BaseURL:  server.URL,
	})

	_, err := c.Complete(context.Background(), contracts.LLMRequest{User: "hi"})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if sawAuth != "Bearer or-test-key" {
		t.Errorf("expected Authorization 'Bearer or-test-key', got %q", sawAuth)
	}
	if sawURL != "/chat/completions" {
		t.Errorf("expected /chat/completions, got %q", sawURL)
	}
}

func TestMockModeIndependentOfProvider(t *testing.T) {
	os.Setenv("LLM_MOCK", "true")
	defer os.Unsetenv("LLM_MOCK")

	c := NewClient(Config{Provider: ProviderOpenRouter})
	resp, err := c.Complete(context.Background(), contracts.LLMRequest{
		System: "You are the Infrastructure cell",
		User:   "Identify damage for stateVersion: 1",
	})
	if err != nil {
		t.Fatalf("mock Complete failed: %v", err)
	}
	if !strings.Contains(resp.Content, `"agent": "Infrastructure"`) {
		t.Errorf("mock response should contain Infrastructure agent marker, got: %s", resp.Content)
	}
}
