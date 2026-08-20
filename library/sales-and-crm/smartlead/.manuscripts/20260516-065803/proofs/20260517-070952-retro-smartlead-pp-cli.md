# Printing Press Retro: SmartLead

## Session Stats
- API: smartlead
- Spec source: reverse-engineered OpenAPI 3.0.3 (SmartLead publishes no official spec)
- Scorecard: 84/100 (A) — after this session's `x-mcp` transport enhancement; 80/100 at end of original run
- Verify pass rate: 100% (63/63)
- Fix loops: 0 (this was a post-ship enhancement session, not a fresh print)
- Manual code edits: 1 (copied regenerated `cmd/<cli>-mcp/main.go` after a spec edit)
- Features built from scratch: 0 this session (5 commands + 6 novel features built in prior sessions)

This retro comes from a post-ship enhancement session: the SmartLead CLI was
already published-grade; the goal was to lift scorecard dimensions toward the
ceiling. Two systemic Printing Press issues surfaced — both blocking, both
generalizable across every OpenAPI-derived CLI.

## Findings

### 1. No `x-cache` OpenAPI extension — cache_freshness permanently capped for OpenAPI CLIs (assumption mismatch)
- **What happened:** `cache_freshness` scores 5/10 for the SmartLead CLI. The missing 5 points come from the auto-refresh capability (`internal/cli/auto_refresh.go` + `internal/cliutil/freshness.go`), which the generator emits only when `g.VisionSet.Store && g.VisionSet.Sync && g.Spec.Cache.Enabled`. `Cache.Enabled` can only be set through the internal-YAML spec format's top-level `cache:` block. The OpenAPI parser has no equivalent extension, so no OpenAPI-derived CLI can ever turn the capability on.
- **Scorer correct?** Yes. The scorecard correctly detects the auto-refresh files are absent and docks 5 points. The CLI genuinely lacks the capability — but lacks it because the toolchain offers no way to opt in, not because the agent declined to.
- **Root cause:** `internal/openapi/parser.go` — extension constants exist for `x-mcp`, `x-tier`, `x-auth-*`, `x-resource-id`, `x-critical`, `x-api-name`, `x-display-name`, `x-website`, `x-proxy-routes`, `x-origin`, `x-providerName`, `x-speakeasy-example`. There is no `x-cache`. The internal-YAML format has a `cache:` block (`spec.CacheConfig`); OpenAPI specs cannot reach it.
- **Cross-API check:** Recurs on every OpenAPI-derived CLI that has a local store. Confirmed against the local library:
  - `smartlead` (openapi3, has store): cache_freshness 5/10 — capped
  - `dataforseo` (OpenAPI-derived, has store): cache_freshness 5/10 — capped
  - `google-search-console` (OpenAPI-derived, has store): cache_freshness 0/10 — capped
  - (`dmachoice` has no store, so the dimension is N/A — not a counter-example, just not applicable)
- **Frequency:** every OpenAPI-derived CLI with a local store.
- **Fallback if the Printing Press doesn't fix it:** None. There is no agent workaround — the agent cannot hand-author `cliutil/freshness.go` because `internal/cliutil/` is a generator-reserved namespace (regen overwrites it). The dimension is simply unreachable.
- **Worth a Printing Press fix?** Yes. `x-mcp` already set the exact precedent: it mirrors the internal-YAML `mcp:` block field-for-field as an OpenAPI extension parsed via `parseTypedExtension[spec.MCPConfig]`. The absence of the parallel `x-cache` is an oversight, not a design decision.
- **Inherent or fixable:** Fixable. Add an `x-cache` extension that mirrors the internal-YAML `cache:` block (`spec.CacheConfig`) field-for-field.
- **Durable fix:** In `internal/openapi/parser.go`, add `extensionCache = "x-cache"` and parse it with `parseTypedExtension[spec.CacheConfig](doc, extensionCache)` into `APISpec.Cache`, accepting it at the OpenAPI root or under `info` (same dual placement as `x-mcp`). Run it through the existing `validateCacheShare` validation. Document it in `docs/SPEC-EXTENSIONS.md` alongside the `x-mcp` entry.
- **Test:** Positive — an OpenAPI spec with `x-cache: { enabled: true }` plus a store-backed sync surface generates `internal/cli/auto_refresh.go` and `internal/cliutil/freshness.go`, and scores cache_freshness 10/10. Negative — an OpenAPI spec with no `x-cache` block keeps today's behavior (no auto-refresh files, no error).
- **Evidence:** Diagnosing why SmartLead's cache_freshness sat at 5/10. Traced `scoreCacheFreshness` in `internal/pipeline/scorecard.go`, then the emission gate in `internal/generator/generator.go`, then confirmed the OpenAPI parser exposes no `x-cache` against `docs/SPEC-EXTENSIONS.md` and the binary's own strings.
- **Related prior retros:** None.

### 2. `verify` / `shipcheck` mishandle a relative `--dir` argument — false-negative verify FAIL (scorer bug)
- **What happened:** `printing-press verify --dir .` run from inside a CLI directory fails with `go build: exit status 1 / no Go files in <dir>/cmd`. The identical command with an absolute `--dir` path passes 100% (63/63). Because `shipcheck` invokes the `verify` leg with a relative path internally, `shipcheck --dir .` always reports a false verify FAIL even on a perfect CLI — the SmartLead CLI showed shipcheck 5/6 (verify FAIL) with a relative `--dir` and 6/6 with an absolute one.
- **Scorer correct?** No — scorer bug. The CLI builds cleanly (`go build ./...` passes; the absolute-path verify passes 63/63). The failure is entirely in verify's CLI-entry-point discovery.
- **Root cause:** `internal/pipeline/runtime_exec.go` — `findCLICommandDir(dir)` builds candidate paths by joining `dir` with `cmd/<name>`. With a relative `dir` (`.`), the cmd-entry-point discovery resolves to the bare `cmd/` directory (which has no Go files — the mains live in `cmd/<cli>/` and `cmd/<cli>-mcp/`), so `go build` is handed a directory with no package.
- **Cross-API check:** Universal — affects every printed CLI, not an API subclass. The `cmd/` layout (`cmd/<cli>-pp-cli/`, `cmd/<cli>-pp-mcp/`) is identical across every CLI the Printing Press emits, so the relative-path resolution fails the same way for all of them.
- **Frequency:** every CLI, whenever verify or shipcheck is invoked with a relative `--dir`.
- **Fallback if the Printing Press doesn't fix it:** Agent must know to always pass an absolute `--dir`. Fragile — the natural invocation from inside a CLI directory is `--dir .`, and shipcheck's *internal* verify call uses a relative path the agent cannot override, so `shipcheck --dir .` is unconditionally broken.
- **Worth a Printing Press fix?** Yes. A verification tool that emits a false FAIL on a correct CLI is worse than no check — it trains agents to distrust or ignore the verdict.
- **Inherent or fixable:** Fixable. One-line class of fix.
- **Durable fix:** Resolve `--dir` to an absolute path (`filepath.Abs`) at the verify/shipcheck command entry point, before `findCLICommandDir` runs. Belt-and-suspenders: also `filepath.Abs` inside `findCLICommandDir` so any caller is covered.
- **Test:** Positive — `verify --dir .` from inside a CLI directory passes identically to `verify --dir <abspath>`. Positive — `shipcheck --dir .` reports verify PASS on a known-good CLI. Negative — a genuinely broken CLI still fails verify regardless of relative vs absolute `--dir`.
- **Evidence:** `shipcheck --dir .` on the SmartLead CLI reported verify FAIL (exit 4); the same CLI passed verify 63/63 with an absolute `--dir`; the failure reproduced identically on an untouched pre-change backup, ruling out the session's edits as the cause.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | `verify`/`shipcheck` false FAIL on relative `--dir` | scorer | every CLI | Low — natural invocation triggers it; shipcheck's internal call is unreachable by the agent | small | none — pure bugfix, hurts nothing |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Add `x-cache` OpenAPI extension | openapi-parser | every OpenAPI CLI with a store | None — no agent workaround exists | medium | opt-in; specs without `x-cache` keep current behavior |

### Skip
*None — both Phase 3 candidates cleared Step G.*

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| MCP server slow to bind | MCP server takes ~4s to register 44 tools before serving | iteration-noise — normal startup cost, surfaced only as a too-short test timeout on my end |
| `mcp-mcp` double-suffix on `--name` | `generate --name smartlead-pp-cli` double-appends the suffix | raised-and-known — documented generator behavior; the fix is to pass the bare name, already understood |

## Work Units

### WU-1: Resolve `--dir` to absolute path in verify/shipcheck (from F2)
- **Priority:** P1
- **Component:** scorer
- **Goal:** `verify --dir .` and `shipcheck --dir .` produce the same verdict as the absolute-path invocation.
- **Target:** `internal/pipeline/runtime_exec.go` (`findCLICommandDir`) and the verify/shipcheck command entry points.
- **Acceptance criteria:**
  - positive test: `verify --dir .` from inside a CLI directory passes 63/63 on a known-good CLI, matching `verify --dir <abspath>`.
  - positive test: `shipcheck --dir .` reports verify PASS (6/6) on a known-good CLI.
  - negative test: a genuinely broken CLI still fails verify regardless of relative vs absolute `--dir`.
- **Scope boundary:** Does not change verify's actual command-testing logic — only the path resolution feeding `go build`.
- **Dependencies:** None.
- **Complexity:** small

### WU-2: Add `x-cache` OpenAPI extension mirroring the internal-YAML `cache:` block (from F1)
- **Priority:** P2
- **Component:** openapi-parser
- **Goal:** OpenAPI specs can opt into the auto-refresh / cache-freshness capability, unblocking cache_freshness 10/10 for OpenAPI-derived CLIs.
- **Target:** `internal/openapi/parser.go` (extension constant + `parseTypedExtension` call into `APISpec.Cache`); `docs/SPEC-EXTENSIONS.md` (new entry).
- **Acceptance criteria:**
  - positive test: an OpenAPI spec with `x-cache: { enabled: true }` and a store-backed sync surface emits `internal/cli/auto_refresh.go` + `internal/cliutil/freshness.go` and scores cache_freshness 10/10.
  - positive test: `x-cache` is accepted at both the OpenAPI root and under `info` (same dual placement as `x-mcp`); malformed values are rejected by `validateCacheShare`.
  - negative test: an OpenAPI spec with no `x-cache` block keeps today's behavior (no auto-refresh files, no error).
- **Scope boundary:** Mirrors the existing `spec.CacheConfig` field-for-field; does not add new cache config fields or change the internal-YAML `cache:` block.
- **Dependencies:** None.
- **Complexity:** medium

## Anti-patterns
- None observed this session. The CLI was already in good shape; the friction was entirely in the toolchain, not in the generated code.

## What the Printing Press Got Right
- `regen-merge` classification was precise: it correctly flagged the one file genuinely affected by the `x-mcp` spec change (`cmd/<cli>-mcp/main.go`) as TEMPLATED-VALUE-DRIFT and left the hand-authored novel features and patches untouched — making a surgical one-file update safe without a full merge.
- The `x-mcp` extension worked exactly as documented: adding `transport: [stdio, http]` to the OpenAPI spec lifted `mcp_remote_transport` 5→10 with no other changes, and the regenerated MCP server served correctly over both stdio and streamable HTTP.
- The scorecard's `cache_freshness` static check is well-designed — it scores the *capability* (schema gate, doctor cache report, auto-refresh) rather than live API behavior, which made the gap diagnosable without credentials.
