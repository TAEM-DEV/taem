package inco

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "INCO" {
		t.Fatalf("expected INCO, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{
		"steps": [
			{"id": "step-1", "name": "Setup", "depends_on": [], "writes": ["config"]},
			{"id": "step-2", "name": "Build", "depends_on": ["step-1"], "writes": ["artifact"]}
		]
	}`), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID:      "test-mission",
		IntegrationMap: mapFile,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO, got %s: %s", sig.SignalValue, sig.Reason)
	}

	// Verify step-plan.json was written.
	planPath := filepath.Join(dir, "step-plan.json")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("step-plan.json not written: %v", err)
	}

	var plan struct {
		Order []string `json:"order"`
	}
	if err := json.Unmarshal(planData, &plan); err != nil {
		t.Fatalf("invalid step-plan.json: %v", err)
	}
	if len(plan.Order) != 2 {
		t.Fatalf("expected 2 steps in order, got %d", len(plan.Order))
	}
	if plan.Order[0] != "step-1" || plan.Order[1] != "step-2" {
		t.Fatalf("expected order [step-1, step-2], got %v", plan.Order)
	}
}

func TestRun_WriteConflict(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{
		"steps": [
			{"id": "step-1", "name": "Writer A", "depends_on": [], "writes": ["shared-config"]},
			{"id": "step-2", "name": "Writer B", "depends_on": [], "writes": ["shared-config"]}
		]
	}`), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID:      "test-mission",
		IntegrationMap: mapFile,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO for write conflict, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_CyclicDependency(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{
		"steps": [
			{"id": "step-1", "name": "A", "depends_on": ["step-2"], "writes": []},
			{"id": "step-2", "name": "B", "depends_on": ["step-1"], "writes": []}
		]
	}`), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID:      "test-mission",
		IntegrationMap: mapFile,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO for cycle, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_NoIntegrationMap(t *testing.T) {
	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO when no integration map, got %s", sig.SignalValue)
	}
}

func TestRun_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New()
	_, err := c.Run(ctx, controller.Inputs{MissionID: "test"})
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}
