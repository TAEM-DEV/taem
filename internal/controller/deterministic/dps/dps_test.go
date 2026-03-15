package dps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "DPS" {
		t.Fatalf("expected DPS, got %s", c.Name())
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

	// Write valid JSONL files.
	signalsData := `{"controller":"GC","signal":"GO","reason":"ok","evidence":[]}
{"controller":"DPS","signal":"GO","reason":"ok","evidence":[]}
`
	manifestData := `{"mission_id":"m-001","phase":0,"event":"start"}
`
	os.WriteFile(filepath.Join(dir, "signals.jsonl"), []byte(signalsData), 0644)
	os.WriteFile(filepath.Join(dir, "manifest.jsonl"), []byte(manifestData), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID:    "test-mission",
		ManifestPath: dir,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	// Write invalid JSONL.
	os.WriteFile(filepath.Join(dir, "signals.jsonl"), []byte("not valid json\n"), 0644)
	os.WriteFile(filepath.Join(dir, "manifest.jsonl"), []byte(`{"ok":true}`+"\n"), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID:    "test-mission",
		ManifestPath: dir,
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_EmptyManifestPath(t *testing.T) {
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
	inputs := controller.Inputs{MissionID: "test-mission", ManifestPath: "/nonexistent"}

	_, err := c.Run(ctx, inputs)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}
