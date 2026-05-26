# Rappi novel-features brainstorm

## Customer model

**Sofía — CDMX neighborhood-focused food blogger / restaurant scout (28, freelance).**

Today she keeps five tabs open on rappi.com.mx: Polanco hamburguesas, Condesa pizza, Roma Norte sushi, Coyoacán tacos, and Narvarte saludables. She manually copies restaurant names, star ratings, and addresses into a Google Sheet so she can pitch a "Top 10 hamburgueserías de Polanco" listicle. She also flips between Rappi and Google Maps to check whether a place she likes still exists or has moved.

Weekly ritual: every Monday morning she pulls a fresh list of restaurants in 3-4 CDMX neighborhoods for one cuisine category, sorts by rating, picks 15 candidates to visit, and reconciles last week's shortlist against this week's catalog to spot newcomers and closures.

Frustration: the Rappi web UI paginates infinitely, has no "sort by rating with min 100 reviews" filter, no export, and no week-over-week diff.

**Diego — retail strategy analyst at a CPG company (34, Monterrey HQ, covers MX).**

Today he keeps a quarterly deck about where his client's product is carried. He pivots between rappi.com.mx market and farmatodo-pharmacy listings city by city, copying store names, neighborhoods, and types into Excel. He cannot easily tell how many "express" stores Rappi runs in Guadalajara vs Monterrey.

Weekly ritual: each Friday he refreshes a coverage snapshot for the three biggest MX cities across all five store types, then compares to last Friday's snapshot to spot new store openings.

Frustration: Rappi has no public "all stores in city X" export and no cross-city coverage view. Counting stores by type per city is a 90-minute manual scroll-and-tally exercise weekly.

**Mateo — agent-builder at a LATAM dev tools startup (31, Guadalajara, remote).**

Today he's prototyping a "concierge" agent (Claude/GPT) that helps users plan a weekend in CDMX. Every existing Rappi scraper is either abandoned or requires paid Bright Data proxies. He resorts to brittle Playwright scripts.

Weekly ritual: ships incremental improvements to his agent; each iteration needs a fresh local snapshot of restaurants and stores plus a fast structured-query layer.

Frustration: no agent-native interface exists. He needs `search "sushi condesa" --open-sundays` to return structured JSON in <500ms from a local cache.

**Lucía — neighborhood Slack admin / community organizer (Roma Norte, 41).**

Today she runs a 1,200-member neighborhood Slack and gets weekly DMs asking "¿alguien sabe de una farmacia abierta cerca de Álvaro Obregón?" She answers from memory or by opening Rappi on her phone.

Weekly ritual: on Sundays she posts a "new restaurants this week in Roma Norte" recap; she maintains a pinned "open late" list of pharmacies.

Frustration: no way to ask "what restaurants opened in Roma Norte this week?" or "which pharmacies in this neighborhood are open past 11pm?"

## Candidates (pre-cut)

(See subagent transcript - 16 candidates generated, 6 cut inline as borderline/duplicate, 10 surviving.)

## Survivors and kills

### Survivors (transcendence table)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Restaurant newcomers/closures diff | `restaurants diff --city <city> --category <cat> --since <date>` | 8/10 | Joins two timestamped snapshots in SQLite, emits added/removed rows | Sofía Monday ritual + Lucía Sunday recap; Rappi UI has no diff view |
| 2 | Top-rated with rating + review floor | `restaurants top --city <city> --category <cat> --min-rating 4.5 --min-reviews 100 --limit 10` | 8/10 | Filters/sorts synced restaurants by rating + review_count thresholds | Rappi UI cannot filter by review-count floor; Sofía listicle workflow |
| 3 | Cross-city store coverage matrix | `stores coverage --cities cdmx,gdl,mty` | 8/10 | count(*) over synced stores grouped by (city, store_type), emits markdown/CSV | Diego retail-analyst persona; Rappi has no cross-city coverage view |
| 4 | Coverage delta over time | `stores coverage-diff --since <date>` | 7/10 | Diffs the (city, store_type) count matrix between snapshots | Brief Data Layer "sync cursor" + Diego weekly delta |
| 5 | Restaurants open at a specific time | `restaurants open --city <city> --at "23:30"` | 7/10 | Parses Restaurant JSON-LD `openingHours`, evaluates requested local time | Brief confirms SSR JSON-LD includes hours; Lucía open-late ritual |
| 6 | Cuisine-by-neighborhood breakdown | `restaurants by-neighborhood --city <city> --category <cat>` | 7/10 | Groups synced restaurants by neighborhood extracted from address | Sofía neighborhood-listicle workflow |
| 7 | Cross-category restaurant overlap | `restaurants multi-category --city <city>` | 6/10 | Self-joins (category, restaurant_id) snapshot, emits restaurants in 2+ categories | Closed category set; fusion places surfaced by Sofía's tab-juggling |
| 8 | Geo-radius restaurant filter | `restaurants near --lat <> --lng <> --radius-km 2 --category <cat>` | 7/10 | Haversine over restaurant geo, sorts by distance | Brief Top Workflow #1; Lucía + Mateo |
| 9 | Brand presence across cities | `restaurants brand --name "Sushi Itto"` | 6/10 | Fuzzy-name match across all city+category snapshots, emits city × category presence matrix | Multi-city presence + Diego chain coverage |
| 10 | Cross-store-type geo adjacency | `stores adjacency --type pharmacy --within-km 1 --of-type market` | 6/10 | Haversine cross-join between two store-type tables | Lucía "pharmacy near supermarket"; agent-shaped output for Mateo |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `restaurants menu-shape` | Wrapper-ish per-restaurant scrape; weak transcendence | `restaurants top` |
| `stores express-density` | Subsumed by `stores coverage` matrix drill-down | `stores coverage` |
| `restaurants timeline` | Per-restaurant slow-walk; covered by `restaurants diff` rows | `restaurants diff` |
| `restaurants rating-histogram` | One-time analytics; weekly workflows go to `top` and `by-neighborhood` | `restaurants top` |
| `categories explain` | Borderline reimplementation, static reference dressed as command | absorb manifest `categories list` |
| `cities geo` | Duplicate of absorb manifest `cities list` (includes coords) | absorb manifest `cities list` |
