# SimpleFIN Novel Features Brainstorm (audit trail)

## Customer model
- **Beancount Brett** — plaintextaccounting purist, 6 institutions, refuses SaaS bank creds. Frustration: mirrored/unstable txn IDs make dedup hell ($3,157 wire 3x), 90-day cap silently truncates backfill, no categories.
- **Net-Worth Nadia** — privacy-conscious multi-account tracker (2 banks + CU + brokerage w/ AAPL). Frustration: no trajectory (protocol is point-in-time), no holdings gain/loss anywhere.
- **Subscription-Hawk Sam** — cash-flow watcher across 3 cards + 2 checking. Frustration: recurring charges hide across institutions; stale "green dot" connections silently wrong.
- **Agentic Aldo** — scripts-everything power user. Frustration: every tool is an importer into someone else's app; 24/day limit disables token; nothing emits clean --json/--select.

## Survivors (transcendence set)
| # | Feature | Command | Score | Build | Persona |
|---|---------|---------|-------|-------|---------|
| 1 | Net worth + trajectory | networth [--at] [--trend] | 9 | hand-code | Nadia |
| 2 | Cash flow | cashflow [--month\|--by-merchant] [--since] | 9 | hand-code | Sam |
| 3 | Recurring detection | recurring [--min-occurrences] | 9 | hand-code | Sam |
| 4 | Holdings gain/loss | portfolio [--gain] | 9 | hand-code | Nadia |
| 5 | Rules categorization | categorize [--rules] [--apply] | 8 | hand-code | Brett/Sam |
| 6 | Ledger export | export --format ledger\|beancount\|csv\|qif | 8 | hand-code | Brett |
| 7 | Content-based dedup | reconcile [--fix] | 9 | hand-code | Brett |

## Killed candidates
- merchants -> subsumed by cashflow --by-merchant
- drift -> subsumed by networth --trend
- quota -> absorbed by doctor
- anomalies -> weak dogfood verifiability
- backfill -> extension of sync, not distinct
- pending -> transactions list --pending + reconcile overlap
- budget -> needs categorize first, app-shaped scope creep
