## Customer model

**Persona A — "The local-scene tracker" (Omar-shaped)**

A long-time resident of a major metro who keeps a curated list of ~15 venues he cares about (theatres, opera houses, jazz clubs, mid-size rock rooms) and wants to know what's on at any of them in the next 60 days, deduplicated and bucketed by category.

- **Today:** 595-line bash `seattle-events` script loops over hardcoded venue IDs, calls `events.json?venueId=…&size=100` per venue, post-processes JSON in `jq` to dedup residencies, bucket by genre, strip On-Sale duplicates.
- **Weekly ritual:** Sunday morning re-run, paste output into Obsidian, share with family iMessage thread.
- **Frustration:** 16-night opera season shows as 16 near-duplicate rows; national tour at 3 venues shows 3×; classification names drift between requests; manual bucketing rules need constant editing.

**Persona B — "The tour-chasing fan"**

A super-fan who follows specific artists across cities and watches for on-sale dates the way other people watch stocks.

- **Today:** Manual website checks per artist + Reddit subs for presale leaks. Forgets which dates are on-sale vs still announced.
- **Weekly ritual:** Monday lunch sweep of 4-6 favorite artists, logs new tour dates in Notion, flags on-sales within 7 days.
- **Frustration:** No first-party "all dates for this artist sorted by on-sale" view. Reconciling `dates.status.code` + `sales.public.startDateTime` across 30+ stops is a hand job.

**Persona C — "The AI-agent operator" (Claude / MCP user)**

A developer who has wired Discovery into an LLM agent and asks natural-language event questions.

- **Today:** Thin MCP server returns raw Discovery JSON; LLM burns context parsing `_embedded` envelopes.
- **Weekly ritual:** Daily "what's on" queries from chat; occasional multi-event JSON blobs into a custom prompt.
- **Frustration:** 60+ fields per event, `_embedded` nesting, redundant images, classification arrays. Without `--select`/`--compact`, every agent call wastes thousands of tokens; without local cache, the same metadata is re-fetched 5× a day.

**Persona D — "The data-curious local-events writer"**

A newsletter / blog author who writes a "this weekend in [metro]" digest and wants raw inventory to slice — by genre, by venue size, by ticket price band.

- **Today:** One-off Python scripts dumping JSON; pandas notebook for slicing.
- **Weekly ritual:** Wednesday pull 7-day forward window, group by classification + venue, write digest by Friday.
- **Frustration:** Pagination (100/page, no reliable totals); computing "top 5 venues by event volume this week" requires reading the API like a database when it isn't one.

## Candidates (pre-cut)

| # | Candidate | Source | Verdict |
|---|-----------|--------|---------|
| C1 | `events upcoming --venues @file --days N` — fan-out across venue IDs, merged deduped list | (a) Persona A, (e) user vision | **Keep** — generic primitive; powers Seattle recipe without baking it in. |
| C2 | `events residency --collapse-window 28d` — collapse same-name+venue runs to first/last/count | (b) service pattern, (a) Persona A | **Keep** — cross-row local SQLite; no API endpoint does this. |
| C3 | `events tour <attraction>` — every event for an attraction with on-sale flag | (a) Persona B, (b) tours/on-sale | **Keep** — joins local events × attractions, parses status_code + sales window. |
| C4 | `events on-sale-soon --window 7d` — local query for events going on-sale soon | (a) Persona B, (b) on-sale pattern | **Keep** — pure date filter on synced rows; no competitor. |
| C5 | `venues nearby --lat --lon --radius` | (b) generic | **Kill (wrapper)** — pure rename of `/venues.json?latlong=…` |
| C6 | `events by-classification --window` — local aggregation with counts + examples | (b) service, (c) cross-entity, (a) Persona A | **Keep** — joins events × classifications locally. |
| C7 | `attractions related <id>` — co-occurrence over co-billed events | (c) cross-entity | **Kill** — co-occurrence inference is fuzzy; weekly-use speculative. |
| C8 | `events watchlist <name>` — persistent named watchlists in local SQLite | (a) Persona A, (e) user vision | **Keep** — generic, persistent, foundational for any metro recipe. |
| C9 | `events dedup --strategy …` — stream-shaped dedup transform | (a) Persona A, (e) user vision | **Keep** — mechanical, composable. |
| C10 | `events what-this-weekend --metro <dma>` | (b) generic | **Kill (wrapper)** — collapses to one `events search` call. |
| C11 | `events at <venue>` | (b) generic | **Kill (wrapper)** — `events search --venue-id` already does it. |
| C12 | `events brief --window 7d --metro <dma>` — markdown what's-on report | (a) Persona D, (a) Persona A, (e) user vision | **Keep** — mechanical group-by + format. |
| C13 | `attractions tour-radius <id>` | (a) Persona B regional fans | **Kill (wrapper-ish)** — `events search` accepts attractionId + latlong + radius. |
| C14 | `events price-bands --metro <dma> --window N` — distribution of priceRanges by band | (a) Persona D, (b) priceRanges pattern | **Keep** — pure local SQLite aggregation. |
| C15 | `classifications drift --since <date>` — diff sync snapshots | (b) service, (c) | **Kill (low frequency)** — taxonomy moves 1-2×/yr. |
| C16 | `events summarize` — LLM what's-on summary | (a) Persona C | **Kill (LLM dependency)** — reframed as `events brief`. |
| C17 | `agent-suggest <NL query>` — parse NL into CLI invocation | (a) Persona C | **Kill (LLM dependency)** — MCP tree + `--help` is the answer. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Multi-venue watchlist sweep | `events upcoming --venues @venues.txt --days 60` | 9/10 | Read venue IDs (file/CSV/stdin), filter local synced events by venueId IN (…) + date window, dedup on event ID, sort by date. Persona A weekly. | Lobster `seattle-events` skill is 595 lines that bake this loop in bash; no competitor offers multi-venue fan-out. |
| 2 | Residency collapse | `events residency --window 28d` | 8/10 | SQL `GROUP BY name, venueId, classification` over local events with configurable window, returns first_date, last_date, night_count, id_list. Persona A weekly. | Brief calls out 16-night opera seasons; Table-Stakes section lists "Date-range collapsing for residency runs" as a competitor gap. |
| 3 | Attraction tour view with on-sale flag | `events tour <attraction>` | 8/10 | Filter local events by attractionId, sort by dates.start.dateTime, project city/venue/status_code/sales.public.startDateTime; flag rows where on-sale is within 7d. Persona B weekly. | Brief documents `dates.status.code` and `sales.public.startDateTime` as parseable; Persona B "where can I see artist Y" is Workflow #2. |
| 4 | On-sale-soon scanner | `events on-sale-soon --window 7d` | 7/10 | `WHERE sales.public.startDateTime BETWEEN now AND now+window AND status_code='offsale'`. Persona B weekly. | Brief explicitly notes status_code + sales as parseable signals; canonical "presale watch" gap. |
| 5 | Classification bucketing | `events by-classification --window 60d` | 7/10 | Local join events × classifications, GROUP BY segment, genre with COUNT + 3 examples per bucket. Personas A + D weekly. | Brief Workflow #5 + Table-Stakes gap "Cross-classification aggregation in one call." |
| 6 | Named persistent watchlists | `events watchlist {save\|run\|ls\|rm} <name>` | 8/10 | New `watchlists(name, venue_ids, attraction_ids, segments, dma_ids)` table in same SQLite store; `run` applies a named filter set to current synced events. Generic. Personas A + D weekly. | User Vision section: "watchlist composition (curated venue/attraction allowlists)" called out as primitive to ship. |
| 7 | Stream-shaped dedup | `events dedup --strategy name-venue-date\|tour-leg` | 6/10 | Reads JSON event array from stdin or local result, applies pure mechanical dedup, writes to stdout. Composable. Personas A + C weekly. | Brief Data Layer mentions "deduplicate across calls"; user-vision recipe needs dedup heuristics. |
| 8 | Markdown what's-on brief | `events brief --window 7d --metro <dma>` | 7/10 | Local query → group by date → group by venue → format markdown table with classification + price band. Pipes into Obsidian/iMessage/agent. Personas A + C + D weekly. | Persona D's workflow; user vision calls for newsletter/Obsidian-friendly composition. |
| 9 | Price-band distribution | `events price-bands --metro <dma> --window 30d` | 5/10 | `priceRanges.min` bucketed in SQL: <$50, $50-100, $100-200, $200+, grouped by classification. Persona D weekly. | Brief Data Layer notes priceRanges shape; Persona D pain documented. |

### Killed candidates

| Killed feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C5 `venues nearby` | Pure rename; rubric wrapper-kill. | #1 `events upcoming --venues` |
| C7 `attractions related <id>` | Verifiability flagged — co-occurrence is fuzzy; weekly-use speculative. | #3 `events tour` |
| C10 `events what-this-weekend` | One-shot wrapper over `events search` with date math; doc as recipe. | #8 `events brief` |
| C11 `events at <venue>` | Pure rename of `events search --venue-id`. | #1 `events upcoming --venues` |
| C13 `attractions tour-radius` | Wrapper-kill — events search already accepts attractionId + latlong + radius. | #3 `events tour` |
| C15 `classifications drift` | Weekly-use fails — taxonomy changes 1-2×/yr. | #5 `events by-classification` |
| C16 `events summarize` | LLM-dependency kill per rubric. | #8 `events brief` (emits markdown the LLM can summarize) |
| C17 `agent-suggest <NL query>` | LLM-dependency kill; MCP tree + `--help` is the right answer. | #8 `events brief` + generator-provided MCP server |
