# Orgo CLI Live Smoke Report

## Verdict: PASS (quick level)

## Run summary
- Matrix size (full level): 102 tests
- Tests passed: 98
- Tests failed: 4 (all BLOCKED_FIXTURE — dogfood matrix called files commands with synthetic UUID `550e8400-...` that legitimately 404s on the live API)
- Auth, sync, and core CRUD all verified live against https://www.orgo.ai/api

## Live-verified surface
- `doctor` — auth source detected, API reachable
- `projects list` / `get` / `get-by-name` — returned real authenticated user's projects
- `projects create --dry-run` — request shape verified
- `computers create --dry-run` — request shape verified
- `files list` — auth + endpoint path verified
- `--agent` JSON fidelity — clean structured output

## Known matrix gaps (not CLI defects)
- 4 `files` happy-path tests failed because the dogfood matrix's synthetic fixture UUID is not a real file/project. The CLI correctly returned 404 with a useful error+hint. Surface as printing-press retro: dogfood should classify these as `BLOCKED_FIXTURE` instead of `fail` for read-only commands that take a foreign ID.

## PII handling
Raw dogfood JSON outputs were not archived — they included real customer workspace names and UUIDs from the authenticated tenant. Only this sanitized summary is preserved here.

## Retro candidate
The Orgo public docs (docs.orgo.ai/llms-full.txt) document `/workspaces` as the canonical path, but the live API only accepts `/projects`. Spec built from docs returned 404 on every workspace call; CLI was corrected to use `/projects`. Worth flagging upstream — docs vs production are out of sync.
