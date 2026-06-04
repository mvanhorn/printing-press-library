# Zoho Expense CLI Brief

## API Identity
- Domain: expense management / SaaS accounting (Zoho One suite)
- Users: solo professionals, SMBs, finance teams; this user is a solo professional in India running monthly expenses
- Data profile: receipts, expenses, expense reports, categories, tags, projects, customers, merchants, currencies, taxes; moderate volume (10s-100s of expenses/month per user)
- Docs root: https://www.zoho.com/expense/api/v1/
- Region (target): India — `www.zohoapis.in/expense/v1`, `accounts.zoho.in/oauth/v2/`, console at `api-console.zoho.in`

## Reachability Risk
- None. www.zohoapis.in is a normal HTTPS REST endpoint. No Cloudflare interstitial, no JS challenge, no reCAPTCHA. Rate limits: 100/min per org, 10000/day on Premium.
- Cross-DC token rejection: a `.com` token will not work against a `.in` org and vice versa. Token response includes `api_domain`; persist and use it.
- Refresh tokens do not expire but cap at 20 per user/client (21st rotates out the oldest); max 5 generated per minute.

## Top Workflows
1. **Invoice ingestion → tagged expense** (Hermes use case): receive PDF/image invoice from email, POST /expenses multipart, poll until `autoscan_status: Processed`, then PUT /expenses/{id} to fill category/project/reporting tags/billable/customer.
2. **Monthly expense bundling**: list unreported expenses for a month, create expense report, attach expenses, submit/auto-approve.
3. **Search & query**: "what did I spend on AWS last quarter", "find duplicates", "uncategorized expenses this month" — powered by local SQLite mirror.
4. **Receipt dedup before upload**: hash incoming file, check local store, skip or merge if already present.
5. **First-merchant categorization**: Zoho auto-categorizes a merchant only after it has been seen + categorized once. Train the local merchant→category map from history and pre-fill on PUT.

## Table Stakes (from Ramp CLI gold standard)
- OAuth login with transparent refresh
- JSON-by-default when piped or `--agent`
- Receipt upload + attach
- Transaction/expense list with filters and pagination
- Reports approve/reject/recall
- Reimbursement actions
- `users me` / `whoami`
- Filters as flags, not raw query strings
- Typed exit codes
- `--dry-run` on mutating commands

## Data Layer (SQLite)
Primary entities (priority order):
- **P0**: expenses, expense_categories, reporting_tags (+ options), merchants (derived)
- **P1**: expense_reports, projects, customers
- **P2**: users, currencies, taxes, organizations

Sync cursor: Zoho Expense does NOT expose `last_modified_time` on list endpoints (unlike Books). Strategy:
- Window by `date_start`/`date_end` (default: last 90 days on first sync, then last 30 days on incremental)
- Dedup on `expense_id`
- Track high-water mark: latest `expense_date + created_time` seen
- Force full re-fetch with `--full`

FTS5: index expense (merchant_name, description, reference_number, line_items[].description), categories (name), reporting_tags (name, option_name), projects (project_name), customers (contact_name).

## Codebase Intelligence
- Source: research subagent + first-party docs (no first-party SDK for Expense; no MCP server for Expense)
- Auth: OAuth 2.0 self-client; access token (1h) + refresh token (no expiry, 20-token cap); header `Authorization: Zoho-oauthtoken <token>`
- Required header on every API call: `X-com-zoho-expense-organizationid: <org_id>` (or `?organization_id=<id>` query param)
- Data model: standard Zoho-style — `expense_id`, `expense_date`, `total`, `currency_id`, `category_id`, `merchant_id|merchant_name`, `customer_id`, `project_id`, `is_billable`, `is_reimbursable`, `is_inclusive_tax`, `tax_id`, `description`, `reference_number`, `custom_fields[]`, `line_items[]` with `tags[]` (`tag_id`/`tag_option_id`), `mileage_type`, per-diem fields
- Rate limiting: 100/min per org, daily by plan; HTTP 429 on overrun. No Retry-After header documented — exponential backoff.
- Architecture: pagination via `page` + `per_page` + `page_context.has_more_page`. Receipt upload is multipart with field `receipt=@file`. Most write endpoints accept either raw JSON or legacy `JSONString=` form field — prefer raw JSON.

## User Vision
**Region:** India (.in). User has Zoho self-client on api-console.zoho.in.
**Primary goal:** Hermes agent ingests invoices from inbox monthly and uploads them as tagged expenses on Zoho Expense.
**Tagging needs:** category, project (for client work), reporting tags (the user's chart of accounts), billable flag, customer attachment. GST handling for India.
**Auth supply:** self-client + 10-min auth code → exchange for refresh + access tokens. User will provide client_id, client_secret, and a fresh auth code at Phase 5 (live smoke).

## Product Thesis
- **Name:** zoho-expense-pp-cli
- **Why it should exist:** Zoho ships half the loop — autoscan extracts the receipt. Tagging (category, project, reporting tags, GST inclusion, billable, customer) is left to the user. No first-party SDK exists for Zoho Expense (CRM/Books/Inventory/Subscriptions/Desk/Creator are covered; Expense is not). No MCP server exists (Zoho's MCP is Books-only). No competing CLI exists. The Ramp CLI is the gold standard for agent-friendly expense management; this CLI matches that shape on Zoho's much larger but unclaimed surface.
- **Wedge:** receipt-first workflow optimized for AI agents (Hermes) that ingest invoices from email and post tagged expenses on a cadence.

## Build Priorities
1. **Auth + org discovery**: `auth self-client` (paste client_id/secret/code), `auth refresh`, `auth status`, `org list`, `org use`. Persist `api_domain` from token response. India default.
2. **Receipt upload + autoscan loop**: `receipt upload <file> [--auto-tag] [--wait] [--timeout 60s]` — POST /expenses multipart, poll until processed, optionally invoke local tagging based on filename/text + local schema.
3. **Expense CRUD + tagging**: `expense list`, `expense get`, `expense create`, `expense update`, `expense tag <id> --category=... --project=... --tag=name:option ...`, `expense merge <id> <id>` (dedup).
4. **Reports lifecycle**: `report list`, `report get`, `report create`, `report add-expenses`, `report submit`, `report approve|reject|recall|reimburse`.
5. **Local sync + offline query**: `sync` (window strategy), `search "<text>"` (FTS5 across expenses + descriptions + line items), `sql <query>` for power use.

### Transcendence (only possible with our approach)
1. **Invoice intake pipeline**: `invoice ingest <dir-or-file> [--auto-tag] [--report-month=YYYY-MM]` — batch upload a folder of saved invoices, hash-dedupe against local store, poll autoscan for all in parallel, auto-tag from learned merchant→category map, optionally bundle into the month's report. The Hermes hot path.
2. **Merchant→tag memory**: locally trained merchant→category, merchant→project, merchant→tag mapping built from synced history. Pre-fills tags on first sight of a new merchant — bypassing Zoho's "first-time merchant is manual" limitation.
3. **Duplicate hash gate**: SHA256 of receipt file + perceptual hash of PDF text, recorded at upload time. `receipt upload` refuses (or `--force`) when the same hash already produced an expense. Solves Zoho's weak server-side dedup.
4. **Monthly close**: `close --month=YYYY-MM` — find unreported expenses for the month, list anything still in `autoscan_status: Processing`, list anything untagged, create the report, attach, optionally submit. One-shot month-end automation.
5. **GST/CGST/SGST split**: India-specific — `expense gst-split <id>` parses the tax_id and line items, computes CGST/SGST/IGST shares, updates expense or emits a CSV the user can hand to their CA.

## Auth Surface
- **Type**: OAuth2 self-client authorization code flow (custom — not standard `authorization_code` with redirect; user generates code from console)
- **Header**: `Authorization: Zoho-oauthtoken <access_token>` (literal prefix, NOT `Bearer`)
- **Required header per request**: `X-com-zoho-expense-organizationid`
- **Token endpoint** (India): `https://accounts.zoho.in/oauth/v2/token`
- **Env vars**: `ZOHO_EXPENSE_CLIENT_ID`, `ZOHO_EXPENSE_CLIENT_SECRET`, `ZOHO_EXPENSE_REFRESH_TOKEN`, `ZOHO_EXPENSE_REGION` (default `in`), `ZOHO_EXPENSE_ORGANIZATION_ID`
- **Scopes** (start): `ZohoExpense.fullaccess.ALL` (solo user) — narrowable per-command later

## Rate Limit Strategy
- Honor 100/min via adaptive limiter (cliutil.AdaptiveLimiter)
- Daily cap surface via doctor (`X-RateLimit-*` headers if present)
- Receipt autoscan poll: backoff 2s → 4s → 8s → 16s, max 60s

## Risks / Gaps
- No `last_modified_time` on list endpoints → date-window sync only
- Autoscan completion has no webhook → polling only
- No documented merchant CRUD endpoint → derive merchants from expense history
- No documented "submit report" endpoint → state changes via PUT
- Some endpoints are plan-gated (advances, certain report actions on Free) → handle 403 gracefully with edition hint
