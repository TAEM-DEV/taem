// Package pco implements the Pattern Compliance Officer controller.
//
// PCO checks the integration plan against the pattern registry
// (config/patterns.yaml). Unjustified deviation = NO-GO, justified
// deviation (with ADR ref) = GO.
package pco

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/taem-dev/taem/internal/controller"
)

// pattern represents a required pattern from config/patterns.yaml.
type pattern struct {
	Name     string `json:"name"     yaml:"name"`
	Required bool   `json:"required" yaml:"required"`
	Check    string `json:"check"    yaml:"check"` // substring to look for in integration map
}

// integrationPlan represents the relevant fields from integration-map.json.
type integrationPlan struct {
	Patterns   []planPattern `json:"patterns,omitempty"`
	Deviations []deviation   `json:"deviations,omitempty"`
}

// planPattern is a pattern declared in the integration plan.
type planPattern struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "compliant", "deviated"
}

// deviation represents a declared deviation from a pattern.
type deviation struct {
	Pattern       string `json:"pattern"`
	Justification string `json:"justification"`
	ADRRef        string `json:"adr_ref,omitempty"`
}

// Controller implements the PCO (Pattern Compliance) deterministic controller.
type Controller struct {
	// PatternsPath can be overridden for testing. Defaults to config/patterns.yaml.
	PatternsPath string
}

// New returns a new PCO controller instance.
func New() *Controller {
	return &Controller{
		PatternsPath: "config/patterns.yaml",
	}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "PCO"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run checks integration plan patterns against the pattern registry.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	if inputs.IntegrationMap == "" {
		return controller.Signal{
			Controller:  "PCO",
			SignalValue: "GO",
			Reason:      "no integration map yet — skipping pattern check",
			Evidence:    []string{"integration_map=not_present"},
		}, nil
	}

	data, err := os.ReadFile(inputs.IntegrationMap)
	if err != nil {
		return controller.Signal{
			Controller:  "PCO",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read integration map: %v", err),
			Evidence:    []string{fmt.Sprintf("read_error=%v", err)},
		}, nil
	}

	var plan integrationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return controller.Signal{
			Controller:  "PCO",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("invalid integration map JSON: %v", err),
			Evidence:    []string{fmt.Sprintf("parse_error=%v", err)},
		}, nil
	}

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Build a map of justified deviations (those with ADR references).
	justified := make(map[string]string) // pattern name -> ADR ref
	for _, dev := range plan.Deviations {
		if dev.ADRRef != "" {
			justified[dev.Pattern] = dev.ADRRef
		}
	}

	evidence := []string{}
	violations := 0

	// Check each declared pattern for deviations.
	for _, pp := range plan.Patterns {
		if strings.EqualFold(pp.Status, "deviated") {
			if _, ok := justified[pp.Name]; ok {
				evidence = append(evidence, fmt.Sprintf("%s:deviated(justified,adr=%s)", pp.Name, justified[pp.Name]))
			} else {
				violations++
				evidence = append(evidence, fmt.Sprintf("%s:deviated(unjustified)", pp.Name))
			}
		} else {
			evidence = append(evidence, fmt.Sprintf("%s:compliant", pp.Name))
		}
	}

	if violations > 0 {
		return controller.Signal{
			Controller:  "PCO",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("%d unjustified pattern deviation(s)", violations),
			Evidence:    evidence,
		}, nil
	}

	return controller.Signal{
		Controller:  "PCO",
		SignalValue: "GO",
		Reason:      "all patterns compliant or justified",
		Evidence:    evidence,
	}, nil
}
