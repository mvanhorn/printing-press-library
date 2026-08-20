# RideWithGPS CLI — Phase 5 Live Dogfood Acceptance

**Level:** Full Dogfood (live, against the user's real RideWithGPS account)
**Result:** PASS — 136/136 tests passed, 0 failed (status: pass)
**Auth:** RIDEWITHGPS_API_KEY + RIDEWITHGPS_AUTH_TOKEN (Basic Auth, read-only; destructive endpoints skipped by default)

PII note: route IDs, the authenticated user's identity, and a private route's privacy_code were observed live; none are recorded here.

## Outcome
- doctor: auth configured, API reachable, both env vars present.
- users/current, routes list (20), trips list (20): live reads succeed.
- All 7 novel features exercised; all generated endpoint reads pass.
- Destructive endpoints (route/trip delete, event/POI create/update/delete) skipped (read-only run).

## Failures found + fixed (2 dogfood loops, 131→135→136)
1. **`tail` polled the wrong path (real bug).** It built `/` + resource = `/routes` (the web HTML page) instead of `/api/v1/routes.json`, yielding "invalid JSON". Fixed: tail now resolves the API path via `syncResourcePath()` (same mapping `sync` uses). This is a **generator bug** for `.json`-path APIs — flagged for retro.
2. **`sync-json` required `--since`** with no dogfood fixture. Added `pp:happy-args=--since=1970-01-01` (initial-sync sentinel).
3. **POI list/get → HTTP 403 "reserved for organization accounts".** Genuine API tier limitation (personal accounts can't access POI endpoints; the CLI surfaces a clear 403 hint). Tagged `pp:requires-tier=org` so dogfood skips them honestly; documented as a known limitation.
4. **`tail` error-path.** Added upfront resource validation (unknown resource → exit 2) so an invalid argument fails fast instead of polling a 404 forever.

## Generated-file hand-edits (reprint-review items)
These edits live in generated files and will surface as TEMPLATED-WITH-ADDITIONS on a future reprint:
- `internal/cli/tail.go` — API-path resolution + resource validation + happy-args (the real fix).
- `internal/cli/promoted_sync-json.go` — `--since` happy-args.
- `internal/cli/points_of_interest_list.go`, `points_of_interest_get-point-of-interest.go` — `pp:requires-tier=org`.

## Known limitation (documented)
- **Points-of-interest endpoints require an organization account.** Personal accounts receive HTTP 403 with a clear hint. Not a CLI defect — an API access-tier restriction.

## Gate: PASS
Full dogfood 136/136. Post-fix shipcheck re-ran 6/6 PASS, scorecard 92/100. Proceeding to promote.
