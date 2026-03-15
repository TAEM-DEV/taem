// internal/kernel/mission.go
//
// Mission lifecycle types for the TAEM kernel.
// This file defines the Launch/Retry entry points and Mission struct.
// Step 11 (kernel.go) will expand this with the full phase sequencer.
//
// These types are used by cmd/taem/main.go (step 12) to wire the CLI
// to the kernel.

package kernel

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/taem-dev/taem/internal/ecosystem"
	"github.com/taem-dev/taem/internal/state"
)

// LaunchParams holds the inputs required to create a new mission.
type LaunchParams struct {
	Repos        []string
	Task         string
	ADRs         []string
	Registry     *Registry
	MCStatePath  string
	ADRsPath     string
	EcosystemURL string
}

// RetryParams holds the inputs required to retry a failed mission.
type RetryParams struct {
	MissionID    string
	FromPhase    int
	Registry     *Registry
	MCStatePath  string
	ADRsPath     string
	EcosystemURL string
}

// Mission represents an active preflight mission managed by the kernel.
// The full phase sequencer, controller dispatch, and gate evaluation
// will be implemented in kernel.go (step 11).
type Mission struct {
	ID          string
	Params      LaunchParams
	FromPhase   int
	mcStatePath string
	adrsPath    string
	registry    *Registry
	eco         *ecosystem.EcosystemClient
}

// Launch creates a new mission, initializes its mc-state directory,
// and writes the MISSION_START manifest event.
func Launch(params LaunchParams) (*Mission, error) {
	// Create mission directory in mc-state.
	missionID := fmt.Sprintf("M%d", time.Now().UnixNano())
	missionDir := params.MCStatePath + "/missions/" + missionID

	if err := os.MkdirAll(missionDir, 0755); err != nil {
		return nil, fmt.Errorf("creating mission directory: %w", err)
	}

	// Write initial manifest event.
	phase := 0
	ts := time.Now().UTC().Format(time.RFC3339)
	err := state.AppendManifest(missionDir, state.ManifestEvent{
		EventID:   fmt.Sprintf("E%d", time.Now().UnixNano()),
		MissionID: missionID,
		Event:     "MISSION_START",
		Phase:     &phase,
		Task:      params.Task,
		Repos:     params.Repos,
		ADRs:      params.ADRs,
		Timestamp: ts,
	})
	if err != nil {
		return nil, fmt.Errorf("writing MISSION_START: %w", err)
	}

	return &Mission{
		ID:          missionID,
		Params:      params,
		FromPhase:   0,
		mcStatePath: params.MCStatePath,
		adrsPath:    params.ADRsPath,
		registry:    params.Registry,
		eco:         ecosystem.NewClient(params.EcosystemURL),
	}, nil
}

// Retry creates a mission that resumes from a previously failed phase.
func Retry(params RetryParams) (*Mission, error) {
	missionDir := params.MCStatePath + "/missions/" + params.MissionID

	// Write retry manifest event.
	ts := time.Now().UTC().Format(time.RFC3339)
	err := state.AppendManifest(missionDir, state.ManifestEvent{
		EventID:   fmt.Sprintf("E%d", time.Now().UnixNano()),
		MissionID: params.MissionID,
		Event:     "MISSION_RETRY",
		Phase:     &params.FromPhase,
		Timestamp: ts,
	})
	if err != nil {
		return nil, fmt.Errorf("writing MISSION_RETRY: %w", err)
	}

	return &Mission{
		ID:          params.MissionID,
		FromPhase:   params.FromPhase,
		mcStatePath: params.MCStatePath,
		adrsPath:    params.ADRsPath,
		registry:    params.Registry,
		eco:         ecosystem.NewClient(params.EcosystemURL),
	}, nil
}

// Run executes the mission phase sequencer. It iterates through phases
// starting from m.FromPhase, dispatching controllers and evaluating gates.
//
// This is a placeholder implementation. The full phase sequencer will be
// implemented in kernel.go (step 11) with:
//   - Per-phase controller dispatch via local.RunPhase() and github dispatch
//   - Gate evaluation via Evaluate() after each phase
//   - Signal and manifest persistence via state.AppendSignal/AppendManifest
//   - Context cancellation propagation to all controllers
func (m *Mission) Run(ctx context.Context) error {
	// Placeholder: the full sequencer will be wired in kernel.go (step 11).
	// For now, this validates that the mission can start.
	if err := m.eco.Health(); err != nil {
		return fmt.Errorf("ecosystem health check failed: %w", err)
	}
	return fmt.Errorf("kernel.go phase sequencer not yet implemented (step 11)")
}
