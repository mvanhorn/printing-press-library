# Copper CRM CLI Brief

## API Identity
- Domain: Copper CRM (formerly ProsperWorks) — sales/relationship CRM tightly integrated with Google Workspace.
- Base URL: https://api.copper.com/developer_api/v1
- Spec source: Official Postman collection (97 requests, authoritative). No OpenAPI spec exists. Internal YAML hand-authored from the collection.
- Users: B2B sales teams, sales ops/RevOps analysts, founders/account managers, and integration engineers syncing Copper with external systems.
- Data profile: Relational CRM graph — people, companies, leads, opportunities, projects, tasks, activities, plus pipelines/stages, custom fields, related links, tags, webhooks.

## Auth (RESOLVED — proven against generator)
- Multi-header API key: `X-PW-AccessToken: <token>`, `X-PW-UserEmail: <account email>`, `X-PW-Application: developer_api`, `Content-Type: application/json`. TLS 1.2+.
- Spec wiring: token via `auth.api_key` (header X-PW-AccessToken), email via `auth.additional_headers` (X-PW-UserEmail, env_var COPPER_USER_EMAIL, kind per_call), app+content-type via `required_headers`. Generated client sends all four on every request; doctor checks both creds. Verified on a 2-resource stub.
- Env vars: COPPER_API_KEY + COPPER_USER_EMAIL. Key obtained in Copper web app → System Settings → API Keys → Create a Key.
- OAuth2 exists (authorization_code, scope developer/v1/all) but is partner-gated (email partners@copper.com). Out of scope for v1; API-key path is correct for an internal CLI.

## Reachability Risk
- None. `GET /account` returns 401 with no creds (expected auth challenge), confirming the API responds programmatically. Phase 1.9 = PASS.
- Ecosystem health: zero 403/blocked/deprecated/rate-limit issues across active community wrappers. Auth model stable across wrapper generations. One historical footgun: old `api.prosperworks.com` host (now api.copper.com) — pin to api.copper.com.

## Top Workflows (named rituals)
1. **Monday pipeline forecast** — A RevOps analyst pulls all open opportunities by pipeline + stage, sums weighted value (monetary_value × win_probability), and reports expected revenue for the quarter. Today: export CSV, pivot in a spreadsheet, every week.
2. **Stale-deal sweep** — A sales manager finds opportunities with no interaction in N days (no recent activity/modified date), then reassigns or nudges. Today: scroll the web UI list, eyeball "last contacted" dates manually.
3. **Bulk stage/owner moves** — At quarter close, an account manager advances or reassigns dozens of opportunities at once. Copper has NO bulk endpoint — today they click each record one at a time, or hand-loop a script that trips the rate limit.
4. **Activity logging** — A rep logs calls/notes/meetings against people and opportunities after each touch (counts toward interaction_count / CRM hygiene). Today: open the record in the web app, click Log Activity, type.
5. **Contact sync / dedupe** — An integration engineer upserts people/leads from an external system idempotently (fetch_by_email, leads/upsert by match field). Today: custom scripts handling Copper's people(emails[]) vs leads(email) shape difference.

## Table Stakes (must match — no existing tool does any of this in a CLI/Go)
- Full CRUD + search across people, companies, leads, opportunities, projects, tasks, activities.
- POST /{entity}/search listing with filters, offset pagination (page_number/page_size, max 200, X-PW-TOTAL header), and --all.
- Reference reads: pipelines, pipeline_stages, customer_sources, loss_reasons, contact_types, activity_types, lead_statuses, account, users, tags.
- Custom field definitions CRUD; related items/links; webhooks; file attachments (3-step S3).
- Activities are create/read/delete only (NO update endpoint — do not emit one).

## Data Layer
- Primary entities (SQLite): people, companies, leads, opportunities, projects, tasks, activities.
- Reference tables: pipelines, pipeline_stages, customer_sources, loss_reasons, contact_types, activity_types, users, custom_field_definitions, tags.
- Sync cursor: offset (page_number); shard large pulls by date range to dodge the 100k search ceiling.
- FTS/search: name, email, company, details across entities; SQL composability for cross-entity joins.

## Codebase Intelligence
- No official Go SDK. Best references: ClaimerApp/copper-sdk (Python, modern), salespreso/prospyr (Python, broad resource coverage incl. tasks/projects), Gamesight/copper-typescript (TS REST client), dazanza/copper-mcp (Node MCP, ~9 tools — minimal sane surface). Mine for resource coverage + field selection; none is a Go client.
- Module-path hygiene: avoid bare `copper` Go module collision with gocopper/copper (unrelated web framework, 938★). Binary/slug `copper` is fine.

## Pain Points (concrete)
1. **No bulk write endpoints** — "bulk" means client-side looping, which collides with the rate limit. A CLI with built-in concurrency + 429 backoff is the killer differentiator.
2. **100k search ceiling + no cursor pagination** — large exports must shard by date range and reassemble; offset model drifts on actively-changing data.
3. **Blind rate limiting** — no X-RateLimit-* / Retry-After headers documented; two doc portals disagree (legacy 180/min vs newer ~1000/5min per user-org). Backoff must be heuristic. Pin to ~180/min conservative ceiling.
4. **Activities immutable** — can't correct a logged note (delete + recreate only).

## Product Thesis
- Name: copper-pp-cli — "The Copper CRM command line no one else built."
- Why it should exist: Copper has zero CLI, zero Go client, zero agent-native tool. This is triple greenfield. It turns a click-heavy web CRM into a scriptable, agent-callable, offline-queryable surface — with the bulk operations, weighted-pipeline forecasting, and stale-deal detection the API itself refuses to provide.

## Build Priorities
1. Data layer + sync/search/SQL for all 7 primary entities.
2. Full CRUD + search (absorbed surface) across all entities + reference reads.
3. Transcendence: weighted pipeline forecast, stale-deal sweep, bulk ops with backoff, activity-logging shortcuts, stage velocity, dedupe/upsert.
