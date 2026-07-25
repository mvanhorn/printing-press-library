# amazon-jobs-pp-cli Shipcheck

## Verdict: SHIP

Shipcheck umbrella: PASS (7/7 legs)
| leg | result |
|-----|--------|
| verify | PASS |
| validate-narrative (--strict --full-examples) | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 84/100 — Grade A
Strong: Output Modes 10, Auth 10 (none), README 10, Doctor 10, Agent Native 10,
Local Cache 10, Workflows 10, Insight 10, MCP Remote Transport 10, Path Validity 10.
Lower (accepted): Cache Freshness 3/10 (job listings are manually synced; pre-read
auto-refresh of a user's intentional snapshot is not valuable — no cache.enabled),
MCP Token Efficiency 7/10, Vision 7, Data Pipeline Integrity 7, Sync Correctness 7,
Type Fidelity 4/5, Dead Code 4/5.

## Live sample probe: 4/5
The one miss — `new sde-seattle` exit 2 "no saved search named sde-seattle" — is
CORRECT behavior (a nonexistent saved search is a usage error with a helpful hint),
not wrong/empty output. It is a fresh-env fixture limitation: `new` requires a prior
`save`. Ship threshold ("no flagship feature returns wrong/empty output") is met.

## MCP surface
tools-manifest.json is the static typed-endpoint manifest (1 tool: postings_search).
The runtime MCP server mirrors the full Cobra tree via cobratree.RegisterAll(RootCmd),
so all 8 hand-written commands (find/get/new/save/searches/stats/skills/sync) are
exposed as agent tools at runtime. Not a gap.

## Behavioral spot-checks (live API, no auth)
- find --city Seattle --json --select -> correct Seattle results, field-narrowed. PASS
- find --intern=false (client-side NULL-safe) -> SDE roles. PASS
- sync aws --max-pages 2 -> 200 jobs stored. PASS
- stats --by city / --by team -> correct GROUP BY counts, true store total. PASS
- skills python --by team -> 33 matches ranked by team. PASS
- save/searches/new -> saved-search created, listed, baseline+delta correct. PASS
- get <id> --plain -> full detail, HTML->text clean. PASS

## Before/after
First generation had sync/search/sql absent (v4.29 emits no framework data layer)
and promoted postings ignored response_path -> entire data layer hand-written against
store.UpsertBatch + store.DB(). Location filter bracket wire keys + result_limit>=1
confirmed. Final: shipcheck 7/7 PASS, scorecard 84 Grade A.
