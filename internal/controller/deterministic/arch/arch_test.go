package arch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "ARCH" {
		t.Fatalf("expected ARCH, got %s", c.Name())
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
	os.WriteFile(mapFile, []byte(`{"wiring":[{"tool":"mcp-server","protocol":"unicast"}]}`), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID:      "test-mission",
		IntegrationMap: mapFile,
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			return []byte(`{"constraints":[{"id":"C-001-001","adr":"ADR-001","type":"HARD","check":"no multicast","banned":["multicast","broadcast"]}]}`), nil
		},
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_HardViolation(t *testing.T) {
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "integration-map.json")
	// Integration map contains "multicast" which is banned.
	os.WriteFile(mapFile, []byte(`{"wiring":[{"tool":"mesh","protocol":"multicast"}]}`), 0644)

	c := New()
	inputs := controller.Inputs{
		MissionID:      "test-mission",
		IntegrationMap: mapFile,
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			return []byte(`{"constraints":[{"id":"C-001-001","adr":"ADR-001","type":"HARD","check":"no multicast","banned":["multicast","broadcast"]}]}`), nil
		},
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
	if sig.ConstraintRef == nil {
		t.Fatal("expected constraint_ref to be set")
	}
	if *sig.ConstraintRef != "C-001-001" {
		t.Fatalf("expected constraint_ref=C-001-001, got %s", *sig.ConstraintRef)
	}
}

func TestRun_NoEcosystemQuery(t *testing.T) {
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

func TestRun_EcosystemError(t *testing.T) {
	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			return nil, fmt.Errorf("connection refused")
		},
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
