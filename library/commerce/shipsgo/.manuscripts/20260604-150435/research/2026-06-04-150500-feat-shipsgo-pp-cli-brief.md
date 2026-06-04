# ShipsGo CLI Brief — Re-Validated for printing-press v4.20.1

> Re-validated 2026-06-04 against printing-press v4.20.1 (prior research targeted v4.5.2). Reachability, spec URL, endpoint count, and auth shape all confirmed unchanged. Full reuse with v4.20-specific notes appended.

## API Identity

- **Domain:** Ocean + Air freight shipment tracking (container, master BOL, AWB)
- **Users:** Freight forwarders, NVOCCs, logistics ops, importers/exporters, BCOs, TMS integrators
- **Data profile:** Long-lived tracking records (weeks per voyage), frequent milestone updates. Small per-record (~50 fields). Volume scales with active shipment count.
- **Spec:** OpenAPI 3.1.0 at `https://api.shipsgo.com/docs/v2/specs/openapi.json` — **22 operations across 16 paths**, two symmetric domains (`/ocean` + `/air`).
- **Base URL:** `https://api.shipsgo.com/v2`
- **Auth:** API key in `X-Shipsgo-User-Token` header → env var `SHIPSGO_USER_TOKEN`. Security scheme name in spec: `Token`.
- **Rate limit:** 100 req/min collective per org. Returned in `X-RateLimit-*` headers.
- **Credits:** 1 credit per container/AWB tracked on POST. 3 credits on free signup. Unlimited reads after creation.

## Reachability Risk — RE-VALIDATED

- **Status:** **Low.** `probe-reachability` returned `browser_clearance_http` (confidence 0.6) but this is a **false positive** confirmed by direct probe:
  - `curl -sI https://api.shipsgo.com/docs/v2/specs/openapi.json` → HTTP/2 200, content-type application/json
  - `curl https://api.shipsgo.com/v2/ocean/shipments` (no auth) → HTTP 401, content-type `application/json`, 27-byte body: `{"message":"TOKEN_MISSING"}`
- This is a documented REST API designed for backend integration. The Cloudflare front is a CDN, not a bot challenge. **Use standard HTTP transport. Do NOT use Surf. Do NOT include `auth login --chrome`.**
- The v4.20 generator may auto-pick `browser-chrome` transport on `browser_clearance_http` mode. **Override with `--transport standard` if needed**, or set `http_transport: standard` in the spec/overlay before generation.

## Top Workflows

1. **Track a container or AWB end-to-end** — POST shipment with container/BOL/booking-ref (ocean) or AWB (air); poll milestones, ETA, vessel position. Webhook events fire on status change.
2. **Maintain a live shipment book** — list active shipments filtered by status / ETA window / port / carrier / tag. The freight-ops dashboard view.
3. **Compare carrier reliability for RFQ** — look up the 160+ ocean carriers / 90+ airlines and historical transit performance to inform quote selection.
4. **Broadcast updates to stakeholders** — followers (per shipment) + tags (RFQ / customer / project grouping). Webhook events relay milestone changes.
5. **Build the RFQ workspace** — group shipments by tag (`rfq:Q3-LCL-Asia`), see transit-time spread across carriers on a lane, project ETAs into rate-comparison sheets.

## Table Stakes

- POST/GET/PATCH/DELETE shipments (ocean + air)
- List shipments with filters (status, carrier, ETA, tags, dates)
- GeoJSON route polyline (vessel/aircraft path)
- Carriers list (ocean) + Airlines list (air)
- Followers and tags subresources (POST/DELETE)
- Standard pagination + sort
- API-key auth in a single header

## Data Layer

- **Primary entities:** `ocean_shipments`, `air_shipments`, `ocean_carriers`, `air_airlines`, `shipment_followers`, `shipment_tags`, `webhook_events`
- **Sync cursor:** `updated_at` per shipment; list endpoint accepts page+limit
- **FTS5 candidates:** container_number, master_bol, booking_ref, awb, carrier_name, port codes, tag names
- **Relations:** `shipments ↔ tags (m2m)`, `shipments ↔ followers (1-many)`, `shipments → carrier_code`
- **Local-store wins:**
  - Offline carrier/airline lookup
  - ETA delta history per shipment for SLA/reliability scoring
  - RFQ aggregation by tag
  - Credit budget tracking (avoid re-tracking known refs)

## Codebase Intelligence

- **No public SDK.** The `shipsgo` GitHub org has 1 repo (`.github` only). Every integrator hand-rolls a REST client.
- **MCP servers:** None published.
- **Community wrappers:** One PHP wrapper covering ~5 endpoints (incomplete; ocean-only).
- **Auth pattern:** single `X-Shipsgo-User-Token` header. No OAuth, no scopes beyond user role.
- **Data model:** shipment is the primary aggregate; everything else is metadata/decoration. Symmetric Air ↔ Ocean shape — same verb set on each domain.
- **Architecture:** pure REST, JSON over Cloudflare-fronted edge. Webhook deliveries to user-controlled URLs.

## User Vision

The user has a working FF system in this repo (`apps/ff-api/src/rst_ff/shipsgo.py`) that already calls ShipsGo for FF tracking. The CLI's purpose is twofold:

1. **Operator/debug tool** — `shipsgo` ops team can run `shipsgo schedules --pol IDJKT --pod SGSIN` from the terminal to sanity-check coverage before quoting a new lane, inspect a single shipment, batch-import BOLs, etc.
2. **Reference for the Python client** — DCSA mapping coverage in [apps/ff-api/src/rst_ff/dcsa.py](apps/ff-api/src/rst_ff/dcsa.py) is currently best-effort (~10 event mappings). The CLI's research artifacts will tighten that mapping.

The CLI is a **complement** to the in-process Python client, not a substitute. The Python client owns the synchronous create-shipment + DB-write path; the CLI owns the read-side + ops + analytics.

## Source Priority

Single-source CLI. ShipsGo is the only data source; RFQ structures and analytics live in the local SQLite store.

## Product Thesis

- **Binary name:** `shipsgo-pp-cli`
- **Display name:** `ShipsGo` (canonical brand casing)
- **Why it should exist:**
  - No CLI, no SDK, no MCP exists for ShipsGo today
  - Every freight forwarder integrating ShipsGo hand-rolls a REST client + spreadsheet workflow
  - A single agent-native CLI that (a) absorbs the full REST surface, (b) persists tracked shipments locally for sub-second reads + offline analytics, (c) layers an RFQ-shaped grouping/comparison model on top, replaces multiple manual workflows.
- **What only this CLI can do:** historical ETA-vs-actual deltas, RFQ-tagged carrier-comparison views, transit-time lane analytics, credit-budget guardrails before write calls. None are available via the dashboard or raw API.

## Build Priorities

1. **Foundation:** unified data layer for ocean + air shipments, carriers, airlines, tags, followers; webhook event log table; cursor-based `sync`; FTS5 over container/BOL/AWB/carrier/port.
2. **Absorb (full REST):** every endpoint on both Air and Ocean, with `--json`, `--select`, `--dry-run`, `--limit`, typed exit codes. Read-only by default; writes require explicit confirmation.
3. **Transcend (RFQ + analytics):** `rfq new/show/compare`, `lane stats`, `eta-drift`, `credit-budget`, `book` (live shipment book), `webhook tail`, etc. — see absorb manifest for the full list of 11.

## v4.20 Re-Validation Deltas vs v4.5.2 Prior Run

| Bucket | Status | Action |
|---|---|---|
| Transport / reachability | Same false-positive `browser_clearance_http` classification on v4.20. Decision unchanged: standard HTTP transport. | Override `http_transport: standard` if generator auto-selects `browser-chrome`. |
| Scoring rubrics | No change to brief's transcendence list. Prior pattern (offline SQLite-backed analytics + RFQ workspace) still scores high on the v4.20 transcendence rubric. | Reuse prior absorb manifest. |
| Auth modes | `api_key` with `X-Shipsgo-User-Token` header. Single env var. | Generator emits canonical env var name; no overlay needed. |
| MCP surface | 22 operations + ~13 framework tools + ~11 transcendence commands ≈ 46 tools. **Below the 50-tool Cloudflare-pattern threshold**, above the 30-tool default-fine threshold. | Add `mcp.transport: [stdio, http]` to enable remote reach. Skip code-orchestration; the surface is small enough for endpoint-mirror to work fine. |
| Discovery | Spec is complete and authoritative. No browser-sniff needed. | Marker file written: `skip-silent` with evidence. |

## Notes for generation

- Spec operations are missing `operationId` — generator will derive command names from method+path. Expect names like `oceanShipmentsList`, `oceanShipmentsCreate`. Polish phase will rename to clean Cobra paths (`shipments ocean list`, `shipments ocean create`).
- Air and Ocean are structurally identical. Generator will mirror per-domain; novel commands fold across both.
- No GraphQL, no SSE, no WebSocket. Pure REST.
- Auth env var: `SHIPSGO_USER_TOKEN` (canonical; matches header name).
- Pre-generation spec enrichment to apply:
  - Add `mcp.transport: [stdio, http]` (remote-capable MCP)
  - Confirm `auth.type: api_key`, `header: X-Shipsgo-User-Token`, `env_vars: [SHIPSGO_USER_TOKEN]`
  - Set `http_transport: standard` if the generator's default reads the probe's `browser_clearance_http` mode literally.
