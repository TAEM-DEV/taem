package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/taem-dev/taem/internal/controller"
)

// ReadSignals reads all signals from signals.jsonl in the given mission
// directory. Returns an empty slice (not an error) if the file does not
// exist yet — a new mission has no signals until the first controller runs.
func ReadSignals(missionDir string) ([]controller.Signal, error) {
	p := filepath.Join(missionDir, signalsFile)

	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var signals []controller.Signal
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var sig controller.Signal
		if err := json.Unmarshal(line, &sig); err != nil {
			return nil, err
		}
		signals = append(signals, sig)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return signals, nil
}

// ReadManifest reads all manifest events from manifest.jsonl in the given
// mission directory. Returns an empty slice if the file does not exist.
func ReadManifest(missionDir string) ([]ManifestEvent, error) {
	p := filepath.Join(missionDir, manifestFile)

	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []ManifestEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt ManifestEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			return nil, err
		}
		events = append(events, evt)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// ReadIntegrationMap reads the raw bytes of integration-map.json from the
// given mission directory. Returns an error if the file does not exist
// (unlike signal/manifest which return empty slices).
func ReadIntegrationMap(missionDir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(missionDir, integrationMapFile))
}

// ReadStepPlan reads the raw bytes of step-plan.json from the given
// mission directory. Returns an error if the file does not exist.
func ReadStepPlan(missionDir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(missionDir, stepPlanFile))
}
