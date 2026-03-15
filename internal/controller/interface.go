// internal/controller/interface.go

package controller

import "context"

type ControllerMode string

const (
	ModeDeterministic ControllerMode = "deterministic"
	ModeInference     ControllerMode = "inference"
	ModeDispatch      ControllerMode = "github_dispatch"
)

type Signal struct {
	Controller     string         `json:"controller"`
	SignalValue    string         `json:"signal"`       // GO, NO-GO, HOLD, WARN, RELAY
	Reason         string         `json:"reason"`
	Evidence       []string       `json:"evidence"`
	ConstraintRef  *string        `json:"constraint_ref"`
	InferenceNotes *InferenceMeta `json:"inference_notes,omitempty"`
}

type InferenceMeta struct {
	Backend      string  `json:"inference_backend"` // "ollama" or "anthropic"
	Confidence   float64 `json:"ollama_confidence"`
	LatencyMS    int64   `json:"latency_ms"`
	FallbackUsed bool    `json:"fallback_used"`
}

type Inputs struct {
	MissionID      string
	ManifestPath   string
	IntegrationMap string // path to integration-map.json, empty if not yet written
	StepPlan       string // path to step-plan.json, empty if not yet written
	ADRsPath       string // path to checked-out adrs/
	EcosystemQuery func(collection, query string) ([]byte, error)
	Phase          int
}

type Controller interface {
	Name() string
	Mode() ControllerMode
	Run(ctx context.Context, inputs Inputs) (Signal, error)
}
