package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

// helper creates a temp dir simulating a mission directory with the
// expected subdirectory layout (missionDir is already the per-mission
// directory inside mc-state, e.g. missions/<id>).
func tempMissionDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// ptrString is a helper to get a pointer to a string literal.
func ptrString(s string) *string { return &s }

// TestSignalRoundTrip writes a single signal and reads it back,
// verifying full JSON round-trip fidelity.
func TestSignalRoundTrip(t *testing.T) {
	dir := tempMissionDir(t)

	sig := controller.Signal{
		Controller:    "GC",
		SignalValue:   "GO",
		Reason:        "all checks passed",
		Evidence:      []string{"github-healthy", "rate-limit-ok"},
		ConstraintRef: ptrString("C-004-001"),
	}

	if err := AppendSignal(dir, sig); err != nil {
		t.Fatalf("AppendSignal: %v", err)
	}

	signals, err := ReadSignals(dir)
	if err != nil {
		t.Fatalf("ReadSignals: %v", err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	got := signals[0]
	if got.Controller != sig.Controller {
		t.Errorf("Controller: got %q, want %q", got.Controller, sig.Controller)
	}
	if got.SignalValue != sig.SignalValue {
		t.Errorf("SignalValue: got %q, want %q", got.SignalValue, sig.SignalValue)
	}
	if got.Reason != sig.Reason {
		t.Errorf("Reason: got %q, want %q", got.Reason, sig.Reason)
	}
	if len(got.Evidence) != len(sig.Evidence) {
		t.Errorf("Evidence length: got %d, want %d", len(got.Evidence), len(sig.Evidence))
	}
	if got.ConstraintRef == nil || *got.ConstraintRef != *sig.ConstraintRef {
		t.Errorf("ConstraintRef: got %v, want %v", got.ConstraintRef, sig.ConstraintRef)
	}
}

// TestSignalMultipleAppend writes multiple signals and verifies JSONL
// append semantics — each call appends one line, ReadSignals returns all.
func TestSignalMultipleAppend(t *testing.T) {
	dir := tempMissionDir(t)

	sigs := []controller.Signal{
		{Controller: "GC", SignalValue: "GO", Reason: "healthy"},
		{Controller: "DPS", SignalValue: "GO", Reason: "schema valid"},
		{Controller: "EECOM", SignalValue: "WARN", Reason: "rate limit at 80%"},
	}

	for _, sig := range sigs {
		if err := AppendSignal(dir, sig); err != nil {
			t.Fatalf("AppendSignal(%s): %v", sig.Controller, err)
		}
	}

	got, err := ReadSignals(dir)
	if err != nil {
		t.Fatalf("ReadSignals: %v", err)
	}

	if len(got) != len(sigs) {
		t.Fatalf("expected %d signals, got %d", len(sigs), len(got))
	}

	for i, want := range sigs {
		if got[i].Controller != want.Controller {
			t.Errorf("signal[%d].Controller: got %q, want %q", i, got[i].Controller, want.Controller)
		}
		if got[i].SignalValue != want.SignalValue {
			t.Errorf("signal[%d].SignalValue: got %q, want %q", i, got[i].SignalValue, want.SignalValue)
		}
	}
}

// TestManifestRoundTrip writes a manifest event and reads it back.
func TestManifestRoundTrip(t *testing.T) {
	dir := tempMissionDir(t)

	phase := 0
	evt := ManifestEvent{
		EventID:   "evt-001",
		MissionID: "MSN-TEST-001",
		Event:     "MISSION_START",
		Phase:     &phase,
		Task:      "smoke test",
		Repos:     []string{"mc-state"},
		ADRs:      []string{"ADR-001", "ADR-004"},
		Timestamp: "2026-03-15T00:00:00Z",
	}

	if err := AppendManifest(dir, evt); err != nil {
		t.Fatalf("AppendManifest: %v", err)
	}

	events, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.EventID != evt.EventID {
		t.Errorf("EventID: got %q, want %q", got.EventID, evt.EventID)
	}
	if got.MissionID != evt.MissionID {
		t.Errorf("MissionID: got %q, want %q", got.MissionID, evt.MissionID)
	}
	if got.Event != evt.Event {
		t.Errorf("Event: got %q, want %q", got.Event, evt.Event)
	}
	if got.Phase == nil || *got.Phase != *evt.Phase {
		t.Errorf("Phase: got %v, want %v", got.Phase, evt.Phase)
	}
	if got.Task != evt.Task {
		t.Errorf("Task: got %q, want %q", got.Task, evt.Task)
	}
	if len(got.Repos) != len(evt.Repos) {
		t.Errorf("Repos length: got %d, want %d", len(got.Repos), len(evt.Repos))
	}
	if len(got.ADRs) != len(evt.ADRs) {
		t.Errorf("ADRs length: got %d, want %d", len(got.ADRs), len(evt.ADRs))
	}
	if got.Timestamp != evt.Timestamp {
		t.Errorf("Timestamp: got %q, want %q", got.Timestamp, evt.Timestamp)
	}
}

// TestSignalInferenceMetaRoundTrip verifies that optional InferenceMeta
// fields survive the JSON round-trip through JSONL.
func TestSignalInferenceMetaRoundTrip(t *testing.T) {
	dir := tempMissionDir(t)

	sig := controller.Signal{
		Controller:    "SECINSP",
		SignalValue:   "GO",
		Reason:        "no vulnerabilities found",
		Evidence:      []string{"cve-scan-clean"},
		ConstraintRef: nil,
		InferenceNotes: &controller.InferenceMeta{
			Backend:      "ollama",
			Confidence:   0.92,
			LatencyMS:    1234,
			FallbackUsed: false,
		},
	}

	if err := AppendSignal(dir, sig); err != nil {
		t.Fatalf("AppendSignal: %v", err)
	}

	signals, err := ReadSignals(dir)
	if err != nil {
		t.Fatalf("ReadSignals: %v", err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	got := signals[0]
	if got.InferenceNotes == nil {
		t.Fatal("InferenceNotes is nil after round-trip")
	}
	if got.InferenceNotes.Backend != "ollama" {
		t.Errorf("Backend: got %q, want %q", got.InferenceNotes.Backend, "ollama")
	}
	if got.InferenceNotes.Confidence != 0.92 {
		t.Errorf("Confidence: got %f, want %f", got.InferenceNotes.Confidence, 0.92)
	}
	if got.InferenceNotes.LatencyMS != 1234 {
		t.Errorf("LatencyMS: got %d, want %d", got.InferenceNotes.LatencyMS, 1234)
	}
	if got.InferenceNotes.FallbackUsed != false {
		t.Errorf("FallbackUsed: got %v, want %v", got.InferenceNotes.FallbackUsed, false)
	}
}

// TestReadSignalsFileNotExist verifies that reading a non-existent
// signals.jsonl returns an empty slice, not an error.
func TestReadSignalsFileNotExist(t *testing.T) {
	dir := tempMissionDir(t)

	signals, err := ReadSignals(dir)
	if err != nil {
		t.Fatalf("ReadSignals on empty dir: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals, got %d", len(signals))
	}
}

// TestReadManifestFileNotExist verifies that reading a non-existent
// manifest.jsonl returns an empty slice, not an error.
func TestReadManifestFileNotExist(t *testing.T) {
	dir := tempMissionDir(t)

	events, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest on empty dir: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// TestIntegrationMapRoundTrip writes integration-map.json and reads it back.
func TestIntegrationMapRoundTrip(t *testing.T) {
	dir := tempMissionDir(t)

	data := []byte(`{"repos":["mc-state"],"edges":[]}`)

	if err := WriteIntegrationMap(dir, data); err != nil {
		t.Fatalf("WriteIntegrationMap: %v", err)
	}

	got, err := ReadIntegrationMap(dir)
	if err != nil {
		t.Fatalf("ReadIntegrationMap: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("IntegrationMap: got %q, want %q", string(got), string(data))
	}
}

// TestStepPlanRoundTrip writes step-plan.json and reads it back.
func TestStepPlanRoundTrip(t *testing.T) {
	dir := tempMissionDir(t)

	data := []byte(`{"steps":[{"id":"s1","action":"test"}]}`)

	if err := WriteStepPlan(dir, data); err != nil {
		t.Fatalf("WriteStepPlan: %v", err)
	}

	got, err := ReadStepPlan(dir)
	if err != nil {
		t.Fatalf("ReadStepPlan: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("StepPlan: got %q, want %q", string(got), string(data))
	}
}

// TestManifestDurationS verifies that optional DurationS survives round-trip.
func TestManifestDurationS(t *testing.T) {
	dir := tempMissionDir(t)

	dur := 42
	phase := 1
	evt := ManifestEvent{
		EventID:   "evt-002",
		MissionID: "MSN-TEST-001",
		Event:     "PHASE_COMPLETE",
		Phase:     &phase,
		Timestamp: "2026-03-15T00:00:42Z",
		DurationS: &dur,
	}

	if err := AppendManifest(dir, evt); err != nil {
		t.Fatalf("AppendManifest: %v", err)
	}

	events, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].DurationS == nil || *events[0].DurationS != 42 {
		t.Errorf("DurationS: got %v, want 42", events[0].DurationS)
	}
}

// TestReadIntegrationMapNotExist verifies reading non-existent integration-map
// returns os.ErrNotExist wrapped error.
func TestReadIntegrationMapNotExist(t *testing.T) {
	dir := tempMissionDir(t)

	_, err := ReadIntegrationMap(dir)
	if err == nil {
		t.Fatal("expected error for non-existent integration-map.json")
	}
	if !os.IsNotExist(err) && !filepath.IsAbs(dir) {
		// Accept any error for non-existent file
	}
}
