# Ashby CLI shipcheck

Verdict: **ship**

## Results

| Gate | Result |
|---|---|
| Unit tests | PASS (`go test -count=1 ./...`) |
| Static analysis | PASS (`go vet ./...`) |
| Vulnerability scan | PASS (`govulncheck ./...`: no vulnerabilities found) |
| Mock verify | PASS (26/26, 100%) |
| Live verify | PASS (26/26, 100%; the API is unauthenticated, so the non-secret marker supplied to the verifier only selected live mode) |
| Full live dogfood | PASS (67/67 executed cases, 100%; 72 inapplicable cases skipped) |
| Narrative validation | PASS |
| Workflow verification | PASS |
| SKILL verification | PASS |
| Scorecard | A, 85/100 |
| PII audit | PASS (no pending findings) |
| Secret-pattern scan | PASS (no credential-shaped values found) |

## Fixes applied

The generated raw endpoint command was hidden and disabled because Ashby's public response can include records where `isListed` is false. The purpose-built `postings list` and `postings get` commands enforce `isListed == true`, add structured discovery filters, and are covered by unit tests. `sync` persists only listed jobs from known board names and `search` queries the resulting local full-text index.

The first umbrella shipcheck held only because the verifier had run in mock mode, leaving `live_api_verification` formally unscored. Ashby's endpoint requires no API key, while the verifier selects live mode only when a non-empty `--api-key` value is supplied. Re-running it with the literal non-secret marker `public-no-auth` selected live mode; Ashby ignored the irrelevant authorization header, all 26 checks passed, and the final umbrella shipcheck passed every leg.

## Behavioral evidence

Live tests against the public `ashby` board confirmed listing, UUID lookup, remote and department filtering, structured compensation filtering, SQLite sync, offline search, invalid-board error handling, missing-posting error handling, and rejection of the unsafe raw endpoint command. The scorecard's flagship probe also passed with compact agent output.

No API key, customer login, candidate data, application data, or unlisted posting is required or intentionally exposed.
