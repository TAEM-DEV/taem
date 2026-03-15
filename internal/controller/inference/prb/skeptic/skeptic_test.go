package skeptic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
	"github.com/taem-dev/taem/internal/controller/inference"
)

// ollamaResponse mirrors the Ollama API response shape for test servers.
type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// setupTestFiles creates temporary step-plan.json, integration-map.json,
// and prompts/prb_skeptic.txt for use in tests.
func setupTestFiles(t *testing.T) (stepPlanPath, integrationMapPath, promptsDir string) {
	t.Helper()

	dir := t.TempDir()

	stepPlan := `{"steps": [{"id": "step-001", "action": "create file"}]}`
	stepPlanPath = filepath.Join(dir, "step-plan.json")
	if err := os.WriteFile(stepPlanPath, []byte(stepPlan), 0644); err != nil {
		t.Fatal(err)
	}

	integrationMap := `{"repos": ["test-repo"], "wiring": []}`
	integrationMapPath = filepath.Join(dir, "integration-map.json")
	if err := os.WriteFile(integrationMapPath, []byte(integrationMap), 0644); err != nil {
		t.Fatal(err)
	}

	promptsDir = filepath.Join(dir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	promptContent := "You are the Skeptic on the TAEM Peer Review Board."
	if err := os.WriteFile(filepath.Join(promptsDir, "prb_skeptic.txt"), []byte(promptContent), 0644); err != nil {
		t.Fatal(err)
	}

	return stepPlanPath, integrationMapPath, promptsDir
}

func TestName(t *testing.T) {
	c := New(inference.InferenceConfig{}, "/tmp")
	if c.Name() != "PRB-SKP" {
		t.Fatalf("expected Name() = PRB-SKP, got %q", c.Name())
	}
}

func TestMode(t *testing.T) {
	c := New(inference.InferenceConfig{}, "/tmp")
	if c.Mode() != controller.ModeInference {
		t.Fatalf("expected Mode() = ModeInference, got %q", c.Mode())
	}
}

func TestRunGOVote(t *testing.T) {
	stepPlanPath, integrationMapPath, promptsDir := setupTestFiles(t)

	goResponse := `{"vote": "GO", "reason": "exhausted all hunt patterns, no concrete failure found", "findings": [], "lessons_checked": true, "prior_failures_found": "none"}`
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{Response: goResponse, Done: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollama.Close()

	cfg := inference.InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "test-key",
	}

	c := New(cfg, promptsDir)

	lessonsQueried := false
	inputs := controller.Inputs{
		MissionID:      "test-mission",
		StepPlan:       stepPlanPath,
		IntegrationMap: integrationMapPath,
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			if collection == "lessons_learned" {
				lessonsQueried = true
			}
			return []byte(`[{"lesson": "prior failure in auth flow"}]`), nil
		},
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO signal, got %q", sig.SignalValue)
	}
	if sig.Controller != "PRB-SKP" {
		t.Fatalf("expected controller PRB-SKP, got %q", sig.Controller)
	}
	if !lessonsQueried {
		t.Fatal("expected lessons_learned to be queried before assessment")
	}
	if sig.InferenceNotes == nil {
		t.Fatal("expected InferenceMeta to be populated")
	}
	if sig.InferenceNotes.Backend != "ollama" {
		t.Fatalf("expected backend ollama, got %q", sig.InferenceNotes.Backend)
	}
}

func TestRunOllamaNotConfigured(t *testing.T) {
	stepPlanPath, integrationMapPath, promptsDir := setupTestFiles(t)

	cfg := inference.InferenceConfig{
		OllamaURL: "",
		TimeoutS:  5,
	}

	c := New(cfg, promptsDir)

	inputs := controller.Inputs{
		MissionID:      "test-mission",
		StepPlan:       stepPlanPath,
		IntegrationMap: integrationMapPath,
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			return []byte("[]"), nil
		},
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "HOLD" {
		t.Fatalf("expected HOLD signal, got %q", sig.SignalValue)
	}
	if sig.Controller != "PRB-SKP" {
		t.Fatalf("expected controller PRB-SKP, got %q", sig.Controller)
	}
}

func TestRunContextCancelled(t *testing.T) {
	stepPlanPath, integrationMapPath, promptsDir := setupTestFiles(t)

	cfg := inference.InferenceConfig{
		OllamaURL:   "http://localhost:99999",
		OllamaModel: "llama3.2",
		TimeoutS:    5,
	}

	c := New(cfg, promptsDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inputs := controller.Inputs{
		MissionID:      "test-mission",
		StepPlan:       stepPlanPath,
		IntegrationMap: integrationMapPath,
	}

	_, err := c.Run(ctx, inputs)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRunNOGOVote(t *testing.T) {
	stepPlanPath, integrationMapPath, promptsDir := setupTestFiles(t)

	nogoResponse := `{"vote": "NO-GO", "reason": "found ordering trap in step-002", "findings": [{"pattern": "ORDERING_TRAP"}], "lessons_checked": true, "prior_failures_found": "auth flow failure in mission-042"}`
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{Response: nogoResponse, Done: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollama.Close()

	cfg := inference.InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "test-key",
	}

	c := New(cfg, promptsDir)

	inputs := controller.Inputs{
		MissionID:      "test-mission",
		StepPlan:       stepPlanPath,
		IntegrationMap: integrationMapPath,
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			return []byte("[]"), nil
		},
	}

	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "NO-GO" {
		t.Fatalf("expected NO-GO signal, got %q", sig.SignalValue)
	}
}

func TestRunLessonsLearnedQueryFails(t *testing.T) {
	stepPlanPath, integrationMapPath, promptsDir := setupTestFiles(t)

	goResponse := `{"vote": "GO", "reason": "cleared despite ecosystem unavailability"}`
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{Response: goResponse, Done: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollama.Close()

	cfg := inference.InferenceConfig{
		OllamaURL:           ollama.URL,
		OllamaModel:         "llama3.2",
		ConfidenceThreshold: 0.85,
		TimeoutS:            5,
		AnthropicAPIKey:     "test-key",
	}

	c := New(cfg, promptsDir)

	inputs := controller.Inputs{
		MissionID:      "test-mission",
		StepPlan:       stepPlanPath,
		IntegrationMap: integrationMapPath,
		EcosystemQuery: func(collection, query string) ([]byte, error) {
			return nil, fmt.Errorf("ecosystem unreachable")
		},
	}

	// Should still succeed — lessons_learned failure is non-fatal.
	sig, err := c.Run(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.SignalValue != "GO" {
		t.Fatalf("expected GO signal even with ecosystem failure, got %q", sig.SignalValue)
	}
}
