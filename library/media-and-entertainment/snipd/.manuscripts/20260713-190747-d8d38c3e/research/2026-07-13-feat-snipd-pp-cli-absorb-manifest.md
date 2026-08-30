# Snipd CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

Snipd has **no CLI, no MCP, and no community wrapper** for snip content. The only programmatic exits
are the Obsidian export (this CLI's source) and Snipd→Readwise sync. Neither offers scoped/ranked/
snippet retrieval, so there is **no competitor feature surface to absorb**. The one absorbed row is the
raw upstream capability this CLI wraps:

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List episodes available upstream | Snipd Obsidian export `fetch-export-metadata` | (generated endpoint) export metadata | JSON, `--select`, agent-native; feeds incremental sync |

## Transcendence (only possible with our approach — the product)

The whole CLI is transcendence: a local SQLite+FTS mirror of your snips that the app, Readwise, and
Tana Outliner cannot match on ranked/snippet retrieval (proven in sessions 1/1bis). Theme spine:
**"Local snip corpus that compounds"** and **"Agent-native retrieval over your own snips."**

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Build/refresh the local corpus | `sync` | hand-code | Requires the labelled-delimiter export parse + typed SQLite upsert; the app has no export-to-query path | Use to pull or incrementally refresh your snips into the local mirror. Run before any search/filter/aggregate. Not a push to anywhere — it is a read-only pull from Snipd. |
| 2 | Ranked full-text search | `search` | hand-code | FTS5 porter index over title/note/quote/transcript with bm25 ranking + compact snippet rows; no Snipd/Readwise surface ranks or snippets | Use to find snips about a concept across every transcript, note, and quote. Returns ranked compact rows with deep-links. Plain stemmed terms, not `*` prefixes. |
| 3 | Pull the exact quote | `quote` | hand-code | The pull-quote (+ speaker) as a distinct unit — the thing Readwise concatenates into transcript prose and loses | Use to get the verbatim pull-quote and speaker for matching snips or a snip id. Do NOT use for the AI note or transcript; use `search`/`synthesize`. |
| 4 | Scoped synthesis feed | `synthesize` | hand-code | A deduped, ranked, snippet-only feed (title+note+quote, deep-link) scoped by query/show/topic, sized in KB for an agent to reason over — the shape Outliner overflowed on | Use to gather a compact evidence set for an agent to synthesize an answer across shows. Fetches transcripts only on demand. |
| 5 | Structured filter | `filter` | hand-code | Local join/filter across show, favorite, tag, date, duration, guest that no single upstream call provides | Use to slice the corpus by show/favorite/tag/date. Returns compact rows or counts. Do NOT use for free-text concepts; use `search`. |
| 6 | Corpus rollups | `aggregate` | hand-code | GROUP BY analytics (snips per show, per tag, favorites, episode density, timeline) over the local mirror | Use for counts and distributions across your corpus (snips per show, top tags, favorites). |
| 7 | Read-only SQL over the corpus | `sql` | (framework, over hand store) | Arbitrary SELECT analytics over the typed episodes+snips schema | Use for arbitrary read-only SQL when the canned commands don't fit. SELECT only. |
| 8 | Curated push to Tana Outliner | `push` | hand-code (OPTIONAL) | Emits the validated 1bis Tana-Paste schema (episode `#podcasts-episode` + snip `#highlight`, separated quote/transcript, canonical Source/Category/About, sanitized names) for a hand-picked subset | Use to weave a handful of curated snips into your Tana graph. Curated subsets only, never bulk. Not a sync. |

## Buildability tally
- Absorbed (generator-emitted): 1 (`export metadata`).
- Transcendence hand-code: 6 core (`sync`, `search`, `quote`, `synthesize`, `filter`, `aggregate`)
  + 1 framework-over-hand-store (`sql`) + 1 **optional** (`push`).

## Stubs
- None planned. `push` is either fully built this session or explicitly deferred (not shipped as a stub).

## Notes / open build decisions
- `sync` collides in name with the generator's framework `sync`. Plan: hand-author the real Snipd
  populate command; if the framework `sync` can't be cleanly repurposed, override/replace it (confirm
  mechanics against substack-reader, which named its populate command `archive` to sidestep this).
- Store schema is hand-built typed tables (episodes/snips/FTS5), not the generator's generic
  `resources` JSON table — mirroring substack-reader's `internal/store` extension.
