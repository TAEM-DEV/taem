# TAEM Knowledge Pipeline

Per ADR-009a: the knowledge pipeline turns every mission into backward
knowledge that improves future missions and produces outward-facing
artifacts (blog content, security reports).

## End-to-End Flow

```
taem launch --task "..." --repos org/repo
    |
    v
Phase 00: Pad Check (GC, DPS, EECOM)
    |
Phase 01: Corpus Ingestion (NAV -> integration-map.json)
    |
Phase 02: Architectural Survey (ARCH, CDS, PCO)
    |
Phase 03: Plan Formulation (INCO -> step-plan.json)
    |
Phase 04: Pre-Code Inspection (SECINSP, TRC, PRB-SKP, PRB-COR, PRB-ADR)
    |
Phase 05: CAPCOM Synthesis
    |  - Renders LANDED.md from signals + step-plan
    |  - Writes synthesis/ artifacts (blog-draft.md, security-report.json)
    |  - RELAY signal -- never blocks
    |
Phase 06: PAO Dispatch
    |  - Builds enriched payload: task, repos, signal_summary, synthesis_artifacts
    |  - Dispatches workflow_dispatch to mc-state PAO controller
    |  - PAO workflow reads synthesis/ dir, forwards to fabric-social
    |  - RELAY signal -- non-blocking, mission is already LANDED
    |
    v
kernel.close("LANDED")
    |  - Writes MISSION_LANDED to manifest.jsonl
    |  - Indexes lessons_learned to ecosystem (Qdrant)
    |  - Non-fatal if ecosystem is unreachable
    |
    v
fabric-social receives repository_dispatch (event_type: taem-mission-landed)
    - client_payload includes: mission_id, task, repos, signal_summary,
      synthesis_artifacts, blog_draft content, security_report content
    - Processes blog draft through narrative pipeline
    - Files security report if present
```

## Domain Knowledge Flow

### Forward Knowledge (mission reads ecosystem)

Controllers query the ecosystem service during execution:

- **NAV** (Phase 01): reads `repo_surfaces` for cached repo structure,
  skips re-read on cache hit. Checks staleness via `IsRepoStale()`.
- **ARCH** (Phase 02): reads `wiring_patterns` for validated integration
  patterns between repos.
- **PRB-SKP** (Phase 04): reads `mission_memory` for prior mission context,
  reads `lessons_learned` to check for known failure patterns.
- **PRB-ADR** (Phase 04): reads `constraint_index` for active ADR constraints.

### Backward Knowledge (mission writes to ecosystem)

After `MISSION_LANDED`, `kernel.close()` calls `indexLessonsLearned()`:

```
LessonRecord {
    mission_id      string
    task            string
    repos           []string
    go_signals      int
    nogo_signals    int
    hold_signals    int
    warn_signals    int
    step_count      int
    duration_s      int64
    outcome         "LANDED" | "ESCALATED"
    key_findings    []string   // NO-GO and WARN reasons
    timestamp       string
}
```

Posted to `POST /api/lessons` on the ecosystem service, which indexes into
the `lessons_learned` Qdrant collection. Future PRB-Skeptic queries surface
these lessons when evaluating similar tasks.

### Knowledge Growth

Each mission contributes to the ecosystem:

1. **repo_surfaces** -- NAV writes updated surfaces after each corpus scan
2. **mission_memory** -- mission manifest indexed for historical lookup
3. **lessons_learned** -- signal summary + key findings indexed post-land
4. **wiring_patterns** -- validated integration patterns from successful missions
5. **constraint_index** -- ADR constraints updated when new ADRs are added

Over time, the ecosystem builds a knowledge graph that reduces:
- Cold-read time (cached repo surfaces)
- Repeated failures (lessons_learned pattern matching)
- Constraint violations (indexed ADR constraints)

## PAO Dispatch Payload

The kernel builds PAO workflow inputs in `dispatchGitHub()`:

| Input | Type | Description |
|---|---|---|
| `mission_id` | string | Mission identifier (MSN-<hex>) |
| `phase` | string | Phase number ("6") |
| `task` | string | Original task description |
| `repos` | string | Comma-separated repo list |
| `signal_summary` | JSON string | `{"go":N,"no_go":N,"hold":N,"warn":N,"relay":N,"total":N}` |
| `synthesis_artifacts` | JSON string | Map of artifact name to mc-state relative path |

The PAO workflow then:
1. Sparse-checks out `missions/<id>/synthesis/` from mc-state
2. Reads blog-draft.md and security-report.json if present
3. Builds a `repository_dispatch` payload with all data
4. Dispatches to `ry-ops/fabric-social` with event `taem-mission-landed`

## Trigger Points

- **Post-land** (automatic): `kernel.close()` triggers lessons indexing and
  PAO dispatch as part of the standard mission lifecycle.
- **On-demand**: `taem knowledge` command (planned) will allow querying the
  ecosystem collections directly for prior mission context.
- **RefExplorer** (planned): triggered by CAPCOM synthesis to explore
  cross-repo reference patterns discovered during the mission.
