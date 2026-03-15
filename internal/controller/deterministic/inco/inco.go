// Package inco implements the Integration Sequencer controller.
//
// INCO performs topological sort of integration steps from
// integration-map.json, detects shared-interface write conflicts,
// and produces step-plan.json. GO when step-plan written with no
// ordering conflicts.
package inco

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taem-dev/taem/internal/controller"
)

// step represents a single integration step from the integration map.
type step struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DependsOn   []string `json:"depends_on"`
	Writes      []string `json:"writes,omitempty"`
	TestHarness *string  `json:"test_harness,omitempty"`
}

// integrationMapSteps is the relevant portion of integration-map.json.
type integrationMapSteps struct {
	Steps []step `json:"steps"`
}

// stepPlan is the output format for step-plan.json.
type stepPlan struct {
	MissionID string `json:"mission_id"`
	Steps     []step `json:"steps"`
	Order     []string `json:"order"` // topologically sorted step IDs
}

// Controller implements the INCO (Integration Sequencer) deterministic controller.
type Controller struct{}

// New returns a new INCO controller instance.
func New() *Controller {
	return &Controller{}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "INCO"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run performs topological sort of integration steps and writes step-plan.json.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	if inputs.IntegrationMap == "" {
		return controller.Signal{
			Controller:  "INCO",
			SignalValue: "NO-GO",
			Reason:      "no integration map available",
			Evidence:    []string{"integration_map=not_present"},
		}, nil
	}

	data, err := os.ReadFile(inputs.IntegrationMap)
	if err != nil {
		return controller.Signal{
			Controller:  "INCO",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read integration map: %v", err),
			Evidence:    []string{fmt.Sprintf("read_error=%v", err)},
		}, nil
	}

	var imap integrationMapSteps
	if err := json.Unmarshal(data, &imap); err != nil {
		return controller.Signal{
			Controller:  "INCO",
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

	// Detect shared-interface write conflicts.
	writeOwners := make(map[string]string) // interface -> step ID
	for _, s := range imap.Steps {
		for _, w := range s.Writes {
			if existing, conflict := writeOwners[w]; conflict {
				return controller.Signal{
					Controller:  "INCO",
					SignalValue: "NO-GO",
					Reason:      fmt.Sprintf("write conflict on interface '%s' between steps '%s' and '%s'", w, existing, s.ID),
					Evidence: []string{
						fmt.Sprintf("conflict_interface=%s", w),
						fmt.Sprintf("step_a=%s", existing),
						fmt.Sprintf("step_b=%s", s.ID),
					},
				}, nil
			}
			writeOwners[w] = s.ID
		}
	}

	// Topological sort.
	order, err := topoSort(imap.Steps)
	if err != nil {
		return controller.Signal{
			Controller:  "INCO",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("topological sort failed: %v", err),
			Evidence:    []string{fmt.Sprintf("topo_error=%v", err)},
		}, nil
	}

	// Write step-plan.json to the same directory as integration-map.json.
	plan := stepPlan{
		MissionID: inputs.MissionID,
		Steps:     imap.Steps,
		Order:     order,
	}

	planData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return controller.Signal{
			Controller:  "INCO",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to marshal step-plan: %v", err),
			Evidence:    []string{fmt.Sprintf("marshal_error=%v", err)},
		}, nil
	}

	planPath := filepath.Join(filepath.Dir(inputs.IntegrationMap), "step-plan.json")
	if err := os.WriteFile(planPath, planData, 0644); err != nil {
		return controller.Signal{
			Controller:  "INCO",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to write step-plan.json: %v", err),
			Evidence:    []string{fmt.Sprintf("write_error=%v", err)},
		}, nil
	}

	return controller.Signal{
		Controller:  "INCO",
		SignalValue: "GO",
		Reason:      fmt.Sprintf("step-plan.json written with %d steps in order", len(order)),
		Evidence: []string{
			fmt.Sprintf("steps=%d", len(imap.Steps)),
			fmt.Sprintf("plan_path=%s", planPath),
		},
	}, nil
}

// topoSort performs a topological sort using Kahn's algorithm.
// Returns the sorted step IDs or an error if a cycle is detected.
func topoSort(steps []step) ([]string, error) {
	// Build adjacency list and in-degree map.
	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	ids := make(map[string]bool)

	for _, s := range steps {
		ids[s.ID] = true
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			adj[dep] = append(adj[dep], s.ID)
			inDegree[s.ID]++
		}
	}

	// Find all nodes with in-degree 0.
	var queue []string
	for _, s := range steps {
		if inDegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, neighbor := range adj[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(steps) {
		return nil, fmt.Errorf("cycle detected: sorted %d of %d steps", len(order), len(steps))
	}

	return order, nil
}
