package fao

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "FAO" {
		t.Fatalf("expected FAO, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath_ADVISORY(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 15, 12, 0, 30, 0, time.UTC)

	// Write a manifest with a phase start 10 seconds ago.
	manifest := `{"mission_id":"m-001","phase":0,"event":"start","timestamp":"2026-03-15T12:00:20Z"}
`
	os.WriteFile(filepath.Join(dir, "manifest.jsonl"), []byte(manifest), 0644)

	c := &Controller{Now: func() time.Time { return now }}
	inputs := controller.Inputs{
		MissionID:    "test-mission",
		ManifestPath: dir,
		Phase:        0,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "ADVISORY" {
		t.Fatalf("expected ADVISORY, got %s: %s", sig.SignalValue, sig.Reason)
	}
	if sig.Controller != "FAO" {
		t.Fatalf("expected controller=FAO, got %s", sig.Controller)
	}
}

func TestRun_ExceededWindow(t *testing.T) {
	dir := t.TempDir()
	// Phase 0 expected window is 30s. Set elapsed to 60s.
	now := time.Date(2026, 3, 15, 12, 1, 0, 0, time.UTC)

	manifest := `{"mission_id":"m-001","phase":0,"event":"start","timestamp":"2026-03-15T12:00:00Z"}
`
	os.WriteFile(filepath.Join(dir, "manifest.jsonl"), []byte(manifest), 0644)

	c := &Controller{Now: func() time.Time { return now }}
	inputs := controller.Inputs{
		MissionID:    "test-mission",
		ManifestPath: dir,
		Phase:        0,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// FAO always emits ADVISORY regardless.
	if sig.SignalValue != "ADVISORY" {
		t.Fatalf("expected ADVISORY, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_NoManifest(t *testing.T) {
	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
		Phase:     0,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "ADVISORY" {
		t.Fatalf("expected ADVISORY, got %s: %s", sig.SignalValue, sig.Reason)
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
