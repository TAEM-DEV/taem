// Package trc implements the Test Readiness Controller.
//
// TRC performs file existence checks for test harnesses. All steps in
// step-plan.json must have a non-null test_harness field. GO if all
// steps have tests, NO-GO if any missing.
package trc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/taem-dev/taem/internal/controller"
)

// planStep represents a step entry in step-plan.json.
type planStep struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	TestHarness *string `json:"test_harness"`
}

// plan represents the step-plan.json structure.
type plan struct {
	Steps []planStep `json:"steps"`
}

// Controller implements the TRC (Test Readiness) deterministic controller.
type Controller struct{}

// New returns a new TRC controller instance.
func New() *Controller {
	return &Controller{}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "TRC"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run checks that every step in step-plan.json has a non-null test_harness field.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	if inputs.StepPlan == "" {
		return controller.Signal{
			Controller:  "TRC",
			SignalValue: "NO-GO",
			Reason:      "step-plan.json not available",
			Evidence:    []string{"step_plan=not_present"},
		}, nil
	}

	data, err := os.ReadFile(inputs.StepPlan)
	if err != nil {
		return controller.Signal{
			Controller:  "TRC",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read step-plan.json: %v", err),
			Evidence:    []string{fmt.Sprintf("read_error=%v", err)},
		}, nil
	}

	var p plan
	if err := json.Unmarshal(data, &p); err != nil {
		return controller.Signal{
			Controller:  "TRC",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("invalid step-plan.json: %v", err),
			Evidence:    []string{fmt.Sprintf("parse_error=%v", err)},
		}, nil
	}

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	evidence := []string{fmt.Sprintf("total_steps=%d", len(p.Steps))}
	missing := []string{}

	for _, s := range p.Steps {
		if s.TestHarness == nil || *s.TestHarness == "" {
			missing = append(missing, s.ID)
			evidence = append(evidence, fmt.Sprintf("%s:test_harness=missing", s.ID))
		} else {
			evidence = append(evidence, fmt.Sprintf("%s:test_harness=%s", s.ID, *s.TestHarness))
		}
	}

	if len(missing) > 0 {
		return controller.Signal{
			Controller:  "TRC",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("%d step(s) missing test harness: %v", len(missing), missing),
			Evidence:    evidence,
		}, nil
	}

	return controller.Signal{
		Controller:  "TRC",
		SignalValue: "GO",
		Reason:      fmt.Sprintf("all %d steps have test harnesses", len(p.Steps)),
		Evidence:    evidence,
	}, nil
}
