// Package state provides the mc-state git interface for the TAEM kernel.
// It reads and writes JSONL signal/manifest files and raw JSON config files
// within mission directories.
package state

// ManifestEvent represents a single lifecycle event in manifest.jsonl.
type ManifestEvent struct {
	EventID   string   `json:"event_id"`
	MissionID string   `json:"mission_id"`
	Event     string   `json:"event"`      // MISSION_START, PHASE_START, PHASE_COMPLETE, etc.
	Phase     *int     `json:"phase"`
	Task      string   `json:"task,omitempty"`
	Repos     []string `json:"repos,omitempty"`
	ADRs      []string `json:"adrs,omitempty"`
	Timestamp string   `json:"timestamp"`
	DurationS *int     `json:"duration_s,omitempty"`
}

const (
	signalsFile        = "signals.jsonl"
	manifestFile       = "manifest.jsonl"
	integrationMapFile = "integration-map.json"
	stepPlanFile       = "step-plan.json"
)
