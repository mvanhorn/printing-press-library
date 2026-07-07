# Paul Graham Essays Read-First Brief

## Source

- Canonical essay index: https://www.paulgraham.com/articles.html
- RSS page: https://www.paulgraham.com/rss.html
- Base URL: https://www.paulgraham.com

`articles.html` is a static HTML page and is the best full-index source. `rss.html` exists and was reachable during validation, but the CLI does not depend on it because the static article index is the durable complete archive.

## Shape

The site does not expose an official JSON API. This package archives a tiny OpenAPI wrapper for `GET /articles.html`, then layers native read-only commands over the HTML:

- `latest` lists newest index entries.
- `list` filters the index by title or slug.
- `search` searches title/slug and can fetch pages for full-text search.
- `read` extracts readable essay text by slug, URL, title, or title substring.
- `links` extracts page links.
- `random` picks an essay from the index.

## Safety

No authentication is required. All commands are read-only. The HTTP client caps response size for both index and essay pages and uses a normal descriptive User-Agent.
