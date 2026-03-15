// internal/kernel/gate.go
//
// Gate state machine — pure function. No network. No side effects.
// Same inputs always produce same output. Unit testable with no deps.
//
// Zero imports outside stdlib and the Signal type (per CLAUDE.md Hard Rules).

package kernel

import (
	"strings"

	"github.com/taem-dev/taem/internal/controller"
)

// GateDecision represents the outcome of a gate evaluation.
type GateDecision string

const (
	GateADVANCE GateDecision = "ADVANCE"
	GateHOLD    GateDecision = "HOLD"
	GateABORT   GateDecision = "ABORT"
)

// prbSubAgents are the three PRB sub-agent callsigns whose votes are
// aggregated into a single PRB decision per ADR-003.
var prbSubAgents = []string{"PRB-SKEPTIC", "PRB-CORRECTNESS", "PRB-ADR-AUDIT"}

// remediationCap is the maximum number of remediation cycles before a
// NO-GO forces ABORT per ADR-003.
const remediationCap = 2

// Evaluate is a pure function. No network. No side effects.
// Same inputs always produce same output. Unit testable with no deps.
//
// Decision logic:
//  1. If not all required controllers have signaled → HOLD (waiting).
//  2. If any signal is HOLD → HOLD (pause, not abort).
//  3. Aggregate PRB sub-agent votes: 2/3 majority NO-GO → ABORT.
//     Tie (no clear GO majority) = NO-GO per ADR-003.
//  4. If any NO-GO and remediation_cycles >= remediationCap → ABORT.
//  5. If any NO-GO and remediation_cycles < remediationCap → HOLD (remediable).
//  6. All GO → ADVANCE.
func Evaluate(
	phase int,
	required []string,
	signals []controller.Signal,
	remediationCycles int,
) GateDecision {
	// Index signals by controller callsign.
	byController := make(map[string]controller.Signal, len(signals))
	for _, s := range signals {
		byController[s.Controller] = s
	}

	// 1. Check if all required controllers have signaled.
	for _, name := range required {
		if _, ok := byController[name]; !ok {
			return GateHOLD
		}
	}

	// 2. Check for any HOLD signals — pause, not abort.
	for _, name := range required {
		if byController[name].SignalValue == "HOLD" {
			return GateHOLD
		}
	}

	// 3. PRB special case: aggregate sub-agent votes.
	//    Per ADR-003: 2/3 majority, tie = NO-GO.
	if prbActive(required) {
		if prbMajorityNoGo(byController) {
			return GateABORT
		}
	}

	// 4–5. Check for any NO-GO among non-PRB controllers.
	//       PRB NO-GO was already handled above as ABORT.
	hasNoGo := false
	for _, name := range required {
		if isPRBSubAgent(name) {
			continue // handled by PRB aggregation
		}
		if byController[name].SignalValue == "NO-GO" {
			hasNoGo = true
			break
		}
	}

	if hasNoGo {
		if remediationCycles >= remediationCap {
			return GateABORT
		}
		return GateHOLD
	}

	// 6. All controllers GO.
	return GateADVANCE
}

// prbActive returns true if any PRB sub-agent is in the required set.
func prbActive(required []string) bool {
	for _, name := range required {
		if isPRBSubAgent(name) {
			return true
		}
	}
	return false
}

// isPRBSubAgent returns true if the callsign is a PRB sub-agent.
func isPRBSubAgent(name string) bool {
	return strings.HasPrefix(name, "PRB-")
}

// prbMajorityNoGo returns true if the PRB sub-agents have a majority
// NO-GO vote. Per ADR-003: 2/3 majority required for GO, tie = NO-GO.
func prbMajorityNoGo(byController map[string]controller.Signal) bool {
	goVotes := 0
	noGoVotes := 0
	total := 0

	for _, agent := range prbSubAgents {
		s, ok := byController[agent]
		if !ok {
			continue
		}
		total++
		switch s.SignalValue {
		case "GO":
			goVotes++
		case "NO-GO":
			noGoVotes++
		}
	}

	if total == 0 {
		return false
	}

	// 2/3 majority required for GO. Tie = NO-GO per ADR-003.
	// So: if GO votes do NOT have strict majority, it's a NO-GO.
	// With 3 agents: need >= 2 GO votes to pass.
	// noGoVotes >= 2 is majority NO-GO → ABORT.
	// 1 GO, 1 NO-GO, 1 HOLD/other → tie → NO-GO.
	// But HOLD is handled before we get here, so in practice votes are GO or NO-GO.
	if noGoVotes >= 2 {
		return true
	}

	// Tie case: if GO votes don't have 2/3 majority, it's a NO-GO.
	// With 3 voters: goVotes must be >= 2 for GO. Otherwise NO-GO.
	// ADR-003 says "2/3 majority". For 3 voters, that means >= 2 GO votes.
	// For any count: need goVotes * 3 >= total * 2 (i.e., goVotes/total >= 2/3).
	if total > 0 && goVotes*3 < total*2 {
		return true
	}

	return false
}
