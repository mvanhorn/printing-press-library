# Goodreads API Map Status

Updated: 2026-05-22T10:08:00Z

## Current Deliverables

- OpenAPI skeleton: `goodreads/api-map/openapi/undocumented/goodreads-web.yaml`
- Markdown reference: `goodreads/api-map/markdown/goodreads-web-routes.md`
- Curl recipes: `goodreads/api-map/curl/goodreads-web.sh`
- Dynamic collection model: `goodreads/docs/dynamic-collection-model.md`
- Parser harness: `goodreads/tools/parse_fixtures.py`, `goodreads/tools/summarize_shelves.py`
- Read-only CLI command contracts: `goodreads/docs/cli-command-contracts.md`
- Personal repo scaffold: `~/Desktop/goodreads-cli` with the copied API map, read-first TypeScript CLI, tests, docs, and planned MCP stub. Current commands cover routes, shelf discovery, paginated book list/export, book page parsing, note metadata, message metadata, and dry-run write plans.
- PP-side scaffold: `~/printing-press/library/media-and-entertainment/goodreads-pp`, linked from `goodreads/pp-scaffold`. It was generated from the OpenAPI map and passes local `go test`, `go vet`, `printing-press verify`, `verify-skill`, and `shipcheck`. The first gardening fix corrected cookie auth transport (`_session_id2` is now sent as a Cookie, not a custom header) and tightened cache permissions.

## Coverage

The map currently covers the authenticated web app surfaces that were actually opened or observed through navigation:

- Profile and delayed profile sections.
- Shelves/books and book tooltip metadata.
- Quotes, comments, notifications, friends, messages, groups/discussions.
- Kindle notes/highlights index, detail pages, pagination, and one approved bulk-publicize write.
- Recommendations, friends' recommendations, public lists, genres, Year in Books, and book detail pages.
- External-first current API sweep: RSS shelves, legacy XML API behavior, commercial scraper APIs, GitHub scraper implementations, X/Twitter signal, and whole-map discovery tricks.
- Shelf read/move mapping: current shelf names, batch move form, single-book shelf chooser script route, and reorder endpoints. Shelf writes remain dry-run only.
- Discovery pages: Amazon purchases, top shelves, all genre shelves, historical-fiction genre page, message folders/actions, and people leaderboards.
- Public utility routes: `/review/list_rss/:user`, `/opensearch.xml`, `/search/search`, JSON-LD and `__NEXT_DATA__` on book pages.
- RSS limit check: public RSS returned all 40 `read` items but only 100 of 132 `to-read` items, so large-shelf exports need authenticated HTML/table parsing.
- Fixture-backed shelf pagination check: authenticated HTML pages 1-2 for `read` produced 40 unique review ids, and pages 1-5 for `to-read` produced 132 unique review ids. Shelf names/counts are account inventory and must be discovered, not hardcoded.

## Not Yet Covered

- Mobile app API or GraphQL request capture from an actual app session.
- Search/autocomplete.
- Review write/edit execution capture.
- Shelf add/remove execution capture.
- Rating write payloads.
- Spoiler toggle payloads.
- Per-note thought submission payload.
- Goodreads built-in export flow behind `/review/import`.

## Next Implementation Step

Run a gardening and endpoint-completeness pass before any publish path:

1. Fetch fixture HTML/XML from the current local browser/cookie session and public RSS into private `0600` files.
2. Parse `/review/list/:user?shelf=<shelf>` and `/review/list_rss/:user?shelf=<shelf>` into books/shelves/reviews with source confidence labels.
3. Add a `shelves discover` layer that reads account-specific shelf slugs/counts from `/review/list/:user`.
4. Parse `/book/show/:slug` JSON-LD and normalized `__NEXT_DATA__` fields without storing raw public review bodies by default.
5. Parse `/notes/:user_slug` and `/notes/:book_slug/:user_slug` into notes metadata without storing highlight text by default.
6. Parse `/quotes/list`, `/comment/list/:user_slug`, `/notifications`, `/message/:folder`, `/shelf`, `/genres/list`, and people discovery pages.
7. Broaden personal TypeScript CLI parser coverage from shelves/books/notes/messages into quotes, comments, friends, and profile after Chrome remote-debugging access is re-approved.
8. Keep the generated PP scaffold's Goodreads browser-cookie/session model pinned with tests and docs.
9. Review the generated 55-tool MCP surface and decide whether to keep full endpoint parity or add a smaller orchestration layer before any publish path.

Reason: Goodreads is HTML-heavy. A generated skeleton can pass local shipcheck while still missing parser fidelity, dynamic account inventory, and endpoint-completeness requirements.
