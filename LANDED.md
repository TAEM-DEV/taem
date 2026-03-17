# LANDED.md — TAEM Full Shakedown Review: openclaw/openclaw

**Mission:** Review openclaw/openclaw codebase (v2026.3.14)
**Date:** 2026-03-17
**Target:** https://github.com/openclaw/openclaw
**Verdict:** **GO** — impressive security posture, advisories below

---

## Executive Summary

OpenClaw is a personal AI assistant gateway — a single TypeScript/Node.js monorepo (9,125 files, 3,316 source `.ts` files, 74 extensions, 55 bundled skills) that connects to 22+ messaging channels (WhatsApp, Telegram, Slack, Discord, Signal, iMessage, IRC, Teams, Matrix, LINE, etc.) and routes conversations through LLM providers. It runs as a local daemon and exposes a WebSocket/HTTP control plane.

This is a *serious* codebase. The security engineering is well above average for open-source AI projects. The project has clearly been through real-world security pressure — the SECURITY.md alone is 22KB of battle-hardened triage policy.

---

## Phase 00 — Pad Check

### GC — Ground Control

| Check | Status | Notes |
|-------|--------|-------|
| Build system | **GO** | TypeScript ESM, tsdown bundler, pnpm workspace monorepo |
| Runtime requirement | **GO** | Node >= 22, Bun optional for dev |
| Test framework | **GO** | Vitest with 9 config variants (unit, e2e, extensions, channels, gateway, live, scoped, UI, node) |
| Dependency manifest | **GO** | `pnpm-lock.yaml`, `pnpm-workspace.yaml`, 474K lockfile |
| Docker build | **GO** | Multi-stage Dockerfile, SHA-pinned base images, non-root user, Buildx cache mounts |
| Entrypoint | **GO** | `openclaw.mjs` → cobra-style CLI via commander/yargs |
| CI pipeline | **GO** | 10 GitHub Actions workflows, Blacksmith runners, multi-platform (Linux/macOS/Windows/Android) |
| Linting | **GO** | Oxlint (type-aware) + Oxfmt + ShellCheck + actionlint + zizmor + SwiftLint |
| Secret scanning | **GO** | detect-secrets v1.5.0 with 18 plugins, `.secrets.baseline` (423KB) |

**GC Signal: GO**

### DPS — Data Processing Systems

| Check | Status | Notes |
|-------|--------|-------|
| Config validation | **GO** | Extensive Zod schemas for all configuration (20+ schema files in `src/config/zod-schema*.ts`). Providers, sessions, agents, secrets, allowlists all validated. |
| Input sanitization | **GO** | External content wrapping with random-ID boundary markers, Unicode homoglyph folding, suspicious pattern detection. `src/security/external-content.ts` (354 lines). |
| URL/path validation | **GO** | Multi-layer path traversal prevention via `src/infra/boundary-path.ts`. Symlink escape hardening. Canvas file resolver rejects symlinks and `..` segments. |
| HTTP path canonicalization | **GO** | Up to 32 decode passes, fail-closed on decode limit and malformed encoding. `src/gateway/security-path.ts`. |
| Skill scanning | **GO** | Static analysis of skill code for dangerous patterns. `src/security/skill-scanner.ts` (583 lines). |

**DPS Signal: GO**

### EECOM — Resource Awareness

| Check | Status | Notes |
|-------|--------|-------|
| Auth rate limiting | **GO** | In-memory sliding-window: 10 attempts/min, 5-min lockout. Loopback exempt. Missing credentials don't trigger limiter (prevents lockout from bare browser requests). `src/gateway/auth-rate-limit.ts`. |
| SSRF protection | **GO** | Comprehensive: blocked hostnames, private IP detection, IPv6-embedded IPv4, legacy IPv4 literal blocking, DNS pinning, post-DNS validation, fail-closed on malformed addresses. `src/infra/net/ssrf.ts`. |
| Channel health monitoring | **GO** | `src/gateway/channel-health-monitor.ts` with configurable policies. |
| Docker resource management | **GO** | `--max-old-space-size=2048` for OOM protection during builds. Health checks with start periods. |

**EECOM Signal: GO**

---

## Phase 02 — Architectural Survey

### ARCH — Architecture Review

#### Strengths

1. **Defense-in-depth security model** — This is not security theater. Every trust boundary has real enforcement: timing-safe token comparison (`crypto.timingSafeEqual` on SHA-256 hashes), SSRF protection with DNS pinning, sandbox container validation blocking `/etc`, `/proc`, `/sys`, Docker socket paths. The `safeEqualSecret()` implementation in `src/security/secret-equal.ts` is textbook correct.

2. **External content isolation** — The `wrapExternalContent()` system in `src/security/external-content.ts` is one of the best prompt-injection defenses I've seen in production code. Random 8-byte hex boundary IDs prevent marker spoofing. Unicode homoglyph folding catches 18+ Unicode angle bracket variants. Zero-width character stripping prevents invisible token splitting. Markers in untrusted content are sanitized to `[[MARKER_SANITIZED]]`.

3. **No shell execution by default** — `src/process/exec.ts:shouldSpawnWithShell()` is hard-coded to return `false`. All command execution uses `execFile`/`spawn` with argv arrays, never string concatenation through a shell. Windows `cmd.exe` metacharacters are fail-closed (reject, don't escape).

4. **Multi-layer approval gates for tool execution** — `src/gateway/node-invoke-system-run-approval.ts` implements field allowlisting, approval record binding (to runId, nodeId, device ID, connection ID), expiration enforcement, command match verification, and atomic allow-once consumption.

5. **Channel access control is well-modeled** — DM pairing codes are 8-char from a 32-char unambiguous alphabet via `crypto.randomInt()`. Max 3 pending per channel. 60-minute TTL. Group command authorization explicitly does NOT inherit DM pairing-store approvals, preventing cross-context privilege escalation.

6. **Plugin SDK with strong boundaries** — 30+ SDK subpath exports (`openclaw/plugin-sdk/*`), each with separate `.d.ts` declarations. Extensions are workspace packages with isolated dependencies.

7. **Massive test suite** — 2,585 test files. Security audit tests alone are 3,771 lines (`src/security/audit.test.ts`).

#### Findings

| ID | Severity | Finding |
|----|----------|---------|
| ARCH-001 | **MEDIUM** | **Coverage exclusion list is broad.** `vitest.config.ts` lines 74-147 exclude `src/agents/**`, `src/channels/**`, `src/gateway/**`, `src/providers/**`, `src/plugins/**`, `src/browser/**`, `src/cli/**`, `src/commands/**` from coverage thresholds. These are the most security-critical surfaces. The 70% line threshold applies only to the remaining core logic. |
| ARCH-002 | **MEDIUM** | **`all: false` in coverage config.** Only files exercised by tests count toward thresholds. Files with zero test coverage are invisible to the metric. This inflates apparent coverage. |
| ARCH-003 | **LOW** | **Branch coverage threshold is 55%.** Lines/functions/statements are at 70%, but branch coverage is 15 points lower. Branch coverage is typically the most meaningful security metric (missed error paths, unchecked conditions). |
| ARCH-004 | **LOW** | **Tag-based action pinning.** GitHub Actions use `@v6` tags, not SHA pins. The `zizmor.yml` config explicitly disables `unpinned-uses`. Tag mutation attacks are possible on third-party actions like `useblacksmith/stickydisk@v1` and `oven-sh/setup-bun@v2.1.3`. |
| ARCH-005 | **INFO** | **ci.yml lacks top-level `permissions:` block.** PRs get read-only tokens by default, but pushes to `main` could get wider permissions. The `zizmor.yml` config explicitly disables `excessive-permissions`. Known accepted risk. |

**ARCH Signal: GO** with advisories

---

### PCO — Pattern Compliance

| Pattern | Status | Notes |
|---------|--------|-------|
| ESM module system | **GO** | `"type": "module"`, `.js` extension imports throughout, tsdown for bundling |
| Zod validation at boundaries | **GO** | Config, secrets, provider settings all schema-validated |
| Timing-safe comparison | **GO** | `crypto.timingSafeEqual` on SHA-256 hashes for all secret comparisons |
| CSRF/origin checking | **GO** | Browser origin validated against allowlist, loopback verified at socket level |
| CSP headers | **GO** | `frame-ancestors 'none'`, `script-src 'self'`, `base-uri 'none'`, `object-src 'none'` |
| Docker best practices | **GO** | Multi-stage build, non-root user (`node`), SHA-pinned base images, apt cache mounts, health checks, `init: true` |
| Pre-commit hooks | **GO** | 14 hooks: trailing-whitespace, detect-private-key, detect-secrets, shellcheck, actionlint, zizmor, oxlint, oxfmt, swiftlint, ruff |
| Dependency auditing | **GO** | `pnpm-audit-prod` in both pre-commit and CI |
| Container sandbox validation | **GO** | Blocked host paths, blocked seccomp/AppArmor profiles, network mode restrictions, bind mount boundary checking |
| Safe binary execution | **GO** | `src/infra/exec-safe-bin-*.ts` — trusted binary path validation with policy profiles |

**PCO Signal: GO**

---

### CDS — Conflict Detection

| ID | Severity | Finding |
|----|----------|---------|
| CDS-001 | **LOW** | **actionlint version mismatch.** Pre-commit pins `v1.7.10`, CI workflow-sanity.yml installs `v1.7.11`. Could cause differing results between local and CI checks. |
| CDS-002 | **INFO** | **Dual runtime support.** Both Bun and Node are supported. `pnpm-lock.yaml` and Bun patching must stay in sync. Documented but adds maintenance surface. |
| CDS-003 | **INFO** | **Extension dependency isolation.** Extensions are workspace packages with own `package.json`. Plugin-only deps must not leak to root. Documented rule in AGENTS.md line 49-50. |

**CDS Signal: GO**

---

## Phase 04 — Pre-Code Inspection

### SECINSP — Security Inspection

#### Pass 1 — Deterministic Scan

| ID | Severity | Pattern | Location | Finding |
|----|----------|---------|----------|---------|
| SEC-001 | **GO** | Timing-safe comparison | `src/security/secret-equal.ts:3-12` | `timingSafeEqual` on SHA-256 hashes. Textbook correct. Hash-then-compare prevents length leakage. |
| SEC-002 | **GO** | Shell injection prevention | `src/process/exec.ts:86-97` | `shouldSpawnWithShell()` hard-returns `false`. Windows cmd metacharacters fail-closed. |
| SEC-003 | **GO** | SSRF protection | `src/infra/net/ssrf.ts` | Blocked hostnames, private IP ranges, DNS pinning via `createPinnedLookup()`, post-DNS IP re-validation. Legacy IPv4 literal formats (octal, hex) treated as private. |
| SEC-004 | **GO** | Prompt injection defense | `src/security/external-content.ts` | Random-ID boundary markers, Unicode homoglyph folding (18+ variants), zero-width char stripping, marker sanitization. |
| SEC-005 | **GO** | Path traversal prevention | `src/infra/boundary-path.ts`, `src/canvas-host/file-resolver.ts` | Multi-layer: lexical check, canonical resolution through existing ancestors, symlink rejection, root boundary enforcement. |
| SEC-006 | **GO** | Container sandbox hardening | `src/agents/sandbox/validate-sandbox-security.ts` | Blocked paths (/etc, /proc, /sys, /dev, /root, /boot, Docker socket), blocked seccomp/AppArmor "unconfined", host network blocked, symlink escape hardening. |
| SEC-007 | **GO** | Token auto-generation | `src/gateway/startup-auth.ts:256` | `crypto.randomBytes(24).toString("hex")` — 192 bits of entropy. |
| SEC-008 | **GO** | Hooks token separation | `src/gateway/startup-auth.ts:291-316` | `hooks.token` must differ from gateway auth token. Prevents webhook→admin escalation. |
| SEC-009 | **GO** | Dangerous tool denylists | `src/security/dangerous-tools.ts` | HTTP surface blocks `sessions_spawn`, `sessions_send`, `cron`, `gateway`. ACP surface requires explicit approval for `exec`, `spawn`, `shell`, `fs_write`, `fs_delete`. |
| SEC-010 | **GO** | Pairing store security | `src/pairing/pairing-store.ts` | Path traversal prevention via `safeChannelKey()`, file locking, 3-pending cap, 60-min TTL, account isolation. |

#### Pass 2 — Novel Attack Surface Analysis

| ID | Severity | Finding |
|----|----------|---------|
| SEC-011 | **MEDIUM** | **22+ channel inbound surfaces.** Each messaging channel (WhatsApp, Telegram, Slack, Discord, Signal, iMessage, IRC, Teams, Matrix, LINE, etc.) is an inbound attack surface. The DM pairing system and external content wrapping mitigate this, but the sheer surface area means each new channel extension is a potential bypass vector if it doesn't properly integrate the security primitives. |
| SEC-012 | **MEDIUM** | **Skill/extension trust model.** Skills are Markdown+code bundles installed from ClawHub or workspace. The `skill-scanner.ts` performs static analysis, but skills can execute arbitrary code once installed. The `coding-agent` skill explicitly runs `claude --permission-mode bypassPermissions`. Trust is placed on the operator to curate skills. |
| SEC-013 | **LOW** | **Canvas host binds 127.0.0.1 by default.** This is good. But `listenHost` is configurable — an operator could bind to `0.0.0.0`. The canvas serves static files from a user-controlled root, so network exposure would expose those files. |
| SEC-014 | **LOW** | **`auth.mode="none"` is a valid configuration.** Intentional for loopback-only deployments, but misconfiguration with `--bind lan` would expose an unauthenticated gateway. The security audit system (`src/security/audit.ts`, 1,318 lines) flags this, and `openclaw doctor` surfaces it. |

**SECINSP Signal: GO** — No blocking findings. The security engineering is genuinely good.

---

### TRC — Test Readiness

| Check | Status | Notes |
|-------|--------|-------|
| Test framework configured | **GO** | Vitest with V8 coverage, 9 config variants |
| Test file count | **GO** | 2,585 test files |
| Security tests | **GO** | `src/security/audit.test.ts` (3,771 lines), `dm-policy-shared.test.ts` (483 lines), `safe-regex.test.ts`, `skill-scanner.test.ts`, `temp-path-guard.test.ts`, `windows-acl.test.ts` (653 lines) |
| SSRF tests | **GO** | `ssrf.test.ts`, `ssrf.dispatcher.test.ts`, `ssrf.pinning.test.ts` — three dedicated SSRF test files |
| Sandbox tests | **GO** | 20+ sandbox test files including `validate-sandbox-security.test.ts`, `fs-bridge.boundary.test.ts`, `sanitize-env-vars.test.ts` |
| Coverage thresholds | **WARN** | 70%/70%/55%/70% (lines/functions/branches/statements). Many critical subsystems excluded from measurement. |
| E2E tests | **GO** | `vitest.e2e.config.ts`, Docker E2E configs, live test configs |
| Pre-commit test gate | **GO** | `prek install` runs same checks as CI |

**TRC Signal: GO** with coverage advisory

---

### PRB — Peer Review Board

#### PRB-SKP (Skeptic) — Assumes Failure

**Vote: GO** (with advisories)

> I looked hard for showstoppers and didn't find any. The security model is well-thought-out and the implementation matches the design.
>
> My main concern is *surface area*. 22 messaging channels, 74 extensions, 55 skills, a WebSocket control plane, an HTTP API, a canvas host, and sandbox container management. Each is a potential bypass vector. The team clearly knows this — the DM pairing system, external content wrapping, and tool approval gates are all designed for a hostile-input world.
>
> The coverage exclusion list is my biggest reservation. The security-critical surfaces (gateway, agents, channels, browser) are excluded from coverage thresholds and rely on "manual/e2e/integration" testing. This means regressions in these paths may not be caught by the CI gate. But the team has 2,585 test files and the security-specific tests are thorough, so this is a process risk, not a code risk.
>
> The `coding-agent` skill running `claude --permission-mode bypassPermissions` is concerning in theory but is operator-initiated and requires explicit installation.

#### PRB-COR (Correctness) — Internal Consistency

**Vote: GO**

> The codebase is internally consistent. The security primitives (external content wrapping, SSRF protection, sandbox validation, auth rate limiting) are used correctly at their call sites. The Zod schemas match the runtime config types. The extension/plugin SDK exports are properly typed with `.d.ts` declarations.
>
> The DM policy decision tree (`resolveDmGroupAccessDecision`) is a well-structured state machine with explicit reason codes. Group command authorization correctly refuses to inherit DM pairing-store approvals.
>
> The sandbox validation layer correctly blocks dangerous host paths, checks symlink targets after resolution, and validates bind mounts against the blocked list. The Docker socket is blocked via three path variants (`/var/run/docker.sock`, `/run/docker.sock`, `/private/var/run/docker.sock`).
>
> I found no logic bugs, no inconsistent state handling, and no security primitives used incorrectly.

#### PRB-ADR (ADR Audit) — Principle Adherence

**Vote: GO**

> Evaluating against OpenClaw's stated design principles:
>
> 1. **"Personal, single-user assistant"** — HONORED. The trust model (`SECURITY.md`) explicitly states that authenticated gateway callers are trusted operators. No multi-tenant authorization. One user, one gateway.
>
> 2. **"Treat inbound DMs as untrusted input"** — HONORED. DM pairing system, external content wrapping with random-ID markers, suspicious pattern detection. This is enforced at the architecture level, not just documented.
>
> 3. **"Local-first"** — HONORED. Gateway binds to 127.0.0.1 by default. Canvas host binds to 127.0.0.1. State in `~/.openclaw/`. No cloud dependency for core functionality.
>
> 4. **"No shell execution"** — HONORED. `shouldSpawnWithShell()` returns `false`. Windows metacharacters fail-closed.
>
> 5. **"Extension isolation"** — HONORED. Extensions are workspace packages with isolated dependencies. Plugin-only deps don't leak to root.

**PRB Aggregate: 3 GO, 0 NO-GO → GO by unanimous**

---

## Gate Decision

```
Phase 04 Gate: GO
  GC:      GO
  DPS:     GO
  EECOM:   GO
  ARCH:    GO (with advisories)
  PCO:     GO
  CDS:     GO
  SECINSP: GO
  TRC:     GO (with coverage advisory)
  PRB:     GO (3/3 unanimous)

Recommendation: ADVANCE
```

---

## Advisories (Non-Blocking)

### Priority 1 — Should Address

| Priority | ID | Recommendation |
|----------|----|----------------|
| P1 | ARCH-001/002 | **Expand coverage measurement.** Remove `all: false` or at minimum include `src/gateway/auth*.ts`, `src/security/**`, `src/agents/sandbox/**` in the coverage report. These are the highest-risk surfaces and should have measured, enforced coverage. |
| P1 | ARCH-003 | **Raise branch coverage threshold to 65%.** Branch coverage catches missed error paths and unchecked conditions — the most common source of security regressions. |
| P1 | ARCH-004 | **SHA-pin third-party actions.** At minimum pin `useblacksmith/*` and `oven-sh/*` actions to commit SHAs. First-party `actions/*` and `docker/*` are lower risk. |

### Priority 2 — Good Hygiene

| Priority | ID | Recommendation |
|----------|----|----------------|
| P2 | SEC-011 | **Channel extension security checklist.** Create a documented checklist for new channel extensions ensuring they integrate DM policy, external content wrapping, pairing, and command gating. |
| P2 | SEC-012 | **Skill installation warning.** When installing skills that contain `exec`, `spawn`, `shell`, or `bypassPermissions`, surface a clear warning to the operator. |
| P2 | CDS-001 | **Pin actionlint version.** Align pre-commit and CI to the same version. |
| P2 | ARCH-005 | **Add `permissions: {}` to ci.yml top level.** Opt-in to least-privilege for push events. |

---

## What's Good — And It's a Lot

This is one of the most security-mature open-source AI projects I've reviewed. Here's what stands out:

1. **The external content wrapping system is exceptional.** Random-ID boundary markers with Unicode homoglyph folding, zero-width character stripping, and marker sanitization. This isn't a toy — it's a real defense against real prompt injection attacks, including sophisticated Unicode-based evasion techniques. Most projects don't even attempt this.

2. **The SSRF protection is production-grade.** DNS pinning to prevent TOCTOU rebinding attacks, legacy IPv4 literal blocking (octal/hex), IPv6-embedded IPv4 extraction, post-DNS IP re-validation, hostname allowlisting with wildcard patterns. Three dedicated test files. This would pass a penetration test.

3. **The sandbox validation layer is thorough.** Blocked host paths, blocked security profiles, network mode restrictions, symlink escape hardening with canonical path resolution through existing ancestors. The Docker socket is blocked via three platform-specific path variants.

4. **No shell execution by default.** A hard `return false` in `shouldSpawnWithShell()`. Windows `cmd.exe` metacharacters are rejected, not escaped. This is the correct approach — fail closed.

5. **The security audit system is self-aware.** `src/security/audit.ts` (1,318 lines) and `src/security/audit-extra.{sync,async}.ts` (1,300+ lines each) implement a comprehensive self-audit. `openclaw doctor` surfaces misconfigurations. The system knows its own risk surfaces and reports on them.

6. **The test suite is massive and targeted.** 2,585 test files. Security-specific tests are among the largest in the codebase (`audit.test.ts` at 3,771 lines). SSRF has three dedicated test files. Sandbox has 20+ test files.

7. **The trust model is documented and enforced.** SECURITY.md (22KB) doesn't just describe policy — it lists specific false-positive patterns, defines triage fast paths, and sets explicit scope boundaries. The code enforces these boundaries (hooks token separation, DM policy preventing cross-context escalation, tool denylists per surface).

8. **The pre-commit and CI pipeline is comprehensive.** 14 pre-commit hooks including detect-secrets, actionlint, zizmor, and type-aware oxlint. CodeQL SAST in CI. Secret scanning with 423KB baseline. This is a project that takes supply chain security seriously.

---

*Generated by TAEM kernel review protocol. Controllers: GC, DPS, EECOM, ARCH, PCO, CDS, SECINSP, TRC, PRB-SKP, PRB-COR, PRB-ADR, CAPCOM.*
