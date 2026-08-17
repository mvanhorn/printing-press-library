# Ticketmaster (Discovery v2) CLI Brief

## API Identity
- **Domain:** Live entertainment discovery — concerts, theatre, sports, comedy. Read-only. Ticketing/checkout is a separate Commerce API (out of scope).
- **Users:** Event-aggregators, ticket-broker tooling, personal "what's on near me" scripts, AI agents asking "is there a show I'd like at venue X this month".
- **Data profile:** Three high-gravity entities — `events` (the most queried), `venues` (the join key in most user workflows), `attractions` (artists/teams/touring shows). Two reference entities — `classifications` (segment/genre/subgenre taxonomy) and `suggest` (typeahead).
- **Auth:** `?apikey=<consumer-key>` query parameter. Free Discovery tier: 5000 req/day, 5/sec. Consumer Secret is not used for Discovery. Spec on apis.guru declares no `security` (this is an enrichment target before generation).
- **Base URL:** `https://app.ticketmaster.com/discovery/v2`
- **Spec:** OpenAPI 3.0, 13 paths / 13 GET ops, ~96 KB. Read-only.

## Reachability Risk
- **None.** API is publicly documented, openly available with a free consumer key, no bot protection, no JS challenges. Long-lived ecosystem (Lobster's seattle-events skill has been using it without issue).

## Top Workflows
1. **"What's happening at venue X in the next N days?"** — per-venue event listing, sorted by date, optionally filtered by classification (music/theatre/sports/comedy).
2. **"Where can I see artist Y live?"** — attraction → events join, often across markets.
3. **"What concerts are in Seattle / Boston / NYC this weekend?"** — DMA or geo-radius search with classification filter.
4. **"Look up a venue / artist / event by ID or name"** — typeahead via `/suggest`, then drill into `/venues/{id}` or `/attractions/{id}` for metadata (capacity, social links, accessible-info, images).
5. **"What classifications/genres exist in the taxonomy?"** — usually a one-shot reference query that consumers cache locally.

## Table Stakes (from competitor scan)

Across the four real MCP servers and three SDK wrappers reviewed, every tool provides:

- `events search` — keyword + city/state/country/postalCode + radius + date-range + classification + venueId + attractionId + sort
- `events get` — by event ID (returns prices, seatmap, accessibility info, images, attractions, venues)
- `venues search` — keyword + city/state + country
- `venues get` — by venue ID
- `attractions search` — keyword + classification + segment
- `attractions get` — by attraction ID
- `suggest` — typeahead across all three entities
- `classifications list/get` — full taxonomy walk

What competitors **do not** do:
- Local SQLite store with sync + offline search
- Cross-classification or cross-DMA aggregation in one call
- Date-range collapsing for residency runs (a 16-night opera "season" shows as 16 rows, not 1)
- Watchlist composition (curated venue/attraction allowlists)
- Agent-native output (`--json --select`, `--compact`, typed exit codes, `--dry-run`)
- Rate-limit-aware retry with backoff against the 5/sec ceiling

## Data Layer

Worth persisting locally — sync via `sync` command, query via `sql`/`search`:
- **Primary entities:** `events`, `venues`, `attractions`, `classifications` (segments/genres/subgenres)
- **Sync cursor:** `events` use `dates.start.dateTime` + event `id`; venues/attractions are slowly-changing reference data — sync less often.
- **FTS5 indexes:** events (name + venue name + city), venues (name + city + aliases), attractions (name + segment + genre).
- **Why:** Discovery's per-page cap is 100 and the API does NOT return totals reliably across deep pagination. Local store lets users compose multi-venue queries, deduplicate across calls, and run rich filters offline.

## Codebase Intelligence (from npm/PyPI/GitHub scan)
- **Source pattern:** Every wrapper bakes the same conversion: `?apikey=...&keyword=...&size=100&page=N`. `dmaId` is the cheapest market filter; `latlong` + `radius` is the most precise.
- **Auth pattern:** Always `apikey` as a query parameter — no SDK uses Authorization headers. Token format is a 32-char alphanumeric consumer key. Env var convention across wrappers is `TICKETMASTER_API_KEY` or `TM_API_KEY`.
- **Rate limit behavior:** Returns HTTP 429 with `Retry-After` header. Reference SDK `arcward/ticketpy` does retry with exponential backoff.
- **Data model quirks:**
  - `_embedded` envelope wraps the entity lists (`_embedded.events`, `_embedded.venues`, etc.).
  - Events have `dates.start.dateTime` (UTC ISO) AND `dates.start.localDate` + `dates.start.localTime` — local times come without timezone, so DST-correct display requires the venue timezone.
  - `priceRanges` is often missing on resale or dynamic-priced events.
  - `classifications` is an array; the first entry is canonical.
  - `_embedded.venues[0].name` is the displayable venue; venue IDs (`KovZ917Ahkk` shape) are stable across requests.

## User Vision

User direction (2026-05-11): keep the printed CLI **generic** — no personal/location-specific opinion in commands, flags, or NOVEL helpers. Document opinionated use cases (curated venue watchlists, categorization rules) as README/SKILL recipes showing composition with primitives. Real-world worked example to include: a Seattle "what's at our 15 watch-listed venues in the next 60 days" recipe matching Lobster's `seattle-events` skill behavior, using only `events search` + `--select` + dedup heuristics.

## Product Thesis
- **Name:** `ticketmaster-pp-cli` (binary), package `media-and-entertainment/ticketmaster/`
- **Why it should exist:** Every existing tool is either an MCP-server wrapper (each is good but stops at "agent calls API"), a stale npm/Python SDK without a CLI surface, or a generated SDK with no opinionated UX. **No one ships a Go CLI for Discovery v2 with local SQLite + offline FTS + cross-venue aggregation + agent-native output.** The published-library entry will be the first single-binary, install-once, no-runtime-deps CLI for this API.

## Build Priorities
1. **P0 Data layer:** events / venues / attractions / classifications tables + FTS5 + `sync` + `search` + `sql` (all generator-provided).
2. **P1 Absorb:** Every Discovery v2 endpoint as a command (13 endpoints → ~10 commands after collapsing get/list pairs); MCP server mirrors the Cobra tree. (Generator handles this from the spec.)
3. **P1 Auth + DX:** `apikey` enrichment in spec, `--json --select --compact --csv`, `--dry-run`, typed exit codes, `doctor` (auth + reachability + rate-limit headroom), agent-context endpoint.
4. **P2 Transcendence (hand-built):** Local-store-powered cross-venue / cross-classification / cross-DMA aggregation, date-range collapsing (residency dedup), watchlist composition primitive, agent-natural search ranking. **Plus a documented Seattle-style recipe in README/SKILL — using only the generic commands**, no Seattle-specific NOVEL command in the binary.
5. **P3 Polish:** README recipes, MCP tool descriptions for the cobratree mirror, agent triggers ("what concerts in Seattle this weekend").
