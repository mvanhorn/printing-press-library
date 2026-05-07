# Customer.io CLI Brief

## API Identity
- **Domain:** Customer engagement / lifecycle marketing platform — sends triggered emails, push, SMS, in-app, and broadcasts based on identified customer events.
- **Users:** Lifecycle / growth marketers, ops engineers running customer comms, devs integrating identify/track from product code.
- **Data profile:** Customers, segments, campaigns, broadcasts, transactional messages, deliveries, exports, journey metrics, suppressions, integrations, objects (relationship graph), webhooks.

## Reachability Risk
- **Low.** Customer.io is a paid SaaS; programmatic access is the supported path. Searched issues across customerio-{node,python,ruby,go} for `429/401/403/blocked/deprecated` — only one historical 2018 rate-limit issue. No 401/403/blocked complaints across any wrapper.
- Documented limits: Transactional API 100 req/sec; API-triggered broadcasts 1 req/10s (the harshest, will trip retry-loops); Track + App not numerically published, generous in practice.

## API Surface
Three production REST APIs, two reachable with the user's `sa_live_*` Service Account token:

| API | Base URL | Auth | Endpoints | In scope? |
|---|---|---|---|---|
| **Journeys UI API** | `https://us.fly.customer.io/v1/...` (US), `https://eu.fly.customer.io/v1/...` (EU) | Bearer JWT minted from `sa_live_*` via OAuth client-credentials | **785 operations / 600 paths** | YES — primary |
| **CDP Pipelines (control-plane)** | `https://us.fly.customer.io/cdp/api/...` | Same Bearer JWT | **94 operations / 66 paths** | YES — secondary |
| **Track API** | `https://track.customer.io/api/v1/...` | HTTP Basic, Site ID + API Key (separate creds) | ~15 operations | NO — different creds, defer to v2 |
| **CDP write/ingest** | `https://cdp.customer.io/v1/...` | Per-source write key (Basic) | small | NO — separate creds |

**SA token mechanics (verified from `customerio/cli` source — `internal/client/auth.go`):**
1. Client-credentials exchange: `POST https://us.fly.customer.io/v1/service_accounts/oauth/token` with `sa_live_*` as the credential, returns short-lived JWT.
2. JWT used as `Authorization: Bearer <jwt>` for both `/v1/...` and `/cdp/api/...` on the same `*.fly.customer.io` host.
3. JWT carries `account_id` — paths containing `{account_id}` resolve from token state.
4. Region (us/eu) baked into token resolution.

## Top Workflows
1. **Trigger a transactional message** — `POST /v1/send/email` (and SMS/push variants), then poll metrics. The lift Customer.io users automate most.
2. **Export segment members** — `POST /v1/environments/{env_id}/exports/segment_memberships` → poll → download via signed URL. 19 distinct export sub-types: delivery_metrics, journey_metrics, profile_data, tracked_responses, email_suppressions, etc.
3. **Suppress / unsuppress users** — both Journeys (`POST .../customers/unsuppress`) and CDP suppressions endpoints; auditable, reversible bulk ops.
4. **Pull campaign / journey metrics** — `GET /v1/environments/{env_id}/campaigns/{id}/metrics` and `.../journey_metrics` for funnel data. The reporting weakness in the UI is the #1 cited pain point.
5. **Sync identified users from a warehouse** — Reverse-ETL via CDP control plane (`POST /cdp/api/workspaces/{workspace_id}/reverse_etl/syncs`, 11 endpoints), Premium-tier feature.

## Table Stakes (from competing tools)
- **Auth via SA token** (matches official `cio` CLI)
- **Region selection** (us/eu)
- **Workspace switching** (multi-workspace accounts are common at Premium+)
- **JSON in/out** for every command; pipe-composable
- **Dry-run for mutating ops** — Customer.io ops touch real customers, dry-run is mandatory
- **Auth diagnostics** — `doctor` that confirms token, region, account_id, workspaces visible
- **Pagination** — every list endpoint
- **Export download** — write the CSV to disk after polling completes
- **Rate-limit awareness** — adaptive throttle on broadcast triggers (1/10s)

## Codebase Intelligence
- Source: customerio/cli README + `internal/client/auth.go` + `internal/routes/cache.go` (the official CLI, v0.0.4, 1 week old)
- Auth: `sa_live_*` client-credentials → JWT, exchanged at `/v1/service_accounts/oauth/token`, cached until expiry. JWT auto-discovers account_id.
- Data model: Two specs (Journeys + CDP), 879 total operations. Specs identical across regions (region only switches host). No `servers:` or `securitySchemes:` blocks — both filled in by the client.
- Spec versioning: 24h ETag-cached download from live host. The spec is the runtime source of truth for the official CLI.
- Architecture insight: cio takes the "thin schema-driven passthrough" path (`cio api <path>` + `cio schema`). It does NOT build named verbs, NOT cache resources locally, NOT expose MCP.

## Data Layer
- **Primary entities:** customers, segments, campaigns, broadcasts, deliveries, transactional_messages, exports, suppressions, integrations, journeys, environments (workspaces).
- **Sync cursor:** `updated_at` per-resource where exposed; campaigns/segments/journeys are slowly-changing, deliveries are fire-hose.
- **FTS/search:** customer email/id, campaign/segment/broadcast names, transactional template names. Deliveries body-search useful for triage.
- **Why local store wins:** the web UI's reporting/segmentation is the #1 pain point; SQL over synced segment members + delivery rows directly answers "what fraction of segment X opened journey Y" without navigating clunky reports.

## Product Thesis
- **Name:** Customer.io CLI — `customer-io-pp-cli` (binary), slug `customer-io`.
- **Why it should exist:** The official `cio` CLI (1 week old, v0.0.4) is a generic `api <path>` passthrough. Customer.io marketers and ops engineers want named verbs (`campaign metrics`, `segment export --download`), an offline SQLite cache with `sql`/`search` over synced data (the UI's reporting is a known weak spot), and an MCP server (none exists). One auth, two APIs (Journeys + CDP control plane) unified behind one binary, with rate-limit awareness baked in.

## Build Priorities
1. **Auth foundation:** `sa_live_*` → JWT exchange, region select (us/eu), token cache file, `doctor` validates round-trip and lists workspaces.
2. **Highest-value typed verbs from spec:** customers, campaigns, broadcasts, segments, transactional messages, deliveries, exports, suppressions. The endpoint-mirror surface for these is what the LLM agent will use.
3. **MCP enrichment** (the API exceeds 50 endpoints; Cloudflare pattern is mandatory): `transport: [stdio, http]`, `orchestration: code` (search + execute pair), `endpoint_tools: hidden`. Without this enrichment, agents see ~880 raw tools — unusable.
4. **Local SQLite store** — sync segments + deliveries + customers, run `sql`/`search` offline. This is the transcendence layer that the official CLI doesn't have.
5. **Adaptive rate limiter** — broadcast triggers respect 1 req/10s.
6. **Novel commands** — surfaced through the absorb-gate brainstorm.

## What's NOT in scope (v1)
- **Track API** — different auth (Site ID + API Key, Basic auth), different host. Defer; the CLI commits to single-credential SA-token model.
- **CDP write/ingest** — per-source write keys, separate from SA token. Defer.
- **Mobile SDK functionality** (push token registration via app SDK) — not a CLI shape.

## Source Priority
- Single source (Customer.io); no priority gate needed.
