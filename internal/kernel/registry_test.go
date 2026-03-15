package kernel

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

// repoRoot returns the absolute path to the repository root by walking up
// from this test file's location (internal/kernel/) two levels.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = .../internal/kernel/registry_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// TestRegistry_LoadActual successfully loads the real config/controllers.yaml.
func TestRegistry_LoadActual(t *testing.T) {
	root := repoRoot(t)
	reg, err := LoadRegistry(filepath.Join(root, "config", "controllers.yaml"))
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}
	if len(reg.Controllers) == 0 {
		t.Fatal("expected at least one controller, got 0")
	}
}

// TestRegistry_Phase0Controllers verifies that Phase 0 contains GC, DPS, and EECOM.
func TestRegistry_Phase0Controllers(t *testing.T) {
	root := repoRoot(t)
	reg, err := LoadRegistry(filepath.Join(root, "config", "controllers.yaml"))
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	phase0 := reg.ControllersForPhase(0)
	callsigns := make(map[string]bool)
	for _, c := range phase0 {
		callsigns[c.Callsign] = true
	}

	expected := []string{"GC", "DPS", "EECOM"}
	for _, cs := range expected {
		if !callsigns[cs] {
			t.Errorf("expected %s in Phase 0, but not found", cs)
		}
	}

	// FAO is in phase 0 but is optional — it should still appear in the list
	if !callsigns["FAO"] {
		t.Error("expected FAO in Phase 0 (optional controller)")
	}
}

// TestRegistry_Phase4Controllers verifies that Phase 4 contains SECINSP, TRC,
// PRB-SKP, PRB-COR, and PRB-ADR.
func TestRegistry_Phase4Controllers(t *testing.T) {
	root := repoRoot(t)
	reg, err := LoadRegistry(filepath.Join(root, "config", "controllers.yaml"))
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	phase4 := reg.ControllersForPhase(4)
	callsigns := make(map[string]bool)
	for _, c := range phase4 {
		callsigns[c.Callsign] = true
	}

	expected := []string{"SECINSP", "TRC", "PRB-SKP", "PRB-COR", "PRB-ADR"}
	for _, cs := range expected {
		if !callsigns[cs] {
			t.Errorf("expected %s in Phase 4, but not found", cs)
		}
	}
}

// TestRegistry_AllFieldsPresent validates that every controller in the real
// config has all required fields: callsign, mode, phase, impl.
func TestRegistry_AllFieldsPresent(t *testing.T) {
	root := repoRoot(t)
	reg, err := LoadRegistry(filepath.Join(root, "config", "controllers.yaml"))
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	for _, c := range reg.Controllers {
		if c.Callsign == "" {
			t.Error("found controller with empty callsign")
		}
		if c.Mode == "" {
			t.Errorf("controller %s: missing mode", c.Callsign)
		}
		if len(c.Phase) == 0 {
			t.Errorf("controller %s: missing phase", c.Callsign)
		}
		if c.Impl == "" {
			t.Errorf("controller %s: missing impl", c.Callsign)
		}

		// Validate mode is one of the known ControllerMode values.
		switch controller.ControllerMode(c.Mode) {
		case controller.ModeDeterministic, controller.ModeInference, controller.ModeDispatch:
			// valid
		default:
			t.Errorf("controller %s: invalid mode %q", c.Callsign, c.Mode)
		}
	}
}

// TestRegistry_RejectsMissingMode verifies that LoadRegistry returns an error
// when a controller is missing the mode field.
func TestRegistry_RejectsMissingMode(t *testing.T) {
	// Write a minimal invalid YAML to a temp file.
	dir := t.TempDir()
	badYAML := filepath.Join(dir, "bad.yaml")

	content := []byte(`controllers:
  - callsign: BROKEN
    name: "Missing mode"
    phase: [0]
    impl: internal/controller/deterministic/broken
    required: true
    timeout_s: 10
`)
	if err := writeFile(badYAML, content); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadRegistry(badYAML)
	if err == nil {
		t.Fatal("expected error for missing mode field, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestRegistry_RejectsMissingCallsign verifies that LoadRegistry returns an
// error when a controller is missing the callsign field.
func TestRegistry_RejectsMissingCallsign(t *testing.T) {
	dir := t.TempDir()
	badYAML := filepath.Join(dir, "bad.yaml")

	content := []byte(`controllers:
  - name: "Missing callsign"
    mode: deterministic
    phase: [0]
    impl: internal/controller/deterministic/broken
    required: true
    timeout_s: 10
`)
	if err := writeFile(badYAML, content); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadRegistry(badYAML)
	if err == nil {
		t.Fatal("expected error for missing callsign field, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestRegistry_RejectsMissingImpl verifies that LoadRegistry returns an
// error when a controller is missing the impl field.
func TestRegistry_RejectsMissingImpl(t *testing.T) {
	dir := t.TempDir()
	badYAML := filepath.Join(dir, "bad.yaml")

	content := []byte(`controllers:
  - callsign: BROKEN
    name: "Missing impl"
    mode: deterministic
    phase: [0]
    required: true
    timeout_s: 10
`)
	if err := writeFile(badYAML, content); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadRegistry(badYAML)
	if err == nil {
		t.Fatal("expected error for missing impl field, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestRegistry_RejectsMissingPhase verifies that LoadRegistry returns an
// error when a controller is missing the phase field.
func TestRegistry_RejectsMissingPhase(t *testing.T) {
	dir := t.TempDir()
	badYAML := filepath.Join(dir, "bad.yaml")

	content := []byte(`controllers:
  - callsign: BROKEN
    name: "Missing phase"
    mode: deterministic
    impl: internal/controller/deterministic/broken
    required: true
    timeout_s: 10
`)
	if err := writeFile(badYAML, content); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadRegistry(badYAML)
	if err == nil {
		t.Fatal("expected error for missing phase field, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestRegistry_RequiredVsOptional verifies that the registry correctly
// distinguishes required vs optional controllers.
func TestRegistry_RequiredVsOptional(t *testing.T) {
	root := repoRoot(t)
	reg, err := LoadRegistry(filepath.Join(root, "config", "controllers.yaml"))
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	required := reg.RequiredForPhase(0)
	requiredCallsigns := make(map[string]bool)
	for _, c := range required {
		requiredCallsigns[c.Callsign] = true
	}

	// GC, DPS, EECOM are required in Phase 0.
	for _, cs := range []string{"GC", "DPS", "EECOM"} {
		if !requiredCallsigns[cs] {
			t.Errorf("expected %s to be required in Phase 0", cs)
		}
	}

	// FAO is optional (required: false) — must NOT appear in RequiredForPhase.
	if requiredCallsigns["FAO"] {
		t.Error("FAO should be optional in Phase 0, but appeared as required")
	}
}

// writeFile is a small helper to write bytes to a path for test fixtures.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// TestRegistry_ControllerMode verifies that mode values are correctly parsed
// for representative controllers of each mode type.
func TestRegistry_ControllerMode(t *testing.T) {
	root := repoRoot(t)
	reg, err := LoadRegistry(filepath.Join(root, "config", "controllers.yaml"))
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	tests := []struct {
		callsign string
		wantMode string
	}{
		{"GC", "deterministic"},
		{"SECINSP", "inference"},
		{"NAV", "deterministic"}, // NAV has mode: deterministic, execution: github_dispatch
		{"PRB-SKP", "inference"},
	}

	for _, tt := range tests {
		c, ok := reg.GetController(tt.callsign)
		if !ok {
			t.Errorf("controller %s not found", tt.callsign)
			continue
		}
		if c.Mode != tt.wantMode {
			t.Errorf("controller %s: expected mode %q, got %q", tt.callsign, tt.wantMode, c.Mode)
		}
	}
}
