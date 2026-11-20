# Readwise spec drift report (2026-11-18)

Compared: `<cli-dir>/printing-press/library/readwise/spec.yaml` (prior hand-authored internal spec, v0.1.0) against the live docs fetched 2026-11-18 with `fetch-docs.sh` (status 200 for all):
- https://readwise.io/api_deets (v2)
- https://readwise.io/reader_api (v3)
- https://docs.readwise.io/readwise/docs/webhooks

spec.yaml was NOT edited. Verdicts: **update spec** or **no action**.

Official OpenAPI: none. `https://readwise.io/openapi.json`, `/api/v2/openapi.json`, `/api/v2/schema/`, `/api/schema/` and `/swagger.json` all return 404. No community OpenAPI repo found on GitHub. The docs HTML is the only contract, so the internal spec stays authoritative.

## A. Endpoints added or missing from the spec

| # | Item | Docs say | Verdict |
|---|------|----------|---------|
| A1 | `POST /api/v2/review/complete/` (mark daily review complete) | Returns `{"review_id":..., "review_completed": true}` | **update spec**: new `review.complete` write endpoint. The spec only has `review.get`. |
| A2 | `PATCH /api/v2/books/{id}/` (Book UPDATE) | Body keys: `title_override`, `author_override`, `category` (books, articles, tweets, podcasts), `cover_image` (base64 or data URI; JPEG/PNG/GIF/WebP; null removes it). Returns the Book. | **update spec**: add `books.update`. Live-test with `title_override` only. |
| A3 | `GET /api/v2/highlights/{highlight_id}/tags/{tag_id}` (Highlight Tag DETAIL) | Returns `{id, name}` | **update spec**: add `highlight-tags.get` (small). |
| A4 | `GET /api/v2/books/{book_id}/tags/{tag_id}` (Book Tag DETAIL) | Returns `{id, name}` | **update spec**: add `book-tags.get` (small). |
| A5 | `GET /api/v2/export/` params `ids`, `includeDeleted`, `updatedAfter`, `pageCursor` | `ids` is comma-separated user_book_ids. `includeDeleted=true` returns deleted highlights so deletions can sync. | **no action** in spec (export is hand-built, `export` is a reserved template name). **Carry into sync design**: use `includeDeleted=true` to propagate deletions, and `ids=` for targeted re-sync. Check whether the prior `sync` used `includeDeleted`. |
| A6 | `GET /api/v2/auth/` returns **204** | "You should receive a 204 response." | **no action**: `health_check_path: /auth/` already set. Make sure doctor treats 204 as success (an empty body must not be a parse error). |
| A7 | Webhooks (Readwise to your endpoint) | Configured in the Readwise web UI (https://readwise.io/webhook), custom or Zapier. No REST endpoint to create or list them. | **no action** in spec: no API surface. See section D. |
| A8 | Reader exports, `reader_export_documents` and `reader_get_export_documents_status` | Exist only on the official MCP/CLI (https://readwise.io/mcp), not in the public v3 REST docs. | **no action**: not a documented REST endpoint. Do not model it. |

## B. Parameters, bodies and enums

| # | Item | Docs say | Spec has | Verdict |
|---|------|----------|----------|---------|
| B1 | v2 `books` list: `last_highlight_at__lt` | Documented alongside `__gt` | only `__gt` | **update spec**: add `last-highlight-before`. |
| B2 | v2 highlight UPDATE body | `text`, `note`, `location`, `url`, `color`. `location_type` is **not** listed. | includes `location_type` enum | **update spec**: remove `location_type` from the update body, or keep it hidden and verify live. The docs are authoritative. |
| B3 | v2 highlight CREATE `category` | "One of: books, articles, tweets or podcasts" | free string | **update spec**: add the enum (`supplementals` is valid for the books LIST filter, not for create). |
| B4 | v2 highlight CREATE field limits | `text` max 8191, `title` 511, `author` 1024, `image_url` 2047, `source_url` 2047, `source_type` 64 (3 to 64 chars, no spaces), `note` 8191, `highlight_url` 4095 | none | **update spec**: add limits to descriptions. They drive client-side validation. |
| B5 | v2 highlight CREATE dedupe and upsert | Dedupe key is title+author+text+source_url. Re-sending the same `highlight_url` with new `text` updates the highlight. | not described | **update spec**: put it in the description. Matters for live write tests (idempotent) and for the `readwise-cli#33` duplicate-book trap. |
| B6 | v2 `highlights`/`books` LIST pagination | Page-based: `{count, next, previous, results}`, `page` and `page_size` (max 1000) | no `pagination` block | **update spec**: declare page-based pagination (follow `next`). |
| B7 | v3 `save` `category` enum | article, email, rss, highlight, pdf, epub, tweet, video. `highlight` requires `parent_id`. `note` cannot be created here. | enum includes `note` | **update spec**: drop `note` from the save enum. |
| B8 | v3 `save` new params `language`, `parent_id`, `content` | `language`: code like `en`, `de`, `en-US`. `parent_id` + `content` creates a **highlight** on an existing document (then only category, content, notes, parent_id, saved_using, tags are accepted). A passage not found in the parent gives 400 and creates nothing. `url` is required unless `parent_id` is set. | none of the three; `url` always required | **update spec**: add all three. Make `url` conditionally required. This is a new write path (Reader highlight creation). |
| B9 | v3 `save` response | 201 on create, 200 if the document already exists. Body `{id, url}`. | `SaveResult` | **no action** on shape. Treat 200 and 201 both as success. |
| B10 | v3 `list` `tag` param | "Pass up to 5 tag parameters" (AND semantics). Empty value finds untagged. | single string | **update spec**: make it repeatable (max 5). |
| B11 | v3 `list` `id` returns one doc; `limit` 1..100 default 100 | same | same | **no action** |
| B12 | v3 `update` new `language`, `reading_progress` (0 to 1 fraction; not yet supported for PDFs) | listed | absent | **update spec**: add both. |
| B13 | v3 `update` on a highlight | Only `notes` and `tags` accepted. Any other key returns 400. Notes cannot be updated directly, so update them on their highlight. | not described | **update spec**: put it in the description. |
| B14 | v3 `update` `category` enum | article, email, rss, highlight, note, pdf, epub, tweet, video | same | **no action** |
| B15 | v3 `bulk_update` item fields | `id` (required), title, author, summary, language, published_date, image_url, seen, reading_progress, location, category (article, email, rss, pdf, epub, tweet, video), tags (replaces all), notes. Max 50 items. **Status 200 all ok, 207 partial/per-item failure, 400 bad payload.** Response `{results: [...]}`. | item lacks `language`, `reading_progress`, `published_date`, `image_url`; no 207 | **update spec**: add the fields. The client must treat 207 as a partial success and print per-item errors, not as an error exit. |
| B16 | v3 `delete` | 204. Deleting a document also deletes its highlights and notes. | response `object` | **update spec** (description): warn that it cascades. Keep a `--dry-run` and confirm gate. |
| B17 | v3 `location` enums | save/update: new, later, archive, feed. list: new, later, shortlist, archive, feed. | same | **no action**. Note that `shortlist` is list-filter only. |
| B18 | v3 `tags` list | `{count, nextPageCursor, results:[{key,name}]}` | `ReaderTag` | **no action** |

## C. Types and fields

| # | Item | Verdict |
|---|------|---------|
| C1 | `Document` is missing `listening_time`, `highlight_offset`, `highlight_location` (the `<start>,<end>` pair of serialized DOM positions in the parent `html_content`), `html_content` (only with `withHtmlContent=true`) and `raw_source_url` (S3 link valid 1 hour, only with `withRawSourceUrl=true`) | **update spec**: add them. Store `html_content` and `raw_source_url` only on demand and never persist `raw_source_url` (it is ephemeral). |
| C2 | `Document.tags` is an object keyed by tag key: `{key: {name, type, created}}` (type e.g. `generated`, created in epoch ms). Spec says `object`. | **no action** on the type. **Note** for store design: flatten into a tags table. |
| C3 | Reader highlights and notes are Documents with `parent_id` set (category `highlight`/`note`) | **update spec**: describe it. The store must model `parent_id` self-reference, and `sync` already mixes them into `documents`. |
| C4 | `Highlight` (v2 list/detail) is missing `external_id`, `end_location` | **update spec**: add. |
| C5 | `Book` is missing `asin`, `resurface_weighting`, `source_syncs_all_books_together`, and on CREATE responses `modified_highlights` (ids created or updated by the request) | **update spec**: add. `modified_highlights` is useful for write verification. |
| C6 | `ExportBook` is missing `external_id` (only when source is reader). The nested export highlight carries `id, is_deleted, text, location, location_type, note, color, highlighted_at, created_at, updated_at, external_id, end_location, url, book_id, tags, is_favorite, is_discard, readwise_url` | **update spec**: add `external_id` and a typed `ExportHighlight` (`is_deleted`, `is_favorite`, `is_discard`, `end_location`). |
| C7 | Daily review response `{review_id, review_url, review_completed, highlights[]}`. Highlights there use `title, author, source_type, category, image_url, highlight_url, api_source`. 404 with `{"detail":"Review not found."}` when none exists. | **update spec**: add a `Review` type. Treat 404 as "no review today", not a failure. |
| C8 | Highlight `updated` (v2 list/detail) vs `updated_at` (export, v3) | **no action**: the spec already keeps both names. |

## D. Rate limits, errors and webhooks

| # | Item | Docs | Verdict |
|---|------|------|---------|
| D1 | v2 default rate | 240 req/min per access token | **update spec**: document as client default budget. |
| D2 | v2 Highlight LIST and Book LIST | 20 req/min per token | **no action**: already in descriptions. |
| D3 | v3 limits | `save` 50/min, `update` 50/min, `list` 20/min, `bulk_update` 20/min, `delete` 20/min, `tags` 20/min | **update spec**: write per-endpoint limits into the descriptions. The spec only says "20 req/min" for list. |
| D4 | Export rate limit | not documented (falls under the 240 default) | **no action** (unverified; see brief). |
| D5 | 429 | Honor the `Retry-After` header (seconds) | **no action** (the generated client already does this). |
| D6 | Webhooks | Event types: `readwise.highlight.created`, `reader.any_document.created`, `reader.feed_document.created`, `reader.non_feed_document.created`, `reader.document.tags_updated`, `reader.document.finished`, `reader.document.archived`, `reader.document.moved_to_later`, `reader.document.moved_to_inbox`, `reader.document.shortlisted`. The payload carries `event_type` and a shared `secret` string **in the JSON body** (not an HMAC header). Setup is UI only, and a test endpoint call must succeed first. | **no action** in spec: no REST surface. Optional new feature: a `webhook receive/verify` helper that checks the body `secret`. The prior CLI's "signed webhook delivery" is **outbound** (the CLI POSTs `watch` alerts, HMAC-SHA256) and is unrelated. Do not conflate the two. |

## Summary

- **Update spec (25 items):** A1 to A4, B1 to B8, B10, B12, B13, B15, B16, C1, C3 to C7, D1, D3.
- **No action:** A5 to A8, B9, B11, B14, B17, B18, C2, C8, D2, D4 to D6.
- **No endpoint removed.** Every prior path still exists. The drift is additive (new endpoints, params and fields), plus three over-claims in the old spec (B2 `location_type` on highlight update, B7 `note` in the save enum, the `tag` param being single-valued).
- **Risk to the generator:** `export` stays hand-built. `review/complete` and v3 highlight creation through `/save/` + `parent_id` are new write paths that need live write tests.
