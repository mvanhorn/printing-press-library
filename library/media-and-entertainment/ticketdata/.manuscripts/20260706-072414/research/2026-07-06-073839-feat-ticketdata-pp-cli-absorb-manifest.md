# TicketData CLI Absorb Manifest

## Landscape
TicketData is a niche consumer price-tracker; there are no competing CLIs or MCP servers for it. Adjacent tools: the Ticketmaster CLI (already in the public library; official Discovery v2 API, event listings — but no get-in-price history/forecast), and marketplace APIs (SeatGeek/StubHub/VividSeats) which sell tickets but do not expose TicketData's cross-market get-in-price + forecast. The absorbed surface below is TicketData's own feature set, matched and beaten with offline storage, `--json`/`--select`, typed exit codes, and a local price-history store.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Event get-in price + max/10th-pct + listing count + quantity | ticketdata.com event page | (generated endpoint) events get | offline cache, --json, --select, typed exit codes |
| 2 | Price forecast + best-time-to-buy hover text | ticketdata.com event page | (behavior in ticketdata-pp-cli events get) forecast_value / forecast_hover_text surfaced | scriptable, parseable |
| 3 | 3/7/14/30-day price change + direction | ticketdata.com event page | (behavior in ticketdata-pp-cli events get) three_day_price_change + price_trend | agent-native |
| 4 | Full get-in-price history time series | ticketdata.com event chart | (generated endpoint) events history | raw hundreds-of-points series stored locally, not just a chart image |
| 5 | Per-zone price history | ticketdata.com zones | (behavior in ticketdata-pp-cli events history) zones[] | offline zone analysis |
| 6 | On-sale / presale dates | ticketdata.com event | (behavior in ticketdata-pp-cli events history) onsale/presale datetime | scriptable |
| 7 | Section catalog | ticketdata.com sections | (generated endpoint) events sections | offline section list |
| 8 | Marketplace buy links (Vivid / StubHub / primary) | ticketdata.com event | (behavior in ticketdata-pp-cli events get) urls | one-command buy links |
| 9 | Performer stats (upcoming events, avg/min/max get-in) | ticketdata.com performer page | (generated endpoint) performers get | agent-native |
| 10 | Performer resale/social links | ticketdata.com performer | (behavior in ticketdata-pp-cli performers get) stats_card resale_links/social_links | scriptable |
| 11 | Venue stats + location | ticketdata.com venue page | (generated endpoint) venues get | offline |
| 12 | Resolve performer by name (single best match) | ticketdata.com search | (generated endpoint) performers search | scriptable resolver |
| 13 | Resolve venue by name (single best match) | ticketdata.com search | (generated endpoint) venues search | scriptable resolver |
| 14 | Price history while browsing (Chrome extension) | ticketdata.com extension | (behavior in ticketdata-pp-cli events history) | terminal-native, no extension, in any script |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Watchlist management | watch | hand-code | No list/browse endpoint exists; the tracked set lives only in local SQLite and drives sync | none |
| 2 | Watchlist price board | board | hand-code | Joins event_snapshots + price_history across all watched events into one sortable table no single API call returns | Use `board` for a current snapshot of the whole watchlist. For what CHANGED since your last sync or for price-target alerts use `drift`; for one event's historical distribution use `stats`. |
| 3 | Drift + target alerts | drift | hand-code | Temporal diff between the two most recent local snapshots per event; the API has no "what changed" call | Use `drift` for what moved since the last sync and for price-target alerts. For a full current snapshot use `board`; for one event's history use `stats`. |
| 4 | History stats + best-time-to-buy | stats | hand-code | Distribution / volatility / weekday seasonality computed over the full local get-in-price series; API returns only a single forecast number | Use `stats` for one event's price distribution and best day to buy. To compare multiple events use `compare`; for the whole-watchlist snapshot use `board`. |
| 5 | Cross-event comparison | compare | hand-code | Ranks multiple local event rows (optionally scoped by performer); no multi-event API call exists | Use `compare` to rank multiple watched events or one performer's events head-to-head. For a single event's own history use `stats`; for the full watchlist snapshot use `board`. |
| 6 | Best-zone / section opportunity | zones | hand-code | Ranks per-zone floors against each zone's own history from the stored per-zone series; site shows zone charts but no cross-zone opportunity ranking | Use `zones` to rank an event's zones by price and by opportunity vs their history. For the plain section name catalog use `events sections`. |
| 7 | Offline multi-result search | search | spec-emits | Local SQLite FTS returns multiple matches; the API resolvers return only a single best match | Use `search` to browse multiple offline matches. For the single canonical resolve of a name to its stats use `performers search` / `venues search`. |

Hand-code transcendence rows: 1-6 (6). spec-emits: 7 (1). No stubs.
