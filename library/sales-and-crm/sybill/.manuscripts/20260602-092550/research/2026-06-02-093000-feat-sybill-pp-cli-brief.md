# Sybill CLI Brief

## API Identity
- Domain: AI sales assistant / conversation intelligence. Records sales calls, generates AI summaries, extracts deal + account + contact insights, suggests CRM field updates, ingests external data into Sybill's AI layer.
- Users: Sales reps, sales managers, RevOps. Power users want to pull call summaries/transcripts/deal data into scripts, dashboards, and agents without clicking through the Sybill web UI.
- Data profile: Conversations (transcript + AI summary + recordings), Deals (AI brief + crmAutofill suggestions + contacts), Accounts, Messages, plus an ingest layer (Sources, Object Types, Rows, Documents) for pushing custom CRM-style data into Sybill.
- API: Public, documented, OpenAPI 3.1.0 spec at `https://api.sybill.ai/docs/openapi.yaml`. Base URL `https://api.sybill.ai`. Status: **alpha** (schemas may change).

## Reachability Risk
- **None.** `GET /v1/health` returns 403 `{"detail":"Not authenticated"}` (clean FastAPI/uvicorn auth rejection) without a key, 401 with a bad key. No bot protection, no Cloudflare, no enterprise sales gate. Keys are self-serve via Dashboard > Settings > Integrations > API Keys. OpenAPI spec fetches at HTTP 200 (86 KB).

## Auth
- Type: HTTP Bearer. Header `Authorization: Bearer sk_live_<key>`. Key prefix `sk_live_`.
- Env var convention: `SYBILL_API_KEY`.
- Scopes per key: `read` (GET), `ingest` (POST/PATCH/DELETE), `ask_sybill` (MCP-only).
- 401 = missing/invalid/revoked key. 403 = valid key, missing scope.

## Top Workflows
1. **Weekly call digest** — pull all external conversations this week, group by deal, surface summary + next steps. Sales manager's Monday review.
2. **Deals gone dark** — list open deals with no activity in N days; surface the re-engagement list (cross-entity query the API can't answer directly).
3. **CRM autofill review** — deals expose `crmAutofill` (AI-suggested CRM field values). Show a diff of pending suggestions. No tool surfaces this today.
4. **Transcript search/grep** — full-text search across cached transcripts for competitor mentions, pricing objections, "legal/contract".
5. **Account roll-up** — calls + deals + contacts per account, offline.

## Table Stakes (from Gong/Fireflies/Fathom/Avoma baseline)
- List/filter calls by date, type, attendee, CRM name
- Fetch full transcript
- AI meeting summary + next steps / action items
- Deal + account + contact linking
- Webhook event awareness (`meeting.new_recording.v1`, Svix-signed)
- Structured/JSON output for piping
- Pagination handling (cursor-based)

## Data Layer
- Primary entities: conversations (list meta + detail: transcript/summary/recordings), deals (meta + summary + crmAutofill + contacts), accounts, messages, contacts (extracted from deal/account detail), sources, object-types, rows, documents.
- Sync cursor: cursor-based (`nextCursor`/`hasMore`). Per-entity `sync_log` table tracking last cursor + last_synced_at for incremental sync.
- FTS/search: FTS5 over transcripts, summaries, deal/account names.
- Why local store: rate limits (60/min) make repeated full pulls slow; cross-entity JOINs (deals with no recent call, accounts with N calls still in Discovery) are impossible via the entity-by-entity API; offline transcript grep.

## Codebase Intelligence
- Server: FastAPI/uvicorn (server header `uvicorn`, `x-request-id` per response).
- Auth: Bearer `sk_live_` keys, scopes read/ingest/ask_sybill.
- Rate limiting: 60/min, 1000/hr, 10000/day per key, moving window. Headers `X-RateLimit-Limit/Remaining/Reset`, `Retry-After`. 429 on exceed → exponential backoff.
- Pagination: cursor opaque. Conversations/Deals/Accounts default 20 / max 50; Messages/Rows/Documents default 50 / max 50.
- Idempotency: records keyed by `(sourceId, remoteId)`; POST on existing pair returns `status: updated`.
- Recording URLs: signed, 24h expiry.
- Soft deletes everywhere.

## Endpoint Surface (31 endpoints, 8 groups)
- Health: GET /v1/health
- Conversations: GET list, GET {id}, POST (ingest), DELETE
- Deals: GET list, GET {id}
- Accounts: GET list, GET {id}
- Messages: GET list, GET {id}, POST, DELETE
- Rows: GET list, GET {id}, POST, PATCH, DELETE
- Documents: GET list, GET {id}, POST, PATCH, DELETE
- Sources: GET list, GET {id}, POST, PATCH, DELETE
- Object Types: GET list, GET {id}, POST, PATCH, DELETE
(Read surface is the headline; ingest/rows/object-types/documents are the custom-data-in layer.)

## Ecosystem
- **No existing CLI, SDK (npm/PyPI), MCP wrapper, or community integration for sybill.ai exists.** Every "sybill" package on registries is a homonym (Kerberos daemon, doc-test lib, ML explainability). Clean field.
- Official surfaces only: REST API (alpha), MCP server (`mcp.sybill.ai/mcp`, OAuth, 8 tools incl. `ask_sybill`), webhook automations (no-code).

## Product Thesis
- Name: **sybill** (slug + binary `sybill-pp-cli`), display name **Sybill**.
- Why it should exist: First and only CLI for Sybill. Pulls call intelligence and deal data into the terminal/agents with offline SQLite caching, FTS transcript search, and cross-entity queries the web UI and entity-by-entity API can't answer — plus surfacing `crmAutofill` suggestions no other tool exposes.

## Build Priorities
1. **Priority 0 (data layer):** SQLite store for conversations, conversation_details, deals, accounts, contacts, messages, sources, object_types, rows, documents + sync_log; cursor-based incremental sync; FTS5 over transcripts/summaries/names.
2. **Priority 1 (absorb):** Every GET endpoint as a typed command with cursor pagination, filters, `--json`/`--select`/`--csv`. Ingest endpoints (POST/PATCH/DELETE for conversations, messages, rows, documents, sources, object-types). `doctor` (health + scope check). Webhook signature verify helper (Svix).
3. **Priority 2 (transcend, local-store-only):** weekly `digest`, `deals dark` (gone-dark detection), `crm-autofill` pending-diff, transcript `grep`/FTS `search`, account roll-up, rep/owner activity aggregation.
