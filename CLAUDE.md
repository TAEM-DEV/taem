# TAEM — Kernel
## taem-dev/taem Repository Brief for Claude Code

Read this completely before writing any code.
This repo IS the TAEM runtime. Everything else serves it.

---

## What This Repo Is

`taem-dev/taem` is the TAEM kernel — a single Go binary that is the
authoritative runtime for all preflight missions. When someone runs `taem`,
they have the complete TAEM system. No servers, no agents, no orchestration
platform required beyond what this binary provides.

The kernel owns (per ADR-007):
- The gate state machine — pure Go, no network, testable in isolation
- The controller registry — loaded from config/controllers.yaml
- Local execution of all 11 deterministic controllers as goroutines
- The inference router — Ollama first, Anthropic fallback per ADR-005
- GitHub Actions dispatch for NAV and PAO
- The mission state interface — reads/writes mc-state via git
- The ecosystem client — reads/writes taem-dev/ecosystem via Qdrant

GitHub Actions is a backend the kernel dispatches to — not the orchestrator.

---

## Active ADRs — Read Before Writing Any Code

### ADR-001 — Unicast Only (HARD)
No multicast, broadcast, service mesh, mDNS. Every connection explicit unicast.
Banned: multicast, broadcast, service_mesh, mdns, consul_connect, istio

### ADR-002 — GitHub PR as Worker Boundary (HARD)
No K8s Jobs, persistent daemons, long-polling loops.
Workers run, produce output, stop. No in-memory state between invocations.

### ADR-003 — Mission Control Preflight Protocol (HARD)
No code before Phase 04 gate. CAPCOM is the only developer output.
PRB: 2/3 majority, tie = NO-GO. Remediation cap: 2 cycles.

### ADR-004 — Git-First Architecture (HARD)
All state in mc-state via git commits. No external databases.
Kernel writes to mc-state. Kernel reads from ecosystem (Qdrant).

### ADR-005 — LLM Boundary (HARD)
Deterministic controllers: ZERO LLM calls. This is enforced at registration.
Inference controllers: Ollama first (0.85 confidence, 45s timeout) → Anthropic.
If OLLAMA_URL not set: inference controllers emit HOLD, not direct Anthropic call.

### ADR-006 — Ecosystem Layer (HARD)
Kernel reads from ecosystem via client — never writes raw mission data to Qdrant.
NAV holds if ecosystem unreachable (not cold-reads repos).

### ADR-007 — Kernel Architecture (HARD)
Gate logic lives only here — not in GitHub Actions YAML.
Controllers implement the Controller interface — Name(), Run(), Mode().
Phases 00–03: zero network except ecosystem + git.

---

## Package Layout

```
taem-dev/taem/
├── cmd/taem/
│   └── main.go                    # cobra root
├── internal/
│   ├── kernel/
│   │   ├── kernel.go              # Mission struct, Run(), phase sequencer
│   │   ├── gate.go                # Gate state machine — pure function
│   │   └── registry.go            # Loads config/controllers.yaml
│   ├── controller/
│   │   ├── interface.go           # Controller interface: Name, Run, Mode
│   │   ├── deterministic/
│   │   │   ├── gc/gc.go
│   │   │   ├── dps/dps.go
│   │   │   ├── eecom/eecom.go
│   │   │   ├── fao/fao.go
│   │   │   ├── arch/arch.go
│   │   │   ├── cds/cds.go
│   │   │   ├── pco/pco.go
│   │   │   ├── inco/inco.go
│   │   │   ├── trc/trc.go
│   │   │   └── capcom/capcom.go
│   │   └── inference/
│   │       ├── router.go          # Ollama→Anthropic fallback per ADR-005
│   │       ├── secinsp/secinsp.go
│   │       └── prb/
│   │           ├── skeptic/skeptic.go
│   │           ├── correctness/correctness.go
│   │           └── adr_audit/adr_audit.go
│   ├── state/
│   │   ├── reader.go              # Read signals.jsonl, manifest.jsonl
│   │   └── writer.go              # Append signals, append manifest events
│   ├── ecosystem/
│   │   └── client.go              # Qdrant client for 5 collections
│   └── dispatch/
│       ├── github.go              # GitHub Actions workflow_dispatch
│       └── local.go               # In-process controller runner
├── config/
│   ├── controllers.yaml           # Normative controller registry (ADR-005)
│   └── patterns.yaml              # PCO pattern registry
├── prompts/
│   ├── secinsp.txt
│   ├── prb_skeptic.txt
│   ├── prb_correctness.txt
│   └── prb_adr_audit.txt
├── schemas/                       # Copy from mc-state/schemas/ — same files
│   ├── signal.schema.json
│   ├── manifest.schema.json
│   ├── integration-map.schema.json
│   ├── step-plan.schema.json
│   └── remediation.schema.json
├── go.mod                         # module: github.com/taem-dev/taem
└── CLAUDE.md                      # this file
```

---

## The Controller Interface — Implement Exactly This

```go
// internal/controller/interface.go

package controller

import "context"

type ControllerMode string

const (
    ModeDeterministic ControllerMode = "deterministic"
    ModeInference     ControllerMode = "inference"
    ModeDispatch      ControllerMode = "github_dispatch"
)

type Signal struct {
    Controller     string   `json:"controller"`
    SignalValue    string   `json:"signal"`       // GO, NO-GO, HOLD, WARN, RELAY
    Reason         string   `json:"reason"`
    Evidence       []string `json:"evidence"`
    ConstraintRef  *string  `json:"constraint_ref"`
    InferenceNotes *InferenceMeta `json:"inference_notes,omitempty"`
}

type InferenceMeta struct {
    Backend      string  `json:"inference_backend"` // "ollama" or "anthropic"
    Confidence   float64 `json:"ollama_confidence"`
    LatencyMS    int64   `json:"latency_ms"`
    FallbackUsed bool    `json:"fallback_used"`
}

type Inputs struct {
    MissionID      string
    ManifestPath   string
    IntegrationMap string  // path to integration-map.json, empty if not yet written
    StepPlan       string  // path to step-plan.json, empty if not yet written
    ADRsPath       string  // path to checked-out adrs/
    EcosystemQuery func(collection, query string) ([]byte, error)
    Phase          int
}

type Controller interface {
    Name() string
    Mode() ControllerMode
    Run(ctx context.Context, inputs Inputs) (Signal, error)
}
```

---

## The Gate State Machine — Pure Function

```go
// internal/kernel/gate.go

type GateDecision string
const (
    GateADVANCE GateDecision = "ADVANCE"
    GateHOLD    GateDecision = "HOLD"
    GateABORT   GateDecision = "ABORT"
)

// Evaluate is a pure function. No network. No side effects.
// Same inputs always produce same output. Unit testable with no deps.
func Evaluate(
    phase int,
    required []string,       // controller callsigns required for this phase
    signals  []Signal,       // signals received so far
    remediationCycles int,
) GateDecision {
    // 1. Check if all required controllers have signaled
    // 2. If any NO-GO: check remediation cap
    // 3. PRB special case: aggregate 3 sub-agent votes → majority
    // 4. HOLD signals pause — not abort
    // Return ADVANCE, HOLD, or ABORT
}
```

---

## The Inference Router — Implement Exactly This Behavior

```go
// internal/controller/inference/router.go

// InferenceRouter handles Ollama→Anthropic fallback per ADR-005.
// Called by all four inference controllers.
func Route(ctx context.Context, systemPrompt, userContent string, cfg InferenceConfig) (InferenceResult, error) {
    // 1. If OLLAMA_URL not set → return HOLD signal immediately, do not call Anthropic
    // 2. Call Ollama with timeout (cfg.TimeoutS, default 45)
    // 3. If Ollama response.confidence >= cfg.ConfidenceThreshold (default 0.85):
    //      return result with backend="ollama"
    // 4. If Ollama timeout or confidence < threshold:
    //      call Anthropic claude-sonnet-4-6
    //      return result with backend="anthropic", fallback_used=true
    // 5. Log: controller, backend, confidence, latency_ms, fallback_reason
    //    This log is appended to the signal's InferenceMeta field
}
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `TAEM_MC_STATE_PATH` | Yes | Local clone path of taem-dev/mc-state |
| `TAEM_ECOSYSTEM_URL` | Yes | Qdrant URL for taem-dev/ecosystem service |
| `TAEM_ADRS_PATH` | Yes | Local clone path of taem-dev/adrs |
| `GH_TOKEN` or `GITHUB_TOKEN` | Yes | For GitHub API calls (EECOM rate check, dispatch) |
| `TAEM_APP_ID` | Yes | GitHub App ID for taem-flight[bot] |
| `TAEM_APP_PRIVATE_KEY_PATH` | Yes | Path to App private key PEM file |
| `OLLAMA_URL` | Optional | Ollama inference endpoint. If absent, inference → HOLD |
| `ANTHROPIC_API_KEY` | Optional | Anthropic fallback. Required if Ollama fallback expected |

---

## Dependencies

```
github.com/oklog/ulid/v2          # ULID generation for mission_id, signal_id
github.com/spf13/cobra            # CLI
github.com/spf13/viper            # config
gopkg.in/yaml.v3                  # controllers.yaml parsing
github.com/qdrant/go-client       # Qdrant ecosystem client
github.com/santhosh-tekuri/jsonschema/v5  # DPS JSON Schema validation
github.com/smacker/go-tree-sitter # NAV AST parsing (if NAV runs locally)
```

No Kubernetes client. No message queue client. No service mesh SDK.
Per ADR-001 and ADR-002.

---

## Build Order

Build in this sequence. The gate state machine first — everything depends on it.

1. `internal/controller/interface.go` — the contract all controllers implement
2. `internal/kernel/gate.go` — pure function, write tests immediately
3. `internal/kernel/registry.go` — loads controllers.yaml, validates mode fields
4. `internal/state/reader.go` + `writer.go` — mc-state git interface
5. `internal/ecosystem/client.go` — Qdrant client, 5 collection reads
6. `internal/dispatch/local.go` — goroutine runner with WaitGroup
7. `internal/dispatch/github.go` — workflow_dispatch client
8. `internal/controller/inference/router.go` — Ollama→Anthropic per ADR-005
9. All 11 deterministic controllers (any order, they're independent)
10. All 4 inference controllers (depend on router.go)
11. `internal/kernel/kernel.go` — mission lifecycle, wires everything together
12. `cmd/taem/main.go` — cobra commands: launch, status, watch, retry, log, adrs, init
13. Integration test: `taem launch --repos mc-state --task "smoke test" --adrs ADR-001`

---

## Hard Rules

- gate.go must have zero imports outside stdlib and the Signal type
- Deterministic controllers must not import the inference package — the kernel
  registry enforces this but you must not introduce it
- All controller Run() methods must respect ctx.Done() — the kernel cancels
  on timeout and controllers must not block after cancellation
- Never write to Qdrant directly from a controller — use ecosystem/client.go
  which enforces write ownership per ADR-006 C-006-002
- Never call git push from inside a controller — use state/writer.go which
  handles the retry-with-rebase pattern from ADR-004
- If you are uncertain whether a pattern violates an ADR, it violates the ADR
