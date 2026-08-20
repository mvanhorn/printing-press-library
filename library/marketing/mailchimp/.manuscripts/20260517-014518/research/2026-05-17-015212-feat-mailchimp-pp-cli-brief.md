# Mailchimp Marketing API — CLI Brief

## API Identity
- **Domain**: Email marketing, audience management, campaign orchestration, e-commerce analytics
- **Users**: Marketers, small-business owners, lifecycle/growth teams, e-commerce ops, agencies
- **Data profile**: Hierarchical and relational. Audiences (lists) own members; members own tags, notes, activity, events. Campaigns target audiences, produce reports. E-commerce stores own customers/orders/products that feed personalization.

## Reachability Risk
- **None.** Probe returned `mode: standard_http`, `confidence: 0.95`. No Cloudflare, no CAPTCHA, no JS challenge. Top operational hazard is the 10-concurrent-connection cap per API key (returns 429), which the official `/batches` endpoint exists to bypass.

## Spec Resolution
- **Source**: `https://raw.githubusercontent.com/mailchimp/mailchimp-client-lib-codegen/main/spec/marketing.json` (10 MB, Swagger 2.0)
- **Converted to**: OpenAPI 3.0.0 at `research/mailchimp-oas3.json` (21 MB after expansion) via `swagger2openapi --patch`
- **Servers patched**: replaced placeholder `server.api.mailchimp.com` with `https://us6.api.mailchimp.com/3.0`. Phase 3 will add runtime datacenter parsing from the `-dc` suffix of `MAILCHIMP_API_KEY` so the same binary works for any account.
- **Shape**: 67 resources, 291 endpoints (149 GET / 71 POST / 34 DELETE / 31 PATCH / 7 PUT, after dropping the root `GET /` which has no derivable resource name). Title `Mailchimp Marketing API`, version `3.0.91`.

## Top Workflows
1. **Add/upsert a subscriber and tag them** — `PUT /lists/{id}/members/{subscriber_hash}` + `POST /lists/{id}/members/{hash}/tags`. The single most-used workflow. `subscriber_hash` = `MD5(lowercase(email))` — universal confusion point.
2. **Bulk subscribe from a CSV/data source** — `POST /batches` with an operations array. Async; results downloadable for 7 days, `response_body_url` valid only 10 minutes.
3. **Send a campaign end-to-end** — `POST /campaigns` → `PUT /campaigns/{id}/content` → `POST /campaigns/{id}/actions/send`. Lots of intermediate state.
4. **Campaign performance digest** — `GET /campaigns` + `GET /reports/{campaign_id}` + `GET /reports/{campaign_id}/email-activity`. Mailchimp's own dashboard groups these poorly; agents and analysts want them joined.
5. **Segment health audit** — list all segments per audience, count members, find unused/empty/stale segments.
6. **E-commerce attribution** — `POST /ecommerce/stores/{id}/orders` from external commerce; then read `/reports/{id}/ecommerce-product-activity` to attribute revenue per campaign.

## Table Stakes (from competing tools and SDKs)
- Every official SDK method (~290): list, get, create, update, delete across lists, campaigns, members, reports, automations, ecommerce, templates, files.
- Local cache of audiences + members for offline lookup (no incumbent CLI offers this).
- `subscriber_hash` auto-computed from email — surface the MD5 quirk as a non-issue.
- `--dry-run` on every mutation, `--json`, `--select`, `--csv` on every read.
- Batch creation from CSV/JSON file with progress tracking.
- Campaign send-checklist preview before actually sending.

## Data Layer
- **Primary entities**: `audiences` (lists), `members`, `tags` (per-list), `segments`, `merge_fields`, `campaigns`, `reports`, `automations`, `templates`, `stores`, `products`, `orders`, `customers`.
- **Sync cursor**: All list-shaped endpoints accept `since_*` and `before_*` ISO timestamps + `offset`/`count` pagination (default 10, max 1000). Members support `since_last_changed`. Campaigns support `since_send_time` / `since_create_time`.
- **FTS/search**: free-text search across member email/name, campaign subject/title, segment name, store product name. Mailchimp has `/search-members` and `/search-campaigns` endpoints but they're scoped and rate-limited; a local FTS5 index over synced data is fundamentally more useful.

## User Pain Points (researched)
1. **10 concurrent connections per key.** Universally cited; 11th request → 429. Workaround: serialize OR use `/batches`.
2. **Datacenter routing.** Every wrapper has to parse `key-us6` and rebuild the base URL. Newcomers hit "wrong host" errors. OAuth tokens have no embedded dc — must call `GET https://login.mailchimp.com/oauth2/metadata` once and cache.
3. **`subscriber_hash` = `MD5(lowercase(email))`.** Documented but constantly trips new users.
4. **`PUT` for upsert, `POST` for create-only.** Newcomers `POST` an existing member → `400 Member Exists`.
5. **Official SDK repos have issues disabled.** Community has nowhere to file bugs; the upstream `mailchimp-client-lib-codegen` also disables issues. Friction signal but not a build blocker.
6. **Batch response decoding.** `response_body_url` expires 10 minutes after the batch finishes; downloads are tar.gz of JSONL with envelope per operation. Tooling rarely makes this easy.

## Codebase Intelligence
- **SDK pattern**: Mailchimp generates Node and Python SDKs from a single spec via `mailchimp-client-lib-codegen`. Both ship as `@mailchimp/mailchimp_marketing` / `mailchimp-marketing`. ~150k weekly npm downloads; ~313k weekly PyPI downloads. High install count, no UX innovation — they are thin endpoint-mirrors.
- **Auth header**: `Authorization: Basic base64("anystring:KEY")` (HTTP Basic). Bearer also accepted (`Authorization: Bearer KEY`). Spec declares only `basicAuth`.
- **Rate limiting**: 10 concurrent per key, no per-minute quota. 429 returns `application/problem+json` with `detail` describing the throttle.
- **Architecture**: Single API surface, no GraphQL, no streaming/webhooks-as-API. Webhooks are receive-side only (config via `/lists/{id}/webhooks`).

## Auth Model
- API key format: `{32-hex}-{dc}` (e.g., `22eb...-us6`). DC suffix is the datacenter.
- Base URL: `https://{dc}.api.mailchimp.com/3.0/`.
- HTTP Basic: username = literal `anystring` (or any non-empty string), password = full key.
- Bearer: `Authorization: Bearer {full-key}` — equivalent.
- Env var: `MAILCHIMP_API_KEY`. Optional `MAILCHIMP_DC` override for OAuth tokens.
- OAuth 2.0 supported for multi-tenant; access tokens do not expire.

## MCP Ecosystem
- **Official Mailchimp MCP** exists only for **Transactional** (Mandrill) at `mandrillapp.com/mcp` — not Marketing.
- **6+ third-party MCP servers exist** for Marketing, low engagement (top is 11 stars). Approaches:
  - 280-tool "expose every endpoint" — done at least three times, not getting traction.
  - 2-tool "search + execute" — done once (livemau5).
  - Read-only with dry-run — done once (damientilman).
- **Differentiation lane**: pair a token-efficient MCP surface (Cloudflare pattern: search + execute over the full 291 endpoints, plus a handful of curated typed tools for the highest-gravity ops) with a real CLI that has offline FTS over synced data.

## Product Thesis
- **Name**: `mailchimp-pp-cli` (binary), CLI surface name `mailchimp`
- **Why it should exist**: Every existing Mailchimp wrapper is a thin endpoint mirror. None offer offline search over your audiences, none compose subscribe-tag-segment in one command, none give you a local SQLite of members for ad-hoc SQL, none provide a campaign performance digest joining campaigns + reports + email-activity. The 290-endpoint surface area means the *value* sits in workflows that span multiple endpoints, and that value is currently locked behind clicking around the Mailchimp dashboard.

## Build Priorities
1. **Foundation**: data layer with FTS5 over audiences + members + campaigns + reports + tags + segments. Sync pulls everything once and incrementally after.
2. **Match everything**: all 291 endpoints exposed as typed Cobra commands (the generator handles this).
3. **Transcend**: workflow commands that compose multiple endpoints + the local store. Top candidates (refined in Phase 1.5):
   - `mailchimp subscribe <email> --list <id> --tags ...` — upsert + tag in one call
   - `mailchimp bulk-subscribe --csv <file> --list <id>` — batch endpoint with progress
   - `mailchimp campaign send <id> --check` — run send-checklist first, fail if items present
   - `mailchimp digest <campaign>` — joined performance summary
   - `mailchimp segments audit --list <id>` — find empty/stale segments
   - `mailchimp members sql "<query>"` — ad-hoc SQL over local member data
4. **Datacenter routing in client**: parse `-dc` suffix from `MAILCHIMP_API_KEY`, override base URL at request time. Validate suffix on `doctor`.
