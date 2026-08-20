# AnkiWeb CLI Brief

## API Identity
- Domain: ankiweb.net — the free companion sync + sharing service for Anki (spaced-repetition flashcards). NOT the desktop app.
- Users: Anki users who sync collections to the cloud, and anyone browsing/downloading community "shared decks" (students, language learners, med students — Anki's heaviest cohort).
- Data profile: shared-deck catalog (title, author, rating, download count, notes count, category, size, last-updated, description); personal synced decks + study stats; account profile. Sync payloads are protobuf.

## Reachability Risk
- **Medium.** ankiweb.net is a Svelte SPA backed by a `/svc/` service layer that speaks **protobuf over HTTP** (`application/octet-stream`), not JSON. Plain `curl`/WebFetch returns empty HTML shells (confirmed: `/shared/decks` and `/shared/info/<id>` both rendered empty). There is **no official REST API** — the Anki maintainer has publicly rejected adding one (forums.ankiweb.net). Discovery requires browser-sniff against the live SPA; the user is logged into Chrome, enabling authenticated capture.
- Mitigation: browser-sniff the `/svc/` endpoints with the logged-in session, capture protobuf request/response shapes, generate replayable HTTP transport with cookie auth.

## Top Workflows
1. **Search & discover shared decks** — find community decks by keyword/category/sort, see ratings & download counts, without the slow SPA. (public)
2. **Download a shared deck** — pull the `.apkg` for a deck id straight to disk. (public)
3. **List my synced decks + study stats** — see my own decks, card counts, review stats from the cloud. (cookie auth)
4. **Inspect a shared deck before downloading** — full metadata + description for a deck id. (public)
5. **Manage/share account decks** — view what I've shared, download counts on my shared decks. (cookie auth)

## Table Stakes
- Shared deck search with sort (rating, downloads, recency) and category filter
- Deck info lookup by id
- Deck download by id to a chosen path
- Account deck listing (authenticated)
- Cookie/session auth via logged-in Chrome (no API key exists)
- `--json`, `--select`, typed exit codes, `--dry-run` on any write/download

## Data Layer
- Primary entities: `shared_deck` (id, title, author, rating, downloads, notes, category, size, updated, description), `my_deck` (id, name, card_count, stats), `account` (profile).
- Sync cursor: `updated` timestamp on shared decks; download-count deltas for owned shared decks.
- FTS/search: FTS5 over `shared_deck` (title, author, description, category) → offline search of a synced catalog snapshot, which the SPA cannot do.

## Codebase Intelligence
- No official spec or SDK. The ecosystem (AnkiConnect, genanki, apy, py-ankiconnect, anki-mcp-server×many) targets the **desktop** app via the AnkiConnect add-on on `localhost:8765` — a completely different surface from ankiweb.net.
- Auth: session cookie set on login at ankiweb.net (email + password). The `/svc/` endpoints require the cookie; shared-deck browse/download are public.

## User Vision (reprint hand-off)
- This is a **reprint** of a never-published local CLI (`ankiweb-pp-cli`). The prior build had two defects to correct:
  1. **Doubled slug**: API slug was captured as `ankiweb-pp-cli` → doubled into `ankiweb-pp-cli-pp-cli` across module path, `cmd/` dirs, binary names, and `ANKIWEB_PP_CLI_COOKIES` env var; broke publish-validate + verify-skill canonical-sections gates. **Reprint MUST use slug `ankiweb`** so canonical layout is produced (`cmd/ankiweb-pp-cli`, `cmd/ankiweb-pp-mcp`, module path `ankiweb`, env var `ANKIWEB_COOKIES`).
  2. **Generic messaging/chat boilerplate** (commands `deliver`, `channel_workflow`, `tail`, `which`, `data_source`, `feedback`; analytics examples using `author_id`/`channel_id`) never adapted to the Anki domain. Reprint must produce domain-appropriate commands.

## Source Priority
- Single source (ankiweb.net). No combo ordering needed.

## Product Thesis
- Name: **AnkiWeb CLI** (`ankiweb-pp-cli`)
- Why it should exist: Every existing Anki tool wraps the **desktop** AnkiConnect add-on. **Nothing targets the AnkiWeb website.** This CLI is the only terminal-native way to search/download the shared-deck catalog and read your cloud-synced decks/stats — with offline FTS search, agent-native JSON, and a local SQLite catalog the SPA can't give you.

## Build Priorities
1. Browser-sniff `/svc/` endpoints (authenticated) → replayable HTTP transport + cookie auth (`auth login --chrome`).
2. Data layer: `shared_deck`, `my_deck`, `account` + sync + FTS5 search.
3. Absorbed commands: shared search, shared info, shared download, my decks list, account.
4. Transcendence: offline catalog search/rank, download-count drift on owned decks, deck-discovery ranking, study-stat aggregation — all SQLite-backed.
