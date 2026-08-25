# Open Library Research Plan

## Goal

Build a read-only Printing Press CLI for Open Library book, author, edition, and subject metadata workflows using Open Library's public JSON APIs.

## Sources

Open Library provides public JSON/YAML/RDF APIs for human-centered book discovery and lookup services at `https://openlibrary.org/developers/api`. The API index points to the Book Search API, Covers API, Work and Edition APIs, Authors API, Subjects API, and Recent Changes API.

The official usage guidelines say the APIs are for low-volume, high-value, real-time discovery and lookup, not as a bulk data backend. Requests should cache responses where practical and identify the application with a `User-Agent` header and contact email when making regular or frequent use. Default non-identified requests are limited to 1 request per second; identified requests with a `User-Agent` and email receive a 3 requests per second limit.

The Book Search API is available at `https://openlibrary.org/search.json` and accepts `q`, `fields`, `sort`, `lang`, `offset`, `page`, and `limit`. Open Library's Books API documentation says Book Search is the preferred general book lookup API, while works and editions can be fetched as JSON by appending `.json` to `/works/<id>` and `/books/<id>`. A work's editions are fetched from `/works/<id>/editions.json`. ISBNs resolve through `/isbn/<isbn>.json`.

The Authors API supports author search through `https://openlibrary.org/search/authors.json?q=<query>`, individual author JSON through `/authors/<id>.json`, and author works through `/authors/<id>/works.json` with `limit` and `offset`.

The Subjects API supports `GET /subjects/<subject>.json`, with `details=true` returning related subjects, publishers, authors, and publishing history. The Subjects API is documented as experimental, so output must call out that caveat.

## Command Surface

The first print keeps the surface intentionally small and agent-native:

- `book <query>` searches Open Library for a title or keyword phrase, returns compact ranked book/work candidates, and includes best identifiers, authors, first publish year, edition count, language hints, and source URL.
- `isbn <isbn>` resolves an ISBN-10 or ISBN-13 to the Open Library edition JSON and returns edition metadata, work links, authors, publishers, publish date, identifiers, covers, and source URL.
- `author <query-or-author-id>` searches authors by name or fetches a specific Open Library author ID, returning top author metadata and a bounded works sample.
- `work <work-id>` fetches a specific Open Library work JSON and returns title, description, subjects, authors, covers, latest revision, and source URL.
- `editions <work-id>` fetches editions for a work from `/works/<id>/editions.json` with bounded `--limit` and `--offset`.
- `subjects <subject>` fetches works for a subject from `/subjects/<subject>.json`, optionally with `--details`, returning compact work candidates and related subject/publisher/author facets when available.
- `sources` reports source coverage, API URLs, rate limits, auth mode, contact-header behavior, freshness, and non-goals.
- `doctor` reports configured environment variables, including optional `OPEN_LIBRARY_USER_AGENT` and `OPEN_LIBRARY_CONTACT_EMAIL`, and explains the active request-rate posture.

## Live Research Findings

- Open Library's API index was last edited May 5, 2026 and states the service supports public JSON, YAML, and RDF/XML APIs for book discovery and lookup.
- Open Library asks clients not to scrape HTML, not to harvest in bulk, and to use data dumps for bulk access.
- Open Library documents default non-identified request limits of 1 request per second and identified request limits of 3 requests per second when a `User-Agent` and contact email are sent.
- The Search API documentation identifies `https://openlibrary.org/search.json` as the book search endpoint and documents pagination through `page`/`limit` or `offset`/`limit`.
- The Books API documentation says Book Search is the preferred book lookup API, while individual works, editions, and ISBNs can be fetched as JSON.
- The Authors API documentation supports author search, individual author JSON, and paginated author works.
- The Subjects API documentation marks the API experimental and supports `details=true` for additional facets.
- No API key or account is required for the read-only command set.

## Authentication

No API key is required.

Optional environment variables:

- `OPEN_LIBRARY_USER_AGENT`: custom application identity for the `User-Agent` header.
- `OPEN_LIBRARY_CONTACT_EMAIL`: contact email to include in the request identity when users make regular or frequent requests.

If these are missing, commands must still work using the non-identified request posture and `doctor`/`sources` must explain the lower default rate limit.

## Output Contract

All commands are read-only. `--agent` and `--json` output must be compact JSON with these recurring fields where applicable:

- `source`
- `configured`
- `query`
- `request`
- `results`
- `work`
- `edition`
- `author`
- `subjects`
- `freshness`
- `caveats`
- `source_url`

Output should favor stable bibliographic facts and source URLs over prose. Commands must avoid making hundreds of per-book requests; use search results and bounded pagination instead.

## Non-Goals

- No borrowing, lending, waitlist, patron, or account actions.
- No write-side catalog edits.
- No list creation or list mutation.
- No HTML scraping.
- No bulk harvesting or local mirror sync in the first print.
- No claim that Open Library metadata is complete or authoritative for every edition.
- No raw endpoint dump.
