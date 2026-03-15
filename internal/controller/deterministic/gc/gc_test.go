package gc

import (
	"context"
	"fmt"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "GC" {
		t.Fatalf("expected GC, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test123")

	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			return []byte(`{"status":"ok"}`), nil
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

func TestRun_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

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

func TestRun_EcosystemUnreachable(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test123")

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
	inputs := controller.Inputs{MissionID: "test-mission"}

	_, err := c.Run(ctx, inputs)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}
