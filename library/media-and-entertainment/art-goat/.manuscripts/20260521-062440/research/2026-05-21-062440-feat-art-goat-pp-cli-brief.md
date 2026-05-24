# art-goat CLI Brief

## API Identity
- **Domain:** Visual art + astronomy aggregator. art-goat is not a "CLI for one museum" — it's a contemplative daily-practice tool with museum and astronomy APIs as substrate.
- **Users:** Operators and agents who want a daily encounter with a single piece of art for extended attention. Adjacent: librarians, art-curious developers, researchers, anyone with a meditation-adjacent ritual.
- **Data profile:** CC0 / public-domain images and metadata from the Art Institute of Chicago (~132k artworks) plus NASA's Astronomy Picture of the Day archive (~10k entries since 1995). Unified into one local SQLite `works` table.

## Reachability Risk
- **None.** AIC `GET /artworks?limit=1` → 200 (verified 2026-05-21). NASA APOD `GET /planetary/apod?api_key=DEMO_KEY` → 200 (verified 2026-05-21). Both are stable government / institutional APIs with documented contracts.

## Top Workflows
1. **`art-goat sit [id]`** — single piece, fixed-duration timer (default 5 min, configurable 10/20), HTML emit to browser with image + curator text, then capture reflection in terminal. Atomic primary; everything else exists to serve this.
2. **`art-goat today`** — curated daily pick using anti-repeat + journal-aware diversity. One piece, one "why this today" one-liner. Same flow as `sit` but pick is opinionated.
3. **`art-goat presence`** — random piece + reflection prompt, lighter than `sit` (no timer).
4. **`art-goat journal write|stats|search|export`** — review and reflect on your practice. Stats are reframed away from streak-as-headline toward source breadth, medium variety, period coverage, mood drift.
5. **`art-goat search 'water'`** / **`art-goat browse --medium=woodblock`** / **`art-goat similar <id>`** — federated cross-source browsing as substrate; not the headline.

## Table Stakes
- AIC API has 103 endpoints (artworks, artists, agents, articles, exhibitions, places, categories, etc.) — generator scaffolds typed clients for all of them.
- NASA APOD has one endpoint (`GET /planetary/apod`) with date params — trivially wrapped.
- Both: list/get/search, JSON output, pagination via `?page=N&limit=M` (AIC) or `?start_date=&end_date=` (APOD).

## Data Layer
- **Primary entity:** unified `works` table with `(id, source, source_id, title, creator, creator_canonical, date_text, date_start, date_end, medium, classification, period, culture_region, description, image_url, thumbnail_url, license, source_url, raw_json, synced_at)`. Lossy across sources but the user doesn't care about source taxonomy.
- **Journal entity:** `sits` table `(id, started_at, ended_at, work_id, duration_seconds, prompt, reflection, mood?, tags?, mode)`.
- **Sync cursor:** AIC has `last_updated_source`; APOD has natural date range. Curated default (~10-30k AIC highlights + full APOD archive); `sync --full` opt-in for ~150k AIC works.
- **FTS5:** virtual table over `works_fts(title, creator, description, medium, period)` and `sits_fts(reflection, prompt, tags)`.

## Codebase Intelligence
- AIC API is well-maintained, OpenAPI 3.1 spec, 132k artworks indexed. Anonymous reads. Generated description by Laravel `docs:openapi` artisan command (current spec version 1.14, dated 2026-03-23).
- NASA APOD is a single-endpoint public API at `api.nasa.gov/planetary/apod`. `DEMO_KEY` works (rate-limited, ~30 requests/IP/hour). User-supplied free key removes the rate limit.
- No competing CLI tool in this exact shape (multi-source contemplative aggregator). The Met has a popular Python client (`metmuseum/openaccess`); AIC has none widely-adopted. APOD has many wrappers, none contemplative.

## User Vision
From the grilled design (`art-goat-design.md` in memory):

> "A contemplative daily art practice with the world's museums as your gallery."

Headline commands `sit`, `today`, `presence` are the soul. Federated `search`/`browse`/`similar` are substrate. The HTML+description+`--inline` display strategy, lazy on-demand auth, algorithmic `today` (anti-repeat + journal-aware diversity + "why this today"), SQLite canonical + Markdown mirror journal, and tracked-but-reframed habit metrics are all locked from the grill session.

## Source Priority (combo CLI)
- **Primary:** AIC — official OpenAPI 3.1 spec, anonymous, 132k works with rich CC-By descriptions, generator scaffolds against this.
- **Secondary:** NASA APOD — one endpoint, anonymous via DEMO_KEY, complements AIC with astronomy / scale dimension. Hand-authored client in `internal/source/apod/`.
- **Economics:** Both primary and secondary are free. No paid keys in MVP.
- **Inversion risk:** None — AIC clearly larger and richer; APOD is the contemplative astronomy companion, not a competing primary.

## MVP scope (reduced from locked 8-source design)
This print is **MVP-first** per user decision after scope-reality check. Locked design called for 8 sources (AIC, Met, Cleveland, APOD, Te Papa, NPM Taiwan, Rijksmuseum, Smithsonian); MVP ships **AIC + APOD only**. Other 6 sources are mechanical additions deferred to follow-up PRs.

This is an architectural decision, not a feature cut: the unified-schema, lazy-auth, journal-mirror, and HTML-emit substrate is identical between MVP and full design. Each additional source is a new file in `internal/source/<name>/` implementing the same `Sync(ctx) ([]Work, error)` interface, plus a registry entry.

## Product Thesis
- **Name:** art-goat (echoes coffee-goat lineage; "goat" = multi-source aggregator pattern)
- **Why it should exist:**
  - No CLI exists that ties museum APIs to a daily contemplative practice
  - Sitting with one piece for ~5 min and writing a reflection is a real practice; no tool currently supports it
  - Aggregation matters because day-after-day variety prevents the practice from going stale
  - AIC + APOD together cover both "old/visual masters" and "cosmic scale" — two distinct contemplative modes
- **What makes it earn its print:** the contemplative spine (`sit` / `today` / `presence` / `journal`) is **fully novel** — no existing CLI does this, and the pattern is hand-authored using the unified store populated by real API calls (anti-reimplementation-safe).

## Build Priorities
1. **Phase 2 — generate AIC scaffold.** `printing-press generate` writes the AIC client, store, MCP, doctor, etc. AIC endpoint mirrors get hidden behind unified commands via post-generation annotation pass.
2. **Phase 3a — APOD source client (hand-authored).** `internal/source/apod/client.go` + registry wire-up + unified schema migration.
3. **Phase 3b — contemplative spine (hand-authored).** `sit`, `today`, `presence`, `journal {write,stats,search,export}` in `internal/cli/`. Includes prompts library at `internal/cli/prompts.go` with `// pp:novel-static-reference`. All interactive commands hidden from MCP via `mcp:hidden=true` and short-circuit on `cliutil.IsVerifyEnv()`.
4. **Phase 3c — cross-source unified commands.** `browse`, `random`, `compare`, `artist`, `similar`, `sources` over the unified `works` table. All read-only, MCP-exposed with `mcp:read-only=true`.
5. **Phase 3d — HTML emit + description-mode fallback + sync orchestrator.** Auto-detect non-graphical terminals; `--inline` opt-in. Sync command iterates registry. Lazy on-demand auth for paid sources (none in MVP; the scaffolding exists for future).
6. **Phase 4 — shipcheck + reviews.**
7. **Phase 5 — live dogfood** against real AIC and APOD endpoints.
8. **Phase 5.5 — polish.**
9. **Phase 6 — promote and offer publish.**

## What this CLI is not
- Not a Met clone, not an AIC standalone CLI, not "everything published by Smithsonian."
- Not a meditation app — there are no breathing animations, no notifications, no streaks-as-headline.
- Not a content stream — `sit` surfaces one piece for extended attention; the opposite of a feed.
- Not authenticated v1 — MVP is fully anonymous via AIC's open API + APOD's DEMO_KEY. The lazy-auth scaffolding is built but no keyed sources ship in MVP.
