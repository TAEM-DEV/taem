package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestOllamaURLEmpty verifies that when OllamaURL is not set, Route returns
// ErrOllamaNotConfigured immediately without calling Anthropic.
func TestOllamaURLEmpty(t *testing.T) {
	anthropicCalled := false
	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anthropicCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer anthropic.Close()

	cfg := InferenceConfig{
		OllamaURL:           "", // not set
		AnthropicAPIKey:     "test-key",
		AnthropicModel:      "claude-sonnet-4-6",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
	}

	result, err := Route(context.Background(), "system", "user", cfg)
	if err != ErrOllamaNotConfigured {
		t.Fatalf("expected ErrOllamaNotConfigured, got: %v", err)
	}
	if result.Backend != "none" {
		t.Fatalf("expected backend 'none', got: %q", result.Backend)
	}
	if result.Content != "" {
		t.Fatalf("expected empty content, got: %q", result.Content)
	}
	if anthropicCalled {
		t.Fatal("Anthropic should not have been called when OllamaURL is empty")
	}
}

// TestOllamaHighConfidence verifies that when Ollama returns a high confidence
// response (>= threshold), the result uses Ollama with no fallback.
func TestOllamaHighConfidence(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Response: "Ollama high confidence answer",
			Done:     true,
			Context:  []int{1, 2, 3},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollama.Close()

	anthropicCalled := false
	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anthropicCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer anthropic.Close()

	cfg := InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "test-key",
		AnthropicModel:      "claude-sonnet-4-6",
	}

	// Inject a confidence extractor that returns 0.90
	origExtractor := confidenceExtractor
	confidenceExtractor = func(response string) float64 { return 0.90 }
	defer func() { confidenceExtractor = origExtractor }()

	result, err := Route(context.Background(), "system", "user content", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Backend != "ollama" {
		t.Fatalf("expected backend 'ollama', got: %q", result.Backend)
	}
	if result.Confidence != 0.90 {
		t.Fatalf("expected confidence 0.90, got: %f", result.Confidence)
	}
	if result.FallbackUsed {
		t.Fatal("expected FallbackUsed=false")
	}
	if result.Content != "Ollama high confidence answer" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if result.LatencyMS < 0 {
		t.Fatalf("expected non-negative LatencyMS, got: %d", result.LatencyMS)
	}
	if anthropicCalled {
		t.Fatal("Anthropic should not have been called on high confidence")
	}
}

// TestOllamaLowConfidenceFallback verifies that when Ollama returns a low
// confidence response (< threshold), Route falls back to Anthropic.
func TestOllamaLowConfidenceFallback(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Response: "Ollama low confidence answer",
			Done:     true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollama.Close()

	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key header
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key 'test-key', got: %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version header")
		}

		resp := anthropicResponse{
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Anthropic fallback answer"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer anthropic.Close()

	cfg := InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "test-key",
		AnthropicModel:      "claude-sonnet-4-6",
	}

	// Override Anthropic URL for test
	origURL := anthropicBaseURL
	anthropicBaseURL = anthropic.URL
	defer func() { anthropicBaseURL = origURL }()

	// Inject a confidence extractor that returns 0.70
	origExtractor := confidenceExtractor
	confidenceExtractor = func(response string) float64 { return 0.70 }
	defer func() { confidenceExtractor = origExtractor }()

	result, err := Route(context.Background(), "system", "user content", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Backend != "anthropic" {
		t.Fatalf("expected backend 'anthropic', got: %q", result.Backend)
	}
	if !result.FallbackUsed {
		t.Fatal("expected FallbackUsed=true")
	}
	if !strings.Contains(result.FallbackReason, "below threshold") {
		t.Fatalf("expected fallback reason to mention threshold, got: %q", result.FallbackReason)
	}
	if result.Content != "Anthropic fallback answer" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if result.LatencyMS < 0 {
		t.Fatalf("expected non-negative LatencyMS, got: %d", result.LatencyMS)
	}
}

// TestOllamaTimeoutFallback verifies that when Ollama times out, Route
// falls back to Anthropic with FallbackUsed=true.
func TestOllamaTimeoutFallback(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow Ollama by sleeping longer than timeout
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ollama.Close()

	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Anthropic timeout fallback answer"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer anthropic.Close()

	cfg := InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            1, // 1 second timeout — Ollama sleeps 3s
		AnthropicAPIKey:     "test-key",
		AnthropicModel:      "claude-sonnet-4-6",
	}

	origURL := anthropicBaseURL
	anthropicBaseURL = anthropic.URL
	defer func() { anthropicBaseURL = origURL }()

	result, err := Route(context.Background(), "system", "user content", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Backend != "anthropic" {
		t.Fatalf("expected backend 'anthropic', got: %q", result.Backend)
	}
	if !result.FallbackUsed {
		t.Fatal("expected FallbackUsed=true")
	}
	if !strings.Contains(result.FallbackReason, "timeout") {
		t.Fatalf("expected fallback reason to mention timeout, got: %q", result.FallbackReason)
	}
	if result.Content != "Anthropic timeout fallback answer" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if result.LatencyMS <= 0 {
		t.Fatalf("expected positive LatencyMS, got: %d", result.LatencyMS)
	}
}

// TestBothBackendsFail verifies that when both Ollama and Anthropic fail,
// Route returns an error.
func TestBothBackendsFail(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "ollama error")
	}))
	defer ollama.Close()

	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "anthropic error")
	}))
	defer anthropic.Close()

	cfg := InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "test-key",
		AnthropicModel:      "claude-sonnet-4-6",
	}

	origURL := anthropicBaseURL
	anthropicBaseURL = anthropic.URL
	defer func() { anthropicBaseURL = origURL }()

	_, err := Route(context.Background(), "system", "user content", cfg)
	if err == nil {
		t.Fatal("expected error when both backends fail")
	}
	if !strings.Contains(err.Error(), "anthropic") {
		t.Fatalf("expected error to mention anthropic, got: %v", err)
	}
}

// TestInferenceResultFieldsPopulated verifies that all InferenceResult fields
// (Backend, Confidence, LatencyMS, FallbackUsed) are properly populated.
func TestInferenceResultFieldsPopulated(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Response: "complete result test",
			Done:     true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollama.Close()

	cfg := InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "test-key",
		AnthropicModel:      "claude-sonnet-4-6",
	}

	// High confidence so we stay on Ollama
	origExtractor := confidenceExtractor
	confidenceExtractor = func(response string) float64 { return 0.92 }
	defer func() { confidenceExtractor = origExtractor }()

	result, err := Route(context.Background(), "system", "user content", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all fields are populated
	if result.Backend == "" {
		t.Fatal("Backend should not be empty")
	}
	if result.Backend != "ollama" {
		t.Fatalf("expected 'ollama', got: %q", result.Backend)
	}
	if result.Content == "" {
		t.Fatal("Content should not be empty")
	}
	if result.Confidence == 0 {
		t.Fatal("Confidence should not be zero")
	}
	if result.Confidence != 0.92 {
		t.Fatalf("expected confidence 0.92, got: %f", result.Confidence)
	}
	if result.LatencyMS < 0 {
		t.Fatalf("LatencyMS should be non-negative, got: %d", result.LatencyMS)
	}
	if result.FallbackUsed {
		t.Fatal("FallbackUsed should be false for direct Ollama result")
	}
	if result.FallbackReason != "" {
		t.Fatalf("FallbackReason should be empty, got: %q", result.FallbackReason)
	}
}

// TestAnthropicAPIKeyMissingOnFallback verifies that if Ollama fails and
// AnthropicAPIKey is not set, an error is returned.
func TestAnthropicAPIKeyMissingOnFallback(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ollama.Close()

	cfg := InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "", // not set
	}

	_, err := Route(context.Background(), "system", "user content", cfg)
	if err == nil {
		t.Fatal("expected error when Anthropic API key is missing on fallback")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("expected error to mention ANTHROPIC_API_KEY, got: %v", err)
	}
}
