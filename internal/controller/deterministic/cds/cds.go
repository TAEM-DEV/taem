// Package cds implements the Conflict Detection Systems controller.
//
// CDS reads the conflicts[] array from integration-map.json, performs set
// intersection on tool names, port numbers, and schema fields. GO if no
// conflicts, NO-GO if unresolvable conflicts.
package cds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/taem-dev/taem/internal/controller"
)

// conflict represents a conflict entry from integration-map.json.
type conflict struct {
	Type       string   `json:"type"`       // "port", "tool", "schema_field"
	Resources  []string `json:"resources"`
	Resolvable bool     `json:"resolvable"`
	Resolution string   `json:"resolution,omitempty"`
}

// integrationMapConflicts is the relevant portion of integration-map.json.
type integrationMapConflicts struct {
	Conflicts []conflict `json:"conflicts"`
}

// Controller implements the CDS (Conflict Detection) deterministic controller.
type Controller struct{}

// New returns a new CDS controller instance.
func New() *Controller {
	return &Controller{}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "CDS"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run reads conflicts from integration-map.json and evaluates them.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	if inputs.IntegrationMap == "" {
		return controller.Signal{
			Controller:  "CDS",
			SignalValue: "GO",
			Reason:      "no integration map yet — no conflicts to check",
			Evidence:    []string{"integration_map=not_present"},
		}, nil
	}

	data, err := os.ReadFile(inputs.IntegrationMap)
	if err != nil {
		return controller.Signal{
			Controller:  "CDS",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read integration map: %v", err),
			Evidence:    []string{fmt.Sprintf("read_error=%v", err)},
		}, nil
	}

	var imap integrationMapConflicts
	if err := json.Unmarshal(data, &imap); err != nil {
		return controller.Signal{
			Controller:  "CDS",
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

	evidence := []string{fmt.Sprintf("total_conflicts=%d", len(imap.Conflicts))}
	unresolvable := 0

	for _, conflict := range imap.Conflicts {
		if !conflict.Resolvable {
			unresolvable++
			evidence = append(evidence, fmt.Sprintf(
				"unresolvable:type=%s,resources=%v",
				conflict.Type, conflict.Resources,
			))
		}
	}

	if unresolvable > 0 {
		return controller.Signal{
			Controller:  "CDS",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("%d unresolvable conflict(s) detected", unresolvable),
			Evidence:    evidence,
		}, nil
	}

	return controller.Signal{
		Controller:  "CDS",
		SignalValue: "GO",
		Reason:      fmt.Sprintf("no unresolvable conflicts (%d total conflicts, all resolvable)", len(imap.Conflicts)),
		Evidence:    evidence,
	}, nil
}
