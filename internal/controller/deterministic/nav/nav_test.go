package nav

import (
	"context"
	"fmt"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "NAV" {
		t.Fatalf("expected NAV, got %s", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New()
	if c.Mode() != controller.ModeDeterministic {
		t.Fatalf("expected deterministic, got %s", c.Mode())
	}
}

func TestRun_HappyPath_EcosystemReachable(t *testing.T) {
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

func TestRun_EcosystemUnreachable_HOLD(t *testing.T) {
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
	if sig.SignalValue != "HOLD" {
		t.Fatalf("expected HOLD, got %s: %s", sig.SignalValue, sig.Reason)
	}
	if sig.ConstraintRef == nil || *sig.ConstraintRef != "C-006-004" {
		t.Fatal("expected constraint_ref=C-006-004")
	}
}

func TestRun_NoEcosystemQuery_HOLD(t *testing.T) {
	c := New()
	inputs := controller.Inputs{
		MissionID: "test-mission",
		// EcosystemQuery is nil.
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "HOLD" {
		t.Fatalf("expected HOLD, got %s: %s", sig.SignalValue, sig.Reason)
	}
	if sig.ConstraintRef == nil || *sig.ConstraintRef != "C-006-004" {
		t.Fatal("expected constraint_ref=C-006-004")
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
