package capcom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "CAPCOM" {
		t.Fatalf("expected CAPCOM, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath_RELAY(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	// Write signals.
	signals := `{"controller":"GC","signal":"GO","reason":"all systems go","evidence":[]}
{"controller":"DPS","signal":"GO","reason":"schemas valid","evidence":[]}
`
	os.WriteFile(filepath.Join(dir, "signals.jsonl"), []byte(signals), 0644)

	// Write step-plan.
	planDir := t.TempDir()
	planFile := filepath.Join(planDir, "step-plan.json")
	os.WriteFile(planFile, []byte(`{
		"mission_id": "m-001",
		"steps": [{"id":"step-1","name":"Setup"},{"id":"step-2","name":"Build"}],
		"order": ["step-1","step-2"]
	}`), 0644)

	c := &Controller{Now: func() time.Time { return now }}
	inputs := controller.Inputs{
		MissionID:    "m-001",
		ManifestPath: dir,
		StepPlan:     planFile,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "RELAY" {
		t.Fatalf("expected RELAY, got %s", sig.SignalValue)
	}
	if sig.Controller != "CAPCOM" {
		t.Fatalf("expected controller=CAPCOM, got %s", sig.Controller)
	}
	if !strings.Contains(sig.Reason, "LANDED") {
		t.Fatalf("expected LANDED.md content in reason, got: %s", sig.Reason[:100])
	}
	if !strings.Contains(sig.Reason, "m-001") {
		t.Fatalf("expected mission ID in output, got: %s", sig.Reason[:100])
	}
}

func TestRun_NoData_StillRELAY(t *testing.T) {
	c := New()
	inputs := controller.Inputs{
		MissionID: "m-empty",
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CAPCOM always emits RELAY — it never blocks.
	if sig.SignalValue != "RELAY" {
		t.Fatalf("expected RELAY, got %s: %s", sig.SignalValue, sig.Reason)
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
