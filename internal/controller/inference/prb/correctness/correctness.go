// Package correctness implements the PRB Correctness Auditor inference controller.
//
// PRB-COR verifies that the step plan is technically accurate, internally
// consistent, and complete enough to implement without improvisation. It checks
// output->input validity, file path consistency, completeness, dependency
// honoring, test harness reachability, and rollback validity.
//
// Uses inference.Route() for Ollama-first LLM calls per ADR-005.
// Emits HOLD when Ollama is not configured (ErrOllamaNotConfigured).
package correctness

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

// Controller implements the PRB-COR inference controller.
type Controller struct {
	config     inference.InferenceConfig
	promptsDir string
}

// New returns a new PRB-COR controller instance.
func New(cfg inference.InferenceConfig, promptsDir string) *Controller {
	return &Controller{
		config:     cfg,
		promptsDir: promptsDir,
	}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "PRB-COR"
}

// Mode returns ModeInference per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeInference
}

// correctnessResponse is the expected JSON structure from the LLM.
type correctnessResponse struct {
	Vote   string `json:"vote"`
	Reason string `json:"reason"`
}

// Run executes the PRB Correctness audit.
//
// Steps:
//  1. Read step-plan.json from inputs
//  2. Load system prompt from prompts/prb_correctness.txt
//  3. Call inference.Route()
//  4. Handle ErrOllamaNotConfigured -> HOLD
//  5. Parse JSON response for vote: GO/NO-GO
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Read step-plan.json.
	if inputs.StepPlan == "" {
		return controller.Signal{
			Controller:  "PRB-COR",
			SignalValue: "NO-GO",
			Reason:      "step-plan.json path not provided",
			Evidence:    []string{"input:step_plan=missing"},
		}, nil
	}
	stepPlan, err := os.ReadFile(inputs.StepPlan)
	if err != nil {
		return controller.Signal{
			Controller:  "PRB-COR",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read step-plan.json: %v", err),
			Evidence:    []string{fmt.Sprintf("file:error=%v", err)},
		}, nil
	}

	// Load system prompt.
	promptPath := filepath.Join(c.promptsDir, "prb_correctness.txt")
	systemPrompt, err := os.ReadFile(promptPath)
	if err != nil {
		return controller.Signal{}, fmt.Errorf("load system prompt: %w", err)
	}

	// Build user content.
	userContent := fmt.Sprintf("## step-plan.json\n```json\n%s\n```\n", string(stepPlan))

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Call inference router.
	result, err := inference.Route(ctx, string(systemPrompt), userContent, c.config)
	if errors.Is(err, inference.ErrOllamaNotConfigured) {
		return controller.Signal{
			Controller:  "PRB-COR",
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
		Controller:     "PRB-COR",
		SignalValue:    vote,
		Reason:         reason,
		Evidence:       []string{fmt.Sprintf("inference:backend=%s", result.Backend)},
		InferenceNotes: meta,
	}, nil
}

// parseVote extracts the vote from a JSON response. If the response is not
// valid JSON or lacks a vote field, defaults to NO-GO with a parse failure reason.
func parseVote(content string) (vote, reason string) {
	var resp correctnessResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
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
