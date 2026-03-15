<p align="center">
  <img src="taem-banner.svg" alt="TAEM Kernel" width="900"/>
</p>

# taem

The **TAEM kernel** — a single Go binary that is the authoritative runtime for all preflight missions. When you run `taem`, you have the complete system. No servers, no agents, no orchestration platform required.

Named after NASA Mission Control's flight controller positions, TAEM runs a structured preflight protocol with 15 autonomous controllers across 7 phases before any code is written.

## Architecture

```
CLI (taem launch)
 └─→ Kernel
      ├─→ Gate State Machine (pure function, no network)
      ├─→ Controller Registry (from controllers.yaml)
      ├─→ Local Execution (11 deterministic controllers as goroutines)
      ├─→ Inference Router (Ollama → Anthropic fallback)
      ├─→ GitHub Dispatch (NAV, PAO)
      └─→ State Interface (mc-state via git)
```

## Mission Phases

| Phase | Controllers | Mode |
|---|---|---|
| 00 — Pad Check | GC, DPS, EECOM | Deterministic |
| 01 — Corpus Ingestion | NAV, FAO | Dispatch + Deterministic |
| 02 — Architectural Survey | ARCH, CDS, PCO | Deterministic |
| 03 — Plan Formulation | INCO | Deterministic |
| 04 — Pre-Code Inspection | SECINSP, TRC, PRB (3 sub-agents) | Inference + Deterministic |
| 05 — CAPCOM Output | CAPCOM | Deterministic |
| 06 — PAO Dispatch | PAO, DPS, SECINSP | Dispatch + Inference |

## Gate Protocol

The gate state machine is a **pure function** — no network, no side effects, testable in complete isolation:

```go
func Evaluate(phase int, required []string, signals []Signal, remediationCycles int) GateDecision
// Returns: ADVANCE | HOLD | ABORT
```

- All required controllers GO → **ADVANCE**
- Any NO-GO → **HOLD** (remediation check)
- Remediation cap (2 cycles) reached → **ABORT** (ESCALATED)
- PRB: 2/3 majority required, tie = NO-GO

## Controllers

15 controllers, each implementing the `Controller` interface:

```go
type Controller interface {
    Name() string
    Mode() ControllerMode  // deterministic | inference | github_dispatch
    Run(ctx context.Context, inputs Inputs) (Signal, error)
}
```

| Callsign | Role | Mode |
|---|---|---|
| GC | Infrastructure Health | Deterministic |
| DPS | Schema Integrity | Deterministic |
| EECOM | Resource Monitor | Deterministic |
| NAV | Integration Map Builder | GitHub Dispatch |
| FAO | Phase Timeline | Deterministic |
| ARCH | ADR Compliance | Deterministic |
| CDS | Conflict Detection | Deterministic |
| PCO | Pattern Compliance | Deterministic |
| INCO | Step Sequencer | Deterministic |
| SECINSP | Security Inspector | Inference |
| TRC | Test Readiness | Deterministic |
| PRB-SKP | Skeptic (adversarial) | Inference |
| PRB-COR | Correctness Auditor | Inference |
| PRB-ADR | ADR Compliance Auditor | Inference |
| CAPCOM | Developer Output | Deterministic |
| PAO | FABRIC/SOCIAL Trigger | GitHub Dispatch |

## Inference Model

Per [ADR-005](https://github.com/TAEM-DEV/adrs/blob/main/ADR-005.yaml):

- **11 deterministic controllers**: zero LLM calls, zero cost, zero latency
- **4 inference controllers**: Ollama first (0.85 confidence threshold, 45s timeout) → Anthropic fallback
- **Phases 00–03 complete with zero LLM calls**
- If `OLLAMA_URL` not set → inference controllers emit HOLD, not direct Anthropic

## Quick Start

```bash
go install github.com/taem-dev/taem/cmd/taem@latest

taem init          # Initialize mission workspace
taem launch \
  --repos org/repo \
  --task "integrate service X" \
  --adrs ADR-001,ADR-002

taem status        # Mission status
taem watch         # Live mission stream
```

## Related Repos

| Repo | Relationship |
|---|---|
| [adrs](https://github.com/TAEM-DEV/adrs) | ADR constraint corpus — ARCH reads this |
| [ecosystem](https://github.com/TAEM-DEV/ecosystem) | Qdrant semantic layer — NAV cache, PRB memory |
| [mc-state](https://github.com/TAEM-DEV/mc-state) | Mission state store — signals, manifests, plans |

## Active ADRs

All code in this repo is governed by [ADR-000](https://github.com/TAEM-DEV/adrs/blob/main/ADR-000.yaml) through [ADR-007](https://github.com/TAEM-DEV/adrs/blob/main/ADR-007.yaml). See the [adrs repo](https://github.com/TAEM-DEV/adrs) for the complete constraint corpus.
