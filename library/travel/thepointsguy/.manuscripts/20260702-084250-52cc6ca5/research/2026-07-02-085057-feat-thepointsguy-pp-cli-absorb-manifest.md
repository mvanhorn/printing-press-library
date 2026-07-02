# The Points Guy CLI — Absorb Manifest

Scope: No existing TPG CLI/API wrapper exists (GitHub/npm searched). Absorbed from
TPG's own site surfaces + adjacent points-and-miles tools (AwardWallet, NerdWallet,
Frequent Miler valuations; RSS readers). Every row works offline where possible,
with --json/--select, typed exits, and SQLite persistence.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Full-text content search | TPG site search (Algolia) | thepointsguy-pp-cli search | Live Algolia + offline FTS fallback, --json, --select, --category/--limit |
| 2 | Search suggestions/autocomplete | TPG suggestions index | thepointsguy-pp-cli suggest | Query-suggestions index, scriptable |
| 3 | Latest articles | TPG homepage / RSS | thepointsguy-pp-cli latest | RSS-backed, --category filter, agent-native |
| 4 | Latest news | TPG news section | (behavior in thepointsguy-pp-cli latest) --category news | news sitemap + RSS, time-windowed |
| 5 | Deals feed | TPG deals category | (behavior in thepointsguy-pp-cli latest) --category deals | filter deals, offline after sync |
| 6 | Read an article | TPG article page | thepointsguy-pp-cli read | Extracts body from __NEXT_DATA__, markdown/plain, --select |
| 7 | Browse by category | TPG category pages/sitemaps | thepointsguy-pp-cli browse | Enumerate via article sitemaps, --limit |
| 8 | Credit-card lookup | TPG card page | thepointsguy-pp-cli card | Structured terms (fee/APRs/bonus/rewards) from card page |
| 9 | List all cards | sitemap_cards.xml (196) | thepointsguy-pp-cli cards list | Full card DB mirror, filter by category |
| 10 | Best cards by category | TPG best-of pages | thepointsguy-pp-cli cards best | travel/airline/no-annual-fee/lounge etc., offline |
| 11 | Points valuations | TPG monthly valuations | thepointsguy-pp-cli valuations | Structured cents-per-point by program/type, agent-native |
| 12 | Glossary lookup | TPG glossary | thepointsguy-pp-cli glossary | Term definitions, --json |
| 13 | Sync local mirror | (none — our infra) | thepointsguy-pp-cli sync | Populate SQLite for offline + transcendence |
| 14 | Health/doctor | (none — our infra) | thepointsguy-pp-cli doctor | Reachability + cred discovery + cache report |

## Transcendence (only possible with our local store + agent-native approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Redeem-vs-cash checker | redeem-check | hand-code | Correlates a specific redemption's cents-per-point against TPG's valuation to give a buy/redeem verdict — no TPG page does this for YOUR numbers | Use to decide points vs cash for one booking. For a whole balance's dollar value, use 'worth'. |
| 2 | Points value calculator | worth | hand-code | Applies TPG valuations to an arbitrary balance instantly; the site only publishes the rate | Use for one program+balance -> dollars. For multi-program totals, use 'portfolio'. |
| 3 | Multi-program portfolio value | portfolio | hand-code | Values many balances across programs from stdin/config in one shot using the local valuations table | none |
| 4 | Valuation drift over time | valuations drift | hand-code | Requires historical monthly valuation snapshots in SQLite; a single live page can't show month-over-month change | Use to see how a program's cpp trended. For current value, use 'valuations' or 'worth'. |
| 5 | Card side-by-side compare | cards compare | hand-code | Local join across mirrored card records (fee/APR/bonus/rewards); the site has no multi-card compare table | none |
| 6 | What TPG published recently | since | hand-code | Time-windowed aggregation over synced articles (last Nh/Nd across all categories) | Use for a recency window across everything. For a single category stream, use 'latest'. |

## Notes / risks
- Search command depends on runtime discovery of the public Algolia app id + search key
  from the site bundle; if discovery fails, falls back to local FTS over synced articles.
- Valuations parsing extracts cents-per-point from the monthly-valuations article HTML
  (table/list structure), normalized into (program, type, cpp, month).
- No auth anywhere; no write operations; entirely read-only. Respectful rate limiting on sync.
