# Memberstack CLI Brief

## API Identity
- **Domain:** Membership / auth / gated content for no-code sites (Webflow first, also Framer / vanilla HTML).
- **Users:** Webflow agencies and indie SaaS founders who need login, plans, custom fields, and a small read/write data store without standing up a backend. Power users run audits, exports, JWT verification, and custom-data CRUD outside the Memberstack dashboard.
- **Data profile:** Members (mem_*), plan-connections (pcn_* — free + Stripe-backed paid), data tables (tbl_*) with typed fields, data records keyed by table + member, JWTs issued to logged-in members.

## Reachability Risk
- **None** — `GET /members?limit=1` returned HTTP 200 with real data on first try using the supplied sandbox key. Rate limit 25 rps documented; no Cloudflare/WAF/bot-detection signals. No reachability issues on community wrappers (Memberstack ships the SDK themselves).
- Probe-safe endpoint used: `GET /members` (read-only).
- Tier hints from 4xx body: n/a — 200 OK.

## Top Workflows
1. **List + filter members for analytics or marketing** — export to CSV, find members on free vs paid plans, segment by `customFields`.
2. **Bulk operations across many members** — re-tag, recompute custom fields, force log-out by clearing tokens, scrub PII.
3. **JWT verification in a backend you control** — accept a Memberstack token from a frontend, return claims so your service can authorize the call.
4. **Custom data table CRUD** — list tables, query records (Prisma-style), and upsert per-member data without spinning up another database.
5. **Sandbox / test-account cleanup** — wipe `sk_sb_*` test members, snapshot live members for staging, diff sandbox vs live.

## Table Stakes (from competing tools)
- **Official `@memberstack/admin` Node SDK** — `members.list / retrieve / create / update / delete / addFreePlan / removeFreePlan`, `verifyToken`, data-table CRUD, webhook verification. Synchronous, no offline state.
- **Official Memberstack MCP Server (Beta)** — ~68 tools across members/data tables/plans/access rules, OAuth 2.1 + PKCE, web/agent-driven. No CLI, no offline cache.
- **Zapier / Composio Memberstack integrations** — surface a subset of the API as triggers/actions inside their automation runtimes. No interactive use.
- **`memberstack-skills` Claude Code skill** — small Claude skill that calls the MCP; no CLI of its own.
- **No public CLI tool exists for Memberstack today.** Every workflow goes through (a) the dashboard, (b) a hand-rolled Node script, or (c) the MCP. The gap is real.

## Codebase Intelligence
- **Auth:** Single secret API key in `X-API-KEY` header. Test keys `sk_sb_*`, live keys `sk_*` / `sk_live_*`. Public key (`pk_*`) is browser-side; not used by the Admin API.
- **Data model:** Members own `planConnections[]` (each a plan-connection ID + planId + status + payment); plans themselves are configured in the dashboard and not exposed by REST. Data tables have a `key` and an inferred schema; records hang under `tableKey/records/`. JWTs decode to `{ id, type, iat, exp, aud, iss }`.
- **Rate limiting:** 25 rps documented; expect `429` if exceeded.
- **Quirks:** `PATCH` (not PUT) for member updates. `DELETE` accepts an optional body (`deleteStripeCustomer`, `cancelStripeSubscriptions`). Data Tables live under the `/v2/` prefix; the rest of the API is unversioned. Records query is a Prisma envelope.

## User Vision
User picked the recommended Admin REST API path. No additional vision context provided — proceed with the default of "best general-purpose CLI for the Admin REST API."

## Product Thesis
- **Name:** `memberstack-pp-cli` (binary), `memberstack` slug.
- **Why it should exist:** Memberstack ships an SDK and an MCP but no CLI. Agencies running Webflow projects, ops/marketing people on the customer team, and indie founders all need a way to *script* member data — exports, audits, bulk fixes, JWT decoding, sandbox cleanup — without writing a Node script for every task. A token-efficient CLI with a local SQLite mirror enables fast offline `search`, `stale`, `audit`, and CSV/JSON pipes that the dashboard, MCP, and SDK can't match.

## Build Priorities
1. **Foundation (Priority 0):** SQLite store with `members` and `data_records` tables, generic `resources` mirror, `sync --full` and incremental cursor sync, FTS5 search.
2. **Absorb (Priority 1):** Every endpoint in the spec gets a typed Cobra command (member list/get/create/update/delete, add/remove free plan, verify-token, data-tables list/get, records create/update/delete/query). Match every method on the Node SDK.
3. **Transcend (Priority 2):** Novel commands a CLI can do that no other tool offers — `stale`, `bulk-delete-test`, `token decode`, `export`, `audit`, `watch`, `plan-coverage`, `fields-flatten`, `records query` shorthand, `webhook-sim`.
