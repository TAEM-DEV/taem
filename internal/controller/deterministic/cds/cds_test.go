package cds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "CDS" {
		t.Fatalf("expected CDS, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath_NoConflicts(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{"conflicts":[]}`), 0644)

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
}

func TestRun_ResolvableConflicts(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{"conflicts":[{"type":"port","resources":["8080"],"resolvable":true,"resolution":"remap to 8081"}]}`), 0644)

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
}

func TestRun_UnresolvableConflict(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{"conflicts":[{"type":"schema_field","resources":["user_id"],"resolvable":false}]}`), 0644)

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
		t.Fatalf("expected NO-GO, got %s: %s", sig.SignalValue, sig.Reason)
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
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO when no integration map, got %s: %s", sig.SignalValue, sig.Reason)
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
