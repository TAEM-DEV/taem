// Package arch implements the Architecture Officer controller.
//
// ARCH loads ADR constraints via inputs.EcosystemQuery, evaluates check
// patterns against integration-map.json, and emits GO or NO-GO. HARD
// constraint violation = NO-GO with constraint_ref citation.
package arch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/taem-dev/taem/internal/controller"
)

// constraint represents a single ADR constraint loaded from the ecosystem.
type constraint struct {
	ID      string   `json:"id"`
	ADR     string   `json:"adr"`
	Type    string   `json:"type"` // HARD or SOFT
	Check   string   `json:"check"`
	Banned  []string `json:"banned,omitempty"`
}

// constraintIndex is the response from the ecosystem constraint_index collection.
type constraintIndex struct {
	Constraints []constraint `json:"constraints"`
}

// integrationMap represents the structure of integration-map.json.
type integrationMap struct {
	Wiring []struct {
		Tool     string   `json:"tool"`
		Protocol string   `json:"protocol"`
		Ports    []int    `json:"ports,omitempty"`
		Patterns []string `json:"patterns,omitempty"`
	} `json:"wiring"`
	RawContent string `json:"-"` // raw file content for pattern matching
}

// Controller implements the ARCH (Architecture Officer) deterministic controller.
type Controller struct{}

// New returns a new ARCH controller instance.
func New() *Controller {
	return &Controller{}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "ARCH"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run evaluates ADR constraints against the integration map.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	// Load constraints from ecosystem.
	if inputs.EcosystemQuery == nil {
		return controller.Signal{
			Controller:  "ARCH",
			SignalValue: "NO-GO",
			Reason:      "EcosystemQuery not available — cannot load ADR constraints",
			Evidence:    []string{"ecosystem_query=nil"},
		}, nil
	}

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	constraintData, err := inputs.EcosystemQuery("constraint_index", "all")
	if err != nil {
		return controller.Signal{
			Controller:  "ARCH",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to load constraints: %v", err),
			Evidence:    []string{fmt.Sprintf("constraint_load_error=%v", err)},
		}, nil
	}

	var idx constraintIndex
	if err := json.Unmarshal(constraintData, &idx); err != nil {
		return controller.Signal{
			Controller:  "ARCH",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("invalid constraint index: %v", err),
			Evidence:    []string{fmt.Sprintf("parse_error=%v", err)},
		}, nil
	}

	// Load integration map.
	if inputs.IntegrationMap == "" {
		return controller.Signal{
			Controller:  "ARCH",
			SignalValue: "GO",
			Reason:      "no integration map yet — skipping constraint check",
			Evidence:    []string{"integration_map=not_present"},
		}, nil
	}

	mapData, err := os.ReadFile(inputs.IntegrationMap)
	if err != nil {
		return controller.Signal{
			Controller:  "ARCH",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read integration map: %v", err),
			Evidence:    []string{fmt.Sprintf("read_error=%v", err)},
		}, nil
	}
	mapContent := string(mapData)

	evidence := []string{}

	// Evaluate each constraint's banned patterns against the integration map.
	for _, con := range idx.Constraints {
		select {
		case <-ctx.Done():
			return controller.Signal{}, ctx.Err()
		default:
		}

		for _, banned := range con.Banned {
			if strings.Contains(strings.ToLower(mapContent), strings.ToLower(banned)) {
				ref := con.ID
				if con.Type == "HARD" {
					return controller.Signal{
						Controller:    "ARCH",
						SignalValue:   "NO-GO",
						Reason:        fmt.Sprintf("HARD constraint violation: %s bans '%s'", con.ID, banned),
						Evidence:      append(evidence, fmt.Sprintf("violation=%s,banned=%s,type=HARD", con.ID, banned)),
						ConstraintRef: &ref,
					}, nil
				}
				// SOFT violations are warnings, not blocking.
				evidence = append(evidence, fmt.Sprintf("soft_violation=%s,banned=%s", con.ID, banned))
			}
		}
	}

	if len(evidence) > 0 {
		return controller.Signal{
			Controller:  "ARCH",
			SignalValue: "GO",
			Reason:      "no HARD constraint violations (soft warnings noted)",
			Evidence:    evidence,
		}, nil
	}

	return controller.Signal{
		Controller:  "ARCH",
		SignalValue: "GO",
		Reason:      "all ADR constraints satisfied",
		Evidence:    []string{fmt.Sprintf("constraints_checked=%d", len(idx.Constraints))},
	}, nil
}
