package trc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "TRC" {
		t.Fatalf("expected TRC, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath_AllHaveTests(t *testing.T) {
	dir := t.TempDir()
	planFile := filepath.Join(dir, "step-plan.json")
	os.WriteFile(planFile, []byte(`{
		"steps": [
			{"id": "step-1", "name": "Setup", "test_harness": "test/setup_test.go"},
			{"id": "step-2", "name": "Build", "test_harness": "test/build_test.go"}
		]
	}`), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
		StepPlan:  planFile,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_MissingTestHarness(t *testing.T) {
	dir := t.TempDir()
	planFile := filepath.Join(dir, "step-plan.json")
	os.WriteFile(planFile, []byte(`{
		"steps": [
			{"id": "step-1", "name": "Setup", "test_harness": "test/setup_test.go"},
			{"id": "step-2", "name": "Build", "test_harness": null}
		]
	}`), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
		StepPlan:  planFile,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_NoStepPlan(t *testing.T) {
	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO, got %s: %s", sig.SignalValue, sig.Reason)
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
