# HotelTonight CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| — | (none) | — | — | **GREENFIELD.** Research found zero community CLI, SDK, API client, or MCP server for HotelTonight. There is nothing to absorb. |

The four endpoint-mirror commands below are the table-stakes base (parity with the app + competitors Priceline Tonight-Only / Booking.com last-minute), not absorbed prior art:

| Command | Endpoint | Purpose |
|---------|----------|---------|
| `inventory search` | `GET /v6/inventory` | Geo deal search (lat/lng + dates + rooms) |
| `markets list` | `GET /v2/market_cities` | List major markets / geo resolver |
| `markets get <id>` | `GET /v2/market_cities/{id}` | Market detail |
| `markets nearby <id>` | `GET /v2/market_cities/{id}/popular_market_cities` | Nearby markets |

Table stakes the base covers: filter by neighborhood/geo, review quality, price; **% off rack rate as a first-class derived field** (`strikethrough_price − customer_price_per_night`); same-day "tonight" search.

## Transcendence (only possible with our approach)

All six are hand-code (local SQLite + time-series + custom output; the generator does not emit these). All scored ≥ 7/10 by the novel-features subagent.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|-------------------------|
| 1 | Price-drop watch | `watch --lat <> --lng <> --when tonight --below 150` | hand-code | 9/10 | Snapshots inventory and diffs against stored observations for the same (geo, check-in); the app has no watchlist, alert, or price memory |
| 2 | Price history | `history <hotel> --days 30` | hand-code | 8/10 | Reads the `price_snapshots` time-series from SQLite; the app deliberately erases prior prices |
| 3 | Is-this-a-deal verdict | `verdict <hotel>` | hand-code | 8/10 | Classifies the current quote against the hotel's own observed low/median/high — an objective baseline the app never surfaces (mechanical percentiles, no LLM) |
| 4 | Neighborhood compare | `compare-neighborhoods --metro <id> --when tonight` | hand-code | 7/10 | Local group-by over inventory rows by neighborhood; the flat `/v6/inventory` feed returns no neighborhood rollup and the app is one-pin-at-a-time |
| 5 | Date-shift scan | `datescan --lat <> --lng <>` | hand-code | 7/10 | Fans out real tonight/tomorrow/weekend inventory calls over one geo and assembles a ranked side-by-side comparison the app shows one-at-a-time |
| 6 | Daily Drop tracker | `daily-drop --metro <id> [--history]` | hand-code | 7/10 | Filters inventory on HotelTonight's signature `deal_type=daily_drop` and persists it; records the once-a-day flash deal the app makes ephemeral |

**Hand-code commitment: 6 features** (each ~50–150 LoC + `root.go` wiring + the `price_snapshots` data layer they share). 0 spec-emitted novel features. No stubs.

### Cross-cutting filter (user-requested at Phase Gate 1.5)
- `--category {basic,solid,hip,luxe,charming,crashpad}` — filter deals by HotelTonight's hotel quality tier. Confirmed feasible: the `/v6/inventory` response carries `hotel.category` with exactly these values (verified live: Basic, Solid, Hip, Luxe, Charming, CrashPad). Client-side filter on the embedded field, anonymous, no new endpoint.
- Applied to the deal-listing commands: a hand-authored `deals`/`search` view, `watch`, `compare-neighborhoods`, `datescan`, `daily-drop`. Case-insensitive; repeatable/CSV to allow multiple tiers.

## Data layer (Priority 0)
- Tables: `hotels`, `markets`, `deals`, `price_snapshots` (hotel_id, market/geo, check_in, observed_at, price, %off, deal_type, num_remaining).
- Sync: poll `/v6/inventory` for a geo+date, append a snapshot row per room. Time-series-shaped, not delta-shaped.
- FTS: hotel name, neighborhood, city.
