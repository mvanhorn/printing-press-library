# Luma CLI — Absorb Manifest

## Landscape (tools found)
| Tool | Lang | Target | Auth | Notes |
|---|---|---|---|---|
| alx1p luma-cal-mcp / Luma Events Discovery | TS (MCP) | public feed + subscribed calendars | optional | distance filter, ICS export — closest peer |
| Apify luma-calendar-events-scraper | hosted | public calendars | none | scraper-as-a-service |
| montaguegabe/luma-events-mcp | TS (MCP) | Luma Plus API | required | create/manage events+guests (paywalled) |
| Zettersten/Lu.Ma | .NET SDK | Luma Plus API | required | manage events/calendars (paywalled) |
| bobtista/luma-ai-mcp-server, douglac/mcp-luma-events | TS (MCP) | Luma API | required | manage path |

Our angle: the **public discovery surface** as a single Go binary with a local store. We match the public-discovery peers (alx1p/Apify) and beat them with offline search, aggregation, drift, and agent-native output — no account, no key, no Node/Python.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Discover home / featured | Luma discover page | `(generated endpoint) discover home` | One call, `--json`, persisted |
| 2 | Browse events by city | alx1p, Luma /sf | `luma-pp-cli events list` (--city) | Offline cache, `--select`, cursor paging |
| 3 | Browse events by category | Luma /discover | `(behavior in luma-pp-cli events list) --category cat-ai` | Same endpoint, category filter |
| 4 | Event detail | alx1p, Luma event page | `luma-pp-cli events get` | Full structured detail, `--json`/`--select` |
| 5 | List categories | Luma /discover | `luma-pp-cli discover categories` | Counts + cursor paging |
| 6 | Place/city details | Luma /sf | `luma-pp-cli places get` (slug→api_id) | Resolves `sf` → place id locally |
| 7 | Calendars in a place | Luma /discover | `luma-pp-cli places calendars` | List communities active in a city |
| 8 | Calendar (community) detail | Luma calendar page | `luma-pp-cli calendars get` | Full community detail |
| 9 | Geo points / mini-map | Luma map view | `luma-pp-cli places map` | Raw geo points, scriptable |
| 10 | Offline full-text search | (none — API has no search) | `(behavior in luma-pp-cli search)` | **We add what the API lacks**: FTS over synced events |
| 11 | Local mirror / sync | alx1p caches in-memory | `(behavior in luma-pp-cli sync)` | Durable SQLite, resumable cursors |
| 12 | SQL over local data | (none) | `(behavior in luma-pp-cli sql)` | Arbitrary read-only SQL joins |

## Transcendence (only possible with our approach) — vetted by novel-features subagent
Customer model: Maya (multi-city meetup hopper), Devin (community organizer tracking ~15 calendars), Priya (agent/automation builder). Full audit: `2026-06-16-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Multi-city/category agenda | agenda | 8/10 | hand-code | API lists one place OR one category per call w/ cursor paging; we union across places+categories, de-page, date-sort in SQLite | none |
| 2 | Geo-radius event search | near | 7/10 | hand-code | API exposes `coordinate` but no radius filter; haversine over local store | none |
| 3 | ICS export of any filtered set | ics | 8/10 | hand-code | API returns JSON only; serialize any filtered/synced set to RFC5545 VEVENTs | none |
| 4 | Drift / new-since-last-sync | watch | 7/10 | hand-code | API is stateless; diff latest sync snapshot vs prior (added/removed, guest/ticket/sold_out deltas) | Use for change-over-time across syncs. For a point-in-time list use `agenda`. |
| 5 | Multi-calendar aggregate compare | calendars compare | 6/10 | hand-code | API is per-calendar; GROUP BY calendar_api_id over local events — counts + total guests side by side | Use to compare activity across calendars. For one calendar's detail use `calendars get`. |

Transcendence rows: **5 planned, all hand-code** (all >= 6/10). Framework `sync`/`search`(FTS)/`sql`/`stale`/`doctor` are generated and cover offline search + local mirror.
Killed by the cut pass: trending, cities, hosts, conflicts, digest, available, event-judge, recommend, map-render (see brainstorm audit).

## Stubs
None. Every row ships fully or is a generated/framework command.
