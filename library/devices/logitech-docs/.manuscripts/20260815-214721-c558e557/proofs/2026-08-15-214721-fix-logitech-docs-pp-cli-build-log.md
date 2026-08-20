# logitech-docs build log

Manifest transcendence rows: 4 planned, 4 built (docs, find, download, compare). PDF-text-extraction (planned `sync` novel) deferred — see gaps.

## What was built (Phase 3)

- **Generator baseline** (`cli-printing-press generate` from `research/logitech-docs-spec.yaml`):
  - Resources: `categories`, `sections`, `articles` (list / get / search) against the Zendesk Help Center API v2 at `support.logi.com`.
  - `http_transport: standard` (probe-reachability confirmed `standard_http`, confidence 0.95 — no browser/clearance needed).
  - Auth: none (public site). All generated quality gates passed at generate time (`go mod tidy`, `go vet`, `go test`, `govulncheck`, `go build`, `doctor`, binary runnable).

- **Hand-built transcendence commands** (replaced TODO scaffolds; `go build` green, live-smoke verified):
  1. `docs <type> <query>` — maps friendly doc types (spec/manual/install/faq/download/warranty/video) to `webcontent=` labels and searches live. Verified: `docs spec "MeetUp"`, `docs install "Rally Bar"`, `docs manual "MX Master 3S"`.
  2. `find <query>` — local FTS5 search (`store.SearchArticles`) with a `--live` fallback to the search API. Verified: local + `--live`.
  3. `download <article-id>` — extracts `download01.logi.com` links from an article body; lists by default, `--save <dir>` downloads. Verified against spec article (correctly empty) and structured.
  4. `compare <a> <b>` — searches both products' `webcontent=productspecs` articles, extracts 2-column spec tables, diffs common keys. Verified: `compare "MeetUp" "MeetUp"` returns spec rows.

## Intentionally deferred (known gaps)

- **PDF text extraction / offline body indexing.** `sync --resources articles` stores the Zendesk list payload, which omits `body`; the local FTS therefore indexes title/name (and any cached `get` bodies) but not full manual bodies offline. Live full-text over bodies works via `docs` / `articles search` / `find --live` (Zendesk's server-side index covers bodies). Deep body sync (33k+ article GETs) and binary-PDF text extraction are deferred.
- **Locale.** Spec hardcodes `en-us`; other locale variants are reachable by hand-editing the path or a future `--locale` flag.
- **Product↔`webproduct` UUID mapping** (friendly product-name filter) not built; sections + free-text search cover the workflow.

## Skipped body fields

- `label_names` is a JSON array and is not modeled in the spec `types` (scalar-only type system); it remains available in `--json` output.
- `body` omitted from `ArticleSummary` (list/search) by design to keep table output lean; `articles get` returns it.
