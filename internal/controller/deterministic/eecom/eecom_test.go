package eecom

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

// mockTransport implements HTTPClient for testing.
type mockClient struct {
	body    string
	err     error
	status  int
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(bytes.NewBufferString(m.body)),
	}, nil
}

func TestName(t *testing.T) {
	c := New()
	if c.Name() != "EECOM" {
		t.Fatalf("expected EECOM, got %s", c.Name())
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

	c := &Controller{
		Client: &mockClient{
			body: `{"resources":{"core":{"limit":5000,"remaining":4500}}}`,
			status: 200,
		},
		BaseURL: "https://api.github.com",
	}

	sig, err := c.Run(context.Background(), controller.Inputs{MissionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_NOGO_LowHeadroom(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test123")

	c := &Controller{
		Client: &mockClient{
			body: `{"resources":{"core":{"limit":5000,"remaining":100}}}`,
			status: 200,
		},
		BaseURL: "https://api.github.com",
	}

	sig, err := c.Run(context.Background(), controller.Inputs{MissionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_WARN_MediumHeadroom(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test123")

	c := &Controller{
		Client: &mockClient{
			body: `{"resources":{"core":{"limit":5000,"remaining":1000}}}`,
			status: 200,
		},
		BaseURL: "https://api.github.com",
	}

	sig, err := c.Run(context.Background(), controller.Inputs{MissionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "WARN" {
		t.Fatalf("expected WARN, got %s: %s", sig.SignalValue, sig.Reason)
	}
}

func TestRun_Unreachable(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test123")

	c := &Controller{
		Client: &mockClient{
			err: fmt.Errorf("connection refused"),
		},
		BaseURL: "https://api.github.com",
	}

	sig, err := c.Run(context.Background(), controller.Inputs{MissionID: "test"})
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
