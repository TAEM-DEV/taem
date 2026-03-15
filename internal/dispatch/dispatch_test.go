package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/taem-dev/taem/internal/controller"
)

// ---------------------------------------------------------------------------
// Mock controller — implements controller.Controller interface
// ---------------------------------------------------------------------------

type mockController struct {
	name    string
	mode    controller.ControllerMode
	runFunc func(ctx context.Context, inputs controller.Inputs) (controller.Signal, error)
}

func (m *mockController) Name() string                { return m.name }
func (m *mockController) Mode() controller.ControllerMode { return m.mode }
func (m *mockController) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	return m.runFunc(ctx, inputs)
}

// ---------------------------------------------------------------------------
// Test 1: LocalDispatcher runs 3 controllers concurrently, all return GO
// ---------------------------------------------------------------------------

func TestLocalDispatcher_AllGo(t *testing.T) {
	d := NewLocalDispatcher()

	var executed atomic.Int32

	mkCtrl := func(name string) controller.Controller {
		return &mockController{
			name: name,
			mode: controller.ModeDeterministic,
			runFunc: func(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
				executed.Add(1)
				return controller.Signal{
					Controller:  name,
					SignalValue: "GO",
					Reason:      fmt.Sprintf("%s checks passed", name),
					Evidence:    []string{"all clear"},
				}, nil
			},
		}
	}

	controllers := []controller.Controller{
		mkCtrl("GC"),
		mkCtrl("DPS"),
		mkCtrl("EECOM"),
	}

	ctx := context.Background()
	signals, err := d.RunPhase(ctx, controllers, controller.Inputs{MissionID: "test-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d", len(signals))
	}

	if executed.Load() != 3 {
		t.Fatalf("expected 3 controllers executed, got %d", executed.Load())
	}

	for _, sig := range signals {
		if sig.SignalValue != "GO" {
			t.Errorf("expected GO signal from %s, got %s", sig.Controller, sig.SignalValue)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: LocalDispatcher with one controller that errors — verify NO-GO
// ---------------------------------------------------------------------------

func TestLocalDispatcher_OneError(t *testing.T) {
	d := NewLocalDispatcher()

	controllers := []controller.Controller{
		&mockController{
			name: "GC",
			mode: controller.ModeDeterministic,
			runFunc: func(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
				return controller.Signal{
					Controller:  "GC",
					SignalValue: "GO",
					Reason:      "all clear",
				}, nil
			},
		},
		&mockController{
			name: "DPS",
			mode: controller.ModeDeterministic,
			runFunc: func(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
				return controller.Signal{}, errors.New("schema validation failed")
			},
		},
		&mockController{
			name: "EECOM",
			mode: controller.ModeDeterministic,
			runFunc: func(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
				return controller.Signal{
					Controller:  "EECOM",
					SignalValue: "GO",
					Reason:      "rate limits ok",
				}, nil
			},
		},
	}

	ctx := context.Background()
	signals, err := d.RunPhase(ctx, controllers, controller.Inputs{MissionID: "test-002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d", len(signals))
	}

	// Find the DPS signal — it should be NO-GO.
	var dpsSignal *controller.Signal
	for i, sig := range signals {
		if sig.Controller == "DPS" {
			dpsSignal = &signals[i]
			break
		}
	}

	if dpsSignal == nil {
		t.Fatal("expected a signal from DPS controller")
	}

	if dpsSignal.SignalValue != "NO-GO" {
		t.Errorf("expected NO-GO for errored controller, got %s", dpsSignal.SignalValue)
	}

	if dpsSignal.Reason == "" {
		t.Error("expected reason to contain error description")
	}
}

// ---------------------------------------------------------------------------
// Test 3: LocalDispatcher respects context cancellation
// ---------------------------------------------------------------------------

func TestLocalDispatcher_ContextCancellation(t *testing.T) {
	d := NewLocalDispatcher()

	// Create a context that we cancel almost immediately.
	ctx, cancel := context.WithCancel(context.Background())

	var started atomic.Int32

	slowCtrl := &mockController{
		name: "SLOW",
		mode: controller.ModeDeterministic,
		runFunc: func(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
			started.Add(1)
			// Block until context is cancelled.
			select {
			case <-ctx.Done():
				return controller.Signal{
					Controller:  "SLOW",
					SignalValue: "NO-GO",
					Reason:      "cancelled",
				}, nil
			case <-time.After(10 * time.Second):
				return controller.Signal{
					Controller:  "SLOW",
					SignalValue: "GO",
					Reason:      "completed",
				}, nil
			}
		},
	}

	fastCtrl := &mockController{
		name: "FAST",
		mode: controller.ModeDeterministic,
		runFunc: func(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
			started.Add(1)
			return controller.Signal{
				Controller:  "FAST",
				SignalValue: "GO",
				Reason:      "done",
			}, nil
		},
	}

	controllers := []controller.Controller{slowCtrl, fastCtrl}

	// Cancel after a short delay so the slow controller sees it.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	signals, err := d.RunPhase(ctx, controllers, controller.Inputs{MissionID: "test-003"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both controllers should have started and returned a signal.
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}

	if started.Load() != 2 {
		t.Fatalf("expected 2 controllers started, got %d", started.Load())
	}

	// The slow controller should have responded to cancellation (NO-GO with "cancelled").
	for _, sig := range signals {
		if sig.Controller == "SLOW" && sig.SignalValue != "NO-GO" {
			t.Errorf("expected SLOW controller to return NO-GO on cancellation, got %s", sig.SignalValue)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 4: GitHubDispatcher.Dispatch sends correct HTTP request
// ---------------------------------------------------------------------------

func TestGitHubDispatcher_Dispatch(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]interface{}
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		receivedBody = body

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	d := &GitHubDispatcher{
		token:   "ghp_test_token",
		appID:   "12345",
		owner:   "taem-dev",
		repo:    "mc-state",
		client:  server.Client(),
		baseURL: server.URL,
	}

	inputs := map[string]string{
		"mission_id": "M-001",
		"phase":      "02",
	}

	err := d.Dispatch(context.Background(), "nav.yml", inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request path.
	expectedPath := "/repos/taem-dev/mc-state/actions/workflows/nav.yml/dispatches"
	if receivedPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, receivedPath)
	}

	// Verify authorization header.
	if receivedAuth != "Bearer ghp_test_token" {
		t.Errorf("expected Bearer token, got %s", receivedAuth)
	}

	// Verify the body contains ref and inputs.
	if ref, ok := receivedBody["ref"].(string); !ok || ref != "main" {
		t.Errorf("expected ref=main, got %v", receivedBody["ref"])
	}

	bodyInputs, ok := receivedBody["inputs"].(map[string]interface{})
	if !ok {
		t.Fatal("expected inputs in request body")
	}
	if bodyInputs["mission_id"] != "M-001" {
		t.Errorf("expected mission_id=M-001, got %v", bodyInputs["mission_id"])
	}
	if bodyInputs["phase"] != "02" {
		t.Errorf("expected phase=02, got %v", bodyInputs["phase"])
	}
}

// ---------------------------------------------------------------------------
// Test 5: GitHubDispatcher.WaitForRun detects completion
// ---------------------------------------------------------------------------

func TestGitHubDispatcher_WaitForRun(t *testing.T) {
	var pollCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := pollCount.Add(1)

		var run workflowRun
		if count < 3 {
			// First two polls: still in progress.
			run = workflowRun{
				ID:     42,
				Status: "in_progress",
			}
		} else {
			// Third poll: completed with success.
			conclusion := "success"
			run = workflowRun{
				ID:         42,
				Status:     "completed",
				Conclusion: &conclusion,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(run)
	}))
	defer server.Close()

	d := &GitHubDispatcher{
		token:        "ghp_test_token",
		appID:        "12345",
		owner:        "taem-dev",
		repo:         "mc-state",
		client:       server.Client(),
		baseURL:      server.URL,
		pollInterval: 10 * time.Millisecond, // fast polling for tests
	}

	// Use a context with timeout so the test doesn't hang.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conclusion, err := d.WaitForRun(ctx, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conclusion != "success" {
		t.Errorf("expected conclusion=success, got %s", conclusion)
	}

	if pollCount.Load() < 3 {
		t.Errorf("expected at least 3 polls, got %d", pollCount.Load())
	}
}
