<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="profile/assets/taem-hero-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="profile/assets/taem-hero-light.svg">
    <img alt="TAEM — Terminal Area Energy Management" src="profile/assets/taem-hero-dark.svg" width="800">
  </picture>
</p>

<p align="center">
  <strong>Mission-grade preflight for every commit.</strong><br>
  A deterministic kernel that validates architecture, security, and correctness<br>
  <em>before</em> a single line of code is written.
</p>

<p align="center">
  <a href="https://github.com/taem-dev/taem"><img src="https://img.shields.io/badge/kernel-v0.1-blue?style=flat-square" alt="Kernel"></a>
  <a href="https://github.com/taem-dev/adrs"><img src="https://img.shields.io/badge/ADRs-7_active-orange?style=flat-square" alt="ADRs"></a>
  <a href="https://github.com/taem-dev/taem/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache_2.0-green?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.25">
</p>

---

<br>

<p align="center">
  <img src="profile/assets/mission-flow.svg" alt="Mission Flow — 7 Phases from Launch to Landing" width="780">
</p>

<br>

## The Problem

Code reviews happen too late. Architecture violations, security gaps, and integration conflicts are caught after the code is written — when the cost to fix them is highest. Teams burn cycles on rework that was preventable at the planning stage.

## The TAEM Approach

Inspired by NASA's Mission Control protocol, TAEM runs a **7-phase preflight mission** against your planned changes. Fifteen autonomous controllers — each a specialist — evaluate your integration plan in parallel. Only when every controller signals **GO** does the mission advance. No code is generated until Phase 04 clears.

```
$ taem launch --repos taem-dev/taem --task "add webhook retry" --adrs ADR-001,ADR-004
```

One command. Full preflight. Every architectural constraint checked before you write a line.

<br>

---

<br>

## How It Works

<p align="center">
  <img src="profile/assets/gate-machine.svg" alt="Gate State Machine — ADVANCE / HOLD / ABORT" width="720">
</p>

The **gate state machine** is a pure function — zero network, zero side effects, fully deterministic. It evaluates controller signals and decides: advance to the next phase, hold for remediation, or abort the mission.

```
Signals In  ──→  [ Gate Evaluate ]  ──→  ADVANCE | HOLD | ABORT
                       │
                 Pure function.
                 Same inputs = same output.
                 Unit testable with no deps.
```

<br>

---

<br>

## Repositories

<p align="center">
  <img src="profile/assets/repo-map.svg" alt="Repository Architecture — How repos connect" width="780">
</p>

The TAEM ecosystem is four repositories, each with a single responsibility:

<br>

<table>
<tr>
<td width="50%" valign="top">

### [`taem`](https://github.com/taem-dev/taem)

**The Kernel** — the single Go binary that is the complete TAEM runtime.

- Gate state machine (pure, deterministic)
- 15 controller implementations
- Ollama-first inference with Anthropic fallback
- GitHub Actions dispatch for remote controllers
- CLI: `launch`, `status`, `watch`, `retry`, `log`, `adrs`, `init`

```
go install github.com/taem-dev/taem/cmd/taem@latest
```

</td>
<td width="50%" valign="top">

### [`mc-state`](https://github.com/taem-dev/mc-state)

**Mission Control State** — the git-native state store.

- Every mission gets a directory: `missions/<ulid>/`
- Append-only JSONL: `signals.jsonl`, `manifest.jsonl`
- Artifacts: `integration-map.json`, `step-plan.json`, `LANDED.md`
- JSON Schema validated by DPS controller at every phase
- All writes are git commits — full audit trail, no external DB

</td>
</tr>
<tr>
<td width="50%" valign="top">

### [`adrs`](https://github.com/taem-dev/adrs)

**Architecture Decision Records** — the constraint corpus.

- 7 active ADRs governing all TAEM behavior
- Each ADR defines HARD or SOFT constraints
- Constraint expressions are machine-evaluable
- ARCH controller checks every plan against these constraints
- PRB-ADR auditor performs semantic second-pass via LLM

</td>
<td width="50%" valign="top">

### [`ecosystem`](https://github.com/taem-dev/ecosystem)

**The Knowledge Layer** — persistent memory across missions.

- Backed by Qdrant vector database
- 5 collections: `repo_surfaces`, `mission_memory`, `constraint_index`, `wiring_patterns`, `lessons_learned`
- Kernel reads only — writes handled by ecosystem service
- PRB Skeptic queries `lessons_learned` to find prior failure patterns
- NAV diffs against `repo_surfaces` to skip re-reads

</td>
</tr>
</table>

<br>

---

<br>

## The 15 Controllers

<p align="center">
  <img src="profile/assets/controllers.svg" alt="15 Controllers across 7 Phases" width="780">
</p>

Every controller implements one interface:

```go
type Controller interface {
    Name() string
    Mode() ControllerMode   // deterministic | inference | github_dispatch
    Run(ctx context.Context, inputs Inputs) (Signal, error)
}
```

<br>

| Phase | Controllers | What They Check |
|:---:|:---|:---|
| **00** | `GC` `DPS` `EECOM` | Infrastructure health, schema integrity, API rate limits |
| **01** | `NAV` `FAO` | Dependency graph construction, phase timing |
| **02** | `ARCH` `CDS` `PCO` | ADR compliance, conflict detection, pattern conformance |
| **03** | `INCO` | Topological step ordering, write-conflict detection |
| **04** | `SECINSP` `TRC` `PRB-SKP` `PRB-COR` `PRB-ADR` | Security analysis, test readiness, peer review board (2/3 vote) |
| **05** | `CAPCOM` | Developer output rendering — the only human-facing channel |
| **06** | `PAO` `DPS` `SECINSP` | External dispatch, final validation |

<br>

---

<br>

## Inference Architecture

<p align="center">
  <img src="profile/assets/inference-router.svg" alt="Inference Router — Ollama first, Anthropic fallback" width="700">
</p>

Phases 00–03 use **zero LLM calls**. All intelligence in those phases is deterministic — set intersection, topological sort, schema validation, regex.

Phase 04+ uses a two-tier inference router:

1. **Ollama first** — local LLM (llama3.2), 45s timeout, 0.85 confidence threshold
2. **Anthropic fallback** — claude-sonnet-4-6, only if Ollama times out or confidence is below threshold
3. **No Ollama? HOLD.** — if `OLLAMA_URL` is not set, inference controllers emit HOLD, never silently fall through to Anthropic

Every inference call logs: backend used, confidence score, latency, and whether fallback was triggered.

<br>

---

<br>

## Design Principles

<table>
<tr>
<td width="33%" align="center">
<br>
<img src="profile/assets/principle-deterministic.svg" alt="Deterministic" width="120">
<br><br>
<strong>Deterministic First</strong><br>
<sub>11 of 15 controllers use zero LLM calls. Architecture is validated with set theory, not vibes.</sub>
<br><br>
</td>
<td width="33%" align="center">
<br>
<img src="profile/assets/principle-git.svg" alt="Git-Native" width="120">
<br><br>
<strong>Git-Native State</strong><br>
<sub>All mission state lives in git commits. No external databases. Full audit trail. Replayable.</sub>
<br><br>
</td>
<td width="33%" align="center">
<br>
<img src="profile/assets/principle-unicast.svg" alt="Unicast Only" width="120">
<br><br>
<strong>Unicast Only</strong><br>
<sub>No service mesh. No message queues. No multicast. Every connection is explicit and traceable.</sub>
<br><br>
</td>
</tr>
</table>

<br>

---

<br>

## Quick Start

```bash
# Install the kernel
go install github.com/taem-dev/taem/cmd/taem@latest

# Initialize workspace
taem init

# Launch a preflight mission
taem launch \
  --repos taem-dev/taem \
  --task "add webhook retry with exponential backoff" \
  --adrs ADR-001,ADR-004

# Watch the mission live
taem watch
```

<br>

---

<br>

## Architecture Decision Records

All code in the TAEM ecosystem is governed by seven ADRs. These are not guidelines — they are hard constraints enforced by the kernel at runtime.

| ADR | Constraint | Enforcement |
|:---|:---|:---|
| **ADR-001** | Unicast only — no multicast, broadcast, service mesh | GC, ARCH |
| **ADR-002** | GitHub PR as worker boundary — no persistent daemons | ARCH, PCO |
| **ADR-003** | No code before Phase 04 — CAPCOM is the only output | Gate machine |
| **ADR-004** | Git-first state — no external databases | DPS, state/writer |
| **ADR-005** | LLM boundary — deterministic = zero LLM, inference = Ollama-first | Registry, Router |
| **ADR-006** | Ecosystem read-only — kernel never writes to Qdrant | ecosystem/client |
| **ADR-007** | Kernel owns gate logic — not GitHub Actions | gate.go |

<br>

---

<br>

<p align="center">
  <img src="profile/assets/taem-footer.svg" alt="TAEM" width="400">
</p>

<p align="center">
  <sub>Built with Go. Governed by ADRs. No code before clearance.</sub>
</p>
