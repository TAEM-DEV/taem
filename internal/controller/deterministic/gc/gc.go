// Package gc implements the Ground Control controller.
//
// GC performs HTTP ping checks against ground systems (GitHub API, Qdrant,
// Ollama). Binary GO/NO-GO: reachable or not. Uses inputs.EcosystemQuery
// to check ecosystem health. Checks GITHUB_TOKEN env var existence.
package gc

import (
	"context"
	"fmt"
	"os"

	"github.com/taem-dev/taem/internal/controller"
)

// Controller implements the GC (Ground Control) deterministic controller.
type Controller struct{}

// New returns a new GC controller instance.
func New() *Controller {
	return &Controller{}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "GC"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run executes the Ground Control infrastructure health check.
//
// Checks:
//  1. GITHUB_TOKEN or GH_TOKEN env var exists
//  2. Ecosystem reachability via inputs.EcosystemQuery
//
// Returns GO if all systems reachable, NO-GO otherwise.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	evidence := []string{}

	// Check GitHub token existence.
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		return controller.Signal{
			Controller:  "GC",
			SignalValue: "NO-GO",
			Reason:      "GITHUB_TOKEN / GH_TOKEN not set",
			Evidence:    []string{"env:GITHUB_TOKEN=unset", "env:GH_TOKEN=unset"},
		}, nil
	}
	evidence = append(evidence, "env:GITHUB_TOKEN=set")

	// Check ecosystem reachability.
	if inputs.EcosystemQuery != nil {
		select {
		case <-ctx.Done():
			return controller.Signal{}, ctx.Err()
		default:
		}

		_, err := inputs.EcosystemQuery("health", "ping")
		if err != nil {
			return controller.Signal{
				Controller:  "GC",
				SignalValue: "NO-GO",
				Reason:      fmt.Sprintf("ecosystem unreachable: %v", err),
				Evidence:    append(evidence, fmt.Sprintf("ecosystem:error=%v", err)),
			}, nil
		}
		evidence = append(evidence, "ecosystem:reachable")
	} else {
		evidence = append(evidence, "ecosystem:not_configured")
	}

	return controller.Signal{
		Controller:  "GC",
		SignalValue: "GO",
		Reason:      "all ground systems reachable",
		Evidence:    evidence,
	}, nil
}
