# axs-pp-cli — Shipcheck Report

## Shipcheck umbrella: PASS (6/6 legs)
| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS — **90/100, Grade A** |

Notable scorecard dims: Doctor 10, Agent Native 10, MCP Quality 10, MCP Remote Transport 10, Local Cache 10, Vision 10, Workflows 10, Auth Protocol 10, Data Pipeline 10, Sync Correctness 10. Lower: Insight 4/10, MCP Desc Quality 5/10, Cache Freshness 5/10, Path Validity 7/10 — polish-gap territory, no blockers.

## Live verification (what's testable without credentials)
- `devicetypes list --json` → **live 200, real data** (public endpoint). `--select name` field narrowing verified.
- `doctor` → **API reachable**; correctly reports missing `SRAM_AXS_TOKEN`.
- Auth-gated commands (bikes/components/garage/firmware-check/battery/wear) → **clean, actionable HTTP 401** with auth hint. Correct unauthenticated behavior, not defects.
- `since` with no mirror → prints sync hint, empty result (correct).
- `auth login` with no creds → usage + exit 2 (correct).
- All novel commands `--dry-run` → exit 0.

## Phase 5 (live dogfood): SKIPPED — auth_required_no_credential
The dominant surface requires a SRAM bearer token (Auth0 password-realm), unavailable this run. Full authenticated live dogfood is the user's follow-up: `axs-pp-cli auth login` (or `auth set-token` / `SRAM_AXS_TOKEN`), then `axs-pp-cli sync` and re-run the novel commands. Marker: `proofs/phase5-skip.json`.

## Behavioral-correctness caveat (honest gap)
The 4 live-API novel commands (firmware-check, battery, garage, wear) join/aggregate on field names inferred from the official web client bundle (firmware_version, battery_level, model/bike refs, total_distance, shift_count), not confirmed against authenticated responses. Helpers try multiple key spellings to absorb naming drift, but the joins are unverified against real authed data. This is the single known gap, documented in the README `## Known Gaps`.

## Verdict: ship-with-gaps
Shipcheck PASS (90/100, 6/6 legs), public surface live-verified, auth surface correct. The one gap (unverified authed field-name joins) requires a SRAM token unavailable in-session — genuinely cannot be closed here — and is documented in README `## Known Gaps`. Meets both ship-with-gaps conditions (a) refactor/access not available in-session and (b) documented.
