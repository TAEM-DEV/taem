package pco

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "PCO" {
		t.Fatalf("expected PCO, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath_AllCompliant(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{
		"patterns": [
			{"name": "mcp-server", "status": "compliant"},
			{"name": "jsonl-state", "status": "compliant"}
		],
		"deviations": []
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
}

func TestRun_JustifiedDeviation(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{
		"patterns": [
			{"name": "mcp-server", "status": "deviated"},
			{"name": "jsonl-state", "status": "compliant"}
		],
		"deviations": [
			{"pattern": "mcp-server", "justification": "using gRPC per ADR-008", "adr_ref": "ADR-008"}
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
}

func TestRun_UnjustifiedDeviation(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	os.WriteFile(mapFile, []byte(`{
		"patterns": [
			{"name": "mcp-server", "status": "deviated"}
		],
		"deviations": []
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
		t.Fatalf("expected GO when no integration map, got %s", sig.SignalValue)
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
