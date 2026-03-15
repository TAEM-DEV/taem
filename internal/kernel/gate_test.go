package kernel

import (
	"testing"

	"github.com/taem-dev/taem/internal/controller"
)

// sig is a helper that builds a controller.Signal with the given callsign and signal value.
func sig(callsign, value string) controller.Signal {
	return controller.Signal{
		Controller:  callsign,
		SignalValue: value,
		Reason:      callsign + " " + value,
		Evidence:    []string{},
	}
}

// TestGate_AllGO_ADVANCE verifies that when every required controller
// emits GO and remediation_cycles is 0, the gate returns ADVANCE.
func TestGate_AllGO_ADVANCE(t *testing.T) {
	required := []string{"GC", "EECOM", "SECINSP", "PRB-SKEPTIC", "PRB-CORRECTNESS", "PRB-ADR-AUDIT"}
	signals := []controller.Signal{
		sig("GC", "GO"),
		sig("EECOM", "GO"),
		sig("SECINSP", "GO"),
		sig("PRB-SKEPTIC", "GO"),
		sig("PRB-CORRECTNESS", "GO"),
		sig("PRB-ADR-AUDIT", "GO"),
	}

	got := Evaluate(3, required, signals, 0)
	if got != GateADVANCE {
		t.Fatalf("expected ADVANCE, got %s", got)
	}
}

// TestGate_SECINSP_NOGO_HOLD verifies that a single NO-GO from SECINSP
// with remediation_cycles < 2 produces HOLD (remediable).
func TestGate_SECINSP_NOGO_HOLD(t *testing.T) {
	required := []string{"GC", "EECOM", "SECINSP", "PRB-SKEPTIC", "PRB-CORRECTNESS", "PRB-ADR-AUDIT"}
	signals := []controller.Signal{
		sig("GC", "GO"),
		sig("EECOM", "GO"),
		sig("SECINSP", "NO-GO"),
		sig("PRB-SKEPTIC", "GO"),
		sig("PRB-CORRECTNESS", "GO"),
		sig("PRB-ADR-AUDIT", "GO"),
	}

	got := Evaluate(3, required, signals, 0)
	if got != GateHOLD {
		t.Fatalf("expected HOLD, got %s", got)
	}
}

// TestGate_PRB_MajorityNOGO_ABORT verifies that when 2 of 3 PRB
// sub-agents vote NO-GO (majority per ADR-003), the gate returns ABORT.
func TestGate_PRB_MajorityNOGO_ABORT(t *testing.T) {
	required := []string{"GC", "EECOM", "SECINSP", "PRB-SKEPTIC", "PRB-CORRECTNESS", "PRB-ADR-AUDIT"}
	signals := []controller.Signal{
		sig("GC", "GO"),
		sig("EECOM", "GO"),
		sig("SECINSP", "GO"),
		sig("PRB-SKEPTIC", "NO-GO"),
		sig("PRB-CORRECTNESS", "NO-GO"),
		sig("PRB-ADR-AUDIT", "GO"),
	}

	got := Evaluate(3, required, signals, 0)
	if got != GateABORT {
		t.Fatalf("expected ABORT, got %s", got)
	}
}

// TestGate_RemediationCap_ABORT verifies that when remediation_cycles == 2
// and any controller emits NO-GO, the gate returns ABORT (cap exceeded per ADR-003).
func TestGate_RemediationCap_ABORT(t *testing.T) {
	required := []string{"GC", "EECOM", "SECINSP", "PRB-SKEPTIC", "PRB-CORRECTNESS", "PRB-ADR-AUDIT"}
	signals := []controller.Signal{
		sig("GC", "GO"),
		sig("EECOM", "NO-GO"),
		sig("SECINSP", "GO"),
		sig("PRB-SKEPTIC", "GO"),
		sig("PRB-CORRECTNESS", "GO"),
		sig("PRB-ADR-AUDIT", "GO"),
	}

	got := Evaluate(3, required, signals, 2)
	if got != GateABORT {
		t.Fatalf("expected ABORT, got %s", got)
	}
}
