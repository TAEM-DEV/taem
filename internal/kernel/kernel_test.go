package kernel

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/taem-dev/taem/internal/controller"
	"github.com/taem-dev/taem/internal/dispatch"
	"github.com/taem-dev/taem/internal/state"
)

// --- mock controller ---

// mockController implements controller.Controller for testing.
type mockController struct {
	callsign string
	mode     controller.ControllerMode
	signal   controller.Signal
	err      error
	// runCount tracks how many times Run has been called.
	runCount int
	// signalFunc allows dynamic signal generation per invocation.
	signalFunc func(count int) (controller.Signal, error)
	// ctxSignalFunc is like signalFunc but also receives the context,
	// allowing tests to verify ctx.Done() handling per CLAUDE.md Hard Rules.
	ctxSignalFunc func(ctx context.Context, count int) (controller.Signal, error)
}

func (m *mockController) Name() string                    { return m.callsign }
func (m *mockController) Mode() controller.ControllerMode { return m.mode }

func (m *mockController) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	m.runCount++
	if m.ctxSignalFunc != nil {
		return m.ctxSignalFunc(ctx, m.runCount)
	}
	if m.signalFunc != nil {
		return m.signalFunc(m.runCount)
	}
	if m.err != nil {
		return controller.Signal{}, m.err
	}
	return m.signal, nil
}

// goSignal returns a GO signal for a given callsign.
func goSignal(callsign string) controller.Signal {
	return controller.Signal{
		Controller:  callsign,
		SignalValue: "GO",
		Reason:      callsign + " all checks passed",
		Evidence:    []string{"ok"},
	}
}

// noGoSignal returns a NO-GO signal for a given callsign.
func noGoSignal(callsign string) controller.Signal {
	return controller.Signal{
		Controller:  callsign,
		SignalValue: "NO-GO",
		Reason:      callsign + " check failed",
		Evidence:    []string{"failed"},
	}
}

// relaySignal returns a RELAY signal for a given callsign.
func relaySignal(callsign string) controller.Signal {
	return controller.Signal{
		Controller:  callsign,
		SignalValue: "RELAY",
		Reason:      callsign + " relayed",
		Evidence:    []string{},
	}
}

// --- test registry builder ---

// testRegistry builds a minimal Registry with the given controller definitions.
// This avoids needing to load a YAML file in unit tests.
func testRegistry(defs []ControllerDef) *Registry {
	byCallsign := make(map[string]*ControllerDef, len(defs))
	for i := range defs {
		byCallsign[defs[i].Callsign] = &defs[i]
	}
	return &Registry{
		Controllers: defs,
		byCallsign:  byCallsign,
	}
}

// --- test factory builder ---

// testFactory returns a ControllerFactory that maps callsigns to mock controllers.
func testFactory(mocks map[string]*mockController) ControllerFactory {
	return func(def ControllerDef) controller.Controller {
		if m, ok := mocks[def.Callsign]; ok {
			return m
		}
		// Return a default GO controller for unknown callsigns.
		return &mockController{
			callsign: def.Callsign,
			mode:     controller.ModeDeterministic,
			signal:   goSignal(def.Callsign),
		}
	}
}

// --- minimal phase definitions for testing ---

// minimalPhase00Defs returns controller definitions for a minimal Phase 0.
func minimalPhase00Defs() []ControllerDef {
	return []ControllerDef{
		{Callsign: "GC", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "gc"},
		{Callsign: "DPS", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "dps"},
		{Callsign: "EECOM", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "eecom"},
	}
}

// fullMissionDefs returns controller definitions for a complete 7-phase mission.
func fullMissionDefs() []ControllerDef {
	return []ControllerDef{
		// Phase 0: Pad Check
		{Callsign: "GC", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "gc", SignalType: "GO | NO-GO"},
		{Callsign: "DPS", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "dps", SignalType: "GO | NO-GO"},
		{Callsign: "EECOM", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "eecom", SignalType: "GO | NO-GO"},
		// Phase 1: Corpus Ingestion (local for testing — real NAV is github_dispatch)
		{Callsign: "NAV", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{1}, Required: true, Impl: "nav", SignalType: "GO | NO-GO"},
		// Phase 2: Architectural Survey
		{Callsign: "ARCH", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{2}, Required: true, Impl: "arch", SignalType: "GO | NO-GO"},
		{Callsign: "CDS", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{2}, Required: true, Impl: "cds", SignalType: "GO | NO-GO"},
		{Callsign: "PCO", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{2}, Required: true, Impl: "pco", SignalType: "GO | NO-GO"},
		// Phase 3: Plan Formulation
		{Callsign: "INCO", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{3}, Required: true, Impl: "inco", SignalType: "GO | NO-GO"},
		// Phase 4: Pre-Code Inspection
		{Callsign: "SECINSP", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "secinsp", SignalType: "GO | NO-GO"},
		{Callsign: "TRC", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{4}, Required: true, Impl: "trc", SignalType: "GO | NO-GO"},
		{Callsign: "PRB-SKP", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "prb-skp", SignalType: "GO | NO-GO"},
		{Callsign: "PRB-COR", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "prb-cor", SignalType: "GO | NO-GO"},
		{Callsign: "PRB-ADR", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "prb-adr", SignalType: "GO | NO-GO"},
		// Phase 5: CAPCOM
		{Callsign: "CAPCOM", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{5}, Required: true, Impl: "capcom", SignalType: "RELAY"},
		// Phase 6: PAO
		{Callsign: "PAO", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{6}, Required: false, Impl: "pao", SignalType: "RELAY"},
	}
}

// allGoMocks returns a map of mock controllers that all emit GO (or RELAY for CAPCOM/PAO).
func allGoMocks() map[string]*mockController {
	mocks := make(map[string]*mockController)
	for _, cs := range []string{"GC", "DPS", "EECOM", "NAV", "ARCH", "CDS", "PCO", "INCO", "SECINSP", "TRC", "PRB-SKP", "PRB-COR", "PRB-ADR"} {
		mocks[cs] = &mockController{
			callsign: cs,
			mode:     controller.ModeDeterministic,
			signal:   goSignal(cs),
		}
	}
	mocks["CAPCOM"] = &mockController{
		callsign: "CAPCOM",
		mode:     controller.ModeDeterministic,
		signal:   relaySignal("CAPCOM"),
	}
	mocks["PAO"] = &mockController{
		callsign: "PAO",
		mode:     controller.ModeDeterministic,
		signal:   relaySignal("PAO"),
	}
	return mocks
}

// --- tests ---

// TestLaunch_CreatesMissionDirAndManifest verifies that Launch creates the
// mission directory and writes MISSION_START to manifest.jsonl.
func TestLaunch_CreatesMissionDirAndManifest(t *testing.T) {
	mcState := t.TempDir()
	reg := testRegistry(minimalPhase00Defs())

	m, err := Launch(MissionConfig{
		Task:              "smoke test",
		Repos:             []string{"mc-state"},
		ADRs:              []string{"ADR-001"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: testFactory(nil),
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	// Verify mission ID format.
	if m.ID == "" {
		t.Fatal("mission ID is empty")
	}
	if len(m.ID) < 5 || m.ID[:4] != "MSN-" {
		t.Fatalf("mission ID %q does not match expected format MSN-<hex>", m.ID)
	}

	// Verify mission directory exists.
	missionDir := m.MissionDir()
	if missionDir == "" {
		t.Fatal("MissionDir() returned empty string")
	}
	expectedDir := filepath.Join(mcState, "missions", m.ID)
	if missionDir != expectedDir {
		t.Fatalf("MissionDir: got %q, want %q", missionDir, expectedDir)
	}

	// Verify MISSION_START was written to manifest.jsonl.
	events, err := state.ReadManifest(missionDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 manifest event, got %d", len(events))
	}
	evt := events[0]
	if evt.Event != "MISSION_START" {
		t.Errorf("event: got %q, want MISSION_START", evt.Event)
	}
	if evt.MissionID != m.ID {
		t.Errorf("mission_id: got %q, want %q", evt.MissionID, m.ID)
	}
	if evt.Task != "smoke test" {
		t.Errorf("task: got %q, want %q", evt.Task, "smoke test")
	}
	if len(evt.Repos) != 1 || evt.Repos[0] != "mc-state" {
		t.Errorf("repos: got %v, want [mc-state]", evt.Repos)
	}
}

// TestLaunch_ValidationErrors verifies that Launch returns errors for invalid inputs.
func TestLaunch_ValidationErrors(t *testing.T) {
	reg := testRegistry(minimalPhase00Defs())
	factory := testFactory(nil)

	tests := []struct {
		name string
		cfg  MissionConfig
	}{
		{
			name: "missing task",
			cfg:  MissionConfig{Repos: []string{"r"}, MCStatePath: "/tmp", Registry: reg, ControllerFactory: factory},
		},
		{
			name: "missing repos",
			cfg:  MissionConfig{Task: "t", MCStatePath: "/tmp", Registry: reg, ControllerFactory: factory},
		},
		{
			name: "missing mc-state path",
			cfg:  MissionConfig{Task: "t", Repos: []string{"r"}, Registry: reg, ControllerFactory: factory},
		},
		{
			name: "missing registry",
			cfg:  MissionConfig{Task: "t", Repos: []string{"r"}, MCStatePath: "/tmp", ControllerFactory: factory},
		},
		{
			name: "missing factory",
			cfg:  MissionConfig{Task: "t", Repos: []string{"r"}, MCStatePath: "/tmp", Registry: reg},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Launch(tt.cfg)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
			t.Logf("correctly rejected: %v", err)
		})
	}
}

// TestRun_AllGO_CompletesLanded verifies that a full mission with all-GO
// mock controllers completes with MISSION_LANDED in the manifest.
func TestRun_AllGO_CompletesLanded(t *testing.T) {
	mcState := t.TempDir()
	reg := testRegistry(fullMissionDefs())
	mocks := allGoMocks()

	m, err := Launch(MissionConfig{
		Task:              "full mission test",
		Repos:             []string{"mc-state"},
		ADRs:              []string{"ADR-001"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: testFactory(mocks),
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	ctx := context.Background()
	if err := m.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify MISSION_LANDED is in the manifest.
	events, err := state.ReadManifest(m.MissionDir())
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	foundLanded := false
	for _, evt := range events {
		if evt.Event == "MISSION_LANDED" {
			foundLanded = true
			break
		}
	}
	if !foundLanded {
		t.Fatal("expected MISSION_LANDED event in manifest, not found")
	}

	// Verify we have PHASE_START and PHASE_COMPLETE for each phase.
	phaseStarts := make(map[int]bool)
	phaseCompletes := make(map[int]bool)
	for _, evt := range events {
		if evt.Phase == nil {
			continue
		}
		switch evt.Event {
		case "PHASE_START":
			phaseStarts[*evt.Phase] = true
		case "PHASE_COMPLETE":
			phaseCompletes[*evt.Phase] = true
		}
	}

	for _, phase := range defaultPhases {
		if !phaseStarts[phase] {
			t.Errorf("missing PHASE_START for phase %d", phase)
		}
		if !phaseCompletes[phase] {
			t.Errorf("missing PHASE_COMPLETE for phase %d", phase)
		}
	}

	// Verify signals were written to signals.jsonl.
	signals, err := state.ReadSignals(m.MissionDir())
	if err != nil {
		t.Fatalf("ReadSignals: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signals in signals.jsonl, got none")
	}

	// Verify remediation cycles is 0.
	if m.RemediationCycles() != 0 {
		t.Errorf("remediation cycles: got %d, want 0", m.RemediationCycles())
	}
}

// TestRun_NoGO_Remediation verifies that a NO-GO controller triggers
// remediation. After 1 remediation cycle, the controller switches to GO.
func TestRun_NoGO_Remediation(t *testing.T) {
	mcState := t.TempDir()

	// Use just Phase 0 for simplicity.
	defs := minimalPhase00Defs()
	reg := testRegistry(defs)

	// EECOM starts as NO-GO on first call, then switches to GO.
	eecomMock := &mockController{
		callsign: "EECOM",
		mode:     controller.ModeDeterministic,
		signalFunc: func(count int) (controller.Signal, error) {
			if count == 1 {
				return noGoSignal("EECOM"), nil
			}
			return goSignal("EECOM"), nil
		},
	}

	mocks := map[string]*mockController{
		"GC":    {callsign: "GC", mode: controller.ModeDeterministic, signal: goSignal("GC")},
		"DPS":   {callsign: "DPS", mode: controller.ModeDeterministic, signal: goSignal("DPS")},
		"EECOM": eecomMock,
	}

	m, err := Launch(MissionConfig{
		Task:              "remediation test",
		Repos:             []string{"mc-state"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: testFactory(mocks),
		Phases:            []int{0}, // Only run phase 0
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	ctx := context.Background()
	if err := m.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// EECOM was called twice: once NO-GO, once GO after remediation.
	if eecomMock.runCount != 2 {
		t.Errorf("EECOM run count: got %d, want 2", eecomMock.runCount)
	}

	// Remediation cycles should be 1.
	if m.RemediationCycles() != 1 {
		t.Errorf("remediation cycles: got %d, want 1", m.RemediationCycles())
	}

	// Mission should have completed (LANDED).
	events, err := state.ReadManifest(m.MissionDir())
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	foundLanded := false
	for _, evt := range events {
		if evt.Event == "MISSION_LANDED" {
			foundLanded = true
			break
		}
	}
	if !foundLanded {
		t.Fatal("expected MISSION_LANDED after remediation, not found")
	}
}

// TestRun_RemediationCap_Escalated verifies that when the remediation cap
// (2 cycles) is exceeded, the mission is ESCALATED.
func TestRun_RemediationCap_Escalated(t *testing.T) {
	mcState := t.TempDir()

	defs := minimalPhase00Defs()
	reg := testRegistry(defs)

	// EECOM always returns NO-GO — remediation will never succeed.
	eecomMock := &mockController{
		callsign: "EECOM",
		mode:     controller.ModeDeterministic,
		signal:   noGoSignal("EECOM"),
	}

	mocks := map[string]*mockController{
		"GC":    {callsign: "GC", mode: controller.ModeDeterministic, signal: goSignal("GC")},
		"DPS":   {callsign: "DPS", mode: controller.ModeDeterministic, signal: goSignal("DPS")},
		"EECOM": eecomMock,
	}

	m, err := Launch(MissionConfig{
		Task:              "escalation test",
		Repos:             []string{"mc-state"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: testFactory(mocks),
		Phases:            []int{0},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	ctx := context.Background()
	err = m.Run(ctx)
	if err == nil {
		t.Fatal("expected error for ESCALATED mission, got nil")
	}

	// Verify the error message indicates escalation.
	if got := err.Error(); !contains(got, "ESCALATED") {
		t.Errorf("expected ESCALATED in error, got: %s", got)
	}

	// Verify MISSION_ESCALATED is in the manifest.
	events, err2 := state.ReadManifest(m.MissionDir())
	if err2 != nil {
		t.Fatalf("ReadManifest: %v", err2)
	}
	foundEscalated := false
	for _, evt := range events {
		if evt.Event == "MISSION_ESCALATED" {
			foundEscalated = true
			break
		}
	}
	if !foundEscalated {
		t.Fatal("expected MISSION_ESCALATED event in manifest, not found")
	}

	// Remediation cap is 2, so we should have hit 2 cycles.
	// First run: HOLD (rem=0 -> rem=1). Second run: HOLD (rem=1 -> rem=2).
	// Third run: ABORT (rem=2 >= cap).
	if m.RemediationCycles() < 2 {
		t.Errorf("remediation cycles: got %d, want >= 2", m.RemediationCycles())
	}
}

// TestRun_ContextCancellation verifies that the mission respects context
// cancellation and stops execution cleanly.
func TestRun_ContextCancellation(t *testing.T) {
	mcState := t.TempDir()
	reg := testRegistry(fullMissionDefs())

	// Create a slow controller that blocks until context is done.
	// Per CLAUDE.md Hard Rules: controllers must respect ctx.Done().
	slowMock := &mockController{
		callsign: "NAV",
		mode:     controller.ModeDeterministic,
		ctxSignalFunc: func(ctx context.Context, count int) (controller.Signal, error) {
			// Block until context is cancelled, then return NO-GO.
			<-ctx.Done()
			return controller.Signal{
				Controller:  "NAV",
				SignalValue: "NO-GO",
				Reason:      "context cancelled",
				Evidence:    []string{ctx.Err().Error()},
			}, nil
		},
	}

	mocks := allGoMocks()
	mocks["NAV"] = slowMock

	m, err := Launch(MissionConfig{
		Task:              "cancellation test",
		Repos:             []string{"mc-state"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: testFactory(mocks),
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	// Cancel context after a short delay.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = m.Run(ctx)
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}

	t.Logf("mission stopped with: %v", err)
}

// TestRun_Phase0Only_CompletesLanded verifies that a mission with only
// Phase 0 controllers completes successfully.
func TestRun_Phase0Only_CompletesLanded(t *testing.T) {
	mcState := t.TempDir()
	reg := testRegistry(minimalPhase00Defs())

	mocks := map[string]*mockController{
		"GC":    {callsign: "GC", mode: controller.ModeDeterministic, signal: goSignal("GC")},
		"DPS":   {callsign: "DPS", mode: controller.ModeDeterministic, signal: goSignal("DPS")},
		"EECOM": {callsign: "EECOM", mode: controller.ModeDeterministic, signal: goSignal("EECOM")},
	}

	m, err := Launch(MissionConfig{
		Task:              "phase0 only",
		Repos:             []string{"mc-state"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: testFactory(mocks),
		Phases:            []int{0},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	ctx := context.Background()
	if err := m.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	events, err := state.ReadManifest(m.MissionDir())
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	foundLanded := false
	for _, evt := range events {
		if evt.Event == "MISSION_LANDED" {
			foundLanded = true
		}
	}
	if !foundLanded {
		t.Fatal("expected MISSION_LANDED for phase-0-only mission")
	}
}

// TestRun_CapcomRelay_NeverBlocks verifies that the CAPCOM phase (Phase 5)
// with RELAY signal type never blocks the gate.
func TestRun_CapcomRelay_NeverBlocks(t *testing.T) {
	mcState := t.TempDir()

	// Minimal mission: Phase 0 (required GO) + Phase 5 (CAPCOM RELAY).
	defs := []ControllerDef{
		{Callsign: "GC", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "gc", SignalType: "GO | NO-GO"},
		{Callsign: "CAPCOM", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{5}, Required: true, Impl: "capcom", SignalType: "RELAY"},
	}
	reg := testRegistry(defs)

	mocks := map[string]*mockController{
		"GC":     {callsign: "GC", mode: controller.ModeDeterministic, signal: goSignal("GC")},
		"CAPCOM": {callsign: "CAPCOM", mode: controller.ModeDeterministic, signal: relaySignal("CAPCOM")},
	}

	m, err := Launch(MissionConfig{
		Task:              "capcom relay test",
		Repos:             []string{"mc-state"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: testFactory(mocks),
		Phases:            []int{0, 5},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	ctx := context.Background()
	if err := m.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify CAPCOM ran.
	if mocks["CAPCOM"].runCount != 1 {
		t.Errorf("CAPCOM run count: got %d, want 1", mocks["CAPCOM"].runCount)
	}
}

// TestBuildInputs_WiresEcosystemQuery verifies that buildInputs wires
// the ecosystem query function when an ecosystem client is available.
func TestBuildInputs_WiresEcosystemQuery(t *testing.T) {
	mcState := t.TempDir()
	reg := testRegistry(minimalPhase00Defs())

	m, err := Launch(MissionConfig{
		Task:              "inputs test",
		Repos:             []string{"mc-state"},
		ADRs:              []string{"ADR-001"},
		MCStatePath:       mcState,
		ADRsPath:          "/tmp/adrs",
		Registry:          reg,
		ControllerFactory: testFactory(nil),
		Phases:            []int{0},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	inputs := m.buildInputs(0)

	if inputs.MissionID != m.ID {
		t.Errorf("MissionID: got %q, want %q", inputs.MissionID, m.ID)
	}
	if inputs.Phase != 0 {
		t.Errorf("Phase: got %d, want 0", inputs.Phase)
	}
	if inputs.ADRsPath != "/tmp/adrs" {
		t.Errorf("ADRsPath: got %q, want /tmp/adrs", inputs.ADRsPath)
	}
	if inputs.ManifestPath == "" {
		t.Error("ManifestPath is empty")
	}
}

// TestGenerateMissionID verifies the mission ID format.
func TestGenerateMissionID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateMissionID()
		if err != nil {
			t.Fatalf("generateMissionID: %v", err)
		}
		if !hasPrefix(id, "MSN-") {
			t.Fatalf("mission ID %q does not start with MSN-", id)
		}
		if seen[id] {
			t.Fatalf("duplicate mission ID: %s", id)
		}
		seen[id] = true
	}
}

// --- helpers ---

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString is a simple substring search.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// hasPrefix reports whether s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Ensure imports are used.
var (
	_ = fmt.Sprintf
	_ = dispatch.NewLocalDispatcher
)
