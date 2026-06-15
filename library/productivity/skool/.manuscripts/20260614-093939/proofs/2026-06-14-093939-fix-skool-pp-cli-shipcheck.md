# skool CLI — Shipcheck Report

## Shipcheck umbrella: PASS (6/6 legs)
| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS (exit 0, 24s) | runtime + auto-fix |
| validate-narrative | PASS | after fixing `course publish` dry-run ordering |
| dogfood | PASS (WARN: 1 structural) | path 4/4, 0 dead flags/funcs, 10/10 examples, novel 5/5, MCP mirror PASS |
| workflow-verify | PASS | no manifest → workflow-pass |
| verify-skill | PASS | flag-names, flag-commands, positional-args, unknown-command all clean |
| scorecard | PASS | Total 88→91/100 Grade A (post-polish) |

## Scorecard (post-polish): 91/100 Grade A
- Local Cache 10/10, Vision 9/10, Agent Workflow 9/10, Path Validity 10/10, Auth Protocol 10/10, Data Pipeline 10/10, Sync Correctness 10/10, Dead Code 5/5, MCP Desc Quality 10/10 (polish).
- Lower (structural, small-API calibration): Breadth 5/10, Cache Freshness 5/10, Workflows 6/10, Insight 4/10, Type Fidelity 3/5.

## Fixes applied this phase
1. `course publish` + `post draft` + `session status`: moved dry-run/verify short-circuit before filesystem/network so verify probes exit 0.
2. Polish: Apify client now uses `cliutil.AdaptiveLimiter` (rate-limit finding cleared); 6 thin MCP tool descriptions rewritten (MCP Desc Quality 0→10/10); `webhook list` Short enriched; gofmt across tree; all tests green.

## Behavioral correctness
- All 25+ commands resolve `--help` and `--dry-run` (exit 0).
- Live novel-feature sampling failed only with actionable auth/config errors (`no group id`, `no active session and no credentials`) — correct behavior with no credentials, not wrong output.

## Known gaps (non-blocking)
- `sync` is a no-op for Skool: native list endpoints require `session_id`/`group_id` query context the flat-pagination sync engine can't inject. The CLI states this at runtime and routes users to single-fetch commands. Transcendence commands use live calls + local snapshot tables instead.
- `chat response-time` dropped (no message timestamps in API); `group set-description` omitted (no confirmed Apify action).
- Live API testing skipped (no credentials available).

## Retro candidates (filed for the Press)
- `cli-printing-press verify` builds `./cmd` rather than the main-package subdir, failing for CLIs with multiple `cmd/<binary>/` subdirs.
- `mcp-sync` derives a spurious slug from `info.title` ("SkoolAPI.com" → "skoolapi-com") that disagrees with the directory slug.

## Verdict: ship
All ship-threshold conditions met; no functional bugs in shipping-scope features. Remaining gaps are documented and structural.
