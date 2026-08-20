# Pointhound CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

**Empty.** No competing CLI, MCP, plugin, SDK wrapper, or skill exists for Pointhound.

Verified by parallel searches on 2026-05-11:
- `pointhound mcp/plugin/cli/skill site:github.com` — zero results.
- `pointhound npm/pip/model context protocol wrapper` — zero results.
- `pointhound CLI site:github.com` — zero results.
- Official Anthropic plugin marketplace (`anthropics/claude-plugins-official`) — no Pointhound entry.

Award-flight-search competitors do exist (seats.aero, point.me, PointsYeah, Roame.Travel, AwardTool, ExpertFlyer, AwardFares), but they are alternative consumer products with proprietary backends — none wraps Pointhound, none ships a CLI. Greenfield territory.

Because there is nothing to absorb, every CLI feature is net-new. The transcendence table below is the entire feature set.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Status | Why Only We Can Do This |
|---|---------|---------|-------|--------|------------------------|
| 1 | Reachability from points balance | `from-home <origin> --balance "ur:250000,mr:80000,bilt:120000"` | 9/10 | shipping | Joins offers × airlines × transferOptions against a user-supplied balance map; filters offers whose `points_cost / transferRatio <= balance[earn_program]`; ranks by lowest effective spend. No web product does this — Pointhound itself can't ask "where can I go with what I have?" |
| 2 | Effective-cost ranking via transfer ratios | `compare-transfer <earn-program> <route>` | 9/10 | shipping | Joins offers × transferOptions × programs locally; multiplies points cost by the per-source transferRatio (1.0 instant for Chase UR→United vs 0.333 for Marriott Bonvoy→United, etc.); ranks by source-program-points-spent. Pointhound's per-source `transferOptions` data is unique among award-search tools. |
| 3 | Saved-route polling with delta exit code | `watch <route>` | 8/10 | shipping | Calls `/api/offers?searchId=...` on each run, diffs offer set against last snapshot in local SQLite, exits 2 if any offer is new or cheaper. Brief: "deals change constantly; book within 24-48h." Cron-compatible (`watch route1 && notify`). |
| 4 | Fan-out search and store | `batch <route-csv>` | 8/10 | shipping | One command issues N `/api/offers` reads with throttling; all snapshots written to local store. Pairs with `watch`. Replaces the typical Sunday-morning "re-run same 5 searches" ritual. |
| 5 | Top-deals matrix with persistence | `top-deals-matrix --origins SFO,LAX --dests LIS,FCO,LHR --months 10-12` | 7/10 | shipping (auth-gated) | Calls search-create POST per (origin, dest, month) cell using imported Chrome cookies, GETs offers per result, stores everything. Requires `auth login --chrome`. Mirrors Pointhound's flagship Top Deals product but with local persistence and `--cabin business` filter. |
| 6 | Cross-snapshot drift report | `drift <route>` | 7/10 | shipping | Local SQLite join across `offer_snapshots` for a route; produces new/cheaper/disappeared/unchanged columns. The web UI is stateless — drift only exists in CLI. |
| 7 | Deal-rating explore via Scout | `explore-deal-rating --metro <code>` | 6/10 | shipping | GET `scout.pointhound.com/places/search?metro=...`; filters `dealRating: high`; optionally chains into `batch` to actually fetch deals for those airports. Pointhound's own UI doesn't surface a standalone deal-rating view. |
| 8 | Best-month heatmap | `calendar --route SFO-NRT --cabin business` | 6/10 | shipping | Fan-out via `batch` across 12 months; groupby month → min(pricePoints); renders as a month-grid table or JSON. Marcus's "best month" view. |
| 9 | Transfer-source lookup | `transferable-sources <redeem-program>` | 6/10 | shipping | Local read of transferOptions table (synced from offers); lists earn programs that feed a given airline redeem program with ratio + transfer time. Companion to `compare-transfer` from the opposite direction. |

All shipping. No stubs.

## Spec-driven endpoint commands (auto-generated)

These come straight from the spec — typed commands, MCP-exposed by default, agent-native flags, --json, --select, --dry-run all standard.

| # | Command | Endpoint | Auth |
|---|---------|----------|------|
| 1 | `offers list` | `GET /api/offers` | none (anonymous) |
| 2 | `offers filter-options` | `GET /api/offers/filter-options` | none (anonymous) |

## Hand-written novel commands (transcendence helpers)

Cross-domain or special-protocol commands that don't fit the spec model.

| # | Command | Domain | Why hand-written |
|---|---------|--------|------------------|
| 1 | `airports <query>` | `scout.pointhound.com/places/search` | Cross-domain (separate base URL); deal-aware autocomplete with `dealRating` and `isTracked` fields. |
| 2 | `search <orig> <dest> <date>` | `POST /flights?q=<base64-protobuf>` | Cloudflare-gated; requires imported Chrome cookies and Go-side protobuf encoding for the `q` query parameter. |
| 3 | `cards`, `cards get <slug>`, `points101` | SSR HTML pages | Page-scrape with `// pp:novel-static-reference` tag; not RESTful so not in the spec. |
| 4 | `auth login --chrome`, `auth status` | n/a | Cookie capture from Chrome; framework-generated when `auth.type: cookie`. |

## Local Store Design

The transcendence features depend on a local SQLite store populated by `sync` and `batch`.

Tables (planned):
- `searches(id PK ofs_*, origin_code, destination_code, depart_date, cabin, passengers, created_at, last_synced_at)`
- `offers(id PK off_*, search_id FK, price_points, best_price_points, price_retail_total, price_retail_currency, cabin_class, total_stops, total_duration, departs_at, arrives_at, source_identifier, quantity_remaining, first_seen_at, last_seen_at)`
- `offer_segments(id PK seg_*, offer_id FK, order, airline_code, flight_number, aircraft_name, origin_code, destination_code, departs_at, arrives_at)`
- `airlines(id PK aln_*, name, iata_code, logo_url)`
- `airports(id PK apt_*, name, iata_code, timezone, municipality, deal_rating, is_tracked)`
- `earn_programs(id PK pep_*, name)`
- `redeem_programs(id PK prp_*, name)`
- `transfer_options(id PK pto_*, earn_program_id FK, redeem_program_id FK, transfer_ratio, total_transfer_ratio, transfer_time, bonus_date_end, bonus_transfer_ratio)`
- `watches(id PK, origin_code, destination_code, depart_date, cabin, passengers, created_at, last_run_at, last_exit_code)`
- `offer_snapshots(id PK, watch_id FK, offer_id, price_points, quantity_remaining, captured_at)` — enables drift detection
- `cards(id PK, slug, name, issuer, content_html, fetched_at)` — for static-reference cards command

FTS5 indexes over `offers` (airline names, route descriptions), `airports` (name, city), `cards` (name, content).
