// Package nav implements the NAV (Dependency Navigator) controller stub.
//
// NAV is mode: deterministic but execution: github_dispatch. The real
// logic runs via GitHub Actions. This local stub checks ecosystem
// reachability; if unreachable, it emits HOLD per ADR-006 C-006-004.
// Otherwise the kernel dispatches the real NAV workflow.
package nav

import (
	"context"
	"fmt"

	"github.com/taem-dev/taem/internal/controller"
)

// Controller implements the NAV (Dependency Navigator) deterministic controller stub.
type Controller struct{}

// New returns a new NAV controller instance.
func New() *Controller {
	return &Controller{}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "NAV"
}

// Mode returns ModeDeterministic.
//
// NAV's mode is deterministic per controllers.yaml, even though its
// execution is github_dispatch. The stub runs locally in the kernel
// to verify ecosystem reachability before dispatch.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run checks ecosystem reachability. If unreachable, returns HOLD
// per ADR-006 C-006-004. If reachable, returns GO to signal that
// the kernel should dispatch the real NAV workflow.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	if inputs.EcosystemQuery == nil {
		ref := "C-006-004"
		return controller.Signal{
			Controller:    "NAV",
			SignalValue:   "HOLD",
			Reason:        "ecosystem query not available — HOLD per ADR-006 C-006-004",
			Evidence:      []string{"ecosystem_query=nil"},
			ConstraintRef: &ref,
		}, nil
	}

	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	_, err := inputs.EcosystemQuery("repo_surfaces", "ping")
	if err != nil {
		ref := "C-006-004"
		return controller.Signal{
			Controller:    "NAV",
			SignalValue:   "HOLD",
			Reason:        fmt.Sprintf("ecosystem unreachable — HOLD per ADR-006 C-006-004: %v", err),
			Evidence:      []string{fmt.Sprintf("ecosystem:error=%v", err)},
			ConstraintRef: &ref,
		}, nil
	}

	return controller.Signal{
		Controller:  "NAV",
		SignalValue: "GO",
		Reason:      "ecosystem reachable — ready for dispatch",
		Evidence:    []string{"ecosystem:reachable", "dispatch:ready"},
	}, nil
}
