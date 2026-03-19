// Package skeptic implements the PRB Skeptic inference controller.
//
// PRB-SKP is the adversarial sub-agent of the Peer Review Board. It assumes
// the plan will fail and hunts for the specific failure mode. Before assessment,
// it queries ecosystem lessons_learned for similar prior failures — this is
// the Skeptic's primary advantage over other PRB members.
//
// ADR-009a Phase 2: also queries domain_knowledge for field intelligence,
// giving PRB-SKP academic literature context to strengthen adversarial analysis.
//
// Uses inference.Route() for Ollama-first LLM calls per ADR-005.
// Emits HOLD when Ollama is not configured (ErrOllamaNotConfigured).
package skeptic

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

// Controller implements the PRB-SKP inference controller.
type Controller struct {
	config     inference.InferenceConfig
	promptsDir string
}

// New returns a new PRB-SKP controller instance.
func New(cfg inference.InferenceConfig, promptsDir string) *Controller {
	return &Controller{
		config:     cfg,
		promptsDir: promptsDir,
	}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "PRB-SKP"
}

// Mode returns ModeInference per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeInference
}

// skepticResponse is the expected JSON structure from the LLM.
type skepticResponse struct {
	Vote   string `json:"vote"`
	Reason string `json:"reason"`
}

// Run executes the PRB Skeptic adversarial assessment.
//
// Steps:
//  1. Read step-plan.json and integration-map.json from inputs
//  2. Query ecosystem lessons_learned via inputs.EcosystemQuery
//  3. Query ecosystem domain_knowledge for field intelligence (ADR-009a)
//  4. Load system prompt from prompts/prb_skeptic.txt
//  5. Construct user content with plan + integration map + lessons learned + domain knowledge
//  6. Call inference.Route()
//  7. Handle ErrOllamaNotConfigured -> HOLD
//  8. Parse JSON response for vote: GO/NO-GO
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Read step-plan.json.
	if inputs.StepPlan == "" {
		return controller.Signal{
			Controller:  "PRB-SKP",
			SignalValue: "NO-GO",
			Reason:      "step-plan.json path not provided",
			Evidence:    []string{"input:step_plan=missing"},
		}, nil
	}
	stepPlan, err := os.ReadFile(inputs.StepPlan)
	if err != nil {
		return controller.Signal{
			Controller:  "PRB-SKP",
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
				Controller:  "PRB-SKP",
				SignalValue: "NO-GO",
				Reason:      fmt.Sprintf("failed to read integration-map.json: %v", err),
				Evidence:    []string{fmt.Sprintf("file:error=%v", err)},
			}, nil
		}
	}

	// Query ecosystem lessons_learned BEFORE assessment.
	var lessonsLearned string
	if inputs.EcosystemQuery != nil {
		select {
		case <-ctx.Done():
			return controller.Signal{}, ctx.Err()
		default:
		}

		result, err := inputs.EcosystemQuery("lessons_learned", string(stepPlan))
		if err != nil {
			// Lessons learned query failure is non-fatal — proceed without.
			lessonsLearned = fmt.Sprintf("lessons_learned query failed: %v", err)
		} else {
			lessonsLearned = string(result)
		}
	} else {
		lessonsLearned = "lessons_learned: not available (ecosystem query not configured)"
	}

	// ADR-009a Phase 2: Query domain_knowledge for field intelligence.
	var domainKnowledge string
	if inputs.EcosystemQuery != nil {
		select {
		case <-ctx.Done():
			return controller.Signal{}, ctx.Err()
		default:
		}

		result, err := inputs.EcosystemQuery("domain_knowledge", string(stepPlan))
		if err != nil {
			// Domain knowledge query failure is non-fatal — proceed without.
			domainKnowledge = fmt.Sprintf("domain_knowledge query failed: %v", err)
		} else {
			domainKnowledge = string(result)
		}
	} else {
		domainKnowledge = "domain_knowledge: not available (ecosystem query not configured)"
	}

	// Load system prompt.
	promptPath := filepath.Join(c.promptsDir, "prb_skeptic.txt")
	systemPrompt, err := os.ReadFile(promptPath)
	if err != nil {
		return controller.Signal{}, fmt.Errorf("load system prompt: %w", err)
	}

	// Build user content with all inputs including domain knowledge.
	userContent := fmt.Sprintf("## Mission Type\n%s\n\n", inputs.MissionType)
	userContent += fmt.Sprintf("## step-plan.json\n```json\n%s\n```\n", string(stepPlan))
	if len(integrationMap) > 0 {
		userContent += fmt.Sprintf("\n## integration-map.json\n```json\n%s\n```\n", string(integrationMap))
	}
	userContent += fmt.Sprintf("\n## lessons_learned\n%s\n", lessonsLearned)
	userContent += fmt.Sprintf("\n## domain_knowledge (field intelligence)\n%s\n", domainKnowledge)

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Call inference router.
	result, err := inference.Route(ctx, string(systemPrompt), userContent, c.config)
	if errors.Is(err, inference.ErrOllamaNotConfigured) {
		return controller.Signal{
			Controller:  "PRB-SKP",
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
		Controller:     "PRB-SKP",
		SignalValue:    vote,
		Reason:         reason,
		Evidence:       []string{fmt.Sprintf("inference:backend=%s", result.Backend)},
		InferenceNotes: meta,
	}, nil
}

// parseVote extracts the vote from a JSON response. If the response is not
// valid JSON or lacks a vote field, defaults to NO-GO with a parse failure reason.
func parseVote(content string) (vote, reason string) {
	var resp skepticResponse
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
