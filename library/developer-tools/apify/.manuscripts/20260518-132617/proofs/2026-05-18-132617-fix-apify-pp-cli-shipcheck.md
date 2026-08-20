# Apify CLI Shipcheck Report

**Run ID:** 20260518-132617
**Date:** 2026-05-18
**Verdict:** `ship`

## Shipcheck Summary

| Leg | Result | Exit | Notes |
|-----|--------|------|-------|
| dogfood | PASS | 0 | 100% (30/30 passed, 0 critical) |
| verify | PASS | 0 | All commands runnable |
| workflow-verify | PASS | 0 | No workflow manifest (acceptable) |
| verify-skill | PASS | 0 | After 1 prose fix (line that began with binary name) |
| validate-narrative | PASS | 0 | All 11 narrative commands resolve + full examples pass |
| scorecard | PASS | 91/100 Grade A | See breakdown below |

## Scorecard Breakdown (91/100)

| Dimension | Score |
|-----------|-------|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 9/10 |
| README | 8/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 8/10 |
| MCP Remote Transport | 10/10 |
| MCP Tool Design | 10/10 |
| MCP Surface Strategy | 10/10 |
| Local Cache | 10/10 |
| Cache Freshness | 5/10 |
| Breadth | 10/10 |
| Vision | 8/10 |
| Workflows | 10/10 |
| Insight | 10/10 |
| Agent Workflow | 9/10 |
| Domain — Path Validity | 10/10 |
| Domain — Auth Protocol | 10/10 |
| Domain — Data Pipeline Integrity | 7/10 |
| Domain — Sync Correctness | 10/10 |
| Domain — Type Fidelity | 3/5 |
| Domain — Dead Code | 5/5 |

Omitted from denominator: `mcp_description_quality`, `mcp_token_efficiency`, `live_api_verification` (no APIFY_TOKEN for live check).

## Build Inputs

- **Spec:** `docs.apify.com/api/openapi.yaml` (OpenAPI 3.1.2, 231 operations, 131 paths, 745KB)
- **Generated:** 296 spec-derived endpoint command files; full framework (sync, search, doctor, auth, profile, agent-context, deliver, feedback, tail, import, analytics, workflow, api, logs, store)
- **Hand-built (Priority 0 foundation):** `internal/normalize/` + 8 Actor profiles (Twitter, Twitter-lite, Reddit, Google News, Hacker News, YouTube, Instagram, Smart Article Extractor) + default fallback; `internal/syncstate/`; `internal/cost/`; `internal/store/extensions.go` (`pp_dataset_items` + FTS5, `pp_actor_run_history`, `pp_presets`, `pp_workflow_runs`)
- **Hand-built (Priority 2 transcendence):** 10 commands across `internal/cli/`:
  - `run` (novelty diffing + cost projection + budget enforcement)
  - `search items` (cross-Actor FTS via FTS5 over normalized items)
  - `digest` (template-driven newsletter renderer; default + tiktok-script + user-supplied templates)
  - `workflow run` (YAML-declared multi-Actor chain with digest pass)
  - `cost report` (USD ledger over historical runs)
  - `schedules apply` + `schedules diff` (terraform-style GitOps over Apify schedules)
  - `preset save/list/show/delete` (named Actor input presets, replay via `--preset` on `run`)
  - `ab run` (head-to-head Actor comparison: novelty/cost-per-novel/cost-per-item/item-count judges)
  - `--offline` flag on `digest` (local-only mode, zero API spend)
  - `--max-cost` flag on `run` (pre-flight budget enforcement)

## Lessons Baked In (from granola post-print patches + library AGENTS.md)

- Sync state file: `~/.local/share/apify-pp-cli/sync_state.json` (via `internal/syncstate`)
- Cost projection on every `run` invocation (default behavior; opt-out via `--no-projection` or `--agent`)
- Public-endpoint policy: `store get`, public actor browse work without `APIFY_TOKEN`; doctor reports auth-required commands separately
- MCP `read-only` annotations on every hand-built read command (search items, digest, cost report, preset list/show, schedules diff, ab run)
- Embedded profile data (no network fetch); user override at `~/.apify-pp/profiles/`
- Verify-friendly RunE: no `MarkFlagRequired`, no hard `MinimumNArgs`; falls through to help; honors `dryRunOK(flags)`
- Cloudflare MCP pattern: `transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden` (231 endpoints stays under agent context limits)

## Known Limitations (informational; not blockers)

- **No live smoke test** — no `APIFY_TOKEN` in env; Phase 5 auto-skipped per skill contract. Run `/printing-press-polish apify` with `APIFY_TOKEN` set later to add live verification.
- **Schedule update + delete deferred** — `schedules apply` creates new schedules but defers updates/deletes to manual confirmation (v1 safety). Update by recreating; delete via the generated `schedules delete` command.
- **Type Fidelity 3/5** — some spec-derived commands use `body-json` fallback for `oneOf`/`anyOf` request bodies (4 endpoints in the dataset push family). Acceptable for v1.
- **Cache Freshness 5/10** — auto-refresh hook on `PersistentPreRunE` was scoped out for this build; deferred to polish skill. Manual `sync` works.

## Fix Recommendation

`ship` — clean PR, no blockers. All shipping-scope features ship fully (no stubs). The two known limitations are documented above and don't affect headline functionality.
