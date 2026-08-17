# Ticketmaster Discovery v2 — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search events by keyword/date/location/classification | delorenj/mcp-server-ticketmaster, npm:ticketmaster | `events search` with all spec params | Offline FTS5 via local store, --json/--select/--compact, typed exits |
| 2 | Get event by ID | All competitors | `events get <id>` | Cached in local store |
| 3 | Get event images by ID | spec /events/{id}/images | `events images <id>` | — |
| 4 | Search venues | All competitors | `venues search` | Offline FTS5 |
| 5 | Get venue by ID | All competitors | `venues get <id>` | Cached |
| 6 | Search attractions | All competitors | `attractions search` | Offline FTS5 |
| 7 | Get attraction by ID | All competitors | `attractions get <id>` | Cached |
| 8 | Classification taxonomy (list + get) | All competitors | `classifications list/get`, plus genres/segments/subgenres get | Cached locally; one-shot reference data |
| 9 | Typeahead suggest | All competitors | `suggest <query>` | — |
| 10 | Date range + radius + DMA filters | All competitors | First-class flags on `events search` | — |
| 11 | Local SQLite store + sync | NONE (no competitor) | `sync` syncs events/venues/attractions/classifications | Foundation for cross-entity queries |
| 12 | Free-text search across local store | NONE | `search <term>` queries all FTS5 indexes | Offline, regex-capable |
| 13 | SQL pass-through | NONE | `sql "SELECT ..."` (SELECT-only) | Composable, agent-friendly |
| 14 | Health/doctor (auth + reachability) | NONE | `doctor` | — |
| 15 | Agent context endpoint | Generator default | `agent-context` | — |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Multi-venue watchlist sweep | `events upcoming --venues @venues.txt --days 60` | 9/10 | Requires local store + fan-out across venue IDs; no Discovery endpoint accepts a venue ID list. |
| 2 | Residency collapse | `events residency --window 28d` | 8/10 | Cross-row local SQL GROUP BY name+venueId; no API endpoint collapses residencies. |
| 3 | Attraction tour view with on-sale flag | `events tour <attraction>` | 8/10 | Joins local events × attractions, parses dates.status.code + sales.public.startDateTime to flag on-sale-soon. |
| 4 | On-sale-soon scanner | `events on-sale-soon --window 7d` | 7/10 | Local date filter on synced rows; canonical presale watch gap. |
| 5 | Classification bucketing | `events by-classification --window 60d` | 7/10 | Local join events × classifications, GROUP BY segment/genre with counts + examples. |
| 6 | Named persistent watchlists | `events watchlist {save\|run\|ls\|rm} <name>` | 8/10 | New `watchlists` table in local SQLite; generic primitive for any metro/genre. |
| 7 | Stream-shaped dedup | `events dedup --strategy name-venue-date\|tour-leg` | 6/10 | Mechanical filter from stdin or local result; composes with any upstream command. |
| 8 | Markdown what's-on brief | `events brief --window 7d --metro <dma>` | 7/10 | Local query → group by date → venue → format markdown. Pipes into Obsidian/agent. |
| 9 | Price-band distribution | `events price-bands --metro <dma> --window 30d` | 5/10 | Local SQL bucketing of priceRanges.min by classification; no API analog. |

## Killed candidates

| Killed feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `venues nearby --lat --lon` | Pure rename of `/venues.json?latlong=`. | #1 events upcoming |
| `attractions related` | Co-occurrence inference fuzzy; weekly-use speculative. | #3 events tour |
| `events what-this-weekend` | Wrapper over events search + date math. | #8 events brief |
| `events at <venue>` | Pure rename of `events search --venue-id`. | #1 events upcoming |
| `attractions tour-radius` | events search already accepts attractionId+latlong+radius. | #3 events tour |
| `classifications drift` | Weekly-use fails; taxonomy changes 1-2×/yr. | #5 events by-classification |
| `events summarize` (LLM) | LLM-dependency kill per rubric. | #8 events brief |
| `agent-suggest <NL>` | LLM-dependency kill; MCP tree is the answer. | #8 events brief + MCP |

## Notes
- Seattle-specific commands intentionally NOT in the manifest. The README will ship a worked Seattle recipe using only the generic novel features (#6 `watchlist save seattle` from a venue ID file + #1 `events upcoming` or #2 `events residency`).
- No stubs. Every novel feature is shipping-scope.
