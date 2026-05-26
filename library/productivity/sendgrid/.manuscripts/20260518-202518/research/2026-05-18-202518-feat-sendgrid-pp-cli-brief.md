# SendGrid CLI Brief

## API Identity
- **Domain**: Twilio SendGrid Email API (v3). Transactional mail send + Marketing Campaigns (lists/contacts/segments/single sends/automations) + Suppressions hygiene + Stats + Templates + Subuser admin + Event webhooks.
- **Users**: SaaS/e-commerce transactional senders, marketing/lifecycle teams, multi-tenant ESPs running subusers, deliverability engineers.
- **Data profile**: High-cardinality (millions of contacts, millions of stat rows), heavy time-series in stats, slow paginated exports, suppression lists that need cross-system reconciliation.

## Spec Source
- **Selected**: `twilio/sendgrid-oai (official, bundled from 46 files). 1.4MB, 240 paths, 391 operations, OpenAPI 3.1).
- **Tradeoff**: Official Twilio source, current as of 2026-03-24, bundled by skill at run time.
- **Canonical upstream**: `twilio/sendgrid-oai` (split across 46 YAML files, current as of 2026-03-24). No first-party bundler. Future polish round can re-bundle and regenerate against the live spec.

## Reachability Risk
- **Low**. SendGrid v3 has stable Bearer-token auth, no bot protection, no Cloudflare gates. The OpenAPI mirror is stale, not the API itself.
- **Rate-limit caveat**: 600 req/min global, but Email Activity API dropped to 6 req/min on 2025-12-09 (Twilio changelog). CLI must implement 429 + `X-RateLimit-Reset` backoff.

## Top Workflows
1. **Suppression hygiene** — sync bounces/blocks/spam reports/global unsubs/group unsubs across systems; diff vs internal CRM; bulk add/remove.
2. **Transactional mail send** — single + bulk `/mail/send` with dynamic templates, scheduling, batch idempotency.
3. **Marketing contacts + lists + segments** — slow paginated exports; local mirror enables segment replay, list dedup, custom-field analytics.
4. **Stats aggregation** — categories/mailbox-provider/geo/subuser stats are time-series; offline rollups across windows.
5. **Template + dynamic template version management** — diff, rollback, test-render with variables.
6. **Subuser admin** (ESP-only) — provision, IP pool assignment, per-subuser quotas.

## Table Stakes (from competitor sweep)
- Send transactional mail (every SDK + twilio-cli)
- List/add/remove suppressions of every type (Garoth/sendgrid-mcp, official SDKs)
- Create/list/duplicate templates + versions (Garoth/sendgrid-mcp, mcpmarket)
- Pull stats by date range (mcpmarket "59 tools" hosted MCP)
- Contact + list CRUD (Marketing Campaigns API; Garoth/sendgrid-mcp)
- Single Send create/schedule/send-now (Garoth/sendgrid-mcp)
- API key create/list/scope-edit (official SDKs)
- Event webhook config + signed key (no competitor exposes the verification helper)

## Data Layer
- **Primary entities (high gravity)**:
  1. **Suppressions** (bounces, blocks, spam reports, global unsubs, group unsubs) — high churn, reconciliation, top pain point.
  2. **Contacts + Lists + Segments** — slow API, ideal local mirror.
  3. **Stats** (categories, mailbox-provider, geo, subuser, browser) — time-series.
  4. **Templates + dynamic template versions** — versioned content.
  5. **Single Sends + Automations** — marketing campaign history.
- **Sync cursor**: per-resource updated_at where supported; otherwise full-pull pagination + content hash for change detection.
- **FTS/search**: FTS5 on contacts (email + custom fields), templates (content), suppressions (email + reason), stats categories.

## Codebase Intelligence
- **Auth**: Bearer token in `Authorization` header, env var `SENDGRID_API_KEY`. Subuser impersonation via `on-behalf-of: <username>` header (admin endpoints only — NOT mail send).
- **Data model**: Each resource group is self-contained; minimal cross-references. Marketing Contacts API uses a job-based ingest model (POST /marketing/contacts returns a job_id; clients poll).
- **Rate limiting**: 600/min global, 6/min Email Activity, 429 responses with `X-RateLimit-Reset` (seconds since epoch). Per-endpoint caps vary.
- **Architecture**: Composed v3 surface — old "Marketing Campaigns" (`/contactdb/*`, deprecated but live) coexists with new "Marketing" (`/marketing/*`). CLI must surface both, mark legacy.

## Competing Tools (absorb targets)
- `sendgrid/sendgrid-cli` (official, archive-quality bash — handful of subcommands)
- `tddschn/sendgrid-cli` (Python, send-only)
- `twilio-cli email plugin` (email:set + email:send only)
- `sendgrid/sendgrid-go`, `-nodejs`, `-python` (full v3 SDKs, no CLI surface)
- `Garoth/sendgrid-mcp` (TypeScript MCP; contacts, templates, single sends, stats)
- `garethcull/sendgrid-mcp` (Python MCP; stats pull + template save)
- `deyikong/sendgrid-mcp` (Node)
- `cong/sendgrid-mcp` (Deno)
- `mcpmarket.com/sendgrid` hosted MCP ("59 tools" — biggest competitor surface)
- `davepoon/buildwithclaude` sendgrid-automation skill (routes through Composio MCP)

No first-party Anthropic plugin in `claude-plugins-official` for SendGrid.

## Product Thesis
- **Name**: `sendgrid-pp-cli` (binary: `sendgrid-pp-cli`)
- **Why it should exist**: Every existing tool either covers a sliver (twilio-cli email plugin, official sendgrid-cli) or wraps mail-send in a tighter package. None offers the full 335-operation surface, none has offline SQLite, none does suppression diff/sync, none aggregates stats time-series locally, none ships with agent-native `--json`/`--select`/`--csv` + typed exit codes. SendGrid power users live in suppression cleanup, stats rollups, and contact mirror replay — three workflows no existing tool addresses.
- **Differentiation in one line**: Every SendGrid endpoint, plus offline suppression diffs, stats rollups, and a contacts/templates mirror no other tool has.

## Build Priorities
1. Foundation: store + sync for suppressions, contacts, lists, templates, single sends, stats categories.
2. Absorb: full surface from every competing tool (~335 operations, marked legacy where applicable).
3. Transcend (novel features — to be defined in Phase 1.5):
   - Suppression diff/sync vs CSV/CRM
   - Stats time-series rollups (offline aggregation by window/category/mailbox-provider)
   - Template version diff + rollback
   - Contact segment replay (run a segment locally without hitting the API)
   - Mail-send batch with retry + idempotency tracking
   - Event webhook signature verifier (local, offline)
