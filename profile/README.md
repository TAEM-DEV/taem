<p align="center">
  <img src="https://raw.githubusercontent.com/TAEM-DEV/.github/main/assets/taem-logo.svg" alt="TAEM" width="720"/>
</p>

<p align="center">
  <b>Mission Control for Code.</b><br/>
  A NASA-inspired preflight protocol that reviews, validates, and gates every change<br/>before a single line of code is written.
</p>

<p align="center">
  <a href="https://github.com/TAEM-DEV/taem"><img src="https://img.shields.io/badge/kernel-Go-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"/></a>
  <a href="https://github.com/TAEM-DEV/adrs"><img src="https://img.shields.io/badge/ADRs-7_active-8B5CF6?style=flat-square" alt="ADRs"/></a>
  <a href="https://github.com/TAEM-DEV/mc-state"><img src="https://img.shields.io/badge/state-git--first-F97316?style=flat-square&logo=git&logoColor=white" alt="Git-first"/></a>
  <a href="https://github.com/TAEM-DEV/ecosystem"><img src="https://img.shields.io/badge/ecosystem-Qdrant-DC382D?style=flat-square" alt="Qdrant"/></a>
</p>

---

## What Is TAEM?

**TAEM** (Terminal Area Energy Management) is an autonomous preflight system — named after the Space Shuttle guidance phase — that runs **15 controllers across 7 sequential phases** before any code is committed. One Go binary. No servers. No orchestration platform. Just `taem launch`.

Every mission flows through the same protocol NASA Mission Control uses: **controllers signal GO/NO-GO**, a **gate state machine** decides ADVANCE/HOLD/ABORT, and **CAPCOM** delivers the final verdict.

<p align="center">
  <img src="https://raw.githubusercontent.com/TAEM-DEV/.github/main/assets/mission-flow.svg" alt="Mission Flow" width="800"/>
</p>

---

## The Mission Sequence

<p align="center">

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 900 420" font-family="'JetBrains Mono', 'Fira Code', monospace">
  <style>
    @keyframes phase-pulse { 0%,100% { opacity: 0.6 } 50% { opacity: 1 } }
    @keyframes signal-go { 0% { fill: #1a1a2e } 100% { fill: #10b981 } }
    @keyframes sweep { 0% { transform: translateX(-100%) } 100% { transform: translateX(900px) } }
    .phase-box { rx: 8; ry: 8; stroke-width: 2; }
    .phase-label { font-size: 11px; fill: #e2e8f0; font-weight: 700; text-anchor: middle; }
    .phase-sub { font-size: 9px; fill: #94a3b8; text-anchor: middle; }
    .ctrl { font-size: 8px; fill: #cbd5e1; text-anchor: middle; font-weight: 600; }
    .gate-diamond { stroke-width: 2; }
    .connector { stroke: #334155; stroke-width: 1.5; fill: none; stroke-dasharray: 4 2; }
    .connector-active { stroke: #10b981; stroke-width: 2; fill: none; }
  </style>

  <!-- Background -->
  <rect width="900" height="420" fill="#0f172a" rx="12"/>

  <!-- Title -->
  <text x="450" y="30" text-anchor="middle" font-size="14" fill="#f8fafc" font-weight="700" letter-spacing="3">MISSION SEQUENCE</text>

  <!-- Sweep line animation -->
  <rect width="3" height="420" fill="url(#sweep-grad)" opacity="0.3">
    <animateTransform attributeName="transform" type="translate" from="-20 0" to="920 0" dur="8s" repeatCount="indefinite"/>
  </rect>
  <defs>
    <linearGradient id="sweep-grad" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="#10b981" stop-opacity="0"/>
      <stop offset="50%" stop-color="#10b981" stop-opacity="1"/>
      <stop offset="100%" stop-color="#10b981" stop-opacity="0"/>
    </linearGradient>
    <linearGradient id="go-grad" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#059669"/>
      <stop offset="100%" stop-color="#10b981"/>
    </linearGradient>
    <linearGradient id="inf-grad" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#7c3aed"/>
      <stop offset="100%" stop-color="#a78bfa"/>
    </linearGradient>
    <linearGradient id="disp-grad" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#d97706"/>
      <stop offset="100%" stop-color="#fbbf24"/>
    </linearGradient>
  </defs>

  <!-- Phase 00: Pad Check -->
  <rect x="20" y="55" width="110" height="90" class="phase-box" fill="#1e293b" stroke="#334155">
    <animate attributeName="stroke" values="#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="75" y="72" class="phase-label">PHASE 00</text>
  <text x="75" y="85" class="phase-sub">Pad Check</text>
  <text x="75" y="105" class="ctrl">GC</text>
  <text x="75" y="116" class="ctrl">DPS</text>
  <text x="75" y="127" class="ctrl">EECOM</text>
  <circle cx="75" cy="137" r="3" fill="#1a1a2e">
    <animate attributeName="fill" values="#1a1a2e;#10b981;#10b981" dur="8s" begin="0s" fill="freeze" repeatCount="indefinite"/>
  </circle>

  <!-- Gate 00 -->
  <polygon points="155,100 170,85 185,100 170,115" fill="#1e293b" stroke="#334155" class="gate-diamond">
    <animate attributeName="stroke" values="#334155;#10b981;#334155" dur="8s" begin="0.8s" repeatCount="indefinite"/>
  </polygon>
  <text x="170" y="104" text-anchor="middle" font-size="7" fill="#10b981" font-weight="700">G</text>

  <!-- Connector 00→01 -->
  <line x1="185" y1="100" x2="205" y2="100" class="connector">
    <animate attributeName="class" values="connector;connector-active;connector" dur="8s" begin="1s" repeatCount="indefinite"/>
  </line>

  <!-- Phase 01: Corpus Ingestion -->
  <rect x="205" y="55" width="110" height="90" class="phase-box" fill="#1e293b" stroke="#334155">
    <animate attributeName="stroke" values="#334155;#334155;#fbbf24;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="260" y="72" class="phase-label">PHASE 01</text>
  <text x="260" y="85" class="phase-sub">Corpus Ingestion</text>
  <rect x="237" y="96" width="46" height="12" rx="3" fill="#92400e" opacity="0.5"/>
  <text x="260" y="105" class="ctrl" fill="#fbbf24">NAV</text>
  <text x="260" y="127" class="ctrl">FAO</text>
  <circle cx="260" cy="137" r="3" fill="#1a1a2e">
    <animate attributeName="fill" values="#1a1a2e;#1a1a2e;#10b981;#10b981" dur="8s" begin="0s" fill="freeze" repeatCount="indefinite"/>
  </circle>

  <!-- Gate 01 -->
  <polygon points="340,100 355,85 370,100 355,115" fill="#1e293b" stroke="#334155" class="gate-diamond">
    <animate attributeName="stroke" values="#334155;#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </polygon>
  <text x="355" y="104" text-anchor="middle" font-size="7" fill="#10b981" font-weight="700">G</text>

  <!-- Connector 01→02 -->
  <line x1="370" y1="100" x2="390" y2="100" class="connector">
    <animate attributeName="class" values="connector;connector;connector-active;connector" dur="8s" begin="0s" repeatCount="indefinite"/>
  </line>

  <!-- Phase 02: Architectural Survey -->
  <rect x="390" y="55" width="110" height="90" class="phase-box" fill="#1e293b" stroke="#334155">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="445" y="72" class="phase-label">PHASE 02</text>
  <text x="445" y="85" class="phase-sub">Arch Survey</text>
  <text x="445" y="105" class="ctrl">ARCH</text>
  <text x="445" y="116" class="ctrl">CDS</text>
  <text x="445" y="127" class="ctrl">PCO</text>
  <circle cx="445" cy="137" r="3" fill="#1a1a2e">
    <animate attributeName="fill" values="#1a1a2e;#1a1a2e;#1a1a2e;#10b981;#10b981" dur="8s" begin="0s" fill="freeze" repeatCount="indefinite"/>
  </circle>

  <!-- Gate 02 -->
  <polygon points="525,100 540,85 555,100 540,115" fill="#1e293b" stroke="#334155" class="gate-diamond">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </polygon>
  <text x="540" y="104" text-anchor="middle" font-size="7" fill="#10b981" font-weight="700">G</text>

  <!-- Connector 02→03 -->
  <line x1="555" y1="100" x2="575" y2="100" class="connector"/>

  <!-- Phase 03: Plan Formulation -->
  <rect x="575" y="55" width="110" height="90" class="phase-box" fill="#1e293b" stroke="#334155">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="630" y="72" class="phase-label">PHASE 03</text>
  <text x="630" y="85" class="phase-sub">Plan Formulation</text>
  <text x="630" y="110" class="ctrl">INCO</text>
  <circle cx="630" cy="137" r="3" fill="#1a1a2e">
    <animate attributeName="fill" values="#1a1a2e;#1a1a2e;#1a1a2e;#1a1a2e;#10b981;#10b981" dur="8s" begin="0s" fill="freeze" repeatCount="indefinite"/>
  </circle>

  <!-- Gate 03 -->
  <polygon points="710,100 725,85 740,100 725,115" fill="#1e293b" stroke="#334155" class="gate-diamond">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </polygon>
  <text x="725" y="104" text-anchor="middle" font-size="7" fill="#10b981" font-weight="700">G</text>

  <!-- Connector to Phase 04 row -->
  <path d="M 740 100 L 770 100 L 770 210 L 20 210 L 20 240" class="connector" stroke-dasharray="4 2"/>

  <!-- Phase 04: Pre-Code Inspection (bottom row) -->
  <rect x="20" y="240" width="280" height="130" class="phase-box" fill="#1e293b" stroke="#334155">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#334155;#334155;#a78bfa;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="160" y="260" class="phase-label">PHASE 04 — PRE-CODE INSPECTION</text>

  <!-- SECINSP -->
  <rect x="35" y="275" width="75" height="40" rx="5" fill="#2d1b69" stroke="#7c3aed" stroke-width="1">
    <animate attributeName="stroke" values="#7c3aed;#7c3aed;#7c3aed;#7c3aed;#7c3aed;#a78bfa;#7c3aed" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="72" y="292" class="ctrl" fill="#a78bfa">SECINSP</text>
  <text x="72" y="305" font-size="7" fill="#7c3aed" text-anchor="middle">inference</text>

  <!-- TRC -->
  <rect x="120" y="275" width="75" height="40" rx="5" fill="#1e293b" stroke="#334155" stroke-width="1"/>
  <text x="157" y="295" class="ctrl">TRC</text>
  <text x="157" y="305" font-size="7" fill="#475569" text-anchor="middle">deterministic</text>

  <!-- PRB cluster -->
  <rect x="205" y="270" width="85" height="90" rx="5" fill="#2d1b69" stroke="#7c3aed" stroke-width="1" opacity="0.7">
    <animate attributeName="stroke" values="#7c3aed;#7c3aed;#7c3aed;#7c3aed;#7c3aed;#a78bfa;#7c3aed" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="247" y="287" class="ctrl" fill="#c4b5fd">PRB</text>
  <text x="247" y="302" font-size="7" fill="#a78bfa" text-anchor="middle">SKP</text>
  <text x="247" y="316" font-size="7" fill="#a78bfa" text-anchor="middle">COR</text>
  <text x="247" y="330" font-size="7" fill="#a78bfa" text-anchor="middle">ADR</text>
  <text x="247" y="350" font-size="7" fill="#7c3aed" text-anchor="middle">2/3 majority</text>

  <!-- PRB vote indicators -->
  <circle cx="225" cy="299" r="3" fill="#1a1a2e">
    <animate attributeName="fill" values="#1a1a2e;#1a1a2e;#1a1a2e;#1a1a2e;#1a1a2e;#10b981;#10b981" dur="8s" begin="0s" repeatCount="indefinite"/>
  </circle>
  <circle cx="225" cy="313" r="3" fill="#1a1a2e">
    <animate attributeName="fill" values="#1a1a2e;#1a1a2e;#1a1a2e;#1a1a2e;#1a1a2e;#10b981;#10b981" dur="8s" begin="0.3s" repeatCount="indefinite"/>
  </circle>
  <circle cx="225" cy="327" r="3" fill="#1a1a2e">
    <animate attributeName="fill" values="#1a1a2e;#1a1a2e;#1a1a2e;#1a1a2e;#1a1a2e;#10b981;#10b981" dur="8s" begin="0.6s" repeatCount="indefinite"/>
  </circle>

  <!-- Gate 04 -->
  <polygon points="325,305 345,285 365,305 345,325" fill="#1e293b" stroke="#334155" class="gate-diamond">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#334155;#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </polygon>
  <text x="345" y="309" text-anchor="middle" font-size="8" fill="#10b981" font-weight="700">G</text>

  <!-- Connector 04→05 -->
  <line x1="365" y1="305" x2="400" y2="305" class="connector"/>

  <!-- Phase 05: CAPCOM Output -->
  <rect x="400" y="260" width="160" height="90" class="phase-box" fill="#1e293b" stroke="#334155">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#334155;#334155;#334155;#10b981;#334155" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="480" y="280" class="phase-label">PHASE 05</text>
  <text x="480" y="293" class="phase-sub">CAPCOM Output</text>
  <text x="480" y="315" class="ctrl">CAPCOM</text>
  <text x="480" y="330" font-size="7" fill="#475569" text-anchor="middle">LANDED.md</text>
  <text x="480" y="340" font-size="7" fill="#475569" text-anchor="middle">Terminal + GitHub Summary</text>

  <!-- Connector 05→06 -->
  <line x1="560" y1="305" x2="595" y2="305" class="connector"/>

  <!-- Phase 06: PAO Dispatch -->
  <rect x="595" y="260" width="130" height="90" class="phase-box" fill="#1e293b" stroke="#334155">
    <animate attributeName="stroke" values="#334155;#334155;#334155;#334155;#334155;#334155;#334155;#fbbf24" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="660" y="280" class="phase-label">PHASE 06</text>
  <text x="660" y="293" class="phase-sub">PAO Dispatch</text>
  <rect x="632" y="300" width="56" height="12" rx="3" fill="#92400e" opacity="0.5"/>
  <text x="660" y="309" class="ctrl" fill="#fbbf24">PAO</text>
  <text x="660" y="330" class="ctrl">DPS</text>
  <text x="660" y="340" class="ctrl">SECINSP</text>

  <!-- LANDED indicator -->
  <rect x="745" y="287" width="80" height="36" rx="6" fill="#052e16" stroke="#10b981" stroke-width="2">
    <animate attributeName="opacity" values="0;0;0;0;0;0;0;1" dur="8s" begin="0s" repeatCount="indefinite"/>
  </rect>
  <text x="785" y="310" text-anchor="middle" font-size="12" fill="#10b981" font-weight="700">LANDED</text>

  <!-- Legend -->
  <rect x="20" y="385" width="860" height="25" rx="4" fill="#1e293b" opacity="0.5"/>
  <circle cx="40" cy="398" r="5" fill="#1e293b" stroke="#10b981" stroke-width="1.5"/>
  <text x="50" y="401" font-size="8" fill="#94a3b8">Deterministic</text>
  <circle cx="140" cy="398" r="5" fill="#2d1b69" stroke="#7c3aed" stroke-width="1.5"/>
  <text x="150" y="401" font-size="8" fill="#94a3b8">Inference (Ollama→Anthropic)</text>
  <rect x="290" y="393" width="12" height="10" rx="2" fill="#92400e" stroke="#fbbf24" stroke-width="1"/>
  <text x="307" y="401" font-size="8" fill="#94a3b8">GitHub Dispatch</text>
  <polygon points="400,398 408,392 416,398 408,404" fill="#1e293b" stroke="#10b981" stroke-width="1"/>
  <text x="422" y="401" font-size="8" fill="#94a3b8">Gate (pure function)</text>
  <circle cx="520" cy="398" r="4" fill="#10b981"/>
  <text x="530" y="401" font-size="8" fill="#94a3b8">GO signal</text>
</svg>
```

</p>

---

## Repositories

<table>
<tr>
<td width="50%" valign="top">

### [`taem`](https://github.com/TAEM-DEV/taem) &nbsp; <img src="https://img.shields.io/badge/Go-kernel-00ADD8?style=flat-square&logo=go&logoColor=white" alt=""/>

**The kernel.** Single Go binary — the complete TAEM runtime. Owns the gate state machine, controller registry, local goroutine execution, inference router, GitHub Actions dispatch, and the mission state interface.

```bash
taem launch --repos org/repo --task "review X" --adrs ADR-001
```

15 controllers. 7 phases. Pure-function gate logic. Zero external dependencies beyond git and optional Ollama.

</td>
<td width="50%" valign="top">

### [`mc-state`](https://github.com/TAEM-DEV/mc-state) &nbsp; <img src="https://img.shields.io/badge/git--first-state_store-F97316?style=flat-square&logo=git&logoColor=white" alt=""/>

**Mission Control state.** Every signal, manifest event, integration map, and step plan is a git commit. No databases. No external state stores. Every mission is auditable, diffable, and replayable.

```
missions/<mission-id>/
  ├── manifest.jsonl      # lifecycle events
  ├── signals.jsonl       # controller output
  ├── integration-map.json
  └── step-plan.json
```

</td>
</tr>
<tr>
<td width="50%" valign="top">

### [`ecosystem`](https://github.com/TAEM-DEV/ecosystem) &nbsp; <img src="https://img.shields.io/badge/Qdrant-semantic_layer-DC382D?style=flat-square" alt=""/>

**The memory.** Qdrant vector database with 5 collections — repo surfaces, mission memory, constraint index, wiring patterns, and lessons learned. The kernel reads; the ecosystem service writes. Controllers never touch Qdrant directly.

| Collection | Purpose |
|---|---|
| `repo_surfaces` | Cached repo metadata (NAV) |
| `mission_memory` | Historical context (PRB) |
| `constraint_index` | ADR corpus (ARCH, PRB-ADR) |
| `wiring_patterns` | Integration patterns (PCO) |
| `lessons_learned` | Prior failures (PRB-SKP) |

</td>
<td width="50%" valign="top">

### [`adrs`](https://github.com/TAEM-DEV/adrs) &nbsp; <img src="https://img.shields.io/badge/YAML-constraints-8B5CF6?style=flat-square" alt=""/>

**The law.** 8 Architecture Decision Records (ADR-000 through ADR-007) that govern every line of code in the TAEM organization. Hard constraints, machine-readable, enforced at the gate.

| ADR | Constraint |
|---|---|
| 001 | Unicast only — no multicast, no mesh |
| 002 | PR as worker boundary — stateless |
| 003 | Preflight protocol — no code before Phase 04 |
| 004 | Git-first — all state in mc-state |
| 005 | LLM boundary — deterministic = zero LLM |
| 006 | Ecosystem layer — kernel reads only |
| 007 | Kernel architecture — gate in kernel only |

</td>
</tr>
</table>

---

## The Gate State Machine

<p align="center">

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 700 340" font-family="'JetBrains Mono', 'Fira Code', monospace">
  <style>
    @keyframes pulse-go { 0%,100% { opacity: 0.4 } 50% { opacity: 1 } }
    @keyframes pulse-hold { 0%,100% { opacity: 0.4 } 50% { opacity: 1 } }
    @keyframes pulse-abort { 0%,100% { opacity: 0.4 } 50% { opacity: 1 } }
    @keyframes data-flow { 0% { stroke-dashoffset: 20 } 100% { stroke-dashoffset: 0 } }
    .flow-line { stroke-dasharray: 6 3; animation: data-flow 1.5s linear infinite; }
    .label { font-size: 10px; fill: #94a3b8; }
    .code { font-size: 9px; fill: #cbd5e1; }
    .title { font-size: 12px; fill: #f8fafc; font-weight: 700; }
  </style>

  <rect width="700" height="340" fill="#0f172a" rx="12"/>

  <text x="350" y="28" text-anchor="middle" class="title" letter-spacing="2">GATE STATE MACHINE — PURE FUNCTION</text>
  <text x="350" y="42" text-anchor="middle" font-size="8" fill="#475569">gate.go — zero imports outside stdlib + Signal type</text>

  <!-- Input box -->
  <rect x="30" y="60" width="180" height="110" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1.5"/>
  <text x="120" y="78" text-anchor="middle" font-size="10" fill="#e2e8f0" font-weight="700">INPUTS</text>
  <text x="45" y="96" class="code">phase: int</text>
  <text x="45" y="112" class="code">required: []string</text>
  <text x="45" y="128" class="code">signals: []Signal</text>
  <text x="45" y="144" class="code">remediationCycles: int</text>
  <text x="45" y="160" class="code" fill="#475569">// same inputs → same output</text>

  <!-- Arrow to decision -->
  <line x1="210" y1="115" x2="260" y2="115" stroke="#334155" stroke-width="2" class="flow-line"/>
  <polygon points="255,110 265,115 255,120" fill="#334155"/>

  <!-- Decision: all signaled? -->
  <polygon points="350,65 420,115 350,165 280,115" fill="#1e293b" stroke="#334155" stroke-width="1.5"/>
  <text x="350" y="110" text-anchor="middle" font-size="8" fill="#e2e8f0" font-weight="600">ALL</text>
  <text x="350" y="122" text-anchor="middle" font-size="8" fill="#e2e8f0" font-weight="600">SIGNALED?</text>

  <!-- No → HOLD (waiting) -->
  <line x1="350" y1="165" x2="350" y2="200" stroke="#f59e0b" stroke-width="1.5" class="flow-line"/>
  <text x="360" y="183" font-size="7" fill="#f59e0b">no</text>
  <rect x="305" y="200" width="90" height="30" rx="5" fill="#78350f" stroke="#f59e0b" stroke-width="1.5">
    <animate attributeName="opacity" values="0.6;1;0.6" dur="2s" repeatCount="indefinite"/>
  </rect>
  <text x="350" y="220" text-anchor="middle" font-size="10" fill="#fbbf24" font-weight="700">HOLD</text>
  <text x="350" y="242" text-anchor="middle" font-size="7" fill="#92400e">waiting for controllers</text>

  <!-- Yes → next decision -->
  <line x1="420" y1="115" x2="465" y2="115" stroke="#10b981" stroke-width="1.5" class="flow-line"/>
  <text x="440" y="108" font-size="7" fill="#10b981">yes</text>

  <!-- Decision: any NO-GO? -->
  <polygon points="535,65 605,115 535,165 465,115" fill="#1e293b" stroke="#334155" stroke-width="1.5"/>
  <text x="535" y="108" text-anchor="middle" font-size="8" fill="#e2e8f0" font-weight="600">ANY</text>
  <text x="535" y="120" text-anchor="middle" font-size="8" fill="#e2e8f0" font-weight="600">NO-GO?</text>

  <!-- No → ADVANCE -->
  <line x1="535" y1="65" x2="535" y2="35" stroke="#10b981" stroke-width="1.5"/>
  <text x="545" y="52" font-size="7" fill="#10b981">no</text>
  <rect x="585" y="22" width="95" height="30" rx="5" fill="#052e16" stroke="#10b981" stroke-width="2">
    <animate attributeName="stroke-opacity" values="0.5;1;0.5" dur="1.5s" repeatCount="indefinite"/>
  </rect>
  <text x="632" y="42" text-anchor="middle" font-size="11" fill="#10b981" font-weight="700">ADVANCE</text>
  <line x1="535" y1="35" x2="585" y2="35" stroke="#10b981" stroke-width="1.5"/>

  <!-- Yes → remediation check -->
  <line x1="535" y1="165" x2="535" y2="210" stroke="#ef4444" stroke-width="1.5" class="flow-line"/>
  <text x="545" y="190" font-size="7" fill="#ef4444">yes</text>

  <!-- Decision: cycles < 2? -->
  <polygon points="535,210 590,245 535,280 480,245" fill="#1e293b" stroke="#334155" stroke-width="1.5"/>
  <text x="535" y="240" text-anchor="middle" font-size="7" fill="#e2e8f0" font-weight="600">CYCLES</text>
  <text x="535" y="252" text-anchor="middle" font-size="7" fill="#e2e8f0" font-weight="600">&lt; 2?</text>

  <!-- Yes → HOLD (remediable) -->
  <line x1="480" y1="245" x2="410" y2="245" stroke="#f59e0b" stroke-width="1.5"/>
  <text x="445" y="238" font-size="7" fill="#f59e0b">yes</text>
  <rect x="305" y="255" width="105" height="30" rx="5" fill="#78350f" stroke="#f59e0b" stroke-width="1.5">
    <animate attributeName="opacity" values="0.6;1;0.6" dur="2.5s" repeatCount="indefinite"/>
  </rect>
  <text x="357" y="275" text-anchor="middle" font-size="10" fill="#fbbf24" font-weight="700">HOLD</text>
  <text x="357" y="296" text-anchor="middle" font-size="7" fill="#92400e">remediation cycle++</text>
  <!-- curved arrow back -->
  <path d="M 305 270 Q 260 270 260 220 Q 260 170 280 115" fill="none" stroke="#f59e0b" stroke-width="1" stroke-dasharray="3 2"/>
  <polygon points="277,120 283,112 286,123" fill="#f59e0b"/>
  <text x="245" y="195" font-size="7" fill="#92400e" transform="rotate(-90 245 195)">re-run</text>

  <!-- No → ABORT -->
  <line x1="590" y1="245" x2="640" y2="245" stroke="#ef4444" stroke-width="1.5"/>
  <text x="612" y="238" font-size="7" fill="#ef4444">no</text>
  <rect x="595" y="228" width="90" height="34" rx="5" fill="#450a0a" stroke="#ef4444" stroke-width="2">
    <animate attributeName="opacity" values="0.5;1;0.5" dur="1.8s" repeatCount="indefinite"/>
  </rect>
  <text x="640" y="250" text-anchor="middle" font-size="11" fill="#ef4444" font-weight="700">ABORT</text>
  <text x="640" y="275" text-anchor="middle" font-size="7" fill="#7f1d1d">mission ESCALATED</text>

  <!-- PRB special case note -->
  <rect x="30" y="285" width="250" height="45" rx="5" fill="#1e293b" stroke="#7c3aed" stroke-width="1" stroke-dasharray="3 2"/>
  <text x="45" y="302" font-size="8" fill="#a78bfa" font-weight="600">PRB SPECIAL CASE</text>
  <text x="45" y="316" class="code" fill="#94a3b8">3 sub-agents vote concurrently</text>
  <text x="45" y="326" class="code" fill="#94a3b8">2/3 majority = GO, tie = NO-GO</text>
</svg>
```

</p>

**The gate is a pure function.** No network calls. No side effects. Same inputs always produce the same output. Testable in complete isolation with zero dependencies.

```go
func Evaluate(phase int, required []string, signals []Signal, remediationCycles int) GateDecision
```

---

## The 15 Controllers

<p align="center">

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 480" font-family="'JetBrains Mono', 'Fira Code', monospace">
  <style>
    @keyframes scan { 0% { y: 55 } 100% { y: 440 } }
    @keyframes blink { 0%,100% { opacity: 0.3 } 50% { opacity: 1 } }
    .row-bg { fill: #1e293b; rx: 4; }
    .callsign { font-size: 11px; font-weight: 700; }
    .role { font-size: 9px; fill: #94a3b8; }
    .mode-det { font-size: 8px; fill: #10b981; }
    .mode-inf { font-size: 8px; fill: #a78bfa; }
    .mode-disp { font-size: 8px; fill: #fbbf24; }
    .phase-tag { font-size: 7px; fill: #64748b; }
    .header { font-size: 10px; fill: #64748b; font-weight: 600; letter-spacing: 1.5px; }
  </style>

  <rect width="800" height="480" fill="#0f172a" rx="12"/>

  <!-- Scanning line -->
  <rect x="15" width="770" height="2" fill="#10b981" opacity="0.15" rx="1">
    <animate attributeName="y" values="55;455" dur="6s" repeatCount="indefinite"/>
  </rect>

  <!-- Headers -->
  <text x="400" y="25" text-anchor="middle" font-size="13" fill="#f8fafc" font-weight="700" letter-spacing="2">CONTROLLER MANIFEST</text>
  <text x="40" y="48" class="header">CALLSIGN</text>
  <text x="150" y="48" class="header">ROLE</text>
  <text x="480" y="48" class="header">MODE</text>
  <text x="600" y="48" class="header">PHASES</text>
  <text x="720" y="48" class="header">TIMEOUT</text>
  <line x1="20" y1="54" x2="780" y2="54" stroke="#334155" stroke-width="1"/>

  <!-- GC -->
  <rect x="20" y="60" width="760" height="24" class="row-bg" opacity="0.3"/>
  <text x="40" y="77" class="callsign" fill="#10b981">GC</text>
  <text x="150" y="77" class="role">Infrastructure health — ping GitHub, Qdrant, Ollama</text>
  <rect x="475" y="66" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="77" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="77" class="phase-tag">00</text>
  <text x="740" y="77" class="phase-tag">10s</text>

  <!-- DPS -->
  <rect x="20" y="88" width="760" height="24" class="row-bg" opacity="0.15"/>
  <text x="40" y="105" class="callsign" fill="#10b981">DPS</text>
  <text x="150" y="105" class="role">Schema integrity — JSON Schema validation on all state files</text>
  <rect x="475" y="94" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="105" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="105" class="phase-tag">00 02 04 06</text>
  <text x="740" y="105" class="phase-tag">10s</text>

  <!-- EECOM -->
  <rect x="20" y="116" width="760" height="24" class="row-bg" opacity="0.3"/>
  <text x="40" y="133" class="callsign" fill="#10b981">EECOM</text>
  <text x="150" y="133" class="role">Resource monitor — API rate limits, token burn estimation</text>
  <rect x="475" y="122" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="133" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="133" class="phase-tag">00</text>
  <text x="740" y="133" class="phase-tag">10s</text>

  <!-- Separator -->
  <line x1="30" y1="148" x2="770" y2="148" stroke="#334155" stroke-width="0.5" stroke-dasharray="2 3"/>

  <!-- NAV -->
  <rect x="20" y="154" width="760" height="24" class="row-bg" opacity="0.15"/>
  <text x="40" y="171" class="callsign" fill="#fbbf24">NAV</text>
  <text x="150" y="171" class="role">Integration map builder — dependency graph via GitHub Actions</text>
  <rect x="475" y="160" width="80" height="14" rx="3" fill="#451a03" stroke="#fbbf24" stroke-width="0.5"/>
  <text x="515" y="171" text-anchor="middle" class="mode-disp">gh_dispatch</text>
  <text x="620" y="171" class="phase-tag">01</text>
  <text x="740" y="171" class="phase-tag">300s</text>

  <!-- FAO -->
  <rect x="20" y="182" width="760" height="24" class="row-bg" opacity="0.3"/>
  <text x="40" y="199" class="callsign" fill="#10b981">FAO</text>
  <text x="150" y="199" class="role">Phase timeline — elapsed tracking, overrun advisories</text>
  <rect x="475" y="188" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="199" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="199" class="phase-tag">00–06</text>
  <text x="740" y="199" class="phase-tag">2s</text>

  <!-- Separator -->
  <line x1="30" y1="214" x2="770" y2="214" stroke="#334155" stroke-width="0.5" stroke-dasharray="2 3"/>

  <!-- ARCH -->
  <rect x="20" y="220" width="760" height="24" class="row-bg" opacity="0.15"/>
  <text x="40" y="237" class="callsign" fill="#10b981">ARCH</text>
  <text x="150" y="237" class="role">ADR compliance — evaluates constraints against plan</text>
  <rect x="475" y="226" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="237" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="237" class="phase-tag">02 03 04</text>
  <text x="740" y="237" class="phase-tag">30s</text>

  <!-- CDS -->
  <rect x="20" y="248" width="760" height="24" class="row-bg" opacity="0.3"/>
  <text x="40" y="265" class="callsign" fill="#10b981">CDS</text>
  <text x="150" y="265" class="role">Conflict detection — set intersection on tools, ports, schemas</text>
  <rect x="475" y="254" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="265" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="265" class="phase-tag">02</text>
  <text x="740" y="265" class="phase-tag">10s</text>

  <!-- PCO -->
  <rect x="20" y="276" width="760" height="24" class="row-bg" opacity="0.15"/>
  <text x="40" y="293" class="callsign" fill="#10b981">PCO</text>
  <text x="150" y="293" class="role">Pattern compliance — validates against patterns.yaml registry</text>
  <rect x="475" y="282" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="293" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="293" class="phase-tag">02 03 04</text>
  <text x="740" y="293" class="phase-tag">10s</text>

  <!-- Separator -->
  <line x1="30" y1="308" x2="770" y2="308" stroke="#334155" stroke-width="0.5" stroke-dasharray="2 3"/>

  <!-- INCO -->
  <rect x="20" y="314" width="760" height="24" class="row-bg" opacity="0.3"/>
  <text x="40" y="331" class="callsign" fill="#10b981">INCO</text>
  <text x="150" y="331" class="role">Step sequencer — topological sort, write conflict detection</text>
  <rect x="475" y="320" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="331" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="331" class="phase-tag">03</text>
  <text x="740" y="331" class="phase-tag">15s</text>

  <!-- Separator -->
  <line x1="30" y1="346" x2="770" y2="346" stroke="#334155" stroke-width="0.5" stroke-dasharray="2 3"/>

  <!-- SECINSP -->
  <rect x="20" y="352" width="760" height="24" class="row-bg" opacity="0.15"/>
  <text x="40" y="369" class="callsign" fill="#a78bfa">SECINSP</text>
  <text x="150" y="369" class="role">Security inspector — regex pass + inference novel attack surface</text>
  <rect x="475" y="358" width="80" height="14" rx="3" fill="#2d1b69" stroke="#7c3aed" stroke-width="0.5"/>
  <text x="515" y="369" text-anchor="middle" class="mode-inf">inference</text>
  <text x="620" y="369" class="phase-tag">04 06</text>
  <text x="740" y="369" class="phase-tag">90s</text>

  <!-- TRC -->
  <rect x="20" y="380" width="760" height="24" class="row-bg" opacity="0.3"/>
  <text x="40" y="397" class="callsign" fill="#10b981">TRC</text>
  <text x="150" y="397" class="role">Test readiness — harness existence, signature grep, stability</text>
  <rect x="475" y="386" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="397" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="397" class="phase-tag">04 06</text>
  <text x="740" y="397" class="phase-tag">20s</text>

  <!-- PRB cluster -->
  <rect x="20" y="408" width="760" height="24" class="row-bg" opacity="0.15"/>
  <text x="40" y="425" class="callsign" fill="#a78bfa">PRB</text>
  <text x="150" y="425" class="role">Peer review board — SKP (skeptic) + COR (correctness) + ADR (audit)</text>
  <rect x="475" y="414" width="80" height="14" rx="3" fill="#2d1b69" stroke="#7c3aed" stroke-width="0.5"/>
  <text x="515" y="425" text-anchor="middle" class="mode-inf">inference ×3</text>
  <text x="620" y="425" class="phase-tag">04</text>
  <text x="740" y="425" class="phase-tag">90s</text>

  <!-- CAPCOM -->
  <rect x="20" y="436" width="760" height="24" class="row-bg" opacity="0.3"/>
  <text x="40" y="453" class="callsign" fill="#10b981">CAPCOM</text>
  <text x="150" y="453" class="role">Developer output — LANDED.md, terminal, GitHub summary</text>
  <rect x="475" y="442" width="80" height="14" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="515" y="453" text-anchor="middle" class="mode-det">deterministic</text>
  <text x="620" y="453" class="phase-tag">05</text>
  <text x="740" y="453" class="phase-tag">10s</text>

  <!-- Counter -->
  <text x="780" y="475" text-anchor="end" font-size="8" fill="#334155">15 controllers | 11 deterministic | 4 inference | 2 dispatch</text>
</svg>
```

</p>

---

## Inference Model

<p align="center">

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 700 260" font-family="'JetBrains Mono', 'Fira Code', monospace">
  <style>
    @keyframes route-ollama { 0% { stroke-dashoffset: 30 } 100% { stroke-dashoffset: 0 } }
    @keyframes route-fallback { 0% { stroke-dashoffset: 30 } 100% { stroke-dashoffset: 0 } }
    @keyframes confidence-fill { 0% { width: 0 } 50% { width: 136 } 100% { width: 136 } }
    .flow { stroke-dasharray: 6 3; animation: route-ollama 2s linear infinite; }
  </style>

  <rect width="700" height="260" fill="#0f172a" rx="12"/>

  <text x="350" y="25" text-anchor="middle" font-size="13" fill="#f8fafc" font-weight="700" letter-spacing="2">INFERENCE ROUTER</text>
  <text x="350" y="40" text-anchor="middle" font-size="8" fill="#475569">ADR-005 — Ollama first, Anthropic fallback</text>

  <!-- Controller input -->
  <rect x="30" y="60" width="120" height="60" rx="6" fill="#2d1b69" stroke="#7c3aed" stroke-width="1.5"/>
  <text x="90" y="82" text-anchor="middle" font-size="9" fill="#c4b5fd" font-weight="600">INFERENCE</text>
  <text x="90" y="95" text-anchor="middle" font-size="9" fill="#c4b5fd" font-weight="600">CONTROLLER</text>
  <text x="90" y="112" text-anchor="middle" font-size="7" fill="#7c3aed">SECINSP | PRB×3</text>

  <!-- Arrow to check -->
  <line x1="150" y1="90" x2="195" y2="90" stroke="#7c3aed" stroke-width="1.5" class="flow"/>

  <!-- OLLAMA_URL check -->
  <polygon points="260,55 305,90 260,125 215,90" fill="#1e293b" stroke="#334155" stroke-width="1.5"/>
  <text x="260" y="86" text-anchor="middle" font-size="7" fill="#e2e8f0" font-weight="600">OLLAMA_URL</text>
  <text x="260" y="97" text-anchor="middle" font-size="7" fill="#e2e8f0" font-weight="600">set?</text>

  <!-- No → HOLD -->
  <line x1="260" y1="125" x2="260" y2="165" stroke="#f59e0b" stroke-width="1.5"/>
  <text x="270" y="148" font-size="7" fill="#f59e0b">no</text>
  <rect x="215" y="165" width="90" height="30" rx="5" fill="#78350f" stroke="#f59e0b" stroke-width="1.5"/>
  <text x="260" y="185" text-anchor="middle" font-size="10" fill="#fbbf24" font-weight="700">HOLD</text>
  <text x="260" y="210" text-anchor="middle" font-size="7" fill="#92400e">never calls Anthropic directly</text>

  <!-- Yes → Ollama -->
  <line x1="305" y1="90" x2="360" y2="90" stroke="#10b981" stroke-width="1.5" class="flow"/>
  <text x="330" y="83" font-size="7" fill="#10b981">yes</text>

  <!-- Ollama box -->
  <rect x="360" y="65" width="120" height="50" rx="6" fill="#052e16" stroke="#10b981" stroke-width="1.5"/>
  <text x="420" y="85" text-anchor="middle" font-size="10" fill="#10b981" font-weight="700">Ollama</text>
  <text x="420" y="100" text-anchor="middle" font-size="7" fill="#059669">45s timeout</text>

  <!-- Confidence bar -->
  <rect x="370" y="125" width="100" height="10" rx="3" fill="#1e293b" stroke="#334155" stroke-width="0.5"/>
  <rect x="370" y="125" width="0" height="10" rx="3" fill="#10b981">
    <animate attributeName="width" values="0;85;85;85;60;60" dur="5s" repeatCount="indefinite"/>
  </rect>
  <text x="370" y="150" font-size="7" fill="#94a3b8">confidence threshold: 0.85</text>

  <!-- Confidence check -->
  <polygon points="420,160 450,180 420,200 390,180" fill="#1e293b" stroke="#334155" stroke-width="1"/>
  <text x="420" y="178" text-anchor="middle" font-size="6" fill="#e2e8f0" font-weight="600">≥ 0.85?</text>

  <!-- Yes → GO with ollama -->
  <line x1="450" y1="180" x2="530" y2="180" stroke="#10b981" stroke-width="1.5"/>
  <text x="485" y="173" font-size="7" fill="#10b981">yes</text>
  <rect x="530" y="165" width="140" height="30" rx="5" fill="#052e16" stroke="#10b981" stroke-width="1.5"/>
  <text x="600" y="185" text-anchor="middle" font-size="9" fill="#10b981" font-weight="600">backend: "ollama"</text>

  <!-- No / timeout → Anthropic -->
  <line x1="420" y1="200" x2="420" y2="225" stroke="#f59e0b" stroke-width="1"/>
  <line x1="420" y1="225" x2="530" y2="225" stroke="#818cf8" stroke-width="1.5" class="flow"/>
  <text x="415" y="218" font-size="7" fill="#f59e0b">no/timeout</text>

  <!-- Anthropic box -->
  <rect x="530" y="210" width="140" height="30" rx="5" fill="#1e1b4b" stroke="#818cf8" stroke-width="1.5"/>
  <text x="600" y="226" text-anchor="middle" font-size="8" fill="#a5b4fc" font-weight="600">Anthropic claude-sonnet-4-6</text>
  <text x="600" y="250" text-anchor="middle" font-size="7" fill="#6366f1">fallback_used: true</text>

  <!-- Phase 00-03 note -->
  <rect x="30" y="220" width="170" height="28" rx="4" fill="#1e293b" stroke="#334155" stroke-width="1" stroke-dasharray="3 2"/>
  <text x="115" y="233" text-anchor="middle" font-size="7" fill="#94a3b8" font-weight="600">PHASES 00–03: ZERO LLM CALLS</text>
  <text x="115" y="243" text-anchor="middle" font-size="7" fill="#475569">11 deterministic controllers, zero cost</text>
</svg>
```

</p>

---

## System Architecture

<p align="center">

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 400" font-family="'JetBrains Mono', 'Fira Code', monospace">
  <style>
    @keyframes data-pulse { 0% { stroke-dashoffset: 16 } 100% { stroke-dashoffset: 0 } }
    @keyframes repo-glow { 0%,100% { filter: none } 50% { filter: drop-shadow(0 0 4px rgba(16,185,129,0.4)) } }
    .data-line { stroke-dasharray: 4 2; animation: data-pulse 2s linear infinite; }
    .repo-box { rx: 8; ry: 8; }
  </style>

  <rect width="800" height="400" fill="#0f172a" rx="12"/>

  <text x="400" y="25" text-anchor="middle" font-size="13" fill="#f8fafc" font-weight="700" letter-spacing="2">SYSTEM ARCHITECTURE</text>

  <!-- taem kernel (center) -->
  <rect x="280" y="110" width="240" height="140" rx="10" fill="#1e293b" stroke="#10b981" stroke-width="2">
    <animate attributeName="stroke-opacity" values="0.5;1;0.5" dur="3s" repeatCount="indefinite"/>
  </rect>
  <text x="400" y="135" text-anchor="middle" font-size="14" fill="#10b981" font-weight="700">taem</text>
  <text x="400" y="150" text-anchor="middle" font-size="8" fill="#475569">single Go binary</text>

  <!-- Kernel components -->
  <rect x="295" y="160" width="95" height="18" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="342" y="173" text-anchor="middle" font-size="7" fill="#10b981">Gate Machine</text>

  <rect x="400" y="160" width="105" height="18" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="452" y="173" text-anchor="middle" font-size="7" fill="#10b981">Controller Registry</text>

  <rect x="295" y="185" width="95" height="18" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="342" y="198" text-anchor="middle" font-size="7" fill="#10b981">Local Dispatch</text>

  <rect x="400" y="185" width="105" height="18" rx="3" fill="#2d1b69" stroke="#7c3aed" stroke-width="0.5"/>
  <text x="452" y="198" text-anchor="middle" font-size="7" fill="#a78bfa">Inference Router</text>

  <rect x="295" y="210" width="95" height="18" rx="3" fill="#052e16" stroke="#10b981" stroke-width="0.5"/>
  <text x="342" y="223" text-anchor="middle" font-size="7" fill="#10b981">State Interface</text>

  <rect x="400" y="210" width="105" height="18" rx="3" fill="#451a03" stroke="#fbbf24" stroke-width="0.5"/>
  <text x="452" y="223" text-anchor="middle" font-size="7" fill="#fbbf24">GitHub Dispatch</text>

  <!-- mc-state (left) -->
  <rect x="30" y="140" width="180" height="80" rx="8" fill="#1e293b" stroke="#f97316" stroke-width="1.5"/>
  <text x="120" y="165" text-anchor="middle" font-size="11" fill="#f97316" font-weight="700">mc-state</text>
  <text x="120" y="180" text-anchor="middle" font-size="7" fill="#475569">signals.jsonl | manifest.jsonl</text>
  <text x="120" y="192" text-anchor="middle" font-size="7" fill="#475569">integration-map | step-plan</text>
  <text x="120" y="208" text-anchor="middle" font-size="7" fill="#ea580c">git-first — every state is a commit</text>

  <!-- Arrows kernel ↔ mc-state -->
  <line x1="210" y1="165" x2="280" y2="175" stroke="#f97316" stroke-width="1.5" class="data-line"/>
  <line x1="280" y1="195" x2="210" y2="190" stroke="#f97316" stroke-width="1" stroke-dasharray="2 2" opacity="0.5"/>
  <text x="245" y="168" font-size="6" fill="#f97316">read/write</text>

  <!-- ecosystem (right) -->
  <rect x="590" y="120" width="180" height="120" rx="8" fill="#1e293b" stroke="#dc2626" stroke-width="1.5"/>
  <text x="680" y="143" text-anchor="middle" font-size="11" fill="#dc2626" font-weight="700">ecosystem</text>
  <text x="680" y="157" text-anchor="middle" font-size="7" fill="#475569">Qdrant vector database</text>
  <text x="610" y="175" font-size="6.5" fill="#64748b">repo_surfaces</text>
  <text x="610" y="188" font-size="6.5" fill="#64748b">mission_memory</text>
  <text x="610" y="201" font-size="6.5" fill="#64748b">constraint_index</text>
  <text x="710" y="175" font-size="6.5" fill="#64748b">wiring_patterns</text>
  <text x="710" y="188" font-size="6.5" fill="#64748b">lessons_learned</text>
  <text x="680" y="230" text-anchor="middle" font-size="7" fill="#b91c1c">kernel reads — never writes</text>

  <!-- Arrow kernel → ecosystem -->
  <line x1="520" y1="175" x2="590" y2="175" stroke="#dc2626" stroke-width="1.5" class="data-line"/>
  <text x="555" y="168" font-size="6" fill="#dc2626">read-only</text>

  <!-- adrs (top) -->
  <rect x="310" y="40" width="180" height="50" rx="8" fill="#1e293b" stroke="#8b5cf6" stroke-width="1.5"/>
  <text x="400" y="62" text-anchor="middle" font-size="11" fill="#8b5cf6" font-weight="700">adrs</text>
  <text x="400" y="78" text-anchor="middle" font-size="7" fill="#475569">ADR-000 → ADR-007 | hard constraints</text>

  <!-- Arrow adrs → kernel -->
  <line x1="400" y1="90" x2="400" y2="110" stroke="#8b5cf6" stroke-width="1.5" class="data-line"/>

  <!-- CLI (top left) -->
  <rect x="30" y="45" width="130" height="40" rx="6" fill="#1e293b" stroke="#38bdf8" stroke-width="1"/>
  <text x="95" y="63" text-anchor="middle" font-size="9" fill="#38bdf8" font-weight="600">$ taem launch</text>
  <text x="95" y="76" text-anchor="middle" font-size="7" fill="#475569">status | watch | retry | log</text>

  <!-- Arrow CLI → kernel -->
  <path d="M 160 65 Q 240 65 290 130" fill="none" stroke="#38bdf8" stroke-width="1" class="data-line"/>

  <!-- GitHub Actions (bottom) -->
  <rect x="290" y="290" width="220" height="50" rx="8" fill="#1e293b" stroke="#334155" stroke-width="1.5"/>
  <text x="400" y="312" text-anchor="middle" font-size="10" fill="#e2e8f0" font-weight="600">GitHub Actions</text>
  <text x="400" y="328" text-anchor="middle" font-size="7" fill="#475569">NAV workflows | PAO dispatch</text>

  <!-- Arrow kernel → GH Actions -->
  <line x1="400" y1="250" x2="400" y2="290" stroke="#fbbf24" stroke-width="1.5" class="data-line"/>
  <text x="415" y="273" font-size="6" fill="#fbbf24">dispatch</text>

  <!-- Ollama (bottom left) -->
  <rect x="50" y="300" width="110" height="40" rx="6" fill="#052e16" stroke="#10b981" stroke-width="1"/>
  <text x="105" y="318" text-anchor="middle" font-size="9" fill="#10b981" font-weight="600">Ollama</text>
  <text x="105" y="330" text-anchor="middle" font-size="7" fill="#059669">local inference</text>

  <!-- Anthropic (bottom right) -->
  <rect x="595" y="300" width="120" height="40" rx="6" fill="#1e1b4b" stroke="#818cf8" stroke-width="1"/>
  <text x="655" y="318" text-anchor="middle" font-size="9" fill="#a5b4fc" font-weight="600">Anthropic</text>
  <text x="655" y="330" text-anchor="middle" font-size="7" fill="#6366f1">fallback only</text>

  <!-- Arrows -->
  <path d="M 315 235 Q 200 280 160 300" fill="none" stroke="#10b981" stroke-width="1" class="data-line"/>
  <path d="M 480 230 Q 600 280 620 300" fill="none" stroke="#818cf8" stroke-width="1" stroke-dasharray="3 3"/>
  <text x="145" y="280" font-size="6" fill="#10b981">primary</text>
  <text x="570" y="275" font-size="6" fill="#818cf8">fallback</text>

  <!-- Target repo (far right) -->
  <rect x="625" y="45" width="140" height="40" rx="6" fill="#1e293b" stroke="#334155" stroke-width="1"/>
  <text x="695" y="63" text-anchor="middle" font-size="9" fill="#e2e8f0" font-weight="600">Target Repo</text>
  <text x="695" y="76" text-anchor="middle" font-size="7" fill="#475569">the code under review</text>

  <!-- Arrow target → ecosystem -->
  <line x1="695" y1="85" x2="695" y2="120" stroke="#334155" stroke-width="1" stroke-dasharray="2 2"/>
  <text x="705" y="105" font-size="6" fill="#475569">indexed</text>
</svg>
```

</p>

---

## Quick Start

```bash
# Install
go install github.com/taem-dev/taem/cmd/taem@latest

# Initialize mission workspace
taem init

# Launch a preflight review
taem launch \
  --repos org/repo \
  --task "integrate service X with the authentication layer" \
  --adrs ADR-001,ADR-002,ADR-005

# Monitor
taem status          # current phase + controller signals
taem watch           # live stream
taem log MSN-xxx     # full signal history
```

---

## Signal Values

| Signal | Meaning | Gate Effect |
|--------|---------|-------------|
| **GO** | Controller approves | Counts toward ADVANCE |
| **NO-GO** | Controller blocks | Triggers remediation or ABORT |
| **HOLD** | Inconclusive / waiting | Pauses — does not abort |
| **WARN** | Advisory | Non-blocking unless FAO timeline at risk |
| **RELAY** | Output only | Never gates (CAPCOM, PAO) |

---

## Design Principles

| Principle | Enforcement |
|-----------|-------------|
| **No code before Phase 04** | Gate machine blocks ADVANCE until all Phase 00–03 controllers GO |
| **Git-first state** | All mission data is git commits in mc-state. No external databases. |
| **Deterministic controllers = zero LLM** | Registry validates mode at load time. 11 of 15 controllers never call an LLM. |
| **Ollama before Anthropic** | Router enforces: local inference first, cloud fallback only on low confidence. |
| **Unicast only** | No multicast, broadcast, service mesh, mDNS. Every connection explicit. |
| **Stateless workers** | GitHub PR as worker boundary. Workers run, produce output, stop. |
| **Kernel owns the gate** | Gate logic lives only in `gate.go` — never in GitHub Actions YAML. |

---

## ADR Corpus

All code across every TAEM repository is governed by these Architecture Decision Records:

| ADR | Title | Classification |
|-----|-------|----------------|
| [ADR-000](https://github.com/TAEM-DEV/adrs/blob/main/ADR-000.yaml) | ADR Framework | HARD |
| [ADR-001](https://github.com/TAEM-DEV/adrs/blob/main/ADR-001.yaml) | Unicast Only | HARD |
| [ADR-002](https://github.com/TAEM-DEV/adrs/blob/main/ADR-002.yaml) | GitHub PR as Worker Boundary | HARD |
| [ADR-003](https://github.com/TAEM-DEV/adrs/blob/main/ADR-003.yaml) | Mission Control Preflight Protocol | HARD |
| [ADR-004](https://github.com/TAEM-DEV/adrs/blob/main/ADR-004.yaml) | Git-First Architecture | HARD |
| [ADR-005](https://github.com/TAEM-DEV/adrs/blob/main/ADR-005.yaml) | LLM Boundary | HARD |
| [ADR-006](https://github.com/TAEM-DEV/adrs/blob/main/ADR-006.yaml) | Ecosystem Layer | HARD |
| [ADR-007](https://github.com/TAEM-DEV/adrs/blob/main/ADR-007.yaml) | Kernel Architecture | HARD |

All ADRs are classified **HARD** — violations are automatic NO-GO signals. No exceptions.

---

<p align="center">
  <sub>Named after NASA's Terminal Area Energy Management — the Space Shuttle guidance phase<br/>that converts orbital energy into a precision runway landing. No second chances.</sub>
</p>
