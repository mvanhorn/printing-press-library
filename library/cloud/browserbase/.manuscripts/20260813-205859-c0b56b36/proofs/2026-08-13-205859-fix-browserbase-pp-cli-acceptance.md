# Browserbase CLI — Phase 5 Acceptance Report

## Level: Full Dogfood (live, real API)

**Gate: PASS** — 238/238 tests passed (100% pass rate), shipcheck 7/7 legs PASS.

## Live test summary
- **Matrix:** 238 tests across every leaf subcommand: help, happy-path, JSON fidelity, error paths, output-mode fidelity.
- **Auth:** api_key (BROWSERBASE_API_KEY, live `bb_live_...` key provided by user).
- **Real resources exercised:** created a real agent + 2 agent runs, created + released real browser sessions, fetched real pages (example.com → markdown), synced 22 sessions + 1 project, captured usage snapshot.
- **Acceptance marker:** `$PROOFS/phase5-acceptance.json` → `{"status":"pass","matrix_size":238,"tests_passed":238}`.

## What was verified live
| Area | Result |
|------|--------|
| doctor | PASS — auth configured, API reachable |
| projects list/get | PASS — real project returned |
| sessions create/list/get/stop | PASS — real session lifecycle, REQUEST_RELEASE verified |
| sessions orphans | PASS — scanned real synced sessions |
| sessions run | PASS — create → hold → release lifecycle (dogfood-curtailed to 2s) |
| fetch (raw/markdown/json) | PASS — example.com returned markdown, statusCode 200 |
| fetch batch | PASS — paced fetch with checkpoint |
| web history + reemit | PASS — 5 fetch entries captured, cached body re-emitted |
| projects digest | PASS — 22 sessions aggregated by day |
| usage trend | PASS — self-populating snapshot captured |
| agents create/run/list/get | PASS — real agent + 2 runs created, run completed with result |
| agents runs diff | PASS — real runs diffed (result_changed: true) |
| sync sessions/projects | PASS — 23 records synced |
| search web | PASS — real web search |
| downloads/contexts/extensions/certificates/functions | PASS — list/get happy paths |

## Fixes applied during dogfood (CLI fixes)
1. `sessions run` — hold governed by session lifetime not root 60s timeout; dogfood env curtails hold to 2s; API range validation on --timeout.
2. `agents runs` — added `pp:happy-args` annotations on flat generated run commands so the matrix uses a real fixture run ID (spec placeholder UUIDs 404'd).
3. `feedback` framework command — added Example + `pp:no-error-path-probe` via novel hook (accepts free-form text, no error path).
4. `usage trend` — self-populating baseline: captures live project usage snapshot on first run (usage is POST-only, not syncable via generic sync).
5. `web history` — reads typed fetch table (synced_at/status_code); verified write-through capture works.
6. `agents runs diff` example in research.json → real run IDs.

## Printing Press issues (for retro)
1. **Generator template gap:** the `feedback` framework command's parent lacks an Examples section, failing dogfood `help` check — had to patch via novel hook.
2. **Generator synthesizes placeholder UUIDs** (`550e8400-...`) for `format: uuid` path params in generated examples; dogfood happy-path then 404s on real APIs. `x-pp-example` at operation level did not influence happy-path arg synthesis; `pp:happy-args` annotation was required.
3. **POST-only resources (usage/fetch/websearch) are in the sync switch but have no sync list path** — `sync --resources usage` errors. Typed-table writers exist but nothing populates them via sync; write-through or self-populating patterns are needed.

## BLOCKED_FIXTURE
- None remaining — created real fixtures (agent, runs, sessions) to exercise run-scoped commands.

## PII note
- Real project id and run ids appear in this report (structural keys, not PII). No user-identifying data (names, emails) present.
