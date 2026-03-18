// Package secinsp implements the Security Inspector inference controller.
//
// SECINSP performs two-pass security analysis on the integration plan.
// Pass 1 (deterministic) runs before this controller. This is Pass 2 —
// the inference pass — which finds novel attack surfaces, privilege
// escalation paths, and auth scope issues that pattern matching missed.
//
// Uses inference.Route() for Ollama-first LLM calls per ADR-005.
// Emits HOLD when Ollama is not configured (ErrOllamaNotConfigured).
package secinsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taem-dev/taem/internal/controller"
	"github.com/taem-dev/taem/internal/controller/inference"
)

// Controller implements the SECINSP inference controller.
type Controller struct {
	config     inference.InferenceConfig
	promptsDir string
}

// New returns a new SECINSP controller instance.
func New(cfg inference.InferenceConfig, promptsDir string) *Controller {
	return &Controller{
		config:     cfg,
		promptsDir: promptsDir,
	}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "SECINSP"
}

// Mode returns ModeInference per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeInference
}

// secinspResponse is the expected JSON structure from the LLM.
type secinspResponse struct {
	Vote   string `json:"vote"`
	Reason string `json:"reason"`
}

// Run executes the Security Inspector inference pass.
//
// Steps:
//  1. Read step-plan.json and integration-map.json from inputs
//  2. Load system prompt from prompts/secinsp.txt
//  3. Call inference.Route() with system prompt and plan content
//  4. Handle ErrOllamaNotConfigured -> HOLD signal
//  5. Parse JSON response for vote: GO/NO-GO
//  6. Return Signal with InferenceMeta populated
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Read step-plan.json.
	if inputs.StepPlan == "" {
		return controller.Signal{
			Controller:  "SECINSP",
			SignalValue: "NO-GO",
			Reason:      "step-plan.json path not provided",
			Evidence:    []string{"input:step_plan=missing"},
		}, nil
	}
	stepPlan, err := os.ReadFile(inputs.StepPlan)
	if err != nil {
		return controller.Signal{
			Controller:  "SECINSP",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read step-plan.json: %v", err),
			Evidence:    []string{fmt.Sprintf("file:error=%v", err)},
		}, nil
	}

	// Read integration-map.json.
	var integrationMap []byte
	if inputs.IntegrationMap != "" {
		integrationMap, err = os.ReadFile(inputs.IntegrationMap)
		if err != nil {
			return controller.Signal{
				Controller:  "SECINSP",
				SignalValue: "NO-GO",
				Reason:      fmt.Sprintf("failed to read integration-map.json: %v", err),
				Evidence:    []string{fmt.Sprintf("file:error=%v", err)},
			}, nil
		}
	}

	// Load system prompt.
	promptPath := filepath.Join(c.promptsDir, "secinsp.txt")
	systemPrompt, err := os.ReadFile(promptPath)
	if err != nil {
		return controller.Signal{}, fmt.Errorf("load system prompt: %w", err)
	}

	// Build user content.
	userContent := fmt.Sprintf("## Mission Type\n%s\n\n", inputs.MissionType)
	userContent += fmt.Sprintf("## step-plan.json\n```json\n%s\n```\n", string(stepPlan))
	if len(integrationMap) > 0 {
		userContent += fmt.Sprintf("\n## integration-map.json\n```json\n%s\n```\n", string(integrationMap))
	}

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Call inference router.
	result, err := inference.Route(ctx, string(systemPrompt), userContent, c.config)
	if errors.Is(err, inference.ErrOllamaNotConfigured) {
		return controller.Signal{
			Controller:  "SECINSP",
			SignalValue: "HOLD",
			Reason:      "Ollama not configured — inference controller holding per ADR-005",
			Evidence:    []string{"inference:ollama=not_configured"},
		}, nil
	}
	if err != nil {
		return controller.Signal{}, fmt.Errorf("inference route: %w", err)
	}

	// Parse vote from JSON response.
	vote, reason := parseVote(result.Content)

	meta := &controller.InferenceMeta{
		Backend:      result.Backend,
		Confidence:   result.Confidence,
		LatencyMS:    result.LatencyMS,
		FallbackUsed: result.FallbackUsed,
	}

	return controller.Signal{
		Controller:     "SECINSP",
		SignalValue:    vote,
		Reason:         reason,
		Evidence:       []string{fmt.Sprintf("inference:backend=%s", result.Backend)},
		InferenceNotes: meta,
	}, nil
}

// parseVote extracts the vote from a JSON response. If the response is not
// valid JSON or lacks a vote field, defaults to NO-GO with a parse failure reason.
func parseVote(content string) (vote, reason string) {
	var resp secinspResponse
	cleaned := inference.ExtractJSON(content)
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return "NO-GO", fmt.Sprintf("failed to parse inference response as JSON: %v", err)
	}

	switch resp.Vote {
	case "GO":
		return "GO", resp.Reason
	case "NO-GO":
		return "NO-GO", resp.Reason
	default:
		return "NO-GO", fmt.Sprintf("unrecognized vote %q in inference response — defaulting to NO-GO", resp.Vote)
	}
}
