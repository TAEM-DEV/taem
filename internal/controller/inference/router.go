// Package inference implements the Ollama-first, Anthropic-fallback inference
// router per ADR-005. All four inference controllers (SECINSP, PRB-SKP,
// PRB-COR, PRB-ADR) call Route to execute LLM inference.
package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// DefaultConfidenceThreshold is the system-wide confidence threshold per
// ADR-005 C-005-003. Ollama responses below this trigger Anthropic fallback.
const DefaultConfidenceThreshold = 0.85

// DefaultTimeoutS is the default Ollama request timeout in seconds.
const DefaultTimeoutS = 45

// ErrOllamaNotConfigured is returned when OLLAMA_URL is not set. Per ADR-005
// C-005-002, inference controllers should emit HOLD — not call Anthropic directly.
var ErrOllamaNotConfigured = errors.New("OLLAMA_URL not configured — inference controller should emit HOLD")

// anthropicBaseURL is the Anthropic Messages API base URL. Overridable in tests.
var anthropicBaseURL = "https://api.anthropic.com"

// confidenceExtractor is a function that extracts a confidence score from an
// Ollama response string. The default implementation returns
// DefaultConfidenceThreshold (assumes borderline confidence when Ollama does
// not embed an explicit score). Tests override this to control behavior.
var confidenceExtractor = defaultConfidenceExtractor

// InferenceConfig holds the configuration for a single inference call.
type InferenceConfig struct {
	OllamaURL           string
	OllamaModel         string
	ConfidenceThreshold float64
	TimeoutS            int
	AnthropicAPIKey     string
	AnthropicModel      string
}

// InferenceResult holds the outcome of an inference call, including routing
// metadata required by ADR-005 C-005-004.
type InferenceResult struct {
	Content        string  // LLM response content
	Backend        string  // "ollama", "anthropic", or "none"
	Confidence     float64 // confidence score from Ollama (0 for Anthropic)
	LatencyMS      int64   // total wall-clock latency in milliseconds
	FallbackUsed   bool    // true if Anthropic was called after Ollama
	FallbackReason string  // reason for fallback (empty if no fallback)
}

// --- Ollama API types ---

type ollamaRequest struct {
	Model  string `json:"model"`
	System string `json:"system"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Context  []int  `json:"context,omitempty"`
}

// --- Anthropic API types ---

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// defaultConfidenceExtractor returns DefaultConfidenceThreshold. In production,
// this would parse Ollama's response metadata for an explicit confidence
// signal. Since the Ollama /api/generate endpoint does not natively return a
// confidence score, we treat all complete responses as meeting the threshold
// exactly. Callers who need custom extraction replace this var.
func defaultConfidenceExtractor(_ string) float64 {
	return DefaultConfidenceThreshold
}

// Route handles Ollama -> Anthropic fallback per ADR-005.
// Called by all four inference controllers (SECINSP, PRB-SKP, PRB-COR, PRB-ADR).
//
// Behavior:
//  1. If OllamaURL not set -> return ErrOllamaNotConfigured (HOLD equivalent)
//  2. Call Ollama with timeout
//  3. If confidence >= threshold -> return Ollama result
//  4. If timeout or low confidence -> fallback to Anthropic
//  5. Log backend, confidence, latency_ms, fallback_reason per C-005-004
func Route(ctx context.Context, systemPrompt, userContent string, cfg InferenceConfig) (InferenceResult, error) {
	start := time.Now()

	// Apply defaults
	if cfg.ConfidenceThreshold == 0 {
		cfg.ConfidenceThreshold = DefaultConfidenceThreshold
	}
	if cfg.TimeoutS == 0 {
		cfg.TimeoutS = DefaultTimeoutS
	}

	// C-005-002: If OLLAMA_URL not set, return HOLD-equivalent immediately.
	// Do NOT call Anthropic.
	if cfg.OllamaURL == "" {
		log.Printf("[inference] backend=none reason=OLLAMA_URL_not_configured latency_ms=0")
		return InferenceResult{
			Backend: "none",
			Content: "",
		}, ErrOllamaNotConfigured
	}

	// Step 2: Call Ollama with timeout
	ollamaContent, ollamaErr := callOllama(ctx, systemPrompt, userContent, cfg)
	ollamaLatency := time.Since(start).Milliseconds()

	if ollamaErr == nil {
		// Extract confidence from Ollama response
		confidence := confidenceExtractor(ollamaContent)

		// Step 3: High confidence — return Ollama result
		if confidence >= cfg.ConfidenceThreshold {
			result := InferenceResult{
				Content:    ollamaContent,
				Backend:    "ollama",
				Confidence: confidence,
				LatencyMS:  ollamaLatency,
			}
			log.Printf("[inference] backend=ollama confidence=%.2f latency_ms=%d fallback=false",
				confidence, ollamaLatency)
			return result, nil
		}

		// Step 4a: Low confidence — fallback to Anthropic
		fallbackReason := fmt.Sprintf("ollama confidence %.2f below threshold %.2f",
			confidence, cfg.ConfidenceThreshold)
		return fallbackToAnthropic(ctx, systemPrompt, userContent, cfg, start, confidence, fallbackReason)
	}

	// Step 4b: Ollama error (timeout or other) — fallback to Anthropic
	fallbackReason := fmt.Sprintf("ollama timeout or error: %v", ollamaErr)
	return fallbackToAnthropic(ctx, systemPrompt, userContent, cfg, start, 0, fallbackReason)
}

// callOllama sends a generate request to the Ollama API with the configured timeout.
func callOllama(ctx context.Context, systemPrompt, userContent string, cfg InferenceConfig) (string, error) {
	timeout := time.Duration(cfg.TimeoutS) * time.Second
	ollamaCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBody := ollamaRequest{
		Model:  cfg.OllamaModel,
		System: systemPrompt,
		Prompt: userContent,
		Stream: false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	url := cfg.OllamaURL + "/api/generate"
	req, err := http.NewRequestWithContext(ollamaCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}

	return ollamaResp.Response, nil
}

// fallbackToAnthropic calls the Anthropic Messages API as fallback.
func fallbackToAnthropic(
	ctx context.Context,
	systemPrompt, userContent string,
	cfg InferenceConfig,
	start time.Time,
	ollamaConfidence float64,
	fallbackReason string,
) (InferenceResult, error) {
	// Check API key before attempting call
	if cfg.AnthropicAPIKey == "" {
		return InferenceResult{}, fmt.Errorf(
			"ANTHROPIC_API_KEY not configured — cannot fallback (reason: %s)", fallbackReason)
	}

	model := cfg.AnthropicModel
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userContent},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return InferenceResult{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	url := anthropicBaseURL + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return InferenceResult{}, fmt.Errorf("create anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.AnthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return InferenceResult{}, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return InferenceResult{}, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return InferenceResult{}, fmt.Errorf("decode anthropic response: %w", err)
	}

	// Extract text content from response blocks
	var content string
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	totalLatency := time.Since(start).Milliseconds()

	result := InferenceResult{
		Content:        content,
		Backend:        "anthropic",
		Confidence:     ollamaConfidence,
		LatencyMS:      totalLatency,
		FallbackUsed:   true,
		FallbackReason: fallbackReason,
	}

	log.Printf("[inference] backend=anthropic confidence=%.2f latency_ms=%d fallback=true reason=%q",
		ollamaConfidence, totalLatency, fallbackReason)

	return result, nil
}
