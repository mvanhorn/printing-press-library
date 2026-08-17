# Copper CLI — Phase 5 Live Acceptance (READ-ONLY against production)

Level: Read-only live verification (production-account carve-out; user-approved read-only scope; no sandbox available).
Auth: api_key multi-header (X-PW-AccessToken + X-PW-UserEmail + X-PW-Application). Credentials from env (key owner's account).

## Why not the full binary-owned matrix
`cli-printing-press dogfood --live` runs happy-path create/update/delete on generated endpoint commands, which would mutate a real production CRM. Per the Phase 5 rule (write-side only with an approved disposable fixture/sandbox), and the user's explicit read-only choice, the mutating matrix was deliberately NOT run. The 3 mutating novel commands (bulk, log, upsert) remain verified by unit tests + dry-run; they default to preview and short-circuit under the verifier.

## Read-only live tests — ALL PASS
| # | Test | Result |
|---|------|--------|
| 1 | doctor | PASS — API reachable, credentials valid, 2/2 env vars |
| 2 | account (live read) | PASS — returned the authenticated account object |
| 3 | api opportunities (interface methods) | PASS |
| 4 | sync opportunities,pipelines,pipeline-stages,users,people,companies | PASS — 96 records, 6/6 resources, 0 errors (POST-search pagination works live) |
| 5 | forecast --by stage (real data) | PASS — correct weighted = monetary_value x win_probability/100 per stage (e.g. a 50%-probability deal yields weighted = half its open value (figures redacted)) |
| 6 | forecast --by assignee | PASS — aggregates with TOTAL; null assignee handled as "(none)" |
| 7 | stale --days 21 (real data) | PASS — open opps listed, never-contacted (days_stale -1) sorted first |
| 8 | dedupe people --on email | PASS — honest empty [] (no duplicate emails in the synced set) |
| 9 | who opportunity:<id> (real data) | PASS — assembled opportunity+company+custom-fields graph for a live deal |

## Gate: PASS (read-only scope)
Live auth + the full read path + all 4 read-only transcendence features verified against real Copper data. Mutating commands intentionally untested live (production safety); covered by 25 unit tests + dry-run. No fixtures created, nothing mutated in the production account.
