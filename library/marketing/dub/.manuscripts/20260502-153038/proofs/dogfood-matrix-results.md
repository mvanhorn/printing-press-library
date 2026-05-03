# Dub Live Dogfood — 2026-05-02

## Help walk (every top-level command)
PASS   | doctor --help                                      |     1686 bytes
PASS   | version --help                                     |     1572 bytes
PASS   | auth --help                                        |     1844 bytes
PASS   | profile --help                                     |     2491 bytes
PASS   | feedback --help                                    |     2271 bytes
PASS   | which --help                                       |     2190 bytes
PASS   | agent-context --help                               |     1821 bytes
PASS   | api --help                                         |     2042 bytes
PASS   | search --help                                      |     2507 bytes
PASS   | export --help                                      |     2296 bytes
PASS   | import --help                                      |     2178 bytes
PASS   | workflow --help                                    |     1872 bytes
PASS   | links --help                                       |     2976 bytes
PASS   | tags --help                                        |     1985 bytes
PASS   | folders --help                                     |     2004 bytes
PASS   | domains --help                                     |     2232 bytes
PASS   | partners --help                                    |     3161 bytes
PASS   | commissions --help                                 |     1971 bytes
PASS   | customers --help                                   |     2118 bytes
PASS   | payouts --help                                     |     2695 bytes
PASS   | bounties --help                                    |     1891 bytes
PASS   | events --help                                      |     7381 bytes
PASS   | track --help                                       |     1887 bytes
PASS   | tokens --help                                      |     1837 bytes
PASS   | qr --help                                          |     2749 bytes
PASS   | funnel --help                                      |     2254 bytes
PASS   | health --help                                      |     2103 bytes
PASS   | since --help                                       |     1968 bytes

## Novel-command help walk
PASS   | links stale --help                                 |     2438 bytes
PASS   | links drift --help                                 |     2205 bytes
PASS   | links duplicates --help                            |     2167 bytes
PASS   | links lint --help                                  |     1995 bytes
PASS   | links rewrite --help                               |     2554 bytes
PASS   | links rollup --help                                |     2209 bytes
PASS   | partners leaderboard --help                        |     2127 bytes
PASS   | partners audit-commissions --help                  |     2138 bytes
PASS   | bounties triage --help                             |     2180 bytes
PASS   | bounties payout-projection --help                  |     2074 bytes
PASS   | customers journey --help                           |     2006 bytes

## Live API reads
PASS   | links list --json                                  | valid JSON,     4062 bytes
FAIL   | links count --json                                 | invalid JSON: Manage links

Usage:
  dub-pp-cli links [command]

Available Commands:
  bulk-cr
PASS   | tags list --json                                   | valid JSON,      162 bytes
PASS   | folders list --json                                | valid JSON,       58 bytes
PASS   | domains list --json                                | valid JSON,       58 bytes
FAIL   | events list --json                                 | exit 7: rate limited, waiting 1m0s (attempt 1/3, rate adjusted to 0.0 req/s) rate limited, waiting 1m0s (attempt 2/3, rate adjus


## Live reads — continued (events skipped: hit per-second rate limit on first call, retry budget too high for matrix; manual verify above)

FAIL   | dub-analytics --event clicks --interval 24h --json | exit 7: rate limited, waiting 1m0s (attempt 1/3, rate adjusted to 0.0 req/s) rate limited, waiting 1m0s (attempt 2/3, rate adjus
FAIL   | customers list --json                              | exit 4: Error: GET /customers returned HTTP 403: {"error":{"code":"forbidden","message":"Unauthorized: Need higher plan.","doc_u
FAIL   | partners list --json                               | exit 4: Error: GET /partners returned HTTP 403: {"error":{"code":"forbidden","message":"Unauthorized: Need higher plan.","doc_ur
FAIL   | commissions list --json                            | exit 3: Error: GET /commissions returned HTTP 404: {"error":{"code":"not_found","message":"Program not found","doc_url":"https:/
FAIL   | payouts list --json                                | exit 3: Error: GET /payouts returned HTTP 404: {"error":{"code":"not_found","message":"Program not found","doc_url":"https://dub

## Transcendence (local store)
PASS   | links stale --days 1 --json                        | valid JSON,        5 bytes
PASS   | links drift --json                                 | valid JSON,        5 bytes
PASS   | links duplicates --json                            | valid JSON,        5 bytes
PASS   | links lint --json                                  | valid JSON,        5 bytes
PASS   | links rollup --json                                | valid JSON,       71 bytes
PASS   | funnel --json                                      | valid JSON,      466 bytes
FAIL   | partners leaderboard --json                        | exit 1: Error: local store has no partners rows yet — run `dub-pp-cli sync` first to populate it local store has no partners r
PASS   | partners audit-commissions --json                  | valid JSON,        5 bytes
FAIL   | bounties triage --json                             | exit 1: Error: local store has no submissions rows yet — run `dub-pp-cli sync` first to populate it local store has no submiss
FAIL   | bounties payout-projection --json                  | exit 1: Error: local store has no submissions rows yet — run `dub-pp-cli sync` first to populate it local store has no submiss
PASS   | health --json                                      | valid JSON,      535 bytes
PASS   | since 24h --json                                   | valid JSON,      412 bytes

## Error paths (negative tests)
PASS   | links get-info missing arg → expect non-zero     |     1299 bytes
PASS   | customers journey no arg → help                  |     2006 bytes
PASS   | links rewrite no flags → help                    |     2554 bytes

## Tally

55 passed, 10 failed (out of 65)
FAIL   | dub-analytics 24h --rate-limit 1                   | timed out (30s)
FAIL   | customers list                                     | exit 4: Error: GET /customers returned HTTP 403: {"error":{"code":"forbidden","message":"Unauthorized: Need higher plan.","doc_u
FAIL   | partners list                                      | exit 4: Error: GET /partners returned HTTP 403: {"error":{"code":"forbidden","message":"Unauthorized: Need higher plan.","doc_ur
FAIL   | commissions list                                   | exit 3: Error: GET /commissions returned HTTP 404: {"error":{"code":"not_found","message":"Program not found","doc_url":"https:/
FAIL   | payouts list                                       | exit 3: Error: GET /payouts returned HTTP 404: {"error":{"code":"not_found","message":"Program not found","doc_url":"https://dub
PASS   | links get-count --json                             | valid JSON,       57 bytes

## Transcendence (local store)
PASS   | links stale --days 1 --json                        | valid JSON,        5 bytes
PASS   | links drift --json                                 | valid JSON,        5 bytes
PASS   | links duplicates --json                            | valid JSON,        5 bytes
PASS   | links lint --json                                  | valid JSON,        5 bytes
PASS   | links rollup --json                                | valid JSON,       71 bytes
PASS   | funnel --json                                      | valid JSON,      466 bytes
FAIL   | partners leaderboard --json                        | exit 1: Error: local store has no partners rows yet — run `dub-pp-cli sync` first to populate it local store has no partners r
PASS   | partners audit-commissions --json                  | valid JSON,        5 bytes
FAIL   | bounties triage --json                             | exit 1: Error: local store has no submissions rows yet — run `dub-pp-cli sync` first to populate it local store has no submiss
FAIL   | bounties payout-projection --json                  | exit 1: Error: local store has no submissions rows yet — run `dub-pp-cli sync` first to populate it local store has no submiss
PASS   | health --json                                      | valid JSON,      535 bytes
PASS   | since 24h --json                                   | valid JSON,      412 bytes

## --select fidelity
PASS   | links list --json --select id,key,clicks           | valid JSON,      275 bytes

## Error paths (negative tests)
PASS   | links delete bogus id → non-zero                 | exit 1
PASS   | customers journey (no arg) → help                | exit 0
PASS   | links rewrite (no args) → help                   | exit 0

## Run 2 tally

14 passed, 8 failed (out of 22 in this run)
