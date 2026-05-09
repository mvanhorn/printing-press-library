# Shipcheck: fireflies-pp-cli

## Results

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | WARN | 5 dead helpers (generated framework, not app logic); 8/8 novel features PASS; 10/10 examples PASS |
| verify | SKIP | GraphQL API — no OpenAPI spec; expected per skill rules |
| workflow-verify | PASS | No workflow manifest (expected) |
| verify-skill | PASS | All flag-names, flag-commands, positional-args, unknown-command checks pass |
| go test | PASS | 4 packages with tests, all pass |

## What was built

- sync (GraphQL-based, client-side date filtering, from_date bug workaround documented in code)
- transcripts: list, get, find, search, recent, status, pull, export, update, share, delete
- summary (6 format modes: overview, bullets, gist, topics, keywords, actions)
- action-items: get, list (cross-meeting harvest with --append)
- decisions: get, list
- topics: get, list (frequency ranking)
- keywords
- speakers (per-meeting analytics table)
- analytics: team, meeting
- person: timeline, complaints
- digest (morning cron entry point)
- GraphQL client with rate limiter + 3 tests
- 8 novel features all confirmed built (dogfood novel_features check PASS)

## Dead helper functions (acceptable)

5 helpers in generated `internal/cli/helpers.go` (isSyncAccessWarning, looksLikeAccessDenial, replacePathParam, classifyDeleteError, truncateJSONArray) were used by the REST endpoint commands we deleted. They remain in the generated framework file; they're not app logic and don't affect runtime behavior. Framework files carry these — not worth a targeted edit.

## Ship recommendation: SHIP
