// Package adr_audit implements the PRB ADR Compliance Auditor inference controller.
//
// PRB-ADR is an independent second-pass ADR compliance check. It does not
// defer to ARCH's deterministic evaluation. It queries the ecosystem
// constraint_index semantically to find constraints by meaning — catching
// novel violations that ARCH's literal pattern matching missed.
//
// Uses inference.Route() for Ollama-first LLM calls per ADR-005.
// Emits HOLD when Ollama is not configured (ErrOllamaNotConfigured).
package adr_audit

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

// Controller implements the PRB-ADR inference controller.
type Controller struct {
	config     inference.InferenceConfig
	promptsDir string
}

// New returns a new PRB-ADR controller instance.
func New(cfg inference.InferenceConfig, promptsDir string) *Controller {
	return &Controller{
		config:     cfg,
		promptsDir: promptsDir,
	}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "PRB-ADR"
}

// Mode returns ModeInference per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeInference
}

// adrAuditResponse is the expected JSON structure from the LLM.
type adrAuditResponse struct {
	Vote   string `json:"vote"`
	Reason string `json:"reason"`
}

// Run executes the PRB ADR Compliance audit.
//
// Steps:
//  1. Read step-plan.json and integration-map.json from inputs
//  2. Query ecosystem constraint_index via inputs.EcosystemQuery
//  3. Load system prompt from prompts/prb_adr_audit.txt
//  4. Construct user content with plan + integration map + constraints
//  5. Call inference.Route()
//  6. Handle ErrOllamaNotConfigured -> HOLD
//  7. Parse JSON response for vote: GO/NO-GO
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Read step-plan.json.
	if inputs.StepPlan == "" {
		return controller.Signal{
			Controller:  "PRB-ADR",
			SignalValue: "NO-GO",
			Reason:      "step-plan.json path not provided",
			Evidence:    []string{"input:step_plan=missing"},
		}, nil
	}
	stepPlan, err := os.ReadFile(inputs.StepPlan)
	if err != nil {
		return controller.Signal{
			Controller:  "PRB-ADR",
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
				Controller:  "PRB-ADR",
				SignalValue: "NO-GO",
				Reason:      fmt.Sprintf("failed to read integration-map.json: %v", err),
				Evidence:    []string{fmt.Sprintf("file:error=%v", err)},
			}, nil
		}
	}

	// Query ecosystem constraint_index.
	var constraintIndex string
	if inputs.EcosystemQuery != nil {
		select {
		case <-ctx.Done():
			return controller.Signal{}, ctx.Err()
		default:
		}

		result, err := inputs.EcosystemQuery("constraint_index", string(stepPlan))
		if err != nil {
			// Constraint index query failure is non-fatal — proceed without.
			constraintIndex = fmt.Sprintf("constraint_index query failed: %v", err)
		} else {
			constraintIndex = string(result)
		}
	} else {
		constraintIndex = "constraint_index: not available (ecosystem query not configured)"
	}

	// Load system prompt.
	promptPath := filepath.Join(c.promptsDir, "prb_adr_audit.txt")
	systemPrompt, err := os.ReadFile(promptPath)
	if err != nil {
		return controller.Signal{}, fmt.Errorf("load system prompt: %w", err)
	}

	// Build user content with all three inputs.
	userContent := fmt.Sprintf("## step-plan.json\n```json\n%s\n```\n", string(stepPlan))
	if len(integrationMap) > 0 {
		userContent += fmt.Sprintf("\n## integration-map.json\n```json\n%s\n```\n", string(integrationMap))
	}
	userContent += fmt.Sprintf("\n## constraint_index\n%s\n", constraintIndex)

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Call inference router.
	result, err := inference.Route(ctx, string(systemPrompt), userContent, c.config)
	if errors.Is(err, inference.ErrOllamaNotConfigured) {
		return controller.Signal{
			Controller:  "PRB-ADR",
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
		Controller:     "PRB-ADR",
		SignalValue:    vote,
		Reason:         reason,
		Evidence:       []string{fmt.Sprintf("inference:backend=%s", result.Backend)},
		InferenceNotes: meta,
	}, nil
}

// parseVote extracts the vote from a JSON response. If the response is not
// valid JSON or lacks a vote field, defaults to NO-GO with a parse failure reason.
func parseVote(content string) (vote, reason string) {
	var resp adrAuditResponse
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
