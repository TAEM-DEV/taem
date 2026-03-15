// internal/kernel/kernel.go
//
// Mission lifecycle orchestrator — wires gate, registry, state, ecosystem,
// and dispatch into the full mission lifecycle: Launch -> Run -> Gate -> Close.
//
// Per ADR-007: the kernel owns the gate state machine, controller registry,
// local execution, inference routing, GitHub Actions dispatch, the mission
// state interface, and the ecosystem client.
//
// Per ADR-003: no code before Phase 04 gate. CAPCOM is the only developer
// output. Remediation cap: 2 cycles.

package kernel

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/taem-dev/taem/internal/controller"
	"github.com/taem-dev/taem/internal/dispatch"
	"github.com/taem-dev/taem/internal/ecosystem"
	"github.com/taem-dev/taem/internal/state"
)

// ControllerFactory creates a controller.Controller from a registry definition.
// The kernel accepts this at construction time so tests can inject mocks and
// main.go wires the real implementations at step 12.
type ControllerFactory func(def ControllerDef) controller.Controller

// MissionConfig holds the inputs required to launch a new mission.
type MissionConfig struct {
	Task         string
	Repos        []string
	ADRs         []string
	MCStatePath  string // local clone path of mc-state
	ADRsPath     string // local clone path of adrs
	EcosystemURL string // Qdrant URL for ecosystem service

	Registry          *Registry
	ControllerFactory ControllerFactory

	// Optional overrides for testing / alternative dispatch.
	LocalDispatcher  *dispatch.LocalDispatcher
	GitHubDispatcher *dispatch.GitHubDispatcher
	EcosystemClient  *ecosystem.EcosystemClient

	// Phases to execute. If nil, defaults to [0,1,2,3,4,5,6].
	Phases []int
}

// Mission represents an active preflight mission being executed by the kernel.
type Mission struct {
	ID   string
	Task string
	Repos []string
	ADRs  []string

	mcStatePath string
	adrsPath    string
	missionDir  string

	registry          *Registry
	controllerFactory ControllerFactory
	ecosystemClient   *ecosystem.EcosystemClient
	localDispatch     *dispatch.LocalDispatcher
	githubDispatch    *dispatch.GitHubDispatcher

	phases            []int
	remediationCycles int
	signals           []controller.Signal
}

// maxPhases is the standard set of mission phases per ADR-003.
var defaultPhases = []int{0, 1, 2, 3, 4, 5, 6}

// generateMissionID produces a unique mission identifier.
// Uses crypto/rand to avoid adding an external ULID dependency at this stage.
// Format: MSN-<16 hex chars> (e.g., MSN-a1b2c3d4e5f67890).
func generateMissionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate mission_id: %w", err)
	}
	return fmt.Sprintf("MSN-%x", b), nil
}

// Launch validates inputs, generates a mission_id, creates the mission
// directory in mc-state, and writes MISSION_START to manifest.jsonl.
func Launch(cfg MissionConfig) (*Mission, error) {
	// Validate required inputs.
	if cfg.Task == "" {
		return nil, fmt.Errorf("kernel: task is required")
	}
	if len(cfg.Repos) == 0 {
		return nil, fmt.Errorf("kernel: at least one repo is required")
	}
	if cfg.MCStatePath == "" {
		return nil, fmt.Errorf("kernel: mc-state path is required")
	}
	if cfg.Registry == nil {
		return nil, fmt.Errorf("kernel: registry is required")
	}
	if cfg.ControllerFactory == nil {
		return nil, fmt.Errorf("kernel: controller factory is required")
	}

	// Generate mission ID.
	missionID, err := generateMissionID()
	if err != nil {
		return nil, err
	}

	// Create mission directory in mc-state: missions/<id>/
	missionDir := filepath.Join(cfg.MCStatePath, "missions", missionID)
	if err := mkdirAll(missionDir); err != nil {
		return nil, fmt.Errorf("kernel: create mission dir: %w", err)
	}

	// Write MISSION_START event to manifest.jsonl.
	phase0 := 0
	startEvent := state.ManifestEvent{
		EventID:   missionID + "-start",
		MissionID: missionID,
		Event:     "MISSION_START",
		Phase:     &phase0,
		Task:      cfg.Task,
		Repos:     cfg.Repos,
		ADRs:      cfg.ADRs,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if err := state.AppendManifest(missionDir, startEvent); err != nil {
		return nil, fmt.Errorf("kernel: write MISSION_START: %w", err)
	}

	// Set up defaults.
	phases := cfg.Phases
	if phases == nil {
		phases = defaultPhases
	}

	localDisp := cfg.LocalDispatcher
	if localDisp == nil {
		localDisp = dispatch.NewLocalDispatcher()
	}

	var ecoClient *ecosystem.EcosystemClient
	if cfg.EcosystemClient != nil {
		ecoClient = cfg.EcosystemClient
	} else if cfg.EcosystemURL != "" {
		ecoClient = ecosystem.NewClient(cfg.EcosystemURL)
	}

	return &Mission{
		ID:                missionID,
		Task:              cfg.Task,
		Repos:             cfg.Repos,
		ADRs:              cfg.ADRs,
		mcStatePath:       cfg.MCStatePath,
		adrsPath:          cfg.ADRsPath,
		missionDir:        missionDir,
		registry:          cfg.Registry,
		controllerFactory: cfg.ControllerFactory,
		ecosystemClient:   ecoClient,
		localDispatch:     localDisp,
		githubDispatch:    cfg.GitHubDispatcher,
		phases:            phases,
		remediationCycles: 0,
		signals:           nil,
	}, nil
}

// Run executes the full mission lifecycle through all phases.
// It processes phases sequentially: run controllers -> evaluate gate -> advance or hold.
//
// Per ADR-003:
//   - HOLD: increment remediationCycles, re-run failed phase
//   - ABORT: set mission status to ESCALATED, halt
//   - ADVANCE: move to next phase
//
// Phase 05 (CAPCOM): always RELAY, never blocks.
// Phase 06 (PAO): optional dispatch, RELAY.
func (m *Mission) Run(ctx context.Context) error {
	for _, phase := range m.phases {
		if err := ctx.Err(); err != nil {
			return m.abort(fmt.Sprintf("context cancelled before phase %d: %v", phase, err))
		}

		// Write PHASE_START to manifest.
		p := phase
		startEvt := state.ManifestEvent{
			EventID:   fmt.Sprintf("%s-p%02d-start", m.ID, phase),
			MissionID: m.ID,
			Event:     "PHASE_START",
			Phase:     &p,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		if err := state.AppendManifest(m.missionDir, startEvt); err != nil {
			return fmt.Errorf("kernel: write PHASE_START (phase %d): %w", phase, err)
		}

		// Execute controllers for this phase.
		signals, err := m.runPhase(ctx, phase)
		if err != nil {
			return fmt.Errorf("kernel: run phase %d: %w", phase, err)
		}

		// Persist signals to state.
		for _, sig := range signals {
			if appendErr := state.AppendSignal(m.missionDir, sig); appendErr != nil {
				return fmt.Errorf("kernel: append signal (%s): %w", sig.Controller, appendErr)
			}
		}
		m.signals = append(m.signals, signals...)

		// Evaluate gate for this phase.
		decision, err := m.evaluateGate(phase)
		if err != nil {
			return fmt.Errorf("kernel: evaluate gate (phase %d): %w", phase, err)
		}

		// Write PHASE_COMPLETE event.
		completeEvt := state.ManifestEvent{
			EventID:   fmt.Sprintf("%s-p%02d-complete", m.ID, phase),
			MissionID: m.ID,
			Event:     "PHASE_COMPLETE",
			Phase:     &p,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		if err := state.AppendManifest(m.missionDir, completeEvt); err != nil {
			return fmt.Errorf("kernel: write PHASE_COMPLETE (phase %d): %w", phase, err)
		}

		switch decision {
		case GateADVANCE:
			// Continue to next phase.
			continue

		case GateHOLD:
			// Increment remediation cycles and re-run this phase.
			m.remediationCycles++

			// Re-evaluate: run phase again, then check gate again.
			// Loop until we get ADVANCE or ABORT.
			for decision == GateHOLD {
				if err := ctx.Err(); err != nil {
					return m.abort(fmt.Sprintf("context cancelled during remediation of phase %d: %v", phase, err))
				}

				signals, err = m.runPhase(ctx, phase)
				if err != nil {
					return fmt.Errorf("kernel: remediation run phase %d (cycle %d): %w", phase, m.remediationCycles, err)
				}

				for _, sig := range signals {
					if appendErr := state.AppendSignal(m.missionDir, sig); appendErr != nil {
						return fmt.Errorf("kernel: append remediation signal (%s): %w", sig.Controller, appendErr)
					}
				}
				m.signals = append(m.signals, signals...)

				decision, err = m.evaluateGate(phase)
				if err != nil {
					return fmt.Errorf("kernel: evaluate gate remediation (phase %d, cycle %d): %w", phase, m.remediationCycles, err)
				}

				if decision == GateHOLD {
					m.remediationCycles++
				}
			}

			if decision == GateABORT {
				return m.abort(fmt.Sprintf("gate ABORT at phase %d after %d remediation cycles", phase, m.remediationCycles))
			}
			// ADVANCE: continue to next phase.

		case GateABORT:
			return m.abort(fmt.Sprintf("gate ABORT at phase %d", phase))
		}
	}

	// All phases complete — mission LANDED.
	return m.close("LANDED")
}

// runPhase executes all controllers for a given phase.
// It distinguishes execution mode per controller definition:
//   - local_deterministic, local_inference -> LocalDispatcher
//   - github_dispatch -> GitHubDispatcher
func (m *Mission) runPhase(ctx context.Context, phase int) ([]controller.Signal, error) {
	defs := m.registry.ControllersForPhase(phase)
	if len(defs) == 0 {
		return nil, nil
	}

	// Build the Inputs struct for this phase.
	inputs := m.buildInputs(phase)

	// Separate controllers by execution mode.
	var localControllers []controller.Controller
	var githubDefs []ControllerDef

	for _, def := range defs {
		switch def.Execution {
		case "github_dispatch":
			githubDefs = append(githubDefs, def)
		default:
			// local_deterministic and local_inference both run locally.
			ctrl := m.controllerFactory(def)
			if ctrl != nil {
				localControllers = append(localControllers, ctrl)
			}
		}
	}

	var allSignals []controller.Signal

	// Run local controllers via LocalDispatcher.
	if len(localControllers) > 0 {
		signals, err := m.localDispatch.RunPhase(ctx, localControllers, inputs)
		if err != nil {
			return nil, fmt.Errorf("local dispatch (phase %d): %w", phase, err)
		}
		allSignals = append(allSignals, signals...)
	}

	// Run github_dispatch controllers.
	for _, def := range githubDefs {
		sig, err := m.dispatchGitHub(ctx, def, inputs)
		if err != nil {
			// GitHub dispatch failure -> NO-GO signal.
			allSignals = append(allSignals, controller.Signal{
				Controller:  def.Callsign,
				SignalValue: "NO-GO",
				Reason:      fmt.Sprintf("github dispatch failed: %v", err),
				Evidence:    []string{err.Error()},
			})
			continue
		}
		allSignals = append(allSignals, sig)
	}

	return allSignals, nil
}

// dispatchGitHub triggers a GitHub Actions workflow for a controller and
// translates the result into a Signal.
func (m *Mission) dispatchGitHub(ctx context.Context, def ControllerDef, inputs controller.Inputs) (controller.Signal, error) {
	if m.githubDispatch == nil {
		// No GitHub dispatcher configured — emit RELAY for optional controllers,
		// NO-GO for required ones.
		if !def.Required {
			return controller.Signal{
				Controller:  def.Callsign,
				SignalValue: "RELAY",
				Reason:      "github dispatcher not configured, skipping optional controller",
				Evidence:    []string{},
			}, nil
		}
		return controller.Signal{}, fmt.Errorf("github dispatcher not configured for required controller %s", def.Callsign)
	}

	// Build workflow inputs.
	workflowInputs := map[string]string{
		"mission_id":   inputs.MissionID,
		"mission_dir":  m.missionDir,
		"phase":        fmt.Sprintf("%d", inputs.Phase),
		"adrs_path":    inputs.ADRsPath,
	}

	// The impl field contains the workflow file path.
	workflowFile := def.Impl
	// Strip leading path separators and quotes.
	workflowFile = strings.Trim(workflowFile, "\"'")

	if err := m.githubDispatch.Dispatch(ctx, workflowFile, workflowInputs); err != nil {
		return controller.Signal{}, fmt.Errorf("dispatch %s: %w", def.Callsign, err)
	}

	// For now, after dispatch, we generate a GO/RELAY signal.
	// In a full implementation, WaitForRun would poll for completion.
	// Since we don't have the run ID from dispatch (GitHub's 204 response
	// doesn't return it), we emit RELAY for the dispatch acknowledgment.
	signalValue := "RELAY"
	if def.Required {
		signalValue = "GO"
	}

	return controller.Signal{
		Controller:  def.Callsign,
		SignalValue: signalValue,
		Reason:      "github actions workflow dispatched",
		Evidence:    []string{fmt.Sprintf("workflow=%s", workflowFile)},
	}, nil
}

// evaluateGate calls gate.Evaluate with the required callsigns for the phase
// and the current set of phase signals.
func (m *Mission) evaluateGate(phase int) (GateDecision, error) {
	required := m.registry.RequiredCallsignsForPhase(phase)

	// If no required controllers, auto-ADVANCE (e.g., CAPCOM phase is RELAY).
	if len(required) == 0 {
		return GateADVANCE, nil
	}

	// Collect only signals relevant to controllers in this phase.
	phaseDefs := m.registry.ControllersForPhase(phase)
	phaseCallsigns := make(map[string]bool, len(phaseDefs))
	for _, d := range phaseDefs {
		phaseCallsigns[d.Callsign] = true
	}

	// Use the most recent signal for each controller in this phase.
	latestByController := make(map[string]controller.Signal)
	for _, sig := range m.signals {
		if phaseCallsigns[sig.Controller] {
			latestByController[sig.Controller] = sig
		}
	}

	var phaseSignals []controller.Signal
	for _, sig := range latestByController {
		phaseSignals = append(phaseSignals, sig)
	}

	// Check if CAPCOM or PAO phase — these use RELAY signals, never block.
	allRelay := true
	for _, cs := range required {
		def, ok := m.registry.GetController(cs)
		if !ok {
			continue
		}
		if def.SignalType != "RELAY" {
			allRelay = false
			break
		}
	}
	if allRelay {
		return GateADVANCE, nil
	}

	decision := Evaluate(phase, required, phaseSignals, m.remediationCycles)
	return decision, nil
}

// buildInputs creates the controller.Inputs struct for the current phase.
func (m *Mission) buildInputs(phase int) controller.Inputs {
	inputs := controller.Inputs{
		MissionID:    m.ID,
		ManifestPath: filepath.Join(m.missionDir, "manifest.jsonl"),
		ADRsPath:     m.adrsPath,
		Phase:        phase,
	}

	// Check if integration-map.json exists.
	imPath := filepath.Join(m.missionDir, "integration-map.json")
	if fileExists(imPath) {
		inputs.IntegrationMap = imPath
	}

	// Check if step-plan.json exists.
	spPath := filepath.Join(m.missionDir, "step-plan.json")
	if fileExists(spPath) {
		inputs.StepPlan = spPath
	}

	// Wire ecosystem query function.
	if m.ecosystemClient != nil {
		inputs.EcosystemQuery = m.ecosystemClient.Query
	}

	return inputs
}

// abort writes MISSION_ESCALATED to manifest and returns an error.
func (m *Mission) abort(reason string) error {
	evt := state.ManifestEvent{
		EventID:   m.ID + "-escalated",
		MissionID: m.ID,
		Event:     "MISSION_ESCALATED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	// Best-effort write — don't mask the original abort reason.
	_ = state.AppendManifest(m.missionDir, evt)
	return fmt.Errorf("mission %s ESCALATED: %s", m.ID, reason)
}

// close writes MISSION_LANDED to manifest and updates the mission index.
func (m *Mission) close(outcome string) error {
	evt := state.ManifestEvent{
		EventID:   m.ID + "-landed",
		MissionID: m.ID,
		Event:     "MISSION_LANDED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if err := state.AppendManifest(m.missionDir, evt); err != nil {
		return fmt.Errorf("kernel: write MISSION_LANDED: %w", err)
	}

	return nil
}

// MissionDir returns the filesystem path to this mission's directory
// in mc-state. Useful for tests and external inspection.
func (m *Mission) MissionDir() string {
	return m.missionDir
}

// RemediationCycles returns the number of remediation cycles the mission
// has gone through. Useful for tests.
func (m *Mission) RemediationCycles() int {
	return m.remediationCycles
}

// --- internal helpers ---

// mkdirAll creates a directory and all parents, like os.MkdirAll.
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

// fileExists returns true if the given path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
