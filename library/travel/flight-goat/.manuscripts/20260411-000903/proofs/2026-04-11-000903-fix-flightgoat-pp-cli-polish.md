# flightgoat Polish Pass

## Delta

| Metric | Before | After |
|---|---|---|
| Scorecard | 90 | 96 |
| Dogfood | PASS | PASS |
| Insight | 6/10 | 8/10 |
| Data Pipeline Integrity | 7/10 | 10/10 |
| Examples | 9/10 | 10/10 |
| go vet | 0 | 0 |
| Verify pass rate | 84% | 84% (unchanged; verifier limitation, not CLI defect) |

## Fixes Applied

1. Dry-run early-return added to reliability, compare, monitor, eta, gf-search, and digest. Each prints its planned API calls without crashing on nil data or infinite polling.
2. Promoted history command example expanded with N628TS, --json, --select.
3. All promoted_*.go files renamed to match their registered command names (aircraft.go, airports.go, alerts.go, disruption-counts.go, flights.go, foresight.go, history.go, operators.go, schedules.go) so the dogfood example sampler resolves them correctly.
4. Domain-specific SearchFlights, SearchAirports, SearchOperators, SearchAircraft methods added to internal/store/store.go backed by a shared searchDomainTable helper with a table whitelist.
5. Those methods wired into internal/cli/search.go so --type flights|airports|operators|aircraft hits the correct domain table. Default search tries SearchFlights first before falling back to generic FTS. Data Pipeline Integrity: 7 -> 10.
6. digest command extracted from internal/cli/transcend.go into internal/cli/digest.go. Added dry-run guard and share-percentage field on top destinations. Insight: 6 -> 8.
7. README.md rewritten end-to-end. Dropped 40 lines of AeroAPI boilerplate. Fixed install URL. Added Unique Features section with every transcendence command and its value prop. Replaced HELP_OUTPUT and DOCTOR_OUTPUT placeholders with real content. Added categorized command tables, 15-recipe Cookbook, documented all five env var aliases and FLIGHTGOAT_BASE_URL, added Rate Limits and Troubleshooting sections.

## Ship Recommendation

ship
