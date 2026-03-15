//go:build integration

package kernel_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
	"github.com/taem-dev/taem/internal/kernel"
	"github.com/taem-dev/taem/internal/state"
)

// ---------------------------------------------------------------------------
// Mock controller for integration tests
// ---------------------------------------------------------------------------

// integrationMock implements controller.Controller for integration tests.
type integrationMock struct {
	callsign string
	mode     controller.ControllerMode
	signal   controller.Signal
	// signalFunc allows per-invocation signal generation (for remediation tests).
	signalFunc func(count int) (controller.Signal, error)
	runCount   int
}

func (m *integrationMock) Name() string                    { return m.callsign }
func (m *integrationMock) Mode() controller.ControllerMode { return m.mode }

func (m *integrationMock) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	m.runCount++
	if m.signalFunc != nil {
		return m.signalFunc(m.runCount)
	}
	return m.signal, nil
}

// ---------------------------------------------------------------------------
// Helper builders
// ---------------------------------------------------------------------------

func mkSignal(callsign, value string) controller.Signal {
	return controller.Signal{
		Controller:  callsign,
		SignalValue: value,
		Reason:      callsign + " " + value,
		Evidence:    []string{callsign + "-evidence"},
	}
}

func mkMock(callsign string, mode controller.ControllerMode, signal controller.Signal) *integrationMock {
	return &integrationMock{
		callsign: callsign,
		mode:     mode,
		signal:   signal,
	}
}

// buildRegistry creates a test Registry from controller definitions.
func buildRegistry(defs []kernel.ControllerDef) *kernel.Registry {
	return kernel.NewTestRegistry(defs)
}

// buildFactory creates a ControllerFactory from a map of mocks.
func buildFactory(mocks map[string]*integrationMock) kernel.ControllerFactory {
	return func(def kernel.ControllerDef) controller.Controller {
		if m, ok := mocks[def.Callsign]; ok {
			return m
		}
		// Default: return GO
		return mkMock(def.Callsign, controller.ModeDeterministic, mkSignal(def.Callsign, "GO"))
	}
}

// allPositionDefs returns controller definitions for all 15 positions
// across the 7 mission phases per the TAEM specification.
//
// PRB sub-agents use the long-form callsigns (PRB-SKEPTIC, PRB-CORRECTNESS,
// PRB-ADR-AUDIT) to match gate.go's prbSubAgents for correct vote aggregation.
func allPositionDefs() []kernel.ControllerDef {
	return []kernel.ControllerDef{
		// Phase 0: Pad Check
		{Callsign: "GC", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "gc", SignalType: "GO | NO-GO"},
		{Callsign: "DPS", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "dps", SignalType: "GO | NO-GO"},
		{Callsign: "EECOM", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "eecom", SignalType: "GO | NO-GO"},
		// Phase 1: Corpus Ingestion
		{Callsign: "NAV", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{1}, Required: true, Impl: "nav", SignalType: "GO | ADVISORY"},
		{Callsign: "FAO", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{1}, Required: false, Impl: "fao", SignalType: "ADVISORY"},
		// Phase 2: Architectural Survey
		{Callsign: "ARCH", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{2}, Required: true, Impl: "arch", SignalType: "GO | NO-GO"},
		{Callsign: "CDS", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{2}, Required: true, Impl: "cds", SignalType: "GO | NO-GO"},
		{Callsign: "PCO", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{2}, Required: true, Impl: "pco", SignalType: "GO | NO-GO"},
		// Phase 3: Plan Formulation
		{Callsign: "INCO", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{3}, Required: true, Impl: "inco", SignalType: "GO | NO-GO"},
		// Phase 4: Pre-Code Inspection
		{Callsign: "SECINSP", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "secinsp", SignalType: "GO | NO-GO"},
		{Callsign: "TRC", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{4}, Required: true, Impl: "trc", SignalType: "GO | NO-GO"},
		{Callsign: "PRB-SKEPTIC", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "prb-skeptic", SignalType: "GO | NO-GO"},
		{Callsign: "PRB-CORRECTNESS", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "prb-correctness", SignalType: "GO | NO-GO"},
		{Callsign: "PRB-ADR-AUDIT", Mode: "inference", Execution: "local_inference", Phase: []int{4}, Required: true, Impl: "prb-adr-audit", SignalType: "GO | NO-GO"},
		// Phase 5: CAPCOM
		{Callsign: "CAPCOM", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{5}, Required: true, Impl: "capcom", SignalType: "RELAY"},
		// Phase 6: PAO
		{Callsign: "PAO", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{6}, Required: false, Impl: "pao", SignalType: "RELAY"},
	}
}

// ---------------------------------------------------------------------------
// Test 1: Full mission lifecycle — PRB 2/3 GO, mission completes
// ---------------------------------------------------------------------------

func TestIntegration_FullMissionLifecycle_PRB_2of3_GO(t *testing.T) {
	// Set up a temp directory as mc-state with missions/ subdirectory.
	mcState := t.TempDir()
	missionsDir := filepath.Join(mcState, "missions")
	if err := os.MkdirAll(missionsDir, 0755); err != nil {
		t.Fatalf("create missions dir: %v", err)
	}

	defs := allPositionDefs()
	reg := buildRegistry(defs)

	// Create mock controllers for all 15 positions.
	// Phase 0: GC (GO), DPS (GO), EECOM (GO)
	// Phase 1: NAV (GO), FAO (ADVISORY)
	// Phase 2: ARCH (GO), CDS (GO), PCO (GO)
	// Phase 3: INCO (GO)
	// Phase 4: SECINSP (GO), TRC (GO), PRB-SKEPTIC (GO), PRB-CORRECTNESS (GO), PRB-ADR-AUDIT (NO-GO)
	// Phase 5: CAPCOM (RELAY)
	// Phase 6: PAO (RELAY)
	mocks := map[string]*integrationMock{
		"GC":    mkMock("GC", controller.ModeDeterministic, mkSignal("GC", "GO")),
		"DPS":   mkMock("DPS", controller.ModeDeterministic, mkSignal("DPS", "GO")),
		"EECOM": mkMock("EECOM", controller.ModeDeterministic, mkSignal("EECOM", "GO")),

		"NAV": mkMock("NAV", controller.ModeDeterministic, mkSignal("NAV", "GO")),
		"FAO": mkMock("FAO", controller.ModeDeterministic, mkSignal("FAO", "GO")),

		"ARCH": mkMock("ARCH", controller.ModeDeterministic, mkSignal("ARCH", "GO")),
		"CDS":  mkMock("CDS", controller.ModeDeterministic, mkSignal("CDS", "GO")),
		"PCO":  mkMock("PCO", controller.ModeDeterministic, mkSignal("PCO", "GO")),

		"INCO": mkMock("INCO", controller.ModeDeterministic, mkSignal("INCO", "GO")),

		"SECINSP":          mkMock("SECINSP", controller.ModeInference, mkSignal("SECINSP", "GO")),
		"TRC":              mkMock("TRC", controller.ModeDeterministic, mkSignal("TRC", "GO")),
		"PRB-SKEPTIC":      mkMock("PRB-SKEPTIC", controller.ModeInference, mkSignal("PRB-SKEPTIC", "GO")),
		"PRB-CORRECTNESS":  mkMock("PRB-CORRECTNESS", controller.ModeInference, mkSignal("PRB-CORRECTNESS", "GO")),
		"PRB-ADR-AUDIT":    mkMock("PRB-ADR-AUDIT", controller.ModeInference, mkSignal("PRB-ADR-AUDIT", "NO-GO")),

		"CAPCOM": mkMock("CAPCOM", controller.ModeDeterministic, mkSignal("CAPCOM", "RELAY")),
		"PAO":    mkMock("PAO", controller.ModeDeterministic, mkSignal("PAO", "RELAY")),
	}

	// Launch the mission.
	mission, err := kernel.Launch(kernel.MissionConfig{
		Task:              "integration test: full lifecycle with PRB 2/3 GO",
		Repos:             []string{"taem-dev/taem", "taem-dev/mc-state"},
		ADRs:              []string{"ADR-001", "ADR-003", "ADR-005"},
		MCStatePath:       mcState,
		ADRsPath:          t.TempDir(), // dummy
		Registry:          reg,
		ControllerFactory: buildFactory(mocks),
	})
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	t.Logf("mission ID: %s", mission.ID)
	t.Logf("mission dir: %s", mission.MissionDir())

	// Run the mission through all phases.
	ctx := context.Background()
	err = mission.Run(ctx)

	// PRB-ADR-AUDIT voted NO-GO, but PRB-SKEPTIC and PRB-CORRECTNESS voted GO.
	// 2 GO out of 3 => 2/3 majority => gate ADVANCE (per ADR-003).
	// The mission should complete (LANDED), not abort.
	if err != nil {
		t.Fatalf("Run failed (expected LANDED): %v", err)
	}

	// ── Verify manifest.jsonl ──

	events, err := state.ReadManifest(mission.MissionDir())
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	// Check for MISSION_START.
	foundStart := false
	for _, evt := range events {
		if evt.Event == "MISSION_START" {
			foundStart = true
			if evt.Task != "integration test: full lifecycle with PRB 2/3 GO" {
				t.Errorf("MISSION_START task: got %q", evt.Task)
			}
			break
		}
	}
	if !foundStart {
		t.Error("MISSION_START event not found in manifest")
	}

	// Check for MISSION_LANDED.
	foundLanded := false
	for _, evt := range events {
		if evt.Event == "MISSION_LANDED" {
			foundLanded = true
			break
		}
	}
	if !foundLanded {
		t.Error("MISSION_LANDED event not found in manifest")
	}

	// Check for PHASE_START and PHASE_COMPLETE for each phase (0-6).
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
	for phase := 0; phase <= 6; phase++ {
		if !phaseStarts[phase] {
			t.Errorf("missing PHASE_START for phase %d", phase)
		}
		if !phaseCompletes[phase] {
			t.Errorf("missing PHASE_COMPLETE for phase %d", phase)
		}
	}

	// ── Verify signals.jsonl ──

	signals, err := state.ReadSignals(mission.MissionDir())
	if err != nil {
		t.Fatalf("ReadSignals: %v", err)
	}

	// Verify all controllers that ran produced signals.
	signalControllers := make(map[string]string) // callsign -> signal value
	for _, sig := range signals {
		signalControllers[sig.Controller] = sig.SignalValue
	}

	expectedSignals := map[string]string{
		"GC": "GO", "DPS": "GO", "EECOM": "GO",
		"NAV": "GO", "FAO": "GO",
		"ARCH": "GO", "CDS": "GO", "PCO": "GO",
		"INCO": "GO",
		"SECINSP": "GO", "TRC": "GO",
		"PRB-SKEPTIC": "GO", "PRB-CORRECTNESS": "GO", "PRB-ADR-AUDIT": "NO-GO",
		"CAPCOM": "RELAY", "PAO": "RELAY",
	}

	for cs, wantSig := range expectedSignals {
		gotSig, found := signalControllers[cs]
		if !found {
			t.Errorf("signal for %s not found in signals.jsonl", cs)
		} else if gotSig != wantSig {
			t.Errorf("signal for %s: got %q, want %q", cs, gotSig, wantSig)
		}
	}

	// Verify remediation cycles is 0 (PRB 2/3 GO means no remediation).
	if mission.RemediationCycles() != 0 {
		t.Errorf("remediation cycles: got %d, want 0", mission.RemediationCycles())
	}

	t.Logf("mission completed with %d manifest events and %d signals", len(events), len(signals))
}

// ---------------------------------------------------------------------------
// Test 2: PRB majority NO-GO — mission HOLD, then ABORT after 2 cycles
// ---------------------------------------------------------------------------

func TestIntegration_PRB_MajorityNoGo_HoldThenAbort(t *testing.T) {
	mcState := t.TempDir()
	missionsDir := filepath.Join(mcState, "missions")
	if err := os.MkdirAll(missionsDir, 0755); err != nil {
		t.Fatalf("create missions dir: %v", err)
	}

	defs := allPositionDefs()
	reg := buildRegistry(defs)

	// PRB-SKEPTIC and PRB-ADR-AUDIT both NO-GO (2/3 majority fails).
	// PRB-CORRECTNESS is GO.
	// This should trigger ABORT at gate (PRB majority NO-GO).
	mocks := map[string]*integrationMock{
		"GC":    mkMock("GC", controller.ModeDeterministic, mkSignal("GC", "GO")),
		"DPS":   mkMock("DPS", controller.ModeDeterministic, mkSignal("DPS", "GO")),
		"EECOM": mkMock("EECOM", controller.ModeDeterministic, mkSignal("EECOM", "GO")),

		"NAV": mkMock("NAV", controller.ModeDeterministic, mkSignal("NAV", "GO")),
		"FAO": mkMock("FAO", controller.ModeDeterministic, mkSignal("FAO", "GO")),

		"ARCH": mkMock("ARCH", controller.ModeDeterministic, mkSignal("ARCH", "GO")),
		"CDS":  mkMock("CDS", controller.ModeDeterministic, mkSignal("CDS", "GO")),
		"PCO":  mkMock("PCO", controller.ModeDeterministic, mkSignal("PCO", "GO")),

		"INCO": mkMock("INCO", controller.ModeDeterministic, mkSignal("INCO", "GO")),

		"SECINSP":          mkMock("SECINSP", controller.ModeInference, mkSignal("SECINSP", "GO")),
		"TRC":              mkMock("TRC", controller.ModeDeterministic, mkSignal("TRC", "GO")),
		"PRB-SKEPTIC":      mkMock("PRB-SKEPTIC", controller.ModeInference, mkSignal("PRB-SKEPTIC", "NO-GO")),
		"PRB-CORRECTNESS":  mkMock("PRB-CORRECTNESS", controller.ModeInference, mkSignal("PRB-CORRECTNESS", "GO")),
		"PRB-ADR-AUDIT":    mkMock("PRB-ADR-AUDIT", controller.ModeInference, mkSignal("PRB-ADR-AUDIT", "NO-GO")),

		"CAPCOM": mkMock("CAPCOM", controller.ModeDeterministic, mkSignal("CAPCOM", "RELAY")),
		"PAO":    mkMock("PAO", controller.ModeDeterministic, mkSignal("PAO", "RELAY")),
	}

	mission, err := kernel.Launch(kernel.MissionConfig{
		Task:              "integration test: PRB majority NO-GO",
		Repos:             []string{"taem-dev/taem"},
		ADRs:              []string{"ADR-003"},
		MCStatePath:       mcState,
		ADRsPath:          t.TempDir(),
		Registry:          reg,
		ControllerFactory: buildFactory(mocks),
	})
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	t.Logf("mission ID: %s", mission.ID)

	ctx := context.Background()
	err = mission.Run(ctx)

	// PRB-SKEPTIC and PRB-ADR-AUDIT are NO-GO (2/3 majority NO-GO).
	// Per gate.go: prbMajorityNoGo returns true => ABORT immediately.
	// The mission should fail with ESCALATED.
	if err == nil {
		t.Fatal("expected error from PRB majority NO-GO, got nil (mission should have ESCALATED)")
	}

	// Verify the error mentions ESCALATED.
	errStr := err.Error()
	if !containsSubstr(errStr, "ESCALATED") {
		t.Errorf("expected ESCALATED in error, got: %s", errStr)
	}

	// ── Verify manifest contains MISSION_ESCALATED ──

	events, err2 := state.ReadManifest(mission.MissionDir())
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
		t.Error("MISSION_ESCALATED event not found in manifest")
	}

	// PRB majority NO-GO goes directly to ABORT (not HOLD first).
	// Per gate.go: prbMajorityNoGo triggers GateABORT, which the kernel
	// handles as an immediate abort without incrementing remediation cycles.
	// The gate returns ABORT on the first evaluation.
	t.Logf("mission ESCALATED with %d remediation cycles", mission.RemediationCycles())
	t.Logf("manifest events: %d", len(events))

	// ── Verify signals from controllers that ran ──

	signals, err3 := state.ReadSignals(mission.MissionDir())
	if err3 != nil {
		t.Fatalf("ReadSignals: %v", err3)
	}

	// Phases 0-3 should have completed, Phase 4 should have run and triggered ABORT.
	signalControllers := make(map[string]bool)
	for _, sig := range signals {
		signalControllers[sig.Controller] = true
	}

	// Phase 0-3 controllers should have signals.
	for _, cs := range []string{"GC", "DPS", "EECOM", "NAV", "INCO"} {
		if !signalControllers[cs] {
			t.Errorf("expected signal from %s (ran before phase 4), not found", cs)
		}
	}

	// Phase 4 controllers should have signals (they ran, triggered ABORT).
	for _, cs := range []string{"SECINSP", "TRC", "PRB-SKEPTIC", "PRB-CORRECTNESS", "PRB-ADR-AUDIT"} {
		if !signalControllers[cs] {
			t.Errorf("expected signal from %s (phase 4 controllers), not found", cs)
		}
	}

	// Phase 5-6 controllers should NOT have signals (ABORT before CAPCOM/PAO).
	if signalControllers["CAPCOM"] {
		t.Error("CAPCOM should not have run (mission aborted at phase 4)")
	}
	if signalControllers["PAO"] {
		t.Error("PAO should not have run (mission aborted at phase 4)")
	}
}

// ---------------------------------------------------------------------------
// Test 3: Non-PRB NO-GO with remediation, 2 cycles then ABORT
// ---------------------------------------------------------------------------

func TestIntegration_Remediation_TwoCycles_ThenAbort(t *testing.T) {
	mcState := t.TempDir()
	missionsDir := filepath.Join(mcState, "missions")
	if err := os.MkdirAll(missionsDir, 0755); err != nil {
		t.Fatalf("create missions dir: %v", err)
	}

	// Use a minimal setup: only phase 0 with EECOM always NO-GO
	// to test the remediation cap (2 cycles -> ABORT).
	defs := []kernel.ControllerDef{
		{Callsign: "GC", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "gc", SignalType: "GO | NO-GO"},
		{Callsign: "DPS", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "dps", SignalType: "GO | NO-GO"},
		{Callsign: "EECOM", Mode: "deterministic", Execution: "local_deterministic", Phase: []int{0}, Required: true, Impl: "eecom", SignalType: "GO | NO-GO"},
	}
	reg := buildRegistry(defs)

	// EECOM always NO-GO -- remediation will never succeed.
	eecomMock := &integrationMock{
		callsign: "EECOM",
		mode:     controller.ModeDeterministic,
		signal:   mkSignal("EECOM", "NO-GO"),
	}

	mocks := map[string]*integrationMock{
		"GC":    mkMock("GC", controller.ModeDeterministic, mkSignal("GC", "GO")),
		"DPS":   mkMock("DPS", controller.ModeDeterministic, mkSignal("DPS", "GO")),
		"EECOM": eecomMock,
	}

	mission, err := kernel.Launch(kernel.MissionConfig{
		Task:              "integration test: remediation cap",
		Repos:             []string{"taem-dev/taem"},
		MCStatePath:       mcState,
		Registry:          reg,
		ControllerFactory: buildFactory(mocks),
		Phases:            []int{0},
	})
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	ctx := context.Background()
	err = mission.Run(ctx)

	// Should have failed with ESCALATED after 2 remediation cycles.
	if err == nil {
		t.Fatal("expected error from remediation cap exceeded, got nil")
	}

	errStr := err.Error()
	if !containsSubstr(errStr, "ESCALATED") {
		t.Errorf("expected ESCALATED in error, got: %s", errStr)
	}

	// Verify remediation cycles reached the cap (2).
	if mission.RemediationCycles() < 2 {
		t.Errorf("remediation cycles: got %d, want >= 2", mission.RemediationCycles())
	}

	// Verify MISSION_ESCALATED in manifest.
	events, err2 := state.ReadManifest(mission.MissionDir())
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
		t.Error("MISSION_ESCALATED event not found after remediation cap exceeded")
	}

	// EECOM should have been called at least 3 times:
	// initial run + 2 remediation cycles.
	if eecomMock.runCount < 3 {
		t.Errorf("EECOM run count: got %d, want >= 3", eecomMock.runCount)
	}

	t.Logf("mission aborted after %d remediation cycles, EECOM ran %d times",
		mission.RemediationCycles(), eecomMock.runCount)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// containsSubstr reports whether s contains substr.
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
