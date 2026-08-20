# Phase 4.95 — Native Code Review Findings

**Verdict:** PASS (no autofixes applied; no blocking findings)
**Reviewer:** general-purpose agent (Phase 4.95 native review pattern; /review skill is GH-PR oriented and didn't fit local-workdir scope)
**Scope:** in-scope files listed in [SKILL Phase 4.95](../../../../../../.claude/skills/printing-press/SKILL.md). Out-of-scope (`internal/cliutil/`, `internal/mcp/cobratree/`) — no findings routed.

## In-scope findings

| File:line | Sev | Category | Description | Outcome |
|---|---|---|---|---|
| `edgar_helpers.go:103-137` | warning | security | `fetchAbsoluteRaw` bypasses `cliutil.AdaptiveLimiter` — direct `c.HTTPClient.Do()` calls for SEC archive/index.json/Form4 XML do not consult the framework throttle. Sequential execution makes burst-violation unlikely in practice but the limiter would be unaware of dozens of intermediate fetches. | **Surfaced as known v1 limitation** (already documented in `phase5-acceptance.json` notes); v2-fix candidate. |
| `edgar_helpers.go:129` | warning | security | `io.ReadAll(resp.Body)` with no size cap on SEC bodies that can be ~10MB. SEC itself won't trigger; transitive risk if a hostile mirror were ever in play. | Surfaced as info; not autofixed. |
| `edgar_helpers.go:321`, `companies_lookup.go:63` | info | correctness | `db.Query(...)` (no `QueryContext`) drops `cmd.Context()` cancellation. Adding `QueryContext` touches generator-managed `store.go`. | **Retro-candidate** (routes to generator). |
| `config.go:90-92` | info | style | Redundant `if c.CompanyPpContactEmail == ""` after `if token == ""` (token derived from that field). Harmless. | Not fixed (cosmetic). |
| `edgar_helpers.go:174` | info | correctness | `strconv.Atoi(strings.TrimLeft(cik, "0"))` on all-zero CIK produces confusing error. Implausible (no real CIK is zero). | Not fixed. |

## Per-file review summary

- `edgar_helpers.go` (~700 lines) — solid. CIK/accession normalization strict; XML unmarshal uses encoding/xml (no XXE by default); SQL placeholders parameterized; v1.1 prefer-later heuristic boundary-safe (every non-chosen path emits `boundary_unverifiable`); `fetchForm4XML` tryParse correctly rejects HTML wrappers + re-validates fallback XML.
- `edgar_schema.go` (460 lines) — all `ExecContext`/`QueryContext` parameterized; `LIMIT %d` uses `int` (safe); `IN (...)` placeholder count derived per-arg; FTS5 MATCH bound as `?`.
- `primary_sources.go`, `insider_summary.go`, `insider_followthrough.go`, `sections.go`, `eightk_items.go`, `fts.go`, `xbrl_pivot.go`, `since.go`, `ownership_crosses.go`, `governance_flags.go`, `accession.go`, `filings_top.go`, `edgar_output.go`, `companies_lookup.go`, `companies_submissions.go`, `filings_get.go` — clean on all checks. `mcp:read-only` annotations present, verify-friendly RunE shape (no `MinimumNArgs`/`MarkFlagRequired`), `dryRunOK` short-circuit before IO, structured `{"error":...,"reason":...}` on parse failures.
- `config.go` — Phase-5 `email` placeholder patch correct.
- `Form4SkipReport.Count` and `len(Entries)` invariant verified (incremented in same branches at lines 211-216 / 221-226). Stderr WARN does NOT echo email or UA contents — only ticker, CIK, counts.

## Pre-publish notes

1. **Form 4 loud-skip** correct and well-instrumented (in-band JSON + stderr WARN).
2. **`extractSections` v1.1** boundary-safe; never returns text from unverified boundary.
3. **No XXE, no path traversal, no SQL injection** in scope.
4. **Secret hygiene:** email lands in outbound User-Agent only (per SEC fair-access). Never logged, never in error messages, never in JSON output or cache files.
5. **Retro candidates for v2:** (a) add `Store.QueryContext` and migrate two hand-built `db.Query` sites; (b) plumb rate-limiter through `fetchAbsoluteRaw`.
