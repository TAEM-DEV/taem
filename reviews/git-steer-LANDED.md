# LANDED.md — TAEM Full Shakedown Review: ry-ops/git-steer

**Mission:** Review git-steer v0.3.0 codebase
**Date:** 2026-03-17
**Target:** https://github.com/ry-ops/git-steer
**Verdict:** **HOLD** — strong foundation, actionable findings below

---

## Phase 00 — Pad Check

### GC — Ground Control

| Check | Status | Notes |
|-------|--------|-------|
| Build system | **GO** | TypeScript + `tsc`, clean `tsconfig.json` targeting ES2022/NodeNext |
| Test framework | **GO** | Vitest configured, 5 test files present |
| Dependency manifest | **GO** | `package.json` well-structured, `engines.node >= 20` enforced |
| Docker build | **GO** | Multi-stage Dockerfile, non-root user, production-only deps |
| Entrypoint | **GO** | `bin/cli.js` → cobra-style CLI via `commander` |

### DPS — Data Processing Systems

| Check | Status | Notes |
|-------|--------|-------|
| Schema validation on inputs | **WARN** | MCP tool `inputSchema` definitions exist but **no runtime validation**. Tool args are cast with `as string`, `as number` etc. No Zod, no JSON Schema validation at the boundary. If an LLM sends malformed args, the tool just crashes. |
| State file schemas | **WARN** | JSONL files (`audit.jsonl`, `rfcs.jsonl`, `quality.jsonl`) have TypeScript interfaces but **no schema enforcement on load**. `loadJsonLines()` does raw `JSON.parse()` per line with a bare `catch {}` — silent data corruption is possible. |
| Config schemas | **WARN** | YAML config files (`managed-repos.yaml`, `policies.yaml`, `schedules.yaml`) parsed with `parseYaml()` returning `any`. No shape validation. |

**DPS Signal: WARN** — Three unvalidated boundaries: MCP tool inputs, state file loads, config loads.

### EECOM — Resource Awareness

| Check | Status | Notes |
|-------|--------|-------|
| Rate limit awareness | **GO** | Excellent. `ThrottledOctokit` with primary + secondary rate limit callbacks, `p-limit` concurrency caps (write:2, read:8, search:1), `steer_status` exposes all buckets. |
| ETag caching | **GO** | Conditional GET requests with persistent ETag cache across restarts. Reduces API cost. Well-implemented. |
| Audit log rotation | **GO** | 10,000 entry cap with `slice(-MAX)` rotation. Prevents unbounded memory. |
| Token burn estimation | **WARN** | No pre-flight token budget check. `security_sweep` across many repos could exhaust rate limits before the sweep completes. EECOM would want a budget gate before dispatch. |

**EECOM Signal: GO** with advisory on missing pre-sweep budget check.

---

## Phase 02 — Architectural Survey

### ARCH — Architecture Review

#### Strengths

1. **Zero-footprint local architecture** — Credentials in Keychain, state in GitHub, compute in Actions. Clean separation. The local machine is genuinely just a control plane.

2. **Graceful degradation** — `gateway.ts` wraps all Fabric initialization in try/catch and returns a `degraded` handle. The MCP server continues without Fabric tools if gateway fails. Good pattern.

3. **State-as-code** — All state in a private git repo (`git-steer-state`). JSONL for append-only logs, YAML for config. Mirrors TAEM's git-first principle (ADR-004).

4. **Separation of concerns** — Clean module boundaries: `core/` (infra), `github/` (API client), `state/` (persistence), `mcp/` (protocol), `fabric/` (gateway), `reports/` (templates), `dashboard/` (viz).

5. **Worker boundary pattern** — GitHub Actions workflows are stateless workers that run, produce output, stop. Aligns with ADR-002 thinking.

#### Findings

| ID | Severity | Finding |
|----|----------|---------|
| ARCH-001 | **HIGH** | **God file: `mcp/server.ts`** — This single file defines all ~50 MCP tools, their schemas, AND their implementation handlers. At 967+ lines of schema definitions alone (before the handler code), this is a maintainability and review bottleneck. Tool definitions should be co-located with their implementations. |
| ARCH-002 | **HIGH** | **God file: `fabric/app.ts`** — 574 lines containing 17 complete tool implementations inline as anonymous functions. No separation between tool definition and execution logic. |
| ARCH-003 | **MEDIUM** | **Dual GitHub client pattern** — `github/client.ts` (Octokit with App auth, throttling, ETag cache) and `fabric/app.ts` (bare `new Octokit({ auth: token })` with zero throttling/caching). The Fabric app bypasses all rate-limit hardening. Two clients with different safety guarantees hitting the same API. |
| ARCH-004 | **MEDIUM** | **Environment mutation** — `gateway.ts:36-38` sets `process.env.GITHUB_TOKEN`, `process.env.STATE_REPO`, `process.env.MANAGED_REPOS` as side effects. Global env mutation to pass config to a dynamically imported module is fragile. |
| ARCH-005 | **LOW** | **Hardcoded owner** — `index.ts:147` hardcodes `const owner = 'ry-ops'` and `const stateRepo = 'git-steer-state'` for dashboard deployment. Should be derived from state. |
| ARCH-006 | **LOW** | **`dist/` checked into source** — Built artifacts committed to the repo. Increases repo size and creates merge conflicts on any source change. |

**ARCH Signal: WARN**

---

### PCO — Pattern Compliance

| Pattern | Status | Notes |
|---------|--------|-------|
| MCP Server | **GO** | Correct use of `@modelcontextprotocol/sdk`, proper `Server` instantiation, `ListToolsRequestSchema`/`CallToolRequestSchema` handlers, stdio + HTTP/SSE transports. |
| GitHub App auth | **GO** | Manifest flow for app creation, installation token generation, Keychain credential storage. |
| JSONL state | **GO** | Append-only audit log, line-per-record format, proper newline joining. |
| TypeScript strict mode | **GO** | `strict: true` in tsconfig, `forceConsistentCasingInFileNames`, declaration maps. |
| Docker best practices | **GO** | Multi-stage build, non-root user, `npm ci --omit=dev`, `.dockerignore` present. |
| Concurrency control | **GO** | `p-limit` for API call concurrency. Write:2, Read:8, Search:1. Conservative and correct. |
| Error boundary | **WARN** | Gateway errors caught. But MCP tool handlers use bare `try/catch` returning string errors. No structured error codes, no correlation IDs. |
| Kubernetes tools without K8s client | **WARN** | `oomkill_detect`, `oomkill_remediate`, `cert_check`, `cert_renew` tools are defined in MCP schemas but their implementations shell out to `kubectl` via `execFileSync`. No actual K8s client library. Fragile, no error typing, assumes kubectl on PATH. |

**PCO Signal: GO** with advisories.

---

### CDS — Conflict Detection

| ID | Severity | Finding |
|----|----------|---------|
| CDS-001 | **HIGH** | **Dual Octokit instances** — `GitHubClient` (throttled, cached, rate-aware) and `fabric/app.ts` `createOctokit(token)` (raw, unthrottled). Both hit GitHub API. The Fabric Octokit has zero rate limit protection and could trigger secondary rate limits that affect the main client's budget. |
| CDS-002 | **MEDIUM** | **Token source ambiguity** — `index.ts:100-102` uses `process.env.GITHUB_TOKEN ?? process.env.GIT_STEER_TOKEN ?? await this.github.getInstallationToken()`. Three token sources with different scopes and rate limit buckets. No documentation of which takes precedence and why. |
| CDS-003 | **MEDIUM** | **State write conflicts** — `StateManager.save()` writes all files sequentially via GitHub Contents API. If two concurrent sessions (MCP server + heartbeat workflow) both save state, last-write-wins on each file. No optimistic locking beyond SHA-based conflict detection (which only prevents overwrites of the exact same file, not logical conflicts across files). |
| CDS-004 | **LOW** | **Tool name collision potential** — Core tools (`repo_list`) and Fabric tools (`fabric_git_list_repos`) overlap in functionality. Both can list repos. No deduplication guidance for the LLM consumer. |

**CDS Signal: WARN**

---

## Phase 04 — Pre-Code Inspection

### SECINSP — Security Inspection

#### Pass 1 — Regex Scan (Deterministic)

| ID | Severity | Pattern | Location | Finding |
|----|----------|---------|----------|---------|
| SEC-001 | **CRITICAL** | Secrets in process | `gateway.ts:36` | `process.env.GITHUB_TOKEN = opts.githubToken` — writes a token into the global environment where any imported module, dependency, or child process can read it. This is the token equivalent of writing a password to a shared file. |
| SEC-002 | **HIGH** | Token exposure via adapter | `fabric/adapter.ts:7` | `FabricGitHubAdapter` interface exposes `token: string` as a public property. Any code with a reference to the adapter can read the raw token. Should be a method that returns headers, not the raw credential. |
| SEC-003 | **HIGH** | Token in Authorization header (raw fetch) | `fabric/git.ts:22-26` | Multiple functions construct `Authorization: token ${github.token}` headers manually. Token is passed through string interpolation, appears in error messages if fetch fails, and is not redacted in logs. |
| SEC-004 | **HIGH** | Unvalidated webhook URL | `server.ts` (slack_notify tool) | `webhook_url` parameter is passed directly to fetch. No URL validation, no allowlist. An LLM could be prompt-injected into sending data to an attacker-controlled URL via Slack notification. |
| SEC-005 | **HIGH** | `npm audit fix --force` in CI | `security-fix.yml:153` | `--force` flag can introduce breaking major version upgrades. In an automated pipeline with no human review gate before merge, this could break production. |
| SEC-006 | **MEDIUM** | `repo_delete` confirmation bypass | `server.ts` | The `confirm` field on `repo_delete` is just a string match. An LLM calling this tool can trivially pass the correct confirmation string. The "safety" mechanism only protects against typos, not against automated misuse. |
| SEC-007 | **MEDIUM** | Keychain as only auth boundary | `core/keychain.ts` | Uses `keytar` (macOS Keychain). No fallback for Linux/Windows. Docker container has no Keychain — the Dockerfile doesn't address credential injection for containerized deployments. |
| SEC-008 | **LOW** | `execFileSync` for code review | `server.ts` (code_review tool) | Shells out to `cr` (CodeRabbit CLI) via `execFileSync`. Path is checked with `existsSync` but the binary is not integrity-verified. Potential for binary substitution. |
| SEC-009 | **LOW** | Silent catch blocks | `state/manager.ts:522-524` | `loadYaml`, `loadJson`, `loadJsonLines` all catch errors silently and return empty defaults. A corrupted state file is indistinguishable from an empty state file. |

#### Pass 2 — Novel Attack Surface Analysis

| ID | Severity | Finding |
|----|----------|---------|
| SEC-010 | **HIGH** | **MCP tool injection surface** — git-steer exposes ~50 tools to an LLM. Tools like `repo_delete`, `branch_reap`, `pr_merge`, `actions_trigger`, `cert_renew` (deletes K8s secrets) are destructive. There is no permission tier, no confirmation flow beyond the LLM's own judgment, and no audit-before-execute gate. A prompt injection attack through a malicious repo name, PR body, or issue comment could chain destructive tool calls. |
| SEC-011 | **HIGH** | **Workflow dispatch injection** — `security-sweep.yml` accepts `target_repos` as a JSON string input and uses `fromJson()` to expand it into a matrix. A malicious JSON payload could inject unexpected values into shell commands via `${{ matrix.target.owner }}` and `${{ matrix.target.repo }}` (which are not quoted in several shell contexts). |
| SEC-012 | **MEDIUM** | **State repo as single point of compromise** — All operational state, audit logs, sweep cursors, and RFC tracking live in `git-steer-state`. If the GitHub App token is compromised, an attacker has read/write access to the complete operational history AND can modify sweep state to skip repos or suppress alerts. |

**SECINSP Signal: NO-GO** — SEC-001 (token in global env) and SEC-010 (unguarded destructive tools) are blocking.

---

### TRC — Test Readiness

| Check | Status | Notes |
|-------|--------|-------|
| Test framework configured | **GO** | Vitest with config |
| Unit tests exist | **WARN** | 5 test files but they test **isolated logic only** (regex escaping, constructor validation, audit rotation, gateway mocking). Zero tests for actual tool execution, state persistence, or MCP protocol handling. |
| Integration tests | **WARN** | `gateway-integration.test.ts` exists but is skipped without `GITHUB_TOKEN`. No CI integration test pipeline visible. |
| Coverage measurement | **NO-GO** | No coverage configuration. No coverage thresholds. No coverage reporting. |
| Security tests | **NO-GO** | Zero security-focused tests. No tests for input validation, token handling, error redaction, or boundary enforcement. |
| E2E tests | **NO-GO** | No end-to-end test that exercises the MCP server, makes tool calls, and validates responses. |

**TRC Signal: NO-GO** — Test coverage is superficial. Core tool handlers are completely untested.

---

### PRB — Peer Review Board

#### PRB-SKP (Skeptic) — Assumes Failure

**Vote: NO-GO**

> The MCP server exposes 50+ tools including `repo_delete`, `branch_reap`, `pr_merge`, `actions_trigger`, and `cert_renew` (which deletes Kubernetes secrets) — all accessible to an LLM with zero permission tiers. The `repo_delete` "confirmation" is just string matching that an LLM trivially satisfies. There is no human-in-the-loop gate, no undo mechanism, and no blast radius estimation.
>
> The dual Octokit pattern (throttled GitHubClient vs. raw Fabric Octokit) means rate limit protections can be bypassed via the Fabric path. Under load, the unthrottled Fabric client could trigger GitHub's secondary rate limit, which would then throttle *all* requests including the main client's critical operations.
>
> The silent `catch {}` blocks in state loading mean corrupted state is treated as empty state — a sweep cursor corruption could cause a full re-sweep of all repos, burning the entire rate limit budget in one call.
>
> Historical pattern: projects with >30 unguarded API tools and zero integration tests have a high escalation rate when deployed to production.

#### PRB-COR (Correctness) — Internal Consistency

**Vote: GO** (with advisories)

> The codebase is internally consistent. The MCP tool definitions match the handler implementations. The state model (JSONL + YAML + cache.json) is coherent. The gateway degradation pattern works correctly — when Fabric fails, core tools continue. The sweep cursor mechanism correctly supports chunked execution with resume.
>
> Advisory: `StateManager.save()` writes 7 files sequentially without a transaction. If the process crashes mid-save (e.g., after writing `audit.jsonl` but before `rfcs.jsonl`), state becomes inconsistent. This is an inherent limitation of the GitHub Contents API, but should be documented.

#### PRB-ADR (ADR Audit) — Principle Adherence

**Vote: NO-GO**

> Evaluating against git-steer's own stated principles:
>
> 1. **"Zero local footprint"** — VIOLATED. `code_review` tool uses `execFileSync` to run a local binary. Kubernetes tools assume `kubectl` on PATH. The Docker image creates a local config directory.
>
> 2. **"Credentials stored exclusively in macOS Keychain"** — VIOLATED. Docker deployment has no Keychain. The fallback is environment variables, which are documented as the primary path for MCP server mode (`GITHUB_TOKEN`/`GIT_STEER_TOKEN`).
>
> 3. **"No local repository clones"** — HONORED. All repo operations go through GitHub API.
>
> 4. **"State persisted in GitHub"** — HONORED with caveat. ETag cache is in-memory during runtime and persisted to `cache.json` on shutdown. If process crashes without graceful shutdown, cache state is lost (not critical, just causes extra API calls on restart).

**PRB Aggregate: 1 GO, 2 NO-GO → NO-GO by majority**

---

## Gate Decision

```
Phase 04 Gate: NO-GO
  SECINSP: NO-GO (SEC-001, SEC-010 blocking)
  TRC:     NO-GO (zero tool handler tests, zero coverage)
  PRB:     NO-GO (2/3 majority — skeptic + ADR audit)

Recommendation: HOLD — remediation cycle 1
```

---

## Remediation Priorities

### Cycle 1 (Blocking — must fix)

| Priority | ID | Fix |
|----------|----|-----|
| P0 | SEC-001 | Stop writing tokens to `process.env`. Pass config objects to Fabric modules directly. |
| P0 | SEC-010 | Add a permission tier to destructive tools. Require explicit confirmation for `repo_delete`, `branch_reap`, `cert_renew`. Consider a `--dry-run` default for sweep tools. |
| P0 | SEC-002/003 | Encapsulate token behind a method that returns headers. Never expose raw credential as a property. |
| P1 | ARCH-001/002 | Split `mcp/server.ts` tool definitions into per-domain modules. Split `fabric/app.ts` into per-tool files. |
| P1 | CDS-001 | Route Fabric API calls through the same throttled Octokit instance. One client, one rate limit budget. |
| P1 | TRC | Add integration tests for top 10 MCP tools. Add coverage reporting with a floor (suggest 60%). |

### Cycle 2 (Important — should fix)

| Priority | ID | Fix |
|----------|----|-----|
| P2 | DPS | Add Zod schemas for MCP tool inputs. Validate on entry, not with `as` casts. |
| P2 | SEC-004 | Validate and allowlist Slack webhook URLs. |
| P2 | SEC-005 | Remove `--force` from `npm audit fix` in CI. Use `--fix` instead and fail on breaking changes. |
| P2 | SEC-011 | Quote all `${{ matrix.target.* }}` values in workflow shell commands. |
| P2 | CDS-003 | Document the state write conflict model. Consider atomic batch commits via Git Data API (create tree + commit) instead of sequential file writes. |
| P2 | ARCH-005 | Derive dashboard deployment owner/repo from state, not hardcoded constants. |

---

## What's Good

This isn't just a list of problems. git-steer has real strengths worth calling out:

1. **Rate limit engineering is excellent.** The throttle/retry plugin stack, concurrency caps, ETag caching, and budget visibility (`steer_status`) are production-grade. Most GitHub automation tools get this wrong.

2. **The zero-clone architecture is sound.** All repo operations go through the GitHub API. No local git clones, no disk state beyond Keychain. This is a hard design choice that pays off in operational simplicity.

3. **Gateway degradation is well-implemented.** The Fabric gateway can fail completely and the core MCP server continues with its native tools. No cascading failures.

4. **The security sweep pipeline is ambitious and mostly correct.** Multi-ecosystem (npm, pip, Go), RFC lifecycle tracking, chunked sweep with resume, PR dedup — this is real operational automation.

5. **The dashboard is self-contained.** Pure SVG charts, no runtime dependencies, deployed to GitHub Pages. Simple and effective.

---

*Generated by TAEM kernel review protocol. Controllers: GC, DPS, EECOM, ARCH, PCO, CDS, SECINSP, TRC, PRB-SKP, PRB-COR, PRB-ADR, CAPCOM.*
