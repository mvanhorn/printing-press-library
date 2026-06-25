# Luma CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

### Maya Chen — AI/Crypto meetup hopper (tech hub city)
- Today: lives off luma.com/discover, clicks AI category, opens each event in a tab, hand-copies dates. Re-browses city-by-city when traveling.
- Weekly ritual: Sunday scan of "what AI/crypto events this week" across SF + travel city; wants flat skimmable list, filter out sold-out, dump to calendar, catch new postings.
- Frustration: no search box (browse by category only), no cross-city comparison, sold-out events clutter the feed.

### Devin Okafor — community organizer / calendar owner
- Today: tracks ~15 adjacent calendars one at a time, reads guest counts off event pages, messy spreadsheet of "who's hot."
- Weekly ritual: pull upcoming events for watched calendars, note guest/ticket/sold_out, find breakout events, pick own dates around busy nights.
- Frustration: no aggregate view across calendars, no history ("guest count jumped 200 since last week"); reconstructs trends manually, misses signal.

### Priya Nair — agent/automation builder
- Today: wiring local-events into a Slack bot; Plus API paywalled, so scraping HTML or heavyweight Node MCP. Wants clean JSON, no account/key.
- Weekly ritual: daily pipeline pulling events for cities+categories, filter by date window + geo radius, emit rows; needs --json/--select, stable ids, changed-since-last-run.
- Frustration: incumbents assume browser/account; coords exist but no radius filter; cursor paging = hand-rolled loops.

## Survivors (transcendence set)
| # | Feature | Command | Score | Buildability | How It Works | Long Description |
|---|---------|---------|-------|--------------|--------------|------------------|
| 1 | Multi-city/category agenda | `agenda --city sf --city nyc --category cat-ai --window 7d` | 8/10 | hand-code | Unions get-paginated-events across places+categories in SQLite, de-pages cursors, date-sorts by start_at | none |
| 2 | Geo-radius search | `near --lat .. --lng .. --radius-km 5 --window 14d` | 7/10 | hand-code | Haversine over event coordinate rows; API has no radius filter | none |
| 3 | ICS export of any filtered set | `ics --city sf --category cat-ai --window 30d -o ai.ics` | 8/10 | hand-code | Serializes local event rows to RFC5545 VEVENTs | none |
| 4 | Drift / new-since-last-sync | `watch --city sf --category cat-ai` | 7/10 | hand-code | Diffs latest sync snapshot vs prior (added/removed, guest/ticket/sold_out deltas) | Use for change-over-time across syncs. For a point-in-time list use `agenda`. |
| 5 | Multi-calendar aggregate compare | `calendars compare cal-a cal-b cal-c --window 14d` | 6/10 | hand-code | GROUP BY calendar_api_id over local events — per-calendar count + total guests | Use to compare activity across calendars. For one calendar's detail use `calendars get`. |

## Killed candidates
| Feature | Kill reason | Closest sibling |
|---|---|---|
| trending | static rank = one --sort on agenda/sql; velocity version IS watch | watch |
| cities | thin sql GROUP BY place; cross-city need met by agenda | agenda |
| hosts | no demand signal; sql GROUP BY host wrapper | calendars compare |
| conflicts | narrow single-persona; sql self-join recipe | agenda |
| digest | overlaps agenda/cities; pivot nice-to-have | agenda |
| available | one predicate; fold as agenda --available | agenda |
| event judge | subjective, no ground truth (verifiability fail) | — |
| recommend | similarity = LLM dependency | agenda |
| map render | scope creep (rendering layer); data version is absorbed `places map` | places map |
