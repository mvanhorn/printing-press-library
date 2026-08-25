# SimpleFIN CLI Absorb Manifest

NOI: SimpleFIN isn't just a bank-data connector. It's a cross-institution net-worth time machine.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Claim setup token -> access URL | csells_rust `claim`, simplefin-api | simplefin-pp-cli auth claim | Accepts base64 token or demo token; saves access URL chmod 600; never logs creds |
| 2 | Server info / versions | csells `info`, npm `info` | (generated endpoint) info get | Offline-safe, --json |
| 3 | List accounts + balances | jeeftor, npm `balances`, arjungandhi | simplefin-pp-cli accounts | Offline from store, --json/--select/--csv, cross-institution in one view |
| 4 | List transactions + date range + pending | npm/@dilllxd `transactions` | simplefin-pp-cli transactions | Relative dates (30d), --account, --pending, offline, FTS-backed |
| 5 | Full-text search across institutions | csells `query` | (generated endpoint) search | Offline FTS over description/payee/memo, all accounts at once |
| 6 | Raw /accounts live passthrough | npm `--raw`, csells `--raw` | (generated endpoint) accounts list | Direct live fetch with all params, --json |
| 7 | Idempotent sync into local SQLite | arjungandhi `fetch`, pysimplefin `sync` | simplefin-pp-cli sync | Nested extraction (accounts+txns+holdings+connections), overlap re-pull, balance snapshots, rate-limit aware |
| 8 | List investment holdings | jazzboME types, opencoffer | simplefin-pp-cli holdings | symbol/shares/cost/market value table, ecosystem-wide gap |
| 9 | Stale-account detection | csells `stale`, finance-reconcile | simplefin-pp-cli stale | Flags accounts whose balance-date age exceeds threshold |
| 10 | Raw SQL over local store | csells `query`, framework | (generated endpoint) sql | Arbitrary SELECT over the ledger |
| 11 | Doctor / status (auth + reach + quota) | csells `status`, @dilllxd | (generated endpoint) doctor | Auth check, /info reachability, 24/day quota awareness |
| 12 | Auth config (set-url/status/logout) | csells `configure`, simplefinr keyring | simplefin-pp-cli auth (set-url/status/logout) | chmod 600 storage, redaction |

## Transcendence (only possible with our local-store approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Net worth + trajectory | networth | hand-code | SUM latest balance across ALL institutions in SQLite; --trend joins balance_snapshot time series the protocol never returns | Use for total/historical net worth across institutions. Do NOT use for investment gain/loss (use 'portfolio') or spending breakdowns (use 'cashflow'). |
| 2 | Cash flow analysis | cashflow | hand-code | GROUP BY month + signed-amount SUM over local txns, cross-institution; --by-merchant groups normalized payee | Use for income-vs-outflow and top-spend merchants. For subscription detection use 'recurring'. Consumes categories from 'categorize' when present. |
| 3 | Recurring/subscription detection | recurring | hand-code | Groups normalized payee + amount tolerance + cadence over local store (interval rules, not LLM) | Use for finding subscriptions/regular obligations across all accounts. For one-off or total spend use 'cashflow'. |
| 4 | Holdings gain/loss | portfolio | hand-code | market_value - cost_basis per holding + aggregate from local holding table | Use for investment positions and gain/loss. For a raw position dump use 'holdings'. For cash+investment net worth use 'networth'. |
| 5 | Rules-based categorization | categorize | hand-code | Keyword/regex rules map payee/description to categories written to local store; protocol has NO native categories | Use to assign categories deterministically. This is NOT spending math; 'cashflow' reads the categories this writes. |
| 6 | Ledger/plaintext export | export | hand-code | Reads local txn store, serializes ledger/beancount/csv/qif | Use to emit accounting-tool formats. Global --csv is a raw table dump; 'export' produces structured ledger/beancount/qif records. |
| 7 | Content-based dedup | reconcile | hand-code | Hashes amount+date+normalized-description over local store to detect mirrored duplicates SimpleFIN's unstable IDs create | Use to find/merge duplicate transactions from mirrored IDs. For stale-connection age problems use 'stale'. |
| 8 | Recent activity since last check | since | hand-code | Diffs local store vs a point in time across every institution; API returns point-in-time only, not 'what changed' | Use for recent activity across accounts since a date/last sync. For full net-worth history use 'networth --trend'. Neutral framing; not alarmist. |

## Stubs
None. All 7 transcendence rows ship fully.

## Hand-code commitment
12 absorbed (generator + hand-wiring) + 8 transcendence (all hand-code, ~50-150 LoC each + root.go wiring). User priorities: content-based dedup (reconcile) + rate-limit/quota awareness (sync + doctor). 'transactions' elevated to prominent account-filterable ledger command (--account/--since/--category). Design mirrors user's proven toddfinances schema.
