# Snipd CLI Brief

Third Printing Press "wrap an undocumented/internal API" CLI, after Medium Reader and Substack Reader.
Domain research was done across sessions 1 + 1bis (see `~/Cowork/projects/personal/snipd-cli/specs/`);
this brief is the build-driving synthesis, not fresh web research.

## API Identity
- **Domain:** Snipd — a mobile-first podcast app for capturing "snips" (highlighted moments with an
  AI-generated note, a pull-quote, and a transcript excerpt). No public REST API, no CLI, no MCP; full
  content normally requires the app.
- **Users:** the account owner (Maxime). This is a personal-knowledge tool over YOUR OWN snips, not a
  public data wrapper.
- **Data profile:** 107 episodes / 2,279 snips / 30 shows (8 MB). Each snip = title + AI note + quote
  (+ speaker) + transcript excerpt + timestamps + tags + deep-link. Rich text, small corpus.

## Access surface (verified, read-only, own account)
- Base `https://api.snipd.com/v1/public/api`; **Bearer** token from in-app pairing (env `SNIPD_TOKEN`).
- `GET /obsidian/fetch-export-metadata[?updated_after=]` → episode batches + ids + `total_snip_count`
  + `episodes_data`/`shows_data` (clean JSON). **Token verified LIVE 2026-07-13** (200, 107 eps).
- `POST /obsidian/export-episode-snips {episode_ids, episode_template, snip_template}` → **ZIP** of
  server-rendered markdown. Template tokens are an allowlist (`EP_TOKENS`/`SNIP_TOKENS`). Server-side
  rendering with labelled delimiters (`<<token>>…<</token>>` + `@@SNIP@@`) makes the parse robust.
- **Nothing richer is reachable** — transcripts/notes/quotes/deep-links/metadata all flow through this
  one sanctioned path; audio playback and the app's internal API are out of scope (`snipd-cli-access`).

## Reachability Risk
- **None.** Token live, endpoints stable, own-account read-only. No Cloudflare/TLS gate (plain Bearer).
- Incremental cursor: `fetch-export-metadata?updated_after=` + upsert keyed on `latest_snip_update_ts`.

## Competitive landscape (why absorb is ~empty)
- No CLI, no MCP, no community wrapper for Snipd content. The only programmatic exits are (a) this
  Obsidian export and (b) Snipd→Readwise sync. **Neither offers scoped, ranked, snippet retrieval.**
- Session 1/1bis proved the alternatives' ceilings: the Snipd app has no query surface; Readwise's
  push mangles structure (see `readwise-outliner-import-analysis.md`); Tana Outliner overflows context
  on free-text retrieval (~1.29 MB / 10 questions vs the corpus's 95 KB, no snippet/ranked mode).
- **Consequence:** this is a near-pure *transcendence* CLI. The value IS the local SQLite+FTS retrieval
  engine, not matching an incumbent feature set.

## Data Layer (canonical store = local SQLite, already realized)
- `episodes` (episode_id PK, title, show, show_id, author, guests, publish_date, ai_description,
  duration, url, snip_count, …).
- `snips` (snip_id PK, episode_id, show, episode_title, favorite, title, note, quote, transcript,
  start/end/duration, start_seconds, tags, url deep-link, audio_embed).
- `snips_fts` FTS5 porter over (title, note, quote, transcript). **Query hygiene: plain stemmed terms,
  not `*` prefixes** (porter stems the index — a raw prefix misses the stemmed root).
- Sync cursor: `updated_after` on metadata + upsert on `snip_id`/`episode_id`.

## Product Thesis
- **Name:** Snipd Reader (`snipd-pp-cli`).
- **Why it should exist:** Your snips are trapped in a mobile app. This turns 2,279 of them into a
  local, ranked, full-text-searchable corpus + MCP that an agent can actually reason over — search a
  concept across every transcript, pull the exact quote, synthesize across shows, all in ~KB not MB —
  and optionally weave hand-picked snips into your Tana knowledge graph.

## Build Priorities
1. **P0 foundation:** hand-built SQLite store (episodes + snips + FTS5) + `sync` (pull/incremental
   from the export API → parse labelled-delimiter MD → upsert).
2. **P2 transcendence (the product):** `search`, `quote`, `synthesize`, `filter`, `aggregate` over
   FTS/SQL; `sql` (read-only) framework command retained.
3. **MCP:** same commands mirrored as compact-row tools (search/quote/synthesize the headline agent
   surface).
4. **Optional:** `push` → curated Tana Outliner export (validated 1bis schema), or explicitly deferred.

## Auth
- `auth.type: bearer_token`, `Authorization: Bearer <SNIPD_TOKEN>`. Required (no keyless tier).
  Precedence: `SNIPD_TOKEN` env > config token file. Mask in all diagnostics; 0600 on any persisted copy.
