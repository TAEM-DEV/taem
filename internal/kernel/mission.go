// internal/kernel/mission.go
//
// Supplementary mission lifecycle entry points for the TAEM kernel.
// LaunchParams and RetryParams are convenience types used by cmd/taem/main.go
// to marshal CLI flags into kernel calls.
//
// The authoritative Mission struct, Launch(), and Run() live in kernel.go (step 11).

package kernel

import (
	"fmt"
	"time"

	"github.com/taem-dev/taem/internal/controller"
	"github.com/taem-dev/taem/internal/dispatch"
	"github.com/taem-dev/taem/internal/ecosystem"
	"github.com/taem-dev/taem/internal/state"
)

// LaunchParams holds the inputs required to create a new mission from the CLI.
// Convenience wrapper around MissionConfig for the launch command.
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

// DefaultControllerFactory is a placeholder factory that returns nil for
// all controller definitions. In production, main.go should wire a real
// factory. This allows LaunchFromParams to compile without requiring the
// caller to provide a factory.
var DefaultControllerFactory ControllerFactory = func(def ControllerDef) controller.Controller {
	return nil
}

// LaunchFromParams creates and launches a mission from LaunchParams.
// This is a convenience bridge for cmd/taem/main.go; it translates
// LaunchParams into MissionConfig and calls the authoritative Launch().
func LaunchFromParams(params LaunchParams) (*Mission, error) {
	var ecoClient *ecosystem.EcosystemClient
	if params.EcosystemURL != "" {
		ecoClient = ecosystem.NewClient(params.EcosystemURL)
	}

	return Launch(MissionConfig{
		Task:              params.Task,
		Repos:             params.Repos,
		ADRs:              params.ADRs,
		MCStatePath:       params.MCStatePath,
		ADRsPath:          params.ADRsPath,
		EcosystemURL:      params.EcosystemURL,
		Registry:          params.Registry,
		ControllerFactory: DefaultControllerFactory,
		EcosystemClient:   ecoClient,
	})
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

	var ecoClient *ecosystem.EcosystemClient
	if params.EcosystemURL != "" {
		ecoClient = ecosystem.NewClient(params.EcosystemURL)
	}

	// Build phases from FromPhase onward.
	var phases []int
	for _, p := range defaultPhases {
		if p >= params.FromPhase {
			phases = append(phases, p)
		}
	}

	return &Mission{
		ID:                params.MissionID,
		Task:              "", // retry: task comes from original mission
		mcStatePath:       params.MCStatePath,
		adrsPath:          params.ADRsPath,
		missionDir:        missionDir,
		registry:          params.Registry,
		controllerFactory: DefaultControllerFactory,
		ecosystemClient:   ecoClient,
		localDispatch:     dispatch.NewLocalDispatcher(),
		phases:            phases,
		remediationCycles: 0,
	}, nil
}
