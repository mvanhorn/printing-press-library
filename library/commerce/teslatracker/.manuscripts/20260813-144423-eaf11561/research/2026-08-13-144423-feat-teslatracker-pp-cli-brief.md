# TeslaTracker CLI Brief

## API Identity
- Domain: used-Tesla inventory aggregation across Tesla, Carvana, CarMax, private party, Premium Autos.
- Users: used-EV buyers who need one ranked view across channels that individually 403 scripts.
- Data profile: ~2,042 pre-owned listings, VIN-keyed, with price, year, model, trim, image.

## Reachability Risk
- **None.** All 13 routes return HTTP 200 cold with a normal User-Agent. No bot wall, no auth,
  no clearance cookie. Verified 2026-08-13.
- No `probe-reachability` escalation needed; `mode: standard_http`.

## Surface discovered (cold HTTP, verified)
| Surface | Shape | Evidence |
|---|---|---|
| `/inventory` | schema.org `ItemList` of `Car` in JSON-LD | 23 items, price $20,600–$25,000 |
| `/inventory?page=N` | real pagination | page 2 = $25,000–$26,300 |
| `/tesla-model-3`, `/tesla-model-y`, `-s`, `-x`, `/cybertruck` | model-scoped lists | model-y: 24 items, $25,600–$28,300 |
| `/used-tesla` | 24 items | largest page, 367 KB |
| `/inventory/<VIN>` | detail; **RSC-deferred** | cold payload carries only `{"vin":"..."}` |
| `/trends` `/value` `/vehicle-history` `/vin-decoder` `/compare` `/calculator` `/dealers` `/alerts` | 200 each | tool pages |

Per-car JSON-LD fields: `name`, `url` (VIN-keyed), `brand`, `model`, `vehicleModelDate`,
`fuelType`, `itemCondition`, `offers.price`, `offers.priceCurrency`, `offers.availability`, `image`.

**Correction to prior project research:** the handoff states "VINs are not exposed in the list
view, so plan detail-page enrichment or VIN-less fuzzy dedupe." That is false as of today —
23 distinct VINs are present in the cold `/inventory` HTML via the JSON-LD `url` field.
VIN-exact dedupe against the cargoat store works with no enrichment step.

## Gap requiring discovery
Detail-page data (battery health, accident/title history, price history, days-on-lot — the
fields that differentiate TeslaTracker) is fetched client-side behind a React Suspense
boundary. Not in cold HTML. Browser-sniff is the way to find that call.

## Top Workflows
1. Rank used Teslas by price under a ceiling, VIN-deduped, across all channels at once.
2. Watch a saved query and report what is new / dropped / gone since last sync.
3. Enrich a known VIN with battery health and title history.
4. Cross-reference a VIN already held in another store (cargoat) to fill title gaps.
5. Track price history per VIN to time an offer.

## Table Stakes (from adjacent tools; none target teslatracker)
- teslahunt/inventory — real-time inventory retrieval
- kaedenbrinkman/tesla-inventory — price-change tracking, CSV export
- JumpBearCode/TeslaWebScrape — MCP server, nodriver to bypass Akamai (targets tesla.com, not this)
- robcerda/tesla-mcp-server, scald/tesla-mcp, TeslaPy — Owner/Fleet API, not inventory

## Data Layer
- Primary entity: `listing` keyed by VIN.
- Secondary: `price_snapshot` (VIN, price, observed_at) — enables drift/drop detection.
- Sync cursor: page walk + last_seen timestamp.
- FTS: name/trim/model.

## Product Thesis
- Name: `teslatracker-pp-cli`
- Why it should exist: no CLI, MCP, or wrapper targets teslatracker.com. It is the one
  aggregator that reaches Carvana, CarMax, Tesla and private party in a single cold-HTTP
  surface — three of which block scripts directly. A local VIN-keyed store turns a
  browse-only site into something an agent can diff, rank and join against other sources.

## Build Priorities
1. Paginated list sync -> SQLite, VIN-keyed, from JSON-LD.
2. Price-snapshot history + drops/new/gone.
3. Ceiling-filtered ranked search offline.
4. Detail enrichment (pending browser-sniff discovery).
5. Cross-store VIN join.
