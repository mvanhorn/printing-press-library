# Zotero Research Library CLI Brief

Phase 1 research brief for regenerating the lost `zotero-research-library-pp-cli` (Printing Press print of the Zotero Web API v3). Evidence: official docs (zotero.org/support/dev/web_api/v3/{basics,syncing,types_and_fields,fulltext_content}), pyzotero README + issue tracker, the prior CLI's SKILL.md at `~/.claude/skills/pp-zotero-research-library/SKILL.md`.

## API Identity

**Domain.** Zotero Web API v3 — the official read/write HTTP API for zotero.org-hosted research libraries. Base URL `https://api.zotero.org`, HTTPS only. Library-scoped URLs begin `/users/<userID>` or `/groups/<groupID>` (the `<userOrGroupPrefix>`). Versioning is pinned per-request via `Zotero-API-Version: 3` header or `?v=3` query param. A parallel **local API** (`http://localhost:23119/api/`, desktop app running) exposes the same read endpoints without auth and without the 100-item page cap — a viable secondary data source but not required for v1.

**Users / personas.**
- **Primary operator: a sport-science director/researcher** (the user) with a large personal Zotero library of sport-science literature. He runs a `sports-science-research-grounding` workflow whose first step is "search my own Zotero library before searching the open web" — the CLI is the substrate for that ritual. He wants answers grounded in *his* curated corpus, offline, in seconds.
- **Agent consumers**: Claude Code skills (`pp-zotero-research-library` SKILL.md drives this binary with `--agent` mode) that need pipeable JSON, non-interactive auth, and stable exit codes.
- **Secondary**: any researcher scripting against a personal or group library (`--library user` | `--library group:<id>`).

**Data profile.** Bibliographic items (journal articles, books, theses, reports…) with rich metadata: `itemType`, `title`, `creators` (array of `{creatorType, firstName, lastName}` or `{name}`), `abstractNote`, `DOI`, `date`, `publicationTitle`, `tags` (array of `{tag, type}`), `collections` (array of collection keys), `dateAdded`/`dateModified`, plus per-object integer `version`. Hierarchy: top-level items → child attachments/notes (`/items/<key>/children`). Collections are hierarchical (parentCollection). Full-text of indexed attachments is retrievable as plain text JSON. Everything is monotonically versioned by a single per-library integer — the foundation of cheap incremental sync.

## Reachability Risk

**None/Low.** Official, public, documented, stable API operated by the Zotero project (Corporation for Digital Scholarship). No scraping, no reverse engineering, no ToS gray areas. Auth is a user-created API key from <https://www.zotero.org/settings/keys>. Rate limiting is documented and cooperative (`Backoff` / `Retry-After` headers, ≤4 concurrent requests recommended). The API surface has been at v3 for a decade; the docs explicitly note v3 is current. Only soft risk: heavy full-library fulltext pulls on very large libraries should honor backoff to avoid temporary throttling.

## Top Workflows

1. **Incremental library sync** (the heartbeat): `sync` pulls items + collections (+ tags, deletions, fulltext) changed since the stored library version into a local SQLite cache. `GET <prefix>/items?since=<v>&format=versions&includeTrashed=1`, then batch-fetch changed items by key; `GET <prefix>/deleted?since=<v>` for tombstones. First run = full sync from version 0. This existed in the lost v0.1 and is the spine everything else hangs on.
2. **Research-grounding search** (the operator's key ritual): "what does my library say about hamstring injury prevention?" → local FTS over title/abstract/tags/fulltext in the SQLite cache, returning ranked items with citation-ready metadata (creators, year, DOI, abstract snippet). Answered **offline** from the cache; optional `--live` passthrough to `GET <prefix>/items?q=<terms>&qmode=everything`.
3. **Collection browsing/filtering**: list the collection tree, list items in a collection (`/collections/<key>/items[/top]`), scope searches to a collection. Mirrors how the operator organizes literature by topic/project.
4. **Tag-based retrieval**: list tags (`/tags`, `/items/tags` with filters), find items by tag (`?tag=<name>`; supports `||` OR-union and `-` negation search syntax), locally via the cache.
5. **Recently-added triage**: "what did I add this week?" — sort by `dateAdded desc` from the cache (or `?sort=dateAdded&direction=desc` live); pairs with export (`--format bibtex`) for dropping fresh citations into a manuscript.

## Table Stakes

Feature parity floor, derived from pyzotero (the dominant wrapper), the v3 API surface, and sibling CLIs (zotero-cli on PyPI/npm, pyzotero-cli, zotero-lib, the ~8 Zotero MCP servers on GitHub — all of which converge on search/collections/tags/fulltext/children as the core toolset):

- **Items (read)**: list (`/items`, `/items/top`, `/items/trash`, `/publications/items`), get by key (`/items/<key>`, up to 50 keys via `?itemKey=k1,k2`), children (`/items/<key>/children`).
- **Collections**: `/collections`, `/collections/top`, `/collections/<key>`, `/collections/<key>/collections` (subcollections), `/collections/<key>/items[/top]`.
- **Tags**: `/tags`, `/items/<key>/tags`, `/items/tags` (filterable by `itemQ`/`itemTag`), per-collection tag endpoints.
- **Saved searches**: `/searches`, `/searches/<key>` (read + include in sync so deletions propagate).
- **Fulltext**: `/fulltext?since=<v>` (changed-content index) and `/items/<key>/fulltext` (content payload with `indexedChars/totalChars` or `indexedPages/totalPages`).
- **Search params**: `q` (phrase search of titles+creators), `qmode=titleCreatorYear|everything` (`everything` includes fulltext), `itemType`, `tag`, `since` — with `||` OR and leading `-` negation syntax.
- **Versions/sync**: `format=versions` (unpaginated key→version map), `?since=<version>`, `Last-Modified-Version` response header, `If-Modified-Since-Version` request header → `304 Not Modified`, `/deleted?since=<v>`.
- **Formats**: `format=json` (default; `include=data,bib,citation` with `style=<CSL style>`), `format=keys`, `format=versions`, and export formats usable as `format=`/`include=`: `bibtex`, `biblatex`, `csljson`, `ris`, `csv`, `mods`, `coins`, `tei`, `wikipedia`, etc. Export formats require explicit `limit` and don't support sorting/pagination when `format=bib`.
- **Key introspection**: `GET /keys/current` (and `GET /keys/<key>`) → `{userID, username, access:{user:{library,files,notes,write}, groups:{...}}}` — this is both the doctor probe and the **userID discovery path**: the operator only needs to paste an API key; the CLI resolves the numeric userID from `/keys/current` and verifies library access before saving config.
- **Prior-CLI carryover (user's stated vision: keep the same features)**: `auth login/status/logout`, `doctor`, `sync [--full|--dry-run|--no-bbt]`, `--library user|group:<id>`, `--agent` mode, env overrides (`ZOTERO_API_KEY`, `ZOTERO_USER_ID`, `ZOTERO_BASE_URL`), config at `~/.config/zotero-research-library-pp-cli/config.toml` (0600), exit codes 0/2/4/5/7/10, optional Better BibTeX citekey backfill via desktop JSON-RPC at `127.0.0.1:23119` (graceful degradation when app closed).

## Data Layer

Local SQLite cache (the prior CLI's design, now extended for search):

- **Entities**: `items` (key PK, version, itemType, title, abstractNote, DOI, date/parsedDate, dateAdded, dateModified, creators JSON, raw data JSON, parentItem for children, deleted flag), `collections` (key, version, name, parentCollection), `item_collections` (join), `tags` + `item_tags` (join; tag type 0=manual/1=automatic), `searches` (key, version, conditions JSON), `fulltext` (itemKey, content, contentVersion), `sync_state` (library id, last library version per object type, last fulltext version, last sync timestamp).
- **Sync cursor** = the library version integer from `Last-Modified-Version`, stored per library (user vs each group) and per object class. Fulltext has its own version cursor via `/fulltext?since=`.
- **Deletions**: apply `/deleted?since=<v>` tombstones (collections, searches, items, tags) each sync.
- **FTS**: SQLite FTS5 virtual table over `title`, `abstractNote`, creator names, tag names, and `fulltext.content`, with rank + snippet() for research-grounding queries. Rebuildable from base tables (`sync --full` or a `reindex` maintenance path).
- **Multi-library**: keyed by library prefix so `--library group:1234567` gets its own cursor and rows.

## Codebase Intelligence

Exact wire-level facts for the spec (all verified against official docs):

- **Auth header**: `Zotero-API-Key: <key>` (preferred) or `Authorization: Bearer <key>`; `?key=` query param exists but is discouraged. No auth needed for public libraries; local API reads are unauthenticated.
- **API version pin**: `Zotero-API-Version: 3` header (or `?v=3`).
- **Version headers**: response `Last-Modified-Version: <int>` on multi-object and single-object reads; request `If-Modified-Since-Version: <int>` → `304 Not Modified` when unchanged (multi-object today; single-object "in the future" per docs — treat single-object 304 as optional).
- **Pagination**: `start` (0-based) + `limit` (1–100, default 25 — **100-item hard page cap**); `Total-Results` response header gives full match count; `Link` response header carries `rel="next"/"prev"/"first"/"last"/"alternate"` URLs — follow `next` rather than computing offsets. `format=keys` and `format=versions` are exempt from the cap (return everything).
- **Rate limiting**: two mechanisms. (1) `Backoff: <seconds>` response header on otherwise-successful responses — server asks the client to pause that long before any further request; (2) `429 Too Many Requests` with optional `Retry-After: <seconds>` (also on `503` during maintenance) — wait at least that long, else exponential backoff. Keep concurrency ≤4. **pyzotero's known pain point (issue #98): it historically honored 429 but not the header-based `Backoff` — our CLI should honor both**, surfacing exit code 7 only after retries are exhausted.
- **Sync recipe** (docs' canonical order): `GET /keys/current` (verify access) → `GET /users/<id>/groups?format=versions` (group membership/versions) → per library: `collections?since&format=versions`, `searches?since&format=versions`, `items?since&format=versions&includeTrashed=1` → batch-fetch changed objects (≤50 item keys per `?itemKey=` request; collections up to 100 via `?collectionKey=`) → `GET /deleted?since=<v>` → store new `Last-Modified-Version`. If `Last-Modified-Version` changes mid-multi-page fetch, restart that phase (concurrent-modification guard).
- **Fulltext sync**: `GET <prefix>/fulltext?since=<v>` → `{itemKey: version}` map → `GET <prefix>/items/<key>/fulltext` per changed attachment.
- **Search syntax**: `tag=foo bar` (single), `tag=a||b` (OR), `tag=-foo` (NOT), same syntax for `itemType`; `q=` is phrase-only; `qmode=everything` adds fulltext to quick search.
- **Schema endpoint**: `https://api.zotero.org/schema` (whole schema, cacheable); `/itemTypes`, `/itemFields`, `/itemTypeFields?itemType=`, `/itemTypeCreatorTypes?itemType=` for piecemeal.
- **Landscape**: pyzotero (urschrei/pyzotero) is the reference implementation and now ships its own MCP server (`pyzotero-mcp`) with tools {search, get_item, get_children, list_collections, list_tags, get_fulltext} — a strong signal of the minimal agent-facing toolset. CLI competitors: `zotero-cli`/`pyzotero-cli`/`zotero-cli-tool` (PyPI), `zotero-lib` (npm, OpenDevEd). ~8+ Zotero MCP servers exist on GitHub (54yyyu/zotero-mcp, kujenga/zotero-mcp, masaki39 local-API variant, etc.); none pair an offline SQLite FTS cache with incremental version-based sync — that combination is this CLI's differentiator.

## User Vision

Rebuild the lost CLI, same surface, same name (`zotero-research-library-pp-cli`), Go, Printing Press house style. Non-negotiables from the prior SKILL.md: `auth login` (prompt or flags; now enhanced — accept just the API key and auto-resolve userID via `GET /keys/current`), `auth status/logout`, `doctor` (config path, auth source, library target, live probe against `api.zotero.org`; exit 0 only when auth + connectivity pass), `sync` with `--full`, `--dry-run`, `--no-bbt`, incremental via the versions API into local SQLite; `--library user|group:<id>`; `--agent` (= `--json --compact --no-input --no-color --yes`); documented exit codes (0 ok, 2 usage, 4 auth, 5 API, 7 rate-limited, 10 config); env overrides; Better BibTeX citekey backfill when the desktop app is up.

The operator's key ritual — the reason the CLI exists — is **research-grounding queries answered offline**: "what does my library say about X" must work with no network, sub-second, from the synced cache, returning citation-ready JSON his `sports-science-research-grounding` workflow can consume directly. v0.1 synced the data but shipped **no way to query it** (SKILL.md explicitly lists search/collections/tags/export as NOT implemented and tells agents to fall back to curl). Closing that gap is the whole point of the reprint.

## Product Thesis

**A local-first research-library engine, not an API wrapper.** Every existing tool (pyzotero, the CLIs, the MCP servers) is a thin live-API proxy: every question costs network round-trips, 100-item pages, and rate-limit exposure. This CLI inverts that: incremental version-based sync makes the SQLite cache a faithful replica (items, collections, tags, searches, fulltext, deletions), and FTS5 over title/abstract/creators/tags/fulltext makes "what does my library say about X" an offline, sub-second, agent-pipeable query. Live API access remains for freshness (`sync`) and passthrough (`--live`), but the default read path is local. For an agent-heavy user, this also means grounding workflows never burn rate limit or latency on repeated queries — sync once per session, query freely.

## Build Priorities

1. **P0 — Auth + doctor** (restore v0.1): `auth login/status/logout`, key validation + userID auto-discovery via `GET /keys/current`, config.toml 0600, env overrides, `doctor` connectivity probe, exit-code contract.
2. **P0 — Incremental sync** (restore + extend v0.1): full docs-recipe sync of items/collections/tags/searches + `/deleted` tombstones into SQLite; per-library version cursors; `--full`, `--dry-run`; mid-sync version-change restart guard; `Backoff`/`Retry-After`/429 handling with ≤4 concurrency.
3. **P0 — Search (the missing organ)**: `search "<query>"` over FTS5 (title/abstract/creators/tags), with `--tag`, `--type`, `--collection`, `--limit`, ranked output + snippets, `--agent` JSON with key/title/creators/year/DOI/abstract. This is the research-grounding command.
4. **P1 — Fulltext sync + search**: `/fulltext?since=` cursor + per-item content fetch into FTS; `search --fulltext` flag; when a fulltext hit is an attachment, resolve and return the parent bibliographic item (pyzotero-cli's proven pattern).
5. **P1 — Browse commands**: `collections list/tree`, `collections items <key>`, `tags list`, `items get <key>`, `items children <key>`, `items recent` (dateAdded desc) — all cache-first with `--live` passthrough.
6. **P2 — Export**: `export --format bibtex|csljson|ris` for a key list / collection / search result (live API `include=`/`format=` path; BBT citekeys from cache when present).
7. **P2 — Better BibTeX backfill** (restore): JSON-RPC to `127.0.0.1:23119`, `--no-bbt` skip, graceful degradation.
8. **Deferred**: writes (item create/edit/delete), attachments/PDF download, group-library multi-sync orchestration, OAuth flow (dedicated keys suffice for a single-operator tool).
