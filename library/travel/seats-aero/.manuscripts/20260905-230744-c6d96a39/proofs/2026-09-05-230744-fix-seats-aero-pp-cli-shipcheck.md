# seats-aero reprint — Phase 4 shipcheck (2026-09-06)

Run `20260905-230744-c6d96a39`, press 4.31.7, spec `research/seats-aero-openapi-2026-09.yaml` (OpenAPI 3.0.3, 7 endpoints). Every run executed with `XDG_*_HOME` → `pipeline/xdg/*` and `SEATS_AERO_NO_AUTO_REFRESH=1` (see build log: live-store incident).

## Runs

| # | Invocation | Verdict | Notes |
|---|---|---|---|
| 1 | `shipcheck --dir … --spec … --research-dir …` | HOLD (6/7) | verify `Mode: mock`; scorecard hold on `live_api_verification`; dogfood synced README/SKILL/manifest from `novel_features_built` (5/5) |
| 2 | + `--env-var SEATS_AERO_API_KEY` | HOLD (6/7) | verify STILL mock — `--env-var` does not enable live mode (retro candidate) |
| standalone | `verify --api-key "$SEATS_AERO_API_KEY" --write-manifest` | PASS | `Mode: live`, 43/43, 0 critical |
| 3 | `shipcheck … --api-key "$SEATS_AERO_API_KEY"` | **PASS (7/7)** | canonical order; key value absent from logs/manifest (grep-checked) |

## Leg results (run 3)

| Leg | Result | Detail |
|---|---|---|
| verify | PASS | live, 43/43 commands, 100 %, 0 critical (`--fix` loop: nothing to fix) |
| validate-narrative | PASS | 10 narrative commands resolved; full examples pass under `PRINTING_PRESS_VERIFY=1` |
| dogfood | PASS (WARN) | novel 5/5 survived; MCP surface mirrors Cobra tree; examples 10/10; 1 dead generated helper `handleBinaryResponseDelivery` (generator-owned → retro) ; data pipeline PARTIAL (search uses generic Search/direct SQL) |
| workflow-verify | PASS | `workflow-pass` |
| apify-audit | PASS | n/a |
| verify-skill | PASS | SKILL.md flags/paths all resolve |
| scorecard | PASS | **93/100 Grade A**; Live API Verification 10/10 |

Scorecard dimensions below 10: Terminal UX 9, MCP Token Efficiency 7, MCP Tool Design 7, Breadth 9, Agent Workflow 9, Data Pipeline Integrity 7, Dead Code 4/5. Prior CLI (3.10.0 print) scored 84/100 with four MCP dimensions unscored.

## Before / after

| Metric | Prior (2026.6.1) | Reprint |
|---|---|---|
| Endpoints | 4 | 7 |
| Source enum | 22 | 26 |
| Scorecard | 84 (A, 3 unverified) | 93 (A, 0 unverified) |
| verify pass rate | n/a | 100 % live |
| MCP | 4 tools, stdio | 7 endpoint tools + command mirror, stdio+http, 3 intents, bound envelope |
| Novel features | 3 (all thin endpoint mirrors) | 5 hand-coded local/quota-aware commands |

## Blockers found and fixed during Phase 3/4
- `availability_all` `SELECT *` UNION broke on upgraded stores (legacy 86-col `availability`) → explicit 78-column view + `DROP VIEW IF EXISTS`.
- Novel tests executed `RootCmd()` un-isolated; auto-refresh hook migrated the operator's live store → `isolateNovelTest` helper + XDG isolation on every gate; x-cache reduced to routes-only.
- `/search` would have emitted the reserved `search` command → `x-pp-resource: awards`.

## Known gaps (documented, not blocking)
- Default `sync` includes `availability` but cannot supply the required `source`; users must pass `--resource-param availability:source=<program>` (README troubleshooting entry). Generator issue.
- `live` (POST /live) is commercial-key only → 403 on Pro keys by design; annotated `x-live-dogfood-requires-tier: commercial`.
- Quota report lives in `recheck` (no doctor extension hook).

## Recommendation: **ship**
All ship-threshold conditions hold: shipcheck exit 0, verify PASS live, dogfood wiring clean, workflow-pass, verify-skill 0, scorecard ≥ 65 with every approved feature producing real output (sample output probe 5/5). No known functional bugs in shipping-scope features. Phase 4.8/4.85/4.9/4.95 reviews follow; Phase 5 live dogfood (`--level full`) next.
