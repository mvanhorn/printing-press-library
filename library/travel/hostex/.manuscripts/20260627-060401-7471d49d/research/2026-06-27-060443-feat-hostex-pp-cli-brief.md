# Hostex CLI Brief

## API Identity
- Domain: Property-management / channel-manager for short-term rentals. Hostex v3 OpenAPI.
- Base URL: `https://api.hostex.io/v3`
- Users: Vacation-rental hosts and property managers; software partners building on top.
- Channels synced: Airbnb, Booking.com, VRBO, Expedia, Agoda, Trip.com, BookingSite, Hostex Direct, + many CN OTAs (Tujia, Xiaozhu, Meituan, Fliggy, Ctrip, etc.).
- Data profile: properties, room types, reservations (stay_code), listings (per-channel), conversations/messages, reviews, tasks, staff, transactions (income/expense), knowledge bases (HostGPT), webhooks, calendar/availability/prices.

## Auth (critical for generation)
- Primary: `apiKey` header **`Hostex-Access-Token`** (server also accepts `Authorization: Bearer`).
- Env var (canonical): `HOSTEX_ACCESS_TOKEN`.
- Scopes: `read-only` (GET only; writes -> error_code 401) and `writable`. Tokens do not expire; revoked in Host Portal.
- OAuth2 + PKCE exists for software partners / MCP `/mcp` endpoint (out of scope for v1 CLI; access-token path is the headline).

## Response Envelope (CRITICAL — generator concern)
- EVERY response is HTTP `200`. Application result is in the body:
  `{ "request_id", "error_code", "error_msg", "data" }`.
- `error_code: 0` == success; non-zero == failure. **Branch on `error_code`, not HTTP status.**
- Error codes mirror HTTP semantics: 400 bad-request, 401 auth/scope, 403 forbidden, 404 not-found, 409 conflict, 422 validation, 420 subscription/account, 429 rate-limit (+`Retry-After`), 500 server, 501 not-implemented-for-account, 502/503/504 upstream channel.
- ACTION: the printed client must unwrap `data` and map non-zero `error_code` to typed exit codes; a naive "HTTP 200 == OK" client will silently treat errors as success. Verify in Phase 3/shipcheck.

## Reachability Risk
- None. `GET /v3/reservations` and `/v3/properties` return clean JSON `{error_code:401,"Invalid access token."}` with no bot wall. Docs host (api-doc.hostex.io) is Cloudflare-challenged, but the API host is not. Reachability gate: PASS.

## Rate Limits (inform sync/backoff design)
- Per-token (all endpoints): 1,200/min, 12,000/5min, 20,000/h, 100,000/day.
- Per-token+endpoint: POST /availabilities & POST /listings/* = 120/min; POST /reservations = 60/min; all others 600/min.
- Per-thread message throttle on POST /conversations/{id}: 5/5s, 10/60s, 30/30min, 60/2h, 120/day.
- 429 arrives as HTTP 200 + error_code 429 + `Retry-After` header. Client backoff must read Retry-After.

## Top Workflows
1. Inbox triage: list conversations, read a thread, reply / send special offer / pre-approval.
2. Reservation ops: query reservations (by date/property/status), create direct booking, approve/decline channel requests, update check-in details, allocate, tag.
3. Calendar & pricing: read listing calendars, push price/inventory/restriction updates across channels (async tasks).
4. Operations: schedule cleaning/maintenance tasks, assign staff, track income/expense per stay/property.
5. Reviews: query reviews, post review/reply.

## Data Layer (local SQLite candidates)
- Primary syncable entities: properties, room_types, reservations, listings, conversations, reviews, tasks, staff, transactions, channel_accounts, knowledge_bases.
- Sync cursor: reservations default to check_out within next 180 days; date-range filters on most list endpoints; offset/limit pagination.
- FTS/search: conversations (guest text), reviews, reservations (guest name / code), properties (title/address), knowledge bases.
- Dictionaries to cache: income_items, income_methods, expense_items, expense_methods, reservation_tags, custom_channels.

## Table Stakes (match any competitor / official surface)
- Full CRUD across the 86 endpoints with typed flags + `--json`/`--select`/`--dry-run`.
- Auth via env var + `doctor` health check.
- Error-code-aware output (typed exit codes), Retry-After backoff.
- Webhook management (query/create/update/delete).

## Reachability/Transport Notes
- Plain HTTPS JSON; no browser transport needed at runtime. Docs were Cloudflare-gated and crawled via browser-clearance cookie + Chrome TLS (Surf) for spec assembly only.

## Product Thesis
- Name: hostex (hostex-pp-cli)
- Why it should exist: The official surface is API + an MCP server, but there is **no first-class CLI**. A Hostex CLI with a local SQLite mirror unlocks offline cross-entity queries that no single API call gives: "which occupied stays have no cleaning task", "revenue by property this month", "unanswered guest threads older than N hours", "price-parity gaps across channels". The 200-with-error_code envelope and multi-layer rate limits are exactly the plumbing a hand-rolled curl integration gets wrong — the CLI gets them right once.

## Build Priorities
1. Foundation: config/auth (Hostex-Access-Token), error_code-aware client + typed exit codes + Retry-After backoff, SQLite store for primary entities, sync/search/sql.
2. Absorb: all 86 endpoints as typed commands (generated), grouped by the 15 tags.
3. Transcend: local-join commands impossible with one API call (occupancy gaps, ops gaps, revenue rollups, inbox SLA, price-parity), built on the SQLite mirror.

## Source
- Spec: `research/hostex-openapi-merged.json` (86 ops / 59 paths / 15 tags), assembled from the official ReadMe per-endpoint OpenAPI blocks (104 `.md` pages crawled via browser-clearance).
- Docs corpus: `discovery/docs-md/*.md` (auth, rate-limits, errors, oauth, webhooks, mcp-tools-reference, supported-channels/currencies + per-endpoint).
