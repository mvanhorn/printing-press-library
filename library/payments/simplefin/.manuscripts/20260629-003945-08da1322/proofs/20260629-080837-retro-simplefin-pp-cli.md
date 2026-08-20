# Printing Press Retro: SimpleFIN

## Session Stats
- API: simplefin (SimpleFIN Protocol v2.0.0)
- Spec source: hand-authored internal YAML (from official protocol docs + live verification)
- Scorecard: 85/100 (A)
- Verify pass rate: 100%
- Fix loops: 2 (shipcheck export-probe; dogfood 4-failure round)
- Manual code edits: ~18 hand-authored files (foundation + 12 absorbed + 8 transcendence)
- Features built from scratch: 8 transcendence + custom auth/sync/store layer

## Findings

### 1. Generated credential-precedence tests fail for HTTP Basic (`Basic {token}`) specs (Bug / Scorer-adjacent)
- **What happened:** `internal/cliutil/credentials_test.go` ships 4 tests (TestCredentialsFileWinsWhenLegacyConfigAlsoHasSecrets, TestCorruptCredentialsFallsBackToLegacyConfig, TestCorruptCredentialsFallsBackToEnvCredential, TestEmptyCredentialsFileDoesNotClearLegacyConfig) that assert `strings.Contains(cfg.AuthHeader(), "<raw-credential>")`. For a spec with `auth.format: "Basic {token}"`, the generated `AuthHeader()` returns `"Basic " + base64(token+":")` — so the raw credential is never a literal substring and all 4 tests fail. `go test ./...` is red in every Basic-auth CLI.
- **Scorer correct?** N/A (these are generated unit tests, not a scorer). The credential-LOADING assertions (`assertConfigCredential`) pass; only the AuthHeader substring check is wrong.
- **Root cause:** generator test template assumes the credential appears verbatim in the auth header (true for `Bearer {token}`, false for base64 `Basic {token}`).
- **Cross-API check:** recurs on every CLI whose auth header is base64-encoded Basic.
- **Frequency:** subclass: HTTP-Basic / `Basic {token}` APIs.
- **Fallback if not fixed:** the agent cannot patch generator-reserved cliutil; ships red `go test`. Fallback reliability: zero (agent must not touch the file).
- **Worth a fix?** Yes — silent across all Basic-auth CLIs; nothing the agent can do.
- **Inherent or fixable:** fixable.
- **Durable fix:** in the credential-test template, decode the Basic header (or assert on the loaded credential field) instead of substring-matching the raw value against `AuthHeader()`. Parameterize by auth shape: substring is valid for `Bearer`/raw-header schemes; base64-Basic needs a decode.
- **Test:** generate a CLI from a `Basic {token}` spec → `go test ./internal/cliutil/` passes (positive). A `Bearer {token}` spec still passes (negative/regression).
- **Evidence:** `go test ./...` in this run: 4 FAIL in cliutil with actual `AuthHeader() = "Basic ZGF0YS1zZWNyZXQ6"` (base64 of synthetic `data-secret:`), expected substring `data-secret`.
- **Related prior retros:** None.
- **Step G (case against):** "SimpleFIN's `api_key`+`Basic {token}` modeling is unusual." Fails because the catalog's own Stytch entry uses the identical `Basic {a}:{b}` pattern, so a Stytch regen ships the same 4 red tests — it's the documented Basic-auth shape, not a SimpleFIN quirk.

### 2. Generated framework code ships HIGH/MED gosec findings in every CLI (Bug / generator)
- **What happened:** gosec flags the generated `internal/client/client.go` (G119: sensitive Authorization header re-added across redirects) and `internal/store/store.go` (G201: SQL built with string formatting) — HIGH/MED — in the generated tree of every CLI. Polish cleared 16 hand-authored gosec findings but left 31 in generated files it must not edit.
- **Scorer correct?** gosec is third-party; both are guarded in practice (client.go re-adds the header only on same-host redirects and dels cross-host; store.go formats SQL from internal resource-type constants, not user input), so they read as guarded-but-flagged rather than live vulns.
- **Root cause:** templates lack `#nosec` annotations-with-rationale (or a small restructure) for patterns gosec always flags.
- **Cross-API check:** identical templates emit in every CLI → universal.
- **Frequency:** every API.
- **Fallback if not fixed:** agent can't touch generated files; every CLI ships with HIGH gosec noise, and any gosec gate (publish/CI) goes red.
- **Worth a fix?** Yes — universal, agent-unfixable.
- **Inherent or fixable:** fixable.
- **Durable fix:** in the generator templates, either restructure (e.g., parameterized queries in store.go where feasible) or add narrow `// #nosec G119`/`// #nosec G201` with a durable rationale comment, so generated CLIs are gosec-clean out of the box.
- **Test:** generate any CLI → `gosec ./...` reports 0 HIGH in generated files (positive).
- **Evidence:** polish result this run: "31 remaining gosec findings are all in generated files incl the HIGH G119 in client.go and G201 in store.go."
- **Related prior retros:** None.
- **Step G (case against):** "These are false positives, leave them." Fails because the noise ships in every CLI and blocks any gosec gate; an annotation-with-rationale is the standard remediation and is a one-time template change benefiting all CLIs.

### 3. Bundled MCP tools-manifest excludes hand-authored novel commands (Discovered optimization / scorer+generator)
- **What happened:** the static `tools-manifest.json` (bundled in the `.mcpb`) is generated at generate-time from the spec endpoints, so it listed 2 tools while the shipped CLI has ~20 user-facing commands. The runtime MCP server is fine (it `RegisterAll`s the live Cobra tree), but the static manifest and the scorecard's "MCP: N tools" both undercount for every CLI that hand-builds transcendence commands (which is every CLI — Phase 3 transcendence is always hand-built).
- **Scorer correct?** Partially — the scorecard reads the static manifest and reports "2 tools" though the runtime exposes all; the count is misleading even though MCP dims still scored 7-10.
- **Root cause:** the manifest/`.mcpb` is emitted before hand-authored commands exist and is never reconciled against the built Cobra tree.
- **Cross-API check:** every CLI with novel commands.
- **Frequency:** every API with transcendence features.
- **Fallback if not fixed:** mostly cosmetic at runtime (server walks the tree), but the bundled manifest and the score display are stale for every CLI.
- **Worth a fix?** Low-priority but clean and universal.
- **Inherent or fixable:** fixable.
- **Durable fix:** regenerate `tools-manifest.json` from the built Cobra tree as a post-build step (or have the scorer validate manifest-vs-runtime and flag/regenerate divergence) so the bundled discovery surface matches what the server actually serves.
- **Test:** generate + hand-add a command + rebuild → manifest tool count equals the runtime `tools/list` count (positive).
- **Evidence:** scorecard "Gaps: MCP: 2 tools (0 public, 2 auth-required)" with ~20 registered commands this run.
- **Related prior retros:** None.
- **Step G (case against):** "Runtime exposes everything, so it's cosmetic." Largely true — hence P3, not higher — but it still misreports for every novel-command CLI and the fix (regen from the tree) is cheap and universal.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Basic-auth credential tests fail (base64 substring) | generator | subclass: Basic-auth | none (agent can't touch cliutil) | small | assert by auth shape; Bearer/raw unchanged |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | gosec HIGH/MED in generated client.go/store.go | generator | every API | none (generated files) | small-medium | annotate with rationale; prefer parameterized SQL where feasible |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 3 | Bundled MCP manifest undercounts hand-added commands | scorer | every novel-command API | n/a (cosmetic at runtime) | medium | regen from built Cobra tree post-build |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| S1 | Internal-YAML specs have no auth setup-URL / instructions field, so doctor/auth-setup/auth-status emit generic onboarding ("No setup URL configured", "export X=your-token-here") | Step B: only SimpleFIN with concrete evidence; generic messaging is acceptable for simple API-key APIs, so the gap is the multi-step-auth (claim/OAuth-device/cookie) subclass — too narrow without 3 named APIs. Mitigated this run with a hand `auth setup` override. |
| S2 | Hand-authored live commands that JSON-parse get cryptic "invalid character '<'" on HTML error pages (e.g. unauthenticated request returns an HTML page) | Step G: the parse is in hand-authored code; the durable fix is a SKILL recipe (precheck creds / detect HTML-not-JSON in the RunE skeletons), not a generator change. Borderline per-CLI. |
| S3 | Generated generic sync/export constructors become dead code when a hand-authored same-name command supersedes them (dogfood "unregistered command", Dead Code 4/5) | Step B: only SimpleFIN (nested single-endpoint) with concrete evidence; the subclass (custom-sync APIs) is real but thinly evidenced. Candidate scorer fix (recognize an intentional override) if it recurs. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| export --json ignored global flag | hand `export` honored only --format until fixed | printed-CLI (my hand-code, fixed this run) |
| categorize status polluted stdout JSON | "categorized N" printed to stdout in --json mode | printed-CLI (fixed this run) |
| accounts --start-date relative -> HTTP 400 | generated promoted `accounts` passed `30d` raw; SimpleFIN needs epoch | API-quirk (generator can't infer date format from a bare spec) |
| networth --at dead flag | flag defined but unused until fixed | printed-CLI (my hand-code, fixed this run) |
| 90-day cap warning surfaced from errlist | informational server message | API-quirk (working as intended) |

## Work Units

### WU-1: Fix credential-test template for base64 Basic auth (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Generated credential-precedence tests pass for `Basic {token}` specs.
- **Target:** the credential-test template under `internal/generator/` that emits `internal/cliutil/credentials_test.go`.
- **Acceptance criteria:**
  - positive: a CLI generated from a `Basic {token}` spec passes `go test ./internal/cliutil/`.
  - negative: a `Bearer {token}` spec still passes (no regression).
- **Scope boundary:** only the credential-test assertion strategy; do not change AuthHeader() behavior.
- **Dependencies:** none.
- **Complexity:** small

### WU-2: Make generated client.go/store.go gosec-clean (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** A freshly generated CLI reports 0 HIGH gosec findings in generated files.
- **Target:** `internal/generator/` templates for `internal/client/client.go` (G119) and `internal/store/store.go` (G201).
- **Acceptance criteria:**
  - positive: `gosec ./...` on a fresh CLI shows 0 HIGH in generated files.
  - negative: behavior unchanged (redirect auth still same-host-only; queries still correct).
- **Scope boundary:** remediate-or-annotate only; no functional change to redirect/auth/query logic.
- **Dependencies:** none.
- **Complexity:** small-medium

### WU-3: Reconcile bundled MCP tools-manifest with the built Cobra tree (from F3)
- **Priority:** P3
- **Component:** scorer
- **Goal:** The bundled tools-manifest reflects all runtime tools (including hand-added commands).
- **Target:** the `.mcpb`/tools-manifest emit step (generator post-build) and/or the scorecard MCP-count check.
- **Acceptance criteria:**
  - positive: after adding a novel command and rebuilding, manifest tool count == runtime `tools/list` count.
  - negative: a spec-only CLI with no novel commands is unaffected.
- **Scope boundary:** manifest reconciliation only; runtime registration already works.
- **Dependencies:** none.
- **Complexity:** medium

## Anti-patterns
- None observed in the machine's defaults beyond the findings above.

## What the Printing Press Got Right
- The generated config already had a `SimplefinAccessUrl` field + env override wired from the single-env-var auth enrichment — the access-URL parsing slotted in cleanly via one Load() hook.
- The hand-edit durability story held: ~18 hand-authored files + root.go AddCommand wiring built and survived shipcheck without fighting the generator.
- The generic `resources` table + FTS + `SaveSyncState` gave framework search/sql/doctor/provenance for free over hand-synced data.
- dogfood's live matrix caught all 3 real functional bugs (export --json, categorize stdout, accounts date-400) that unit smoke tests missed.
- The verify-friendly RunE skeleton (dryRunOK, missing-mirror guard, boundCtx) made every command pass dry-run probes on the first try.
