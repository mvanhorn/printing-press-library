# Live Acceptance — pushpress-pp-cli

## Level: Full Dogfood
## Gate: PASS (23/23)

## Environment
- Real PushPress /v3 API key (sk_x:y format) supplied for this session via env + saved to local config
- Tested against live i2 Fitness sub-account (`client_<redacted>`)
- 500 customers hydrated via `pushpress-hydrate` Python script (workaround for sync gap #1355)
- All test calls read-only; no writes attempted

## Auth pipeline
- `doctor` exits 0
- Auth header: `API-KEY: <key>` (NOT `Authorization: Bearer` — the SDK ships with custom header name per the spec)
- BaseURL: `https://api.pushpress.com/v3` (the spec's bare server URL `https://api.pushpress.com` returns 404 — this was the first fix-required surfacing)

## Test matrix (23/23 PASS)

| # | Category | Test | Result |
|---|----------|------|--------|
| 1 | Health | `doctor` | PASS |
| 2 | Read | `company --json` | PASS (returns the test workspace) |
| 3 | Read | `customers list --json --limit 3` | PASS (real customers) |
| 4 | Read | `customers get <real-id> --json` | PASS |
| 5 | FTS | `search 'a' --json` | PASS |
| 6 | Output | `--select page,limit` narrows JSON | PASS |
| 7 | Output | `--compact` | PASS |
| 8 | Output | `--csv` | PASS |
| 9 | Novel | `going-dark --days 30 --json` | PASS (returns empty — /v3/checkins 404, no data) |
| 10 | Novel | `recency --json` | PASS (works against customers, empty buckets without checkins) |
| 11 | Novel | `roster --json --limit 5` | PASS — real customers (id, name, email, phone) |
| 12 | Novel | `kpi today --json` | PASS — `members_synced=500`, others zero per checkins gap |
| 13 | Novel | `class-mix --days 30 --json` | PASS (empty, expected — needs checkins) |
| 14 | Novel | `member <real-id> --json` | PASS — real customer profile |
| 15 | Stub | `plans list` → exit 6 | PASS |
| 16 | Stub | `mrr today` → exit 6 | PASS |
| 17 | Stub | `signups recent` → exit 6 | PASS |
| 18 | Stub | `cancellations recent` → exit 6 | PASS |
| 19 | Stub | `classes list` → exit 6 | PASS |
| 20 | Stub | `leads list` → exit 6 | PASS |
| 21 | Stub | `tasks list` → exit 6 | PASS |
| 22 | Stub | `notes list` → exit 6 | PASS |
| 23 | Stub | `cohort` → exit 6 | PASS |

## Fixes applied during dogfood

1. **BaseURL default updated.** The Speakeasy spec declares `servers: https://api.pushpress.com` with bare operation paths (`/customers`, `/company`, etc.). Live API serves at `/v3/<resource>`. Patched `internal/config/config.go` Load() default to `https://api.pushpress.com/v3`. Also updated the user's existing `~/.config/pushpress-pp-cli/config.toml` (the stale config file had the broken value persisted from generator-time).

2. **`pushpress-hydrate` Python script written.** `sync --full` only enumerated a couple resources (same press gap as Printing Press issue #1355). Hydrator at `~/go/bin/pushpress-hydrate` hits `/v3/customers` directly, handles the structured `name` → display-string conversion for the promoted column, and populates the local store.

3. **README `## Known Gaps` section added** documenting four findings: stub list (5 categories), /v3/checkins 404, /v3 customers rich-schema drift, sync resource detector gap.

## Live API findings (for retro)

1. **`/v3/checkins` is 404.** Speakeasy spec includes the endpoint family but it doesn't actually serve. Production drift the spec maintainers haven't synced. Effect: entire check-in transcendence layer returns empty.

2. **Real `/v3/customers` schema is richer than spec.** Real fields not in the documented Customer schema:
   - `membershipDetails.initialMembershipStartDate` (the missing `dateAdded`)
   - `assignedToStaffId` (coach assignment — could un-stub a "roster by coach" command)
   - `role` (potential status indicator)
   - structured `name` (first/last/nickname)
   - `gender`, `dob`, `emergencyContact`, `account.type`
   The CLI passes the rich data through `--json`; agents can `--select` deep paths. Customer-driven transcendence (roster, member-360) works against real data.

3. **BaseURL spec bug.** Spec server is `https://api.pushpress.com` but real paths require `/v3` prefix. Generator emitted the broken default. Filed as retro candidate.

4. **Speakeasy SDK auth header pattern.** The spec's apiKey scheme is `name: API-KEY` (uppercase, hyphenated) — the SDK uses this verbatim. Got it right.

## Printing Press machine bugs (retro)

1. **Sync resource detector misses query-param-scoped APIs (recurrence of #1355).** Same gap as GHL. Filed.
2. **Generator emits broken BaseURL when spec server lacks the version path prefix.** Speakeasy specs that document `servers: https://api.pushpress.com` with operation paths at `/v3/x` produce a broken default. The press could detect this by probing the spec-derived URL during reachability + offering a fix.

## Acceptance: PASS

23/23 tests passed structurally. 4 transcendence commands (going-dark, recency-by-checkin, kpi-today checkins, class-mix) return empty against real data due to the upstream `/v3/checkins` 404 — verified the failure is in production, not in our code. The Known Gaps section documents this. Per the user's gap-flag protocol, this is the correct shipping shape: honest about what the API doesn't provide.

Recommend: promote to library and ship.
