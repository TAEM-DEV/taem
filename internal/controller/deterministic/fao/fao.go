// Package fao implements the Flight Activities Officer controller.
//
// FAO timestamps phase start/completion, computes elapsed time, and
// emits ADVISORY signals. Non-blocking — ADVISORY does not gate
// phase advancement.
package fao

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/taem-dev/taem/internal/controller"
)

// expectedPhaseSeconds defines expected maximum duration per phase.
var expectedPhaseSeconds = map[int]float64{
	0: 30,  // pad check
	1: 300, // corpus ingestion
	2: 60,  // architectural survey
	3: 60,  // plan formulation
	4: 180, // pre-code inspection
	5: 30,  // CAPCOM output
	6: 60,  // PAO dispatch
}

// manifestEntry represents a single event in manifest.jsonl.
type manifestEntry struct {
	MissionID string  `json:"mission_id"`
	Phase     int     `json:"phase"`
	Event     string  `json:"event"`
	Timestamp string  `json:"timestamp"`
	DurationS float64 `json:"duration_s,omitempty"`
}

// Controller implements the FAO (Flight Activities Officer) deterministic controller.
type Controller struct {
	// Now is injectable for testing. Defaults to time.Now.
	Now func() time.Time
}

// New returns a new FAO controller instance.
func New() *Controller {
	return &Controller{Now: time.Now}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "FAO"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run computes phase elapsed time and emits an ADVISORY signal.
//
// Reads manifest.jsonl for phase timing entries. Computes elapsed time
// for the current phase. Always returns ADVISORY — non-blocking.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	now := c.Now()
	evidence := []string{
		fmt.Sprintf("phase=%d", inputs.Phase),
		fmt.Sprintf("timestamp=%s", now.UTC().Format(time.RFC3339)),
	}

	// Try to find the phase start time from manifest.jsonl.
	var elapsed float64
	if inputs.ManifestPath != "" {
		manifestFile := filepath.Join(inputs.ManifestPath, "manifest.jsonl")
		data, err := os.ReadFile(manifestFile)
		if err == nil {
			elapsed = c.computeElapsed(data, inputs.Phase, now)
			evidence = append(evidence, fmt.Sprintf("elapsed_s=%.1f", elapsed))
		}
	}

	reason := fmt.Sprintf("phase %d timing: %.1fs elapsed", inputs.Phase, elapsed)

	// Check if phase exceeds expected window.
	expected, ok := expectedPhaseSeconds[inputs.Phase]
	if ok && elapsed > expected {
		reason = fmt.Sprintf("phase %d exceeds expected window: %.1fs elapsed (expected %.0fs)", inputs.Phase, elapsed, expected)
		evidence = append(evidence, fmt.Sprintf("expected_s=%.0f", expected), "status=exceeded")
	} else {
		evidence = append(evidence, "status=nominal")
	}

	return controller.Signal{
		Controller:  "FAO",
		SignalValue: "ADVISORY",
		Reason:      reason,
		Evidence:    evidence,
	}, nil
}

// computeElapsed finds the phase start entry in manifest data and returns
// the elapsed seconds since that timestamp.
func (c *Controller) computeElapsed(data []byte, phase int, now time.Time) float64 {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry manifestEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Phase == phase && entry.Event == "start" && entry.Timestamp != "" {
			t, err := time.Parse(time.RFC3339, entry.Timestamp)
			if err == nil {
				return now.Sub(t).Seconds()
			}
		}
	}
	return 0
}
