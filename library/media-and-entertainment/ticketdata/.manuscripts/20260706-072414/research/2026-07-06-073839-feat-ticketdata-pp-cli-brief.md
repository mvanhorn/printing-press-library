# TicketData CLI Brief

## API Identity
- Domain: consumer ticket-price intelligence. TicketData tracks each event's **Get-In Price** (lowest all-in resale price across marketplaces, fees included), plus historical trends, a price forecast, 3/7/14/30-day change, and best-time-to-buy signals. Prices update as often as every 15 minutes. It is **not** a marketplace — it sells no tickets; it links out to StubHub/VividSeats/primary for the actual buy.
- Data API host: `https://data.ticketdata.com/api` (JSON, no auth). Site: `https://www.ticketdata.com`.
- Users: fans deciding when/where to buy, resellers watching floors, sports/concert price-watchers, journalists (site is "As featured in" NYT / Sports Illustrated / SiriusXM / Sports Business Journal / The Athletic).
- Data profile: events (numeric id), performers (slug), venues (slug), get-in-price time series (hundreds of points/event), per-zone series, section catalogs, marketplace buy links, forecasts.

## Reachability Risk
- **Low.** probe-reachability = `standard_http` (stdlib 200, surf-chrome 200, conf 0.95). Cloudflare challenges browser-UA curl intermittently but Go stdlib clears it. Ship direct HTTP; Surf/Chrome-fingerprint is the fallback transport.
- No tier/permission gating on the public surface. `price-history.auth_required = false`; all 791 history points return anonymously. (An account unlocks *custom zones* only — out of scope.)
- Probe-safe endpoint used: `GET /api/events/{id}` (2xx, no auth).

## Top Workflows
1. **Get-in price + forecast for an event** — "cheapest ticket right now for Ariana Grande in Atlanta, and is it forecast to drop?" (`event get`, forecast fields).
2. **Price history & trend analysis** — pull the get-in-price time series and see the floor's trajectory, historical low, volatility (`event history`, offline analysis).
3. **Resolve by name** — turn "ariana grande" / "state farm arena" into the canonical performer/venue and their stats (`performer`, `venue`, search resolvers).
4. **Watch an event over time** — sync snapshots into the local store, then detect drift ("floor dropped 12% since Tuesday").
5. **Compare across dates/performers** — rank multiple events/performers by get-in price and % change.

## Table Stakes (what the site does that the CLI must match)
- Show get-in price, max price, 10th-percentile price, listing count, total quantity for an event.
- Show 3/7/14/30-day change with direction.
- Show the price forecast + best-time-to-buy hover text.
- Show performer stats (upcoming events, avg/min/max get-in price) + resale/social links.
- Show venue stats.
- Section catalog per event.
- Marketplace buy links (Vivid, StubHub, primary).

## Data Layer
- Primary entities: `events`, `performers`, `venues`, `price_history` (time-series points), `sections`, `event_snapshots` (for drift).
- Sync cursor: `price_history.inserted_at` per event; `metadata.timestamps.stats_last_updated` for staleness.
- FTS/search: events (title), performers (name/slug), venues (name/slug) — the site only exposes single-best-match resolvers, so **offline multi-result search is a real differentiator**.

## Why install this instead of the website
- The site gives you one forecast number and a chart per event. The CLI owns the **raw 791-point time series locally**, so it can compute historical lows, volatility, day-of-week seasonality, and multi-event/performer comparisons the UI never surfaces — and do it offline, scriptable, agent-native (`--json`, `--select`, typed exit codes).
- No login, no marketplace fees, no per-event clicking. One `sync` and you have every tracked event's price DNA in SQLite.

## Product Thesis
- **Name:** `ticketdata` (binary `ticketdata-pp-cli`). Display name **TicketData**.
- **Thesis:** Every TicketData price signal — get-in price, full history, forecast, N-day change — for any event, performer, or venue, plus a local price-history store that powers trend math, drift alerts, and cross-event comparisons no ticket tool ships.

## User Vision
- User gave `https://www.ticketdata.com/` and clarified: "I just want TICKETDATA CLI" (the consumer price-tracker site itself). Single source. Codex mode ON for code-writing.

## Build Priorities
1. Data layer for events / performers / venues / price_history / sections + sync.
2. Absorbed read surface: event get, event history, event sections, performer, venue, performer/venue search resolvers, N-day change, forecast.
3. Transcendence: price-watch drift, best-time-to-buy analysis from local history, cross-event/performer comparison, offline multi-result search, section opportunity finder.
