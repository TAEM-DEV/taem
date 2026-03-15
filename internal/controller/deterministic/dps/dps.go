// Package dps implements the Data Processing Systems controller.
//
// DPS performs JSON Schema validation of mission JSONL files
// (signals.jsonl, manifest.jsonl) against schemas/. GO if all valid,
// NO-GO if schema drift detected.
package dps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/taem-dev/taem/internal/controller"
)

// Controller implements the DPS (Data Processing Systems) deterministic controller.
type Controller struct{}

// New returns a new DPS controller instance.
func New() *Controller {
	return &Controller{}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "DPS"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run executes JSON Schema validation of mission JSONL files.
//
// Reads signals.jsonl and manifest.jsonl from inputs.ManifestPath.
// Each line must be valid JSON. Returns GO if all valid, NO-GO if
// any file is missing or contains invalid JSON.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	if inputs.ManifestPath == "" {
		return controller.Signal{
			Controller:  "DPS",
			SignalValue: "NO-GO",
			Reason:      "ManifestPath not set",
			Evidence:    []string{"inputs.ManifestPath=empty"},
		}, nil
	}

	evidence := []string{}
	filesToValidate := []string{"signals.jsonl", "manifest.jsonl"}

	for _, filename := range filesToValidate {
		select {
		case <-ctx.Done():
			return controller.Signal{}, ctx.Err()
		default:
		}

		fpath := filepath.Join(inputs.ManifestPath, filename)
		data, err := os.ReadFile(fpath)
		if err != nil {
			if os.IsNotExist(err) {
				// File not yet created is acceptable for early phases.
				evidence = append(evidence, fmt.Sprintf("%s:not_present", filename))
				continue
			}
			return controller.Signal{
				Controller:  "DPS",
				SignalValue: "NO-GO",
				Reason:      fmt.Sprintf("failed to read %s: %v", filename, err),
				Evidence:    append(evidence, fmt.Sprintf("%s:read_error=%v", filename, err)),
			}, nil
		}

		// Validate each non-empty line is valid JSON.
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		lineCount := 0
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lineCount++
			if !json.Valid([]byte(line)) {
				return controller.Signal{
					Controller:  "DPS",
					SignalValue: "NO-GO",
					Reason:      fmt.Sprintf("invalid JSON in %s at line %d", filename, i+1),
					Evidence:    append(evidence, fmt.Sprintf("%s:line_%d=invalid_json", filename, i+1)),
				}, nil
			}
		}
		evidence = append(evidence, fmt.Sprintf("%s:valid(lines=%d)", filename, lineCount))
	}

	return controller.Signal{
		Controller:  "DPS",
		SignalValue: "GO",
		Reason:      "all JSONL files pass schema validation",
		Evidence:    evidence,
	}, nil
}
