# Dice FM Partners API CLI — Absorb Manifest

## Ecosystem Sources
- **Dice FM Partners GraphQL API** (docs) — primary spec source; all entities documented
- **Audience Republic DICE integration** — real-world consumer of the Partners API; establishes workflow patterns (90-min sync, event/order/fan data, Mailchimp export)
- **Apify Dice.fm Scraper MCP** — public consumer-side scraper; establishes "no-API-key" event listing pattern (not partners data, but shows demand)
- **Dice FM GitHub org** — 48 repos, no Partners API SDK; confirms no existing CLI tooling

## Absorbed Features
Match or beat everything that exists across ecosystem tools.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List events with date/state filters | Dice FM API | `events list --state APPROVED --from 2026-01-01` | --json, --select, FTS-searchable, offline-cached after sync |
| 2 | Get event details | Dice FM API | `events get <id>` | Full schema: ticket types, venues, genres, dates, state |
| 3 | List ticket holders per event | Dice FM API | `tickets list --event <id>` | Fan contact included, --json, --select, --csv |
| 4 | Filter tickets by claim status | Dice FM API | `tickets list --event <id> --claimed` | `claimAllowed` filter from spec |
| 5 | Filter tickets by fan phone | Dice FM API | `tickets list --event <id> --fan-phone +1...` | `fanPhoneNumber` filter from spec |
| 6 | List orders per event | Dice FM API | `orders list --event <id>` | Financial breakdown, geographic data, date range filter |
| 7 | List returns per event | Dice FM API | `returns list --event <id>` | Reason codes, amounts, date range filter |
| 8 | List ticket transfers per event | Dice FM API | `transfers list --event <id>` | Transfer timestamps, old/new holder IDs |
| 9 | List extras/add-ons per event | Dice FM API | `extras list --event <id>` | Product, variant, barcode details |
| 10 | Filter extras by separate barcode | Dice FM API | `extras list --event <id> --separate-barcode` | `hasSeparateAccessBarcode` filter |
| 11 | Export fan contacts with opt-in filter | Audience Republic pattern | `fans list --event <id> --optin --csv` | Offline after sync, opt-in filter, geography join |
| 12 | List genre types | Dice FM API | `genres list` | Offline lookup, searchable |
| 13 | Get any node by ID | Dice FM API | `node get <id>` | Any entity type via `node` root query |
| 14 | Sync all data to local SQLite | Audience Republic (90-min auto-sync) | `sync --full` | Cursor-resumable, all entities, incremental via --since |
| 15 | Incremental sync | Audience Republic | `sync --since <timestamp>` | Only fetch what changed |
| 16 | Full-text search across synced data | No equivalent | `search <term>` | FTS5 over fan names, emails, event names |
| 17 | Export any entity list to CSV | General API tooling | All list commands accept `--csv` | Consistent output formatting |
| 18 | Paginate large result sets | Dice FM API (cursor pagination) | All list commands support `--limit` + cursor continuation | Automatic multi-page fetch |

## Transcendence Features
Only possible with our approach (SQLite store + cross-entity joins).

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Door list with transfer resolution | `door list --event <id>` | 9/10 | Three-table SQLite join (tickets LEFT JOIN returns LEFT JOIN transfers) produces valid-holder list with resolved new-holder names | Brief workflow #1; Sam persona; no single API call or dashboard view covers this |
| 2 | Cross-event repeat buyer report | `fans repeat [--since <date>] [--min-events 2]` | 8/10 | SQLite GROUP BY fans.email across distinct event_ids, summing spend per fan | Brief workflow #5; Audience Republic 90-min lag alternative; no API equivalent exists |
| 3 | Revenue summary (per-event and cross-event) | `revenue summary [--event <id>] [--from <date>]` | 9/10 | SQLite SUM over orders (gross, dice_fee, net) per event or across date range | Brief workflow #3; Priya's Monday CFO report; no API aggregation endpoint |
| 4 | Ticket velocity (cumulative sales over time) | `velocity show --event <id> [--bucket day\|hour]` | 8/10 | SQLite bucketing of orders.purchasedAt vs event on-sale date, day-by-day cumulative count | Brief workflow #4; Keisha's 72-hour on-sale watch; Dice dashboard shows only current snapshot |
| 5 | Opt-in fan list with geography filter | `fans optin --event <id> [--country <cc>] [--city <str>] [--csv]` | 8/10 | SQLite join fans (optInPartners=true) + orders.ipCity/ipCountry, CSV for Mailchimp import | Brief workflow #2; Marco's Monday newsletter ritual; geography join not in dashboard |
| 6 | Return rate anomaly report | `returns anomalies [--threshold 0.05]` | 7/10 | SQLite COUNT(returns)/COUNT(orders) per event, filter above threshold, ranked descending | Brief workflow #3; Keisha pricing-problem detection; no API analytics endpoint |
| 7 | Top spenders per event or across events | `fans top [--event <id>] [--n 20]` | 7/10 | SQLite SUM(orders.purchasePrice) GROUP BY fan_id JOIN fans ORDER BY total DESC | Brief workflow #5; Marco VIP identification; Priya sponsor deck input; no API leaderboard |

## Build Priorities

**Priority 0 — Foundation**
- GraphQL HTTP client (POST to /graphql with Bearer auth + query body)
- SQLite data layer: events, tickets, orders, returns, transfers, extras, fans, genres tables
- `sync --full` and `sync --since` with cursor pagination across all entities

**Priority 1 — Absorb (rows 1-18 above)**
- All event, ticket, order, return, transfer, extra, genre, and node commands
- --json, --csv, --select on all list commands
- Incremental sync

**Priority 2 — Transcend (rows 1-7 above)**
- `door list` — the flagship command (9/10)
- `revenue summary` — the CFO report (9/10)
- `fans repeat` — cross-event dedup (8/10)
- `velocity show` — sale velocity curve (8/10)
- `fans optin` — Mailchimp export (8/10)
- `returns anomalies` — pricing alert (7/10)
- `fans top` — VIP list (7/10)

**Total: 18 absorbed + 7 transcendence = 25 features**
