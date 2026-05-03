# FedEx CLI Brief

## API Identity
- **Domain:** Shipping logistics — labels, rates, tracking, address validation, pickup scheduling, location lookup
- **Users:** Shipping operations teams at e-commerce / 3PL / B2B companies; integrators wiring FedEx into ERPs/WMS; engineers automating fulfillment
- **Data profile:** Per-call API surface (no inherent "list shipments" endpoint exists — FedEx makes you maintain your own ledger). Tracking events are the only naturally-paginated stream. Most operations are imperative (create/quote/track) rather than queryable.

## Reachability Risk
- **Low** for sandbox (`https://apis-sandbox.fedex.com`) and production (`https://apis.fedex.com`); both serve the modern OAuth2 REST endpoints.
- **One IP-throttle trap**: ~1 request/sec sustained for 2 minutes triggers a 10-minute 403 ban. CLI must use `cliutil.AdaptiveLimiter` for any bulk operation. Per-project quota is 1,400 tx / 10s; per-org Track is 100k/day.
- **June 1, 2026 SOAP retirement** (~30 days from now): every existing community wrapper is SOAP-based and goes dark. This is a strong demand signal — a REST-native Go CLI lands at the moment integrators urgently need one.

## Top Workflows
1. **Rate-shop before tendering**: quote multiple service types (Priority, Standard, Ground, Home Delivery, Ground Economy) and pick lowest-cost or fastest. Today's pain: 200+ field rate JSON, no native ranking.
2. **Bulk label creation from CSV**: orders CSV → validated addresses → batch labels → printable PDFs/ZPL. Today's pain: serial calls, no rate-limit handling, label format wrangling, no resume after partial failure.
3. **Track + diff**: poll a list of tracking numbers and surface only what changed since last poll. Today's pain: full-event payload returned every time; consumer must dedupe.
4. **End-of-day pickup**: confirm pickup availability for a postal code, schedule pickup with ready/close times. Today's pain: multi-step (availability → schedule → confirmation), no idempotency.
5. **Address validation as a cache layer**: avoid re-billing for repeat lookups; return suggested corrections on bad input. Today's pain: each call costs money and same address may be validated 100×/day.

## Table Stakes (every wrapper has these; we must too)
- Rate quote
- Create / validate / cancel shipment
- Generate label (PDF, PNG, ZPL/EPL)
- Track by tracking number
- Address validation
- Pickup scheduling
- Service availability check
- Location finder
- Postal code lookup

## Data Layer
- **Primary entities (write-only history, local store IS source of truth):** `shipments`, `rate_quotes`, `pickups`, `labels` (blob refs)
- **Sync cursor entities:** `tracking_events` (per tracking_number, append-only), `webhook_events` (signed callbacks)
- **TTL cache entities:** `address_validations` (avoid re-billing), `locations`, `postal_codes`, `service_availability`
- **FTS indexes:** `shipments_fts` (recipient name, address, reference, tracking), `tracking_fts` (tracking number, status, location)

## Codebase Intelligence
- No vendor OpenAPI spec exists. Authoritative sources are developer.fedex.com REST docs and the official Postman workspace at postman.com/devrel/workspace/fedex.
- Auth: OAuth2 client_credentials, body-encoded (NOT HTTP Basic). 1-hour bearer TTL. Required headers per call: `Authorization: Bearer <token>`, `Content-Type: application/json`, plus `X-locale` and `X-customer-transaction-id` for production.
- Practically every existing wrapper (python-fedex, happyreturns/fedex, asrx/go-fedex-api-wrapper) is **SOAP-only** and going dark June 1, 2026.
- The lone REST-aware ecosystem entry is `karrio/karrio` (multi-carrier abstraction in Python); it is not FedEx-specific.

## User Vision
The user wants the CLI focused on the **shipping use case** for companies — creating shipments, managing them, getting rates. They provided a sandbox dev key (Client ID format). Implication: the CLI should foreground the Ship/Rate/Track loop and treat secondary APIs (Postal Code, Country Service) as supporting cast rather than peers.

## Product Thesis
- **Name:** `fedex-pp-cli` (slug: `fedex`)
- **Display name:** FedEx
- **Why it should exist:** Every existing FedEx automation tool is dying on June 1, 2026 (SOAP cutoff). No REST-native Go CLI exists. Companies have ~30 days to migrate. We give them a single static binary with the full FedEx REST surface plus an offline ledger nobody else has.
- **Differentiators:** (a) static Go binary with REST/OAuth2 wired correctly, (b) offline rate-shop ledger that compounds across calls, (c) bulk CSV operations with adaptive rate limiting, (d) SQLite + FTS shipment archive, (e) ETD-aware single-command shipping, (f) multi-account support, (g) tracking diff (only-what-changed), (h) address-validation cache that prevents re-billing.

## Build Priorities
1. **Foundation:** OAuth2 client_credentials login (custom; generator OAuth template assumes auth_code), token cache with proactive refresh, adaptive rate limiter wired into client.
2. **Absorbed (table stakes):** Rate, Ship (create/validate/cancel/retrieve), Track, Address Validation, Pickup, Locator, Postal Code, Service Availability, Country Service.
3. **Transcendence:** rate-shop, bulk-label, track-diff, address-cache, multi-account, ETD-aware ship, end-of-day manifest, shipment archive (SQLite + FTS).
4. **Polish:** Doctor command surfaces BAG (Bar Code Analysis Group) approval status, sandbox vs prod warnings, label format guidance.
