# japan-theme-parks-pp-cli Absorb Manifest (2026-10-09)

Sources searched: queue-times.com API page and public JSON; ThemeParks.wiki MCP servers (habuma/tpapi-mcp-server, mcp-server-theme-parks, Pipeworx ThemeParks, ParkAlert MCP); Raymond Camden's queue-times wait alert script; Shanghai Disney MCP (ticket sale-info pattern, other resort); official TDR ticket calendar and FAQ (calendar symbol aggregates all ticket types; sales open 2 months ahead at 14:00 JST); official USJ park-hour calendar; romualdoag/parksapi-mcp (USJ Hours endpoint). No queue-times-specific CLI/MCP and no TDR ticket-availability tool was found.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List parks with ids/timezone | queue-times /parks.json; ThemeParks.wiki MCP getAllParks | japan-theme-parks-pp-cli parks | Japan-only, per-park source coverage (live waits / hours / tickets known or unknown), JA/EN names |
| 2 | Live ride waits per park | queue-times queue_times.json; ThemeParks.wiki MCP getEntityLive; ParkAlert MCP | japan-theme-parks-pp-cli waits | Sorted, land grouping, source last_updated, "Powered by Queue-Times.com" attribution |
| 3 | Ride open/closed status | queue-times is_open | (behavior in japan-theme-parks-pp-cli waits) is_open field and --open-only | Closed vs unknown kept apart |
| 4 | Short-wait filter | Raymond Camden alert script; ParkAlert MCP | (behavior in japan-theme-parks-pp-cli waits) --max-wait N | Scriptable, typed exit codes |
| 5 | Park hours by date | ThemeParks.wiki MCP getEntityScheduleForDate; official sites | (behavior in japan-theme-parks-pp-cli dates) hours block per date and park | Official TDR hours (ticketPopup openTime); USJ and Fuji-Q from ThemeParks.wiki labelled third-party with fetched_at; null + reason past its horizon |
| 6 | TDR dated sale status (○/△/X/ー) | tokyodisneyresort.jp ticket calendar | (behavior in japan-theme-parks-pp-cli dates) tickets[] and --ticket/--status filters | Per ticket type (official symbol aggregates all types), JA+EN names, not-yet-on-sale vs sold out vs no online sale |
| 7 | TDR dated price tier | tokyodisneyresort.jp ticket calendar | (behavior in japan-theme-parks-pp-cli dates) prices_yen adult/junior/child per ticket and date | Machine-readable, comparable across dates |
| 8 | Local store / sync | Printing Press framework | (behavior in japan-theme-parks-pp-cli snapshot) local SQLite wait history | The generator emitted no sync command for this spec; snapshot writes the local store and typical/waits read it |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Cross-park date planner | dates | 10/10 | hand-code | Joins TDR JA+EN month pages (per-ticket status, prices, hours) with ThemeParks.wiki USJ/Fuji-Q hours over a date range; decodes five explicit states; USJ dated price shown as explicit unknown; rule-labeled sale-open time for not-yet dates | Use this command to compare dates across parks (sale status, price and hours in one row). Do NOT use this command for ride waits; use 'waits' or 'typical' instead. |
| 2 | Typical waits from local history | typical | 9/10 | hand-code | queue-times has no historical API; median/p75 by weekday x hour (Asia/Tokyo) with n, distinct days, first/last date; "insufficient" below --min-samples; no forecasts | Use this command for historical wait patterns by weekday and hour from local snapshots. Do NOT use this command for a live comparison on the park day; use 'waits' instead. |
| 3 | Wait snapshot recorder | snapshot | 8/10 | hand-code | Append-only history (dedupe on ride id + last_updated) for 5 Japan parks; cron-friendly; framework sync keeps only latest state | Use this command to record wait history for later use by 'typical' and 'waits'. Do NOT use this command to read current waits; use 'waits' instead. |
| 4 | Ride order now vs typical (folded into waits) | waits | 7/10 | hand-code | Joins live waits with local typical for current weekday/hour; live minus typical with n; "no history" when n = 0 | Use this command on the park day to decide which ride to queue for now (live wait against the local typical wait). Do NOT use this command for weekday or hour patterns; use 'typical' instead. |

Killed in brainstorm: day (dup of dates), sale-opens (field of dates), watch (poll loop), sellout-log (no demand), cheapest (sort of dates), crowd (forecast risk; park row of typical), down (subset of order), trip (invented forecast), usj-price (store behind queue + robot check), ics (no planning decision).

## Explicit unknowns (no command; shown as null + reason)
- USJ dated Studio Pass price and Express Pass availability (WEB ticket store is behind a waiting room and robot check; only "from" prices public).
- Fuji-Q Highland tickets, and Legoland Japan hours and tickets (not in the source list). Fuji-Q hours come from ThemeParks.wiki (Phase 1.9 amendment).
- queue-times crowd calendar predictions (HTML only; owner prefers links; not scraped).

## Gate decision (Phase Gate 1.5)
Approved 2026-10-09 by the orchestrator (user delegated absorb-gate approval to the orchestrator): option 1, approve with recommended cuts.

### Final approved command surface
| Command | Covers | Notes |
|---|---|---|
| japan-theme-parks-pp-cli parks | absorbed 1 | park ids, JA/EN names, source coverage per park, crowd-calendar link (link only, no scraping) |
| japan-theme-parks-pp-cli waits | absorbed 2, 3, 4; transcendence 4 (order folded in) | --open-only, --max-wait; typical/delta/n columns only when local history exists; meta attribution "Powered by Queue-Times.com" + https://queue-times.com/ |
| japan-theme-parks-pp-cli dates | absorbed 5, 6, 7 (tickets/hours cut, covered by --park/--ticket); transcendence 1 | USJ dated price and Express Pass = null with reason; sale_opens_at labelled as published rule; TDR parser fails loudly with coverage reason on structure change |
| japan-theme-parks-pp-cli snapshot | transcendence 3 | dedupe ride id + source last_updated; README documents an example cron line only (do not install cron/launchd) |
| japan-theme-parks-pp-cli typical | transcendence 2 | median/p75 by weekday x hour Asia/Tokyo, n, distinct days, first/last date, "insufficient" under --min-samples; no forecasts |
| japan-theme-parks-pp-cli doctor | framework | built-in |
| (no sync) | absorbed 8 | the generator emitted no sync command for this spec; local store is written by snapshot |

Cut: tickets, hours (into dates); order (into waits).

### Phase 1.9 amendment (user answer 2026-10-09)
- USJ and Fuji-Q hours in `dates` come from api.themeparks.wiki (third-party aggregator, labelled, fetched_at per value, "Powered by ThemeParks.wiki" credit); null with reason "beyond third-party source horizon" past its ~31-day schedule. USJ mobile-service endpoint not used (401, needs OAuth client token).
