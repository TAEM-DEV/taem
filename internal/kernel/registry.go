// internal/kernel/registry.go
//
// Controller registry — loads config/controllers.yaml at startup.
// Validates that all required fields are present and mode values are valid.
// Provides phase-based controller lookup for the kernel sequencer.
//
// Per ADR-007: the kernel refuses to start with any missing fields.

package kernel

import (
	"fmt"
	"os"

	"github.com/taem-dev/taem/internal/controller"
	"gopkg.in/yaml.v3"
)

// ControllerDef represents a single controller definition parsed from
// config/controllers.yaml.
type ControllerDef struct {
	Callsign  string `yaml:"callsign"`
	Name      string `yaml:"name"`
	Mode      string `yaml:"mode"`
	Execution string `yaml:"execution"`
	Phase     []int  `yaml:"phase"`
	Required  bool   `yaml:"required"`
	TimeoutS  int    `yaml:"timeout_s"`
	Impl      string `yaml:"impl"`
	SignalType string `yaml:"signal_type"`
	Description string `yaml:"description"`
	Inference  *InferenceConfig `yaml:"inference,omitempty"`
}

// InferenceConfig holds inference-specific settings for controllers that use
// the Ollama->Anthropic fallback per ADR-005.
type InferenceConfig struct {
	OllamaModel         string  `yaml:"ollama_model"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
	Fallback            string  `yaml:"fallback"`
	AnthropicModel      string  `yaml:"anthropic_model"`
	SystemPromptFile    string  `yaml:"system_prompt_file"`
}

// InferenceDefaults holds the global inference defaults from controllers.yaml.
type InferenceDefaults struct {
	OllamaModel         string  `yaml:"ollama_model"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
	TimeoutS            int     `yaml:"timeout_s"`
	Fallback            string  `yaml:"fallback"`
	AnthropicModel      string  `yaml:"anthropic_model"`
}

// registryFile is the raw YAML structure of config/controllers.yaml.
type registryFile struct {
	InferenceDefaults InferenceDefaults `yaml:"inference_defaults"`
	Controllers       []ControllerDef   `yaml:"controllers"`
}

// Registry holds the validated controller definitions and provides
// lookup methods for the kernel.
type Registry struct {
	InferenceDefaults InferenceDefaults
	Controllers       []ControllerDef

	// byCallsign provides O(1) lookup by callsign.
	byCallsign map[string]*ControllerDef
}

// validModes is the set of valid controller mode values per ADR-005/ADR-007.
var validModes = map[string]controller.ControllerMode{
	"deterministic":   controller.ModeDeterministic,
	"inference":       controller.ModeInference,
	"github_dispatch": controller.ModeDispatch,
}

// LoadRegistry reads and validates a controllers.yaml file.
// Returns an error if any controller is missing a required field or has
// an invalid mode. Per ADR-007, the kernel refuses to start on any error.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
	}

	var raw registryFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("registry: parse %s: %w", path, err)
	}

	if len(raw.Controllers) == 0 {
		return nil, fmt.Errorf("registry: %s contains no controllers", path)
	}

	// Validate required fields for every controller.
	for i, c := range raw.Controllers {
		if c.Callsign == "" {
			return nil, fmt.Errorf("registry: controller[%d]: missing required field 'callsign'", i)
		}
		if c.Mode == "" {
			return nil, fmt.Errorf("registry: controller %q: missing required field 'mode'", c.Callsign)
		}
		if len(c.Phase) == 0 {
			return nil, fmt.Errorf("registry: controller %q: missing required field 'phase'", c.Callsign)
		}
		if c.Impl == "" {
			return nil, fmt.Errorf("registry: controller %q: missing required field 'impl'", c.Callsign)
		}

		// Validate mode is a known value.
		if _, ok := validModes[c.Mode]; !ok {
			return nil, fmt.Errorf("registry: controller %q: invalid mode %q (must be deterministic, inference, or github_dispatch)", c.Callsign, c.Mode)
		}
	}

	// Build the lookup index.
	byCallsign := make(map[string]*ControllerDef, len(raw.Controllers))
	for i := range raw.Controllers {
		cs := raw.Controllers[i].Callsign
		if _, exists := byCallsign[cs]; exists {
			return nil, fmt.Errorf("registry: duplicate callsign %q", cs)
		}
		byCallsign[cs] = &raw.Controllers[i]
	}

	return &Registry{
		InferenceDefaults: raw.InferenceDefaults,
		Controllers:       raw.Controllers,
		byCallsign:        byCallsign,
	}, nil
}

// ControllersForPhase returns all controllers (required and optional) that
// participate in the given phase.
func (r *Registry) ControllersForPhase(phase int) []ControllerDef {
	var result []ControllerDef
	for _, c := range r.Controllers {
		for _, p := range c.Phase {
			if p == phase {
				result = append(result, c)
				break
			}
		}
	}
	return result
}

// RequiredForPhase returns only the required controllers for the given phase.
func (r *Registry) RequiredForPhase(phase int) []ControllerDef {
	var result []ControllerDef
	for _, c := range r.Controllers {
		if !c.Required {
			continue
		}
		for _, p := range c.Phase {
			if p == phase {
				result = append(result, c)
				break
			}
		}
	}
	return result
}

// RequiredCallsignsForPhase returns the callsigns of required controllers
// for the given phase. This is the format expected by gate.Evaluate().
func (r *Registry) RequiredCallsignsForPhase(phase int) []string {
	required := r.RequiredForPhase(phase)
	callsigns := make([]string, len(required))
	for i, c := range required {
		callsigns[i] = c.Callsign
	}
	return callsigns
}

// GetController returns the definition for a controller by callsign.
func (r *Registry) GetController(callsign string) (ControllerDef, bool) {
	c, ok := r.byCallsign[callsign]
	if !ok {
		return ControllerDef{}, false
	}
	return *c, true
}

// ControllerMode returns the typed ControllerMode for a controller by callsign.
func (r *Registry) ControllerMode(callsign string) (controller.ControllerMode, error) {
	c, ok := r.byCallsign[callsign]
	if !ok {
		return "", fmt.Errorf("registry: unknown controller %q", callsign)
	}
	mode, ok := validModes[c.Mode]
	if !ok {
		return "", fmt.Errorf("registry: controller %q has invalid mode %q", callsign, c.Mode)
	}
	return mode, nil
}
