# PushPress CLI Brief

## API Identity
- **Domain:** gym management software (members, attendance/check-ins, classes, billing) for i2 Fitness and similar small-to-mid gyms.
- **Users (this run):** i2 Fitness operator (Alex) and the operator's downstream consumers — [[trainer-dashboard]] and [[business-dashboard]]. The daily churn-dashboard view (signups + cancellations + going-dark) is what gets checked every morning; those commands must be fast and one-line-per-row by default.
- **Data profile:** thousands of customers, many check-ins per day, dozens of classes/week. Reads vastly outnumber writes.

## Reachability
- **PASS.** `GET https://api.pushpress.com/v3/customers` with a fake bearer returns HTTP 401 "Unauthorized" — DNS + TLS + spec shape all correct; the API rejects only on auth.
- Risk: **None** for the `/v3/` surface. The internal `/v2/` surface (see Coverage Gap below) wasn't probed live and isn't part of this run.

## Auth
- **API key as `Authorization: Bearer <key>`** plus an optional **`companyId` HEADER** for tenant scoping.
- Discovered companyId tenant-scoping pattern; this is a third tenant-scoping shape beyond press issue #1305 (path-positional `/tenant/{id}/...`) and #1355 (query-param `?locationId=<id>`). Worth noting in retro.
- Servers: `https://api.pushpress.com` (production), plus `pushpressstage.com` and `pushpressdev.com` non-prod environments.
- Per the briefing, Alex has the key and will paste it via env var (`PUSHPRESS_API_KEY`) for live smoke testing.

## Coverage Gap (load-bearing for this run)
PushPress maintains two parallel APIs. **`/v3/` (the published Platform API) covers only ~3 of Alex's 8 must-have categories.** The other 5 categories exist only on `/v2/` (the internal dashboard API), which isn't publicly documented and requires browser-sniffing the logged-in dashboard to reach.

| # | Must-have | `/v3/` coverage | `/v2/` evidence |
|---|---|---|---|
| 1 | Members list + filters + search | Partial — `/customers` list (page/limit only), `/customers/{id}` | `/v2/client` (richer) |
| 2 | Signups + source attribution | Partial — `customers.dateAdded` derives recent signups; **source field not exposed** | `/v2/activity` likely |
| 3 | Cancellations + freeze-vs-cancel | **Not in /v3** | PushPress publishes a churn report; `/v2/` likely has it |
| 4 | Attendance / going-dark | Partial — `/checkins` list (page/limit only); derive going-dark by joining cached customers + last check-in locally | `/v2/calendar/report/no-show/get-buckets`, `/get-persons`, `/get-summary` (no-show reporting explicit) |
| 5 | Classes/sessions + instructor view | **Not in /v3** (class names appear inside `ClassCheckin` schema but no list endpoint) | `/v2/calendar` |
| 6 | Plans + MRR + failed payments | **Not in /v3** | `/v2/plans`, `/v2/billing`, `/v2/product`, `/v2/subscription` |
| 7 | Tasks/notes | **Not in /v3** | `/v2/task` |
| 8 | Leads + source + conversion | **Not in /v3** | `/v2/client` general surface |

**Per Alex's briefing rule** ("flag the gaps — do NOT silently skip or fake the data"), the 5 `/v2/`-only categories ship as **explicit stub commands** in this CLI. Each stub prints a `not supported by PushPress /v3` message naming the missing endpoint family and points the user at the documented follow-up: a `/v2/` browser-sniff that hasn't been authorized yet for actual capture. See `$DISCOVERY_DIR/v2-paths.md` for the enumerated `/v2/` path list (extracted from the dashboard SPA bundle) that would feed that follow-up.

## Top Workflows (priority order from briefing)
1. **Daily churn dashboard:** signups today/this-week + cancellations today/this-week + going-dark report. The first two are gap-stubbed; the third is the headline `/v3/` transcendence (joins synced customers × cached check-ins).
2. **Member lookup:** by id, email, phone, name. `/v3/customers/{id}` for id; client-side filtering for email/phone/name after sync.
3. **Attendance audit:** who attended in date range, who hasn't visited in N days. Both via the local store (the API exposes `/checkins` list but with no server-side date or member filter).
4. **Trainer dashboard read:** for each customer, last visit + plan + status. `/v3/customers` covers the data; the CLI exposes it in a one-line-per-row default.
5. **Cross-system reference:** stable customer IDs (matches what GHL writes back). Don't build the cross-join command this run; keep field shapes friendly for it later.

## Table Stakes (existing tooling)
- **Official Speakeasy SDK** — `PushPress/pushpress-ts` (private repo; mirror at `speakeasy-sdks/pushpress-typescript-sdk`). Generates from the same `openapi.draft.yaml` we're consuming here. 20 endpoints. Pagination on list endpoints via `for await...of`.
- **npm `@pushpress/pushpress`** — published SDK package.
- **Zapier integration** — three pushpress zaps available (find-person, create-person, subscribe-to-plan). Customer-focused, same surface as `/v3/`.
- **No MCP server, no community CLI, no Claude skill found** for PushPress. The CLI is first of its kind.

## Data Layer
- **Primary entities:** `customers` (members), `checkins` (with three subtypes: `ClassCheckin`, `AppointmentCheckin`, `EventCheckin`), `company`. Webhook configs and message-send endpoints don't need a store.
- **Sync cursor:** `/v3/customers` and `/v3/checkins` paginate via `page`/`limit`. No `since`/cursor mechanism documented — store stamps `synced_at` on each row and refreshes the full page set on `sync --full`.
- **FTS / search:** SQLite FTS5 over `customers.firstName`, `customers.lastName`, `customers.email`, `customers.phone`. For check-ins, FTS on event/class titles where present.

## Codebase Intelligence
- Source: `speakeasy-sdks/pushpress-typescript-sdk` (Speakeasy-generated; openapi.draft.yaml ~33KB, OpenAPI 3.1).
- Auth pattern: `apiKey` security scheme, sent as `Authorization: Bearer <key>`.
- Required-header pattern: optional `companyId` for some endpoints.
- Pagination: `page` + `limit` query params on list endpoints; `for await...of` shape in the SDK.
- Rate limiting: not documented in the public spec. Apply conservative `cliutil.AdaptiveLimiter` defaults.

## User Vision (from briefing)
- "Build a CLI for PushPress — gym management software. … This is the system-of-record for member ops at i2."
- "The churn dashboard view (signups, cancellations, going-dark) is what Alex checks daily — those commands must be fast and one-line-per-row by default."
- "member-list default: one line per member (id, name, plan, status, last visit). Full payload behind --full or --select."
- "If the official API is missing any of the use-cases above (e.g., attendance detail, lead source), flag the gaps — do NOT silently skip or fake the data."
- "I'll authorize reverse-engineering as a follow-up if needed."
- Cross-system reasoning with GHL → keep IDs/field shapes compatible.

## Product Thesis
- **Name:** `pushpress-pp-cli` (binary), `pp-pushpress` (skill).
- **Why it should exist:**
  1. First Claude-grade CLI for PushPress — no existing CLI or MCP today.
  2. The local-store joins enable "going-dark" (`who hasn't visited in N days`) and `customers recency` reports that the API genuinely cannot answer in one call.
  3. Companion to [[ghl-cli-shipped]] on Alex's stack — keeps customer IDs/field shapes friendly so a later cross-CLI join is possible.
  4. Honest about gaps — the 5 categories not in `/v3/` ship as explicit stub commands per the briefing rule, NOT silently dropped, so an agent reading help text knows exactly which surface is real.

## Build Priorities
1. **Generator handles the /v3 typed mirror** — 20 endpoints across customers, checkins, apps, webhooks, messages, keys, company.
2. **Hand-build the headline going-dark report** — joins synced customers with their most-recent check-in; `--days N` filter; one-line default.
3. **Hand-build the daily KPI ticker** for the business-dashboard cron (signups today from cached customers, total check-ins today, etc.).
4. **Hand-build the gap-stub commands** for billing/plans/leads/cancellations/classes — they print `not supported by PushPress /v3` and document the /v2 follow-up. This is the brief's load-bearing instruction.
5. **README `## Known Gaps`** with the same 5-category list so users know before invoking.

## Source Catalog
- [GoHighLevel/pushpress-ts (private)](https://github.com/PushPress/pushpress-ts) — official TS SDK
- [speakeasy-sdks/pushpress-typescript-sdk](https://github.com/speakeasy-sdks/pushpress-typescript-sdk) — public mirror; openapi.draft.yaml source
- [Scalar API docs](https://ppe.apidocumentation.com/) — published Platform API docs
- [PushPress developer portal](https://developer.pushpress.com/docs) — SPA portal (login-gated, no public spec)
- [Zapier PushPress integration](https://zapier.com/apps/pushpress/integrations) — alternative integration surface
- [npm @pushpress/pushpress](https://www.npmjs.com/package/@pushpress/pushpress) — published SDK
