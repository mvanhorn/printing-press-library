# FedEx Developer API — Research Report

Target: Go CLI for shipping ops teams wrapping the modern FedEx REST APIs (OAuth2). Sandbox `https://apis-sandbox.fedex.com`, prod `https://apis.fedex.com`.

## 1. API Surface Inventory

The catalog page (`developer.fedex.com/api/en-us/catalog.html`) returns errors when fetched headlessly; inventory below is reconstructed from per-API doc pages, the official Postman workspace, and the migration guide.

**Core (top-tier; CLI must cover well):**

- **Ship API** (`/ship/v1/...`) — labels, tags, validation, cancellation.
  - `POST /ship/v1/shipments` — Create Shipment (single or multi-piece, generates label)
  - `POST /ship/v1/shipments/packages/validate` — Validate without consuming a label
  - `PUT  /ship/v1/shipments/cancel` — Cancel by tracking# + account
  - `POST /ship/v1/shipments/tag` — Create return tag
  - `PUT  /ship/v1/shipments/tag/cancel` — Cancel return tag
  - `POST /ship/v1/shipments/results` — Async ship results by jobId
- **Open Ship API** (`/openship/v1/...`) — Multi-piece shipments built across calls.
- **Rates and Transit Times API** — `POST /rate/v1/rates/quotes`. All services for an O-D pair, surcharges, transit days, package-level charges, on-call pickup pricing. Supports alcohol/dangerous goods specials.
- **Tracking API ("Basic Integrated Visibility")** — `POST /track/v1/trackingnumbers`, `/associatedshipments`, `/shipments` (by reference), `/proofofdelivery`. **30 numbers per request max**. ETA + picture POD (US only).
- **Address Validation API** — `POST /address/v1/addresses/resolve`. **100 addresses per request**. Full validation US/CA; partial international.

**Secondary:**

- **Pickup Request API** — `/pickup/v1/pickups`, `/availability`, `/cancel`.
- **Locator API** — `POST /location/v1/locations/search-locations`.
- **Postal Code Validation API** — `/country/v1/postalcodes`.
- **Service Availability API** — `/availability/v1/...`.
- **Country Service API** — `/country/v1/countries/...` (clearance docs metadata).

**Specialized:**

- **Trade Documents Upload (ETD)** — `POST /documents/v1/etds/upload`. Pre-shipment (returns docId for reference) or post-shipment (PSDU). Up to 5 docs per call. Accepts DOC/XLS/TXT/RTF/JPG/GIF/BMP/TIF/PNG/PDF.
- **Image Management** — Letterhead/signature PNGs (14:1 / 10:1) for ETD.
- **FedEx Ground Economy** (was SmartPost) — Service type on Ship API, not separate.
- **Advanced Integrated Visibility (Webhooks)** — `POST /track/v1/notifications`. HMAC-signed (`fdx-signature` header). US accounts only.

**Out of scope:** FedEx Supply Chain APIs (`dev.supplychain.fedex.com`) are a separate portal and OAuth surface — don't target unless asked.

## 2. Auth Flow

**OAuth2 client_credentials**, 1-hour bearer, `Authorization: Bearer <token>` on subsequent calls.

- **Token endpoints:**
  - Sandbox: `POST https://apis-sandbox.fedex.com/oauth/token`
  - Production: `POST https://apis.fedex.com/oauth/token`
- **Request:** `Content-Type: application/x-www-form-urlencoded`, body `grant_type=client_credentials&client_id=...&client_secret=...`. Credentials in body, NOT HTTP Basic (despite some blog samples).
- **Grant types:**
  - `client_credentials` — standard projects (the common case).
  - `csp_credentials` — Compatible Solution Providers / Integrators. Adds `child_key` + `child_secret` (Customer Key/password from Credential Registration API).
  - `client_pc_credentials` — Proprietary Parent-Child accounts. Same `child_key`/`child_secret` extension.
- **Response:** `{ access_token, token_type: "bearer", expires_in: 3600, scope: "CXS" }`.
- **Headers on API calls:** `Authorization: Bearer <token>`, `Content-Type: application/json`. Practically required: `X-locale` (e.g., `en_US`) and `X-customer-transaction-id` (echoed back for tracing).
- **Live shipping prerequisites:** Production label printing requires **Bar Code Analysis Group (BAG) / Shipper Validation** approval per project — generate test labels in sandbox, submit, FedEx validates barcode placement, then enables prod transmission. Account number is in the request body (`accountNumber.value`), not a header. **Meter number is gone in REST** (it was a SOAP concept). 3rd-party billing uses `shippingChargesPayment.payor`.

## 3. OpenAPI Spec Availability

**No canonical OpenAPI spec is published by FedEx.** Searches across `github.com/FedEx`, `apis-guru/openapi-directory`, generic GitHub code search, and the developer portal turn up nothing usable.

**What does exist:**

- **Official Postman workspace** (FedEx DevRel): `https://www.postman.com/devrel/workspace/fedex/overview` — separate collections for Ship, Track, others. **Convertible to OpenAPI 3** via `postman-to-openapi` / APIMatic. This is the cleanest spec source. Quality after conversion: moderate (types inferred from examples).
- **Community Postman collections:** `postman.com/trackingmore/fedex-tracking-api/...` (tracking only, third-party); `postman.com/justransform-dev/jt-external/.../fedex-restful-api`.
- **FedEx Supply Chain (separate product) DOES expose swagger JSON** at `developer-sandbox.supplychain.fedex.com/sandbox/ibm_apim/swaggerjson/...`. **Not applicable** to the shipping/rate/track APIs the CLI targets.
- Per-endpoint HTML docs on `developer.fedex.com` show schemas in tabbed views but aren't downloadable.

**Recommendation:** convert the official Postman collection to OpenAPI 3, hand-augment auth (OAuth2 grant types), required headers (`X-locale`, `X-customer-transaction-id`), and the Tracking webhook (Postman collections rarely model webhooks). Treat as Postman-derived crowd-sniff, not vendor spec.

## 4. Competing Tools

| Tool | URL | Lang | Stars | Notes |
|---|---|---|---|---|
| **python-fedex** | github.com/python-fedex-devs/python-fedex | Py | ~155 | ~890 wk dl. **SOAP only** (rate, track, AVS, location, ship, pickup, country availability, commitment). Last release Feb 2020. Dead at SOAP retirement. |
| **happyreturns/fedex** | github.com/happyreturns/fedex | Go | 3 | SOAP only. `Ship()` + tracking by tracking#/PO/shipper-ref. v1.0.19 Dec 2023. |
| **tcolar/fedex** | github.com/tcolar/fedex | Go | small | SOAP, tracking-only. |
| **fedex-nodejs** | npmjs.com/package/fedex-nodejs | Node | small | WSDL/SOAP, only `ProcessShipmentRequest`. |
| **karrio-fedex / purplship.fedex** | pypi.org | Py | inherits karrio (~2k+) | Multi-carrier abstraction; **karrio supports FedEx REST**. Best Python REST competitor. |
| **shippo / easypost / shipengine SDKs** | multi-lang | various | thousands | Re-sell FedEx via own auth/account; different value prop. |
| **markswendsen-code/mcp-fedex** | github.com/markswendsen-code/mcp-fedex | TS | 0 | Only FedEx MCP server. **Does NOT call the API** — Playwright + stealth scraping fedex.com (Akamai bypass). 4 tools: track_package, get_rates, find_locations, schedule_pickup. Brittle. |
| **Pipedream FedEx MCP** | mcp.pipedream.com/app/fedex | hosted | n/a | Hosted SaaS wrapper. |

**Key observations:** No mature REST/OAuth2-native FedEx-specific Go (or even Python) wrapper exists. Every direct wrapper is a SOAP relic. karrio is the only credible REST option but is a multi-carrier abstraction. **No FedEx CLI exists.** **No FedEx Claude Code plugin/skill exists.** Unusually open competitive field.

## 5. Reachability / Breakage Signals

- **API reliably reachable from CI** with one caveat: **per-IP throttle is aggressive**. >1 hit/sec sustained × 2 min from one IP triggers a **10-min 403 ban** (separate from project-level limits). Shared CI NAT IPs (GitHub Actions) can trip this.
- **Token regeneration bug pattern** seen in n8n/Auth0 forums: systems don't refresh on 401, silently use expired tokens. The CLI must refresh proactively (~5 min before `expires_in`) or on 401.
- **SOAP retirement:** **June 1, 2026** is the hard cutoff for FedEx Web Services (Compatible Providers had to be done by March 31, 2026). Anyone on SOAP today is in active crisis — good demand timing.
- **Rate limits to design around:**
  - Per-project: **1,400 tx / 10s** rolling window → 429.
  - Per-capability daily: e.g., **Track default 100K/day per org** across all 6 Track endpoints → 429.
  - Per-IP: 1 req/s sustained × 2 min → **10-min 403 ban**.
- **403 vs 429:** 403 = auth failure OR IP penalty; 429 = quota/burst. CLI errors should distinguish.

## 6. Common Workflows / Power-User Pain

**Top workflows to surface as one-shot commands:**

1. **Rate-shop across services.** API returns all services in one call; value is parsing noisy response → tidy comparison sorted by cost or transit. `fedex rate shop --from ZIP --to ZIP --weight 5lb`.
2. **Bulk address validation.** AV takes 100/call; CSV in → annotated CSV out is widely re-implemented badly.
3. **Batch label creation from CSV.** Loop Ship API at rate-limit-capped concurrency; emit labels (PDF/ZPL) per row; track failures.
4. **Track batch + emit milestone deltas.** Track returns 30/call; "what changed since last poll" is missing from API — local sync ledger provides it.
5. **Pickup with availability check.** Pickup creation needs availability windows first; users routinely get rejected for cutoff-time mistakes.
6. **End-of-day manifest / close-out.** Synthesize from local store.

**Pain points repeatedly cited:**

- Verbose nested JSON (200-field rate responses where users want 5).
- No native diff for tracking — every team rebuilds last-event-cursor.
- Validate vs Create Shipment confusion — users skip validation, burn labels.
- **Decimal dimensions truncated** — entering 9.4 inches uses 9 in some calcs (documented gotcha).
- Hardcoded service types break — service catalog changes; should fetch+cache per O-D.
- Label format wrangling — PDF vs PNG vs ZPL; thermal printers want ZPL but samples emit PDF.
- ETD multi-step dance — upload doc → get docId → reference in shipment; few wrappers automate.
- List vs account-specific rates confusion — users see list rates without realizing.

**"Hard with API but should be one command":**
- Rate-shop with cost ranking + transit filter
- Bulk AV from CSV with diff output
- Tracking ledger with "since last poll" event extraction
- ETD upload + ship-with-attached-doc as single command
- Multi-account rate compare for a given lane

## 7. Data Model for Offline Store

| Entity | Cursor | Sync strategy | Notes |
|---|---|---|---|
| `shipments` | `shipDatestamp` (client) | Append-only on label create. **No "list shipments" endpoint** — client-populated only. | Crucial. Without it, `sql`/`search` are empty. |
| `tracking_events` | `dateAndTime` per event | Poll Track per number; dedupe by (trackingNumber, eventType, dateAndTime). | Webhook is parallel ingest. |
| `rate_quotes` | `quotedAt` (client) | Insert on every rate call; never expires. | Power-user gold for invoice reconciliation. |
| `pickups` | `confirmationNumber`+`dispatchDate` | Insert on create; cancel marks status. | No list endpoint. |
| `address_validations` | `validatedAt` | Hash-key cache to avoid re-billing. | Pure cache. |
| `locations` | none | LRU cache by (postal, radius). | Pure lookup. |
| `webhook_events` | `eventTimestamp` | Append from webhook deliveries; HMAC-verified. | If webhook daemon ships. |

**Pure lookups (cache-with-TTL, no real cursor):** `address_validations`, `locations`, `postal_codes`, `service_availability`, `country_service`.

**Real sync cursors:** `tracking_events` (per-trackingNumber timeline), `webhook_events` (delivery timestamp).

**Critical:** `shipments` and `rate_quotes` are write-only from CLI's side — FedEx has no "list my shipments since X" endpoint, so the local store IS the source of truth for history.

## 8. Differentiators for the CLI

Competitive bar is essentially zero — no FedEx-native REST CLI exists. Differentiation is "shipping ops daily-driver":

1. **REST/OAuth2-native + Go static binary.** Every existing wrapper is SOAP. June 2026 makes this mandatory; nothing else fills the gap.
2. **Offline rate-shop ledger.** Every quote ever requested + the service selected + (later) actual invoiced cost. FedEx provides neither historical quotes nor a quote→invoice join. Big value.
3. **Multi-account support.** Companies with 2-5 FedEx accounts (cost-center, region, returns) want one CLI managing all + rate-shop across accounts. `~/.config/fedex/accounts.toml`, `--account` flag, default from `FEDEX_ACCOUNT` env.
4. **Bulk operations from CSV/JSON.** `fedex ship batch shipments.csv`, `fedex track batch numbers.txt`, `fedex address validate addresses.csv`. Concurrency capped to rate-limit budget.
5. **SQLite + FTS shipment archive.** `fedex sql "SELECT ... FROM shipments WHERE recipient_state='CA' AND ship_date > date('now','-7 days')"`. `fedex search "broken pallet"` over notes/refs.
6. **Webhook + polling daemon.** `fedex track watch <number>` polls every N min (rate-aware), emits stdout/JSON on event change. `fedex webhook serve` for HMAC-verified inbound.
7. **Cost reconciliation against invoices.** Stretch: ingest FedEx invoice CSV/PDF, join to local rate_quotes + shipments, flag discrepancies.
8. **Address-validation cache.** Hash-key cache prevents re-charging for same lookup; valuable in high-volume returns.
9. **ETD-aware ship.** `fedex ship create --etd-doc commercial-invoice.pdf` does upload+reference automatically.

## 9. Risk Factors Specific to FedEx

- **SOAP-only legacy:** A few specialized SOAP services (FedEx Ship Manager Server / "Quick Ship API" 20.09 family, certain Freight LTL rating) still exist as on-prem software with their own SOAP surface. **For the user's named scope (Ship/Rate/Track/AV), all are REST-native** — no SOAP fallback needed for v1.
- **Production approval gate (BAG / Shipper Validation):** Sandbox keys get everything *except* live label printing. Prod requires submitting test labels for FedEx BAG approval, **per project**. CLI's `doctor` should call this out.
- **Webhook subscription approval:** Advanced Integrated Visibility / Tracking Number Subscription requires a separate FedEx-provisioned "webhook project". **US accounts only**; FedEx Freight requires 9-digit account #.
- **Geographic restrictions:**
  - Tracking push notifications: US accounts only.
  - Picture POD: US accounts only.
  - Address Validation: full in US/CA; partial international.
  - Some rate features (alcohol, certain dangerous goods) region-gated.
- **Rate-limiting (published):** 1,400 tx / 10s per project; 100K/day per org for Track (default); ~1 req/s × 2 min per IP → 10-min 403. The 100K/day Track quota is the most likely production-scale issue.
- **CSP / Parent-Child:** If user is a Compatible Solution Provider, `csp_credentials` grant + `child_key`/`child_secret` per managed account. Treat as opt-in (`fedex auth login --csp`).
- **Migration deadline:** Compatible Provider cutoff (March 31, 2026) has passed; **end-customer SOAP cutoff is June 1, 2026** — ~1 month from today. Strong demand timing.
- **Ground Economy (formerly SmartPost):** Different label/manifest requirements; routed through Ship API as a service type but with extra fields (`hubId`, etc.). Worth dedicated test coverage.
- **Decimal-dimension truncation** is a documented data-loss bug; CLI should round explicitly and warn.
