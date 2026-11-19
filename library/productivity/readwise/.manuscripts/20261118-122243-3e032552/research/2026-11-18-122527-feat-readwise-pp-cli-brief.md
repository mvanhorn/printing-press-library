# Readwise CLI Brief (reprint on press 4.35.6)

Sources are cited inline. Docs captured 2026-11-18 via `fetch-docs.sh`. Companion file: `2026-11-18-122527-readwise-spec-drift.md`.

## API Identity
- Domain: personal reading and knowledge capture. Two REST APIs share one token. **Readwise v2** (`https://readwise.io/api/v2`) holds highlights, books/sources, tags, the daily spaced-repetition review, and the bulk `export`. **Reader v3** (`https://readwise.io/api/v3`) holds Reader documents (articles, PDFs, emails, RSS, tweets, videos), Reader tags, and Reader highlights and notes, which are themselves documents with `parent_id`. Docs: https://readwise.io/api_deets and https://readwise.io/reader_api.
- Users (substantive):
  1. **The heavy personal reader / knowledge worker.** Has thousands of highlights from Kindle, articles and podcasts, plus a Reader inbox of hundreds of unread items. They want to find what they already read on a topic, clear their backlog, and avoid re-saving things. The web UI is per-item and the list APIs are throttled to 20 requests/min, so a library this size is painful to query live.
  2. **The PKM / note-taking exporter.** Pipes highlights into Obsidian, Notion, Logseq or Markdown archives. Needs incremental, deletion-aware sync (`updatedAfter`, `includeDeleted`) and a verifiable backup. Today they rely on the single `export` endpoint or the Obsidian plugin, with no local diffable mirror.
  3. **The agent/automation builder** (Claude Code, Codex, cron, Zapier-style flows). Needs deterministic `--json`, `--dry-run`, typed exit codes, bounded MCP output, and safe write paths: save URL to Reader, create or tag highlights, bulk move or archive up to 50 documents, with no accidental destructive deletes.
  4. **The inbox triager.** Runs a daily or weekly pass: what is new, what is stale, what fits in 20 minutes, what to archive. Official tools do this through an LLM conversation. This user wants a fast, offline, repeatable answer.
- Data profile: highlights and books are integer-ID; Reader documents are string ULID-style IDs. Highlights are small text rows. Reader documents can be large (word_count, optional `html_content`). Tags are per-highlight, per-book and per-Reader-document (different shapes: `{id,name}` vs `{key,name}` vs an object keyed by tag key). Everything has an update timestamp (`updated`, `updated_at`), so incremental sync is natural. Reading data is personal, so the store must stay local.
- Homepage: https://readwise.io (Reader at https://readwise.io/read). Evidence: docs pages above return 200.

## Reachability Risk
- **Low.** Official, documented, token-based REST API. Live probe 2026-11-18: `GET https://readwise.io/api/v2/auth/` with the operator's token returned **HTTP 204** (expected per docs). Without a token it returns 401.
- No bot blocking seen: all docs and API calls return 200/204 with plain curl.
- Rate limiting is the real hazard. v2 default is 240/min per token, but Highlight LIST and Book LIST are 20/min. v3 limits: save 50/min, update 50/min, list 20/min, bulk_update 20/min, delete 20/min, tags 20/min. 429 responses carry `Retry-After` (seconds). Evidence of pain in the wild: `arnaldo-delisio/readwise-mcp-enhanced#3` (open, 2025-11-21) "readwise_topic_search ... consistently hits rate limits even with minimal usage" (https://github.com/arnaldo-delisio/readwise-mcp-enhanced/issues/3). Fan-out wrappers burn the 20/min list budget.
- 403 evidence (not the API itself): `readwiseio/readwise-mcp#3` (open since 2025-09-01) reports "403 error in Claude" for the official MCP despite a paid plan and valid token (https://github.com/readwiseio/readwise-mcp/issues/3). That repo is the older MCP, and the current hosted server is https://mcp2.readwise.io/mcp. The 403 relates to the MCP wrapper and Claude client, not the REST API we call. No "deprecated" or "blocked" issues found for the REST API. One doc-level deprecation signal: `location` was formerly `triage_status` (https://readwise.io/reader_api).
- Tier/permission hints from 4xx body: none observed. `GET /review/` returns 404 `{"detail": "Review not found."}` when no review exists, which is a normal state.
- Probe-safe endpoint used: `GET /api/v2/auth/` (204, no body).
- Mitigation to build in: all bulk reads go through the local store (`sync` using `export` + `v3/list`), never live `list` fan-out. A shared token-bucket limiter keyed per endpoint family. Honor `Retry-After`.

## Top Workflows
1. **Mirror once, query forever.** `sync` pulls v2 `export` (cursor paged, `updatedAfter` incremental, `includeDeleted=true` to propagate deletions) and v3 `list` (cursor, `updatedAfter`) into SQLite. `search` runs FTS across highlights, books and documents offline. This avoids the 20/min LIST wall.
2. **Topic dossier.** `ask "<topic>"` assembles every matching highlight and document passage into source-attributed, note-ready blocks (the prior CLI's flagship). The official MCP/CLI does hybrid vector search server-side but does not return a local, composable dossier.
3. **Inbox triage and backlog budgeting.** `backlog --budget 30m`, `read-next`, `triage stale`: unread by location, minutes (from `word_count` / `reading_time`), age buckets, abandoned (`reading_progress` > 0, old `last_opened_at`), orphan sources (zero highlights). Then act: bulk move or archive up to 50 docs via `bulk_update` (handle 207 per-item failures).
4. **Capture and annotate (write paths, live-tested).** Save URL or HTML to Reader (`/save/`, 50/min, 201 vs 200-exists); create highlights (v2 `POST /highlights/`, idempotent upsert by title/author/text/source_url, or `highlight_url`); create a Reader highlight on a document (`/save/` with `parent_id` + `content`); tag highlights, books and documents; update Reader doc location/seen/notes/reading_progress; mark the daily review complete.
5. **Digest, dedupe, connections, verifiable backup.** `digest` (everything highlighted in a window across sources), `dedupe` (same article twice, or Reader doc vs classic source), `connections` (non-obvious links via shared tags/terms/authors), `mirror` (incremental diffable export with a verifiable manifest).

## Table Stakes
Drawn from the official CLI (https://readwise.io/cli, npm `@readwise/cli` 0.5.11, MIT, TypeScript, repo `readwiseio/readwise-cli`) and official MCP tool list (https://readwise.io/mcp):
- Highlights: search, list, create, update, delete, daily review (`readwise_search_highlights`, `_list_`, `_create_`, `_update_`, `_delete_`, `_get_daily_review`).
- Reader documents: search, list (location/category/tag), create (save URL or HTML), get details with Markdown content, move documents, bulk edit metadata (50 at a time), export ZIP of Markdown.
- Reader tags: list, add, remove on documents and on highlights. Reader highlights: get, create by exact text, set notes.
- Readwise Books (shelves, ISBN/ASIN, ratings, reading sessions): new product area exposed in the MCP/CLI (`books_list_books`, `books_create_books`, `books_update_books`, `books_delete_books`). **Not in the public REST docs** (no `/books/` shelf endpoints beyond v2 `books`). Out of scope for this reprint; record as a gap.
- Auth: `readwise login` (browser OAuth) or `login-with-token`. We keep `READWISE_API_TOKEN` as the env var.
- Competitors beyond official: `arnaldo-delisio/readwise-mcp-enhanced` (69 stars; unifies Reader + Highlights with text processing), `edricgsh/Readwise-Reader-MCP` (47 stars; Reader only), `readwiseio/readwise-mcp` (152 stars; v2 highlights, older, last push 2026-03-14), `IAmAlexander/readwise-mcp` (24), `Nardjo/readwise-cli` (0). PyPI: no maintained client found; the `readwise` package on npm is an unrelated Ghost theme. Community clients: `tcnksm/go-readwise-reader` (Go, 6), `Scarvy/readwise-reader-api` (JS, 13).
- **Top 2 competitors:** (1) the official **Readwise MCP + CLI** (hosted `https://mcp2.readwise.io/mcp`, stdio via `npx -y mcp-remote ...`; semantic/hybrid search, Reader + Highlights + Books; skills for triage, feed catchup, quiz, now-reading from https://github.com/readwiseio/readwise-skills, 329 stars). (2) **readwise-mcp-enhanced** (community; topic search, context optimization).

## Data Layer
- Primary entities: `highlights` (id, text, note, location, location_type, color, highlighted_at, created_at, updated, book_id, external_id, end_location, url, readwise_url), `books` (id, title, author, category, source, num_highlights, last_highlight_at, updated, cover_image_url, source_url, asin, document_note), `documents` (Reader: id, url, source_url, title, author, category, location, tags, site_name, word_count, reading_time, listening_time, created_at, updated_at, saved_at, last_moved_at, first_opened_at, last_opened_at, reading_progress, parent_id, notes, summary), `highlight_tags`, `book_tags`, `reader_tags` (key, name), plus `watch_saved_search`, `watch_seen`, `webhook` (CLI-side). Reader highlights and notes live in `documents` with `parent_id`; model the self-reference. Optional on demand: `html_content` (only `withHtmlContent=true`). Never persist `raw_source_url` (S3 link valid 1 hour).
- Sync cursor: two cursors. (a) v2 `export`: `pageCursor`/`nextPageCursor` within a run; `updatedAfter` (ISO 8601, UTC) as the persisted high-water mark between runs; `includeDeleted=true` so remote deletions reach the mirror (the prior CLI did not use it; add it). (b) v3 `list`: `pageCursor`/`nextPageCursor`, `updatedAfter`, `limit` up to 100. v3 `tags` list: cursor. The cursor may arrive as a string or number (the prior code has a `flexCursor` for this; keep it).
- FTS/search: FTS5 over highlights (text, note), books (title, author), documents (title, author, summary, notes) plus a generic `resources_fts`. Unified `search` across all three, ranked.
- Write-through rule: after a successful write (save, tag, update, delete), refresh or delete the local row so the mirror does not lag.

## Codebase Intelligence
- Omitted: DeepWiki not run. The API is documented directly and the prior print's code is the local reference.

## User Vision
Reprint of an existing, never-published local CLI on press 4.35.6. Preserve the 10 prior novel features where they still earn their place, and the hand-adopted canonical MCP output-bounding contract (internal/mcp/envelope, PP_MCP_MAX_BYTES default 32000, cobratree mirror paging). Keep READWISE_API_TOKEN as the auth env var. The user wants live testing of both read and write paths.

Notes for the build derived from that:
- Prior novel features and my read on each: Topic dossier (`ask`), Reading backlog & time-budget triage (`backlog`), Cross-corpus duplicate detection (`dedupe`), What-to-read-next ranker (`read-next`), Cross-source highlight digest (`digest`), Serendipity connections (`connections`), Incremental verifiable local mirror (`mirror`), Stale & orphan finder (`triage stale`), Saved-query watches (`watch`), Signed webhook delivery (`webhook`). All 10 still earn a place because the official MCP/CLI offers server-side search and CRUD but nothing local, offline or diffable. The strongest differentiators are `ask`, `backlog`/`read-next`, `mirror`, `dedupe`.
- "Signed webhook delivery" is **outbound** (CLI POSTs `watch` alerts with HMAC-SHA256, SSRF-validated URLs). Readwise's own inbound webhooks (https://docs.readwise.io/readwise/docs/webhooks) are UI-configured and carry a shared `secret` in the JSON body, not an HMAC header. Keep the two separate.
- Live write testing: use a clearly tagged test namespace (title/tag prefix such as `pp-test-`), prefer reversible writes (tag add/remove, `location` move and back, a Reader highlight on a throwaway saved doc), clean up what is created, and gate deletes behind `--dry-run`/explicit confirm. v3 `delete` cascades to a document's highlights and notes.

## Product Thesis
- Name: `readwise-pp-cli` (display name "Readwise"; MCP binary `readwise-pp-mcp`).
- Why it should exist: the official CLI/MCP is a live, server-side, LLM-oriented surface that is rate-limited and cannot answer cross-corpus questions offline. This CLI mirrors both APIs into one local SQLite+FTS store and layers reading-debt, dedupe, digest and backup workflows on top, with agent-safe flags (`--json`, `--select`, `--dry-run`, typed exit codes) and a byte-bounded, pageable MCP surface. It is the tool you use for "what have I already read about X", "what should I read in 20 minutes", and "back it all up, verifiably", and it does so without spending the 20/min list budget.

## Build Priorities
1. **Foundation:** spec.yaml updated from the drift report (new endpoints, fields, enums), auth (`Authorization: Token <token>`, `READWISE_API_TOKEN`), doctor via `GET /auth/` (treat 204 as healthy), per-endpoint rate limiter + `Retry-After`, MCP output-bounding envelope (`PP_MCP_MAX_BYTES`, cobratree mirror paging).
2. **Data layer + sync:** hand-built `sync` over `export` (with `includeDeleted`) and v3 `list`/`tags`; documents `parent_id` model; FTS5; unified `search`.
3. **Typed CRUD incl. write paths:** highlights, books (add `PATCH /books/{id}/`), tags (v2 detail GETs), review (add `complete`), v3 save (with `language`, `parent_id`/`content`), update (`reading_progress`), bulk_update (207 handling), delete (cascade warning).
4. **Novel features:** `ask`, `backlog`, `read-next`, `dedupe`, `digest`, `connections`, `mirror`, `triage stale`, `watch`, `webhook`.
5. **Verification:** live read and write dogfood with cleanup; peer-review gate with Tier-3 lenses (privacy, database, performance, interface, reliability) per the store-backed reprint rule.

## Spec Source
Use the prior internal `<cli-dir>/printing-press/library/readwise/spec.yaml`, updated per `2026-11-18-122527-readwise-spec-drift.md` (25 update items; no endpoint removed). **No official OpenAPI exists**: `https://readwise.io/openapi.json`, `/api/v2/openapi.json`, `/api/v2/schema/`, `/api/schema/` and `/swagger.json` all return 404, and GitHub search found no community OpenAPI. The HTML docs pages are the only contract, so exact field names and enums were taken from the raw captures.

## Pain Points (with sources)
1. **Rate-limit wall on list endpoints.** Highlight/Book LIST at 20/min and v3 list at 20/min make wrappers unusable at scale (`readwise-mcp-enhanced#3`: https://github.com/arnaldo-delisio/readwise-mcp-enhanced/issues/3; limits at https://readwise.io/api_deets and https://readwise.io/reader_api).
2. **Duplicate sources on create.** Official CLI `readwise-create-highlights` ignores `book_id` and creates a second `api_book` instead of adding to an existing manual book (`readwiseio/readwise-cli#33`: https://github.com/readwiseio/readwise-cli/issues/33). Reader docs also get saved twice. This motivates `dedupe` and idempotent writes.
3. **Reliability of the official surface.** `reader-list-documents` fails on any page containing a PDF (`readwise-cli#8`: https://github.com/readwiseio/readwise-cli/issues/8); requests time out on install (`#9`); there is no delete-documents command (`#5`); the MCP returned 403 in Claude (`readwise-mcp#3`). A direct REST CLI with local data avoids these.

## Could not verify
- Export endpoint rate limit (not documented; assumed the 240/min default).
- Whether v2 `highlight update` really rejects `location_type` (docs omit it, old spec included it); test live.
- Whether `/save/` rate limit is shared with `/update/` (both say 50/min).
- Official Readwise Books shelf endpoints: only visible in MCP/CLI listings, no REST docs.
