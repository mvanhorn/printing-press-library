# Durianpay CLI Brief

## API Identity
- Domain: Indonesian payment gateway (durianpay.id). Accept payments (VA, QRIS, e-wallet, cards, BNPL, online banking, retail store) and send pay-outs/disbursements (130+ banks + top-5 e-wallets), plus refunds, settlements, sub-accounts.
- Users: Indonesian merchants' backend/integration engineers, Durianpay's own solutions/CSM teams, finance-ops people doing reconciliation, and AI agents automating payment ops.
- Data profile: orders, payments, refunds, disbursements, virtual accounts, sub-accounts — all list/get-able, ID-keyed (`ord_`, `pay_`, `dis_`, `va_`, `ref_`), time-series. Strong fit for SQLite sync + FTS. Plus rich static reference data: payment-method limits, SNAP bank/e-wallet codes, SNAP response codes (48 rows), webhook event catalog, sandbox simulation rules.

## The Dual-API Reality (USER VISION — central design constraint)
Durianpay exposes **two API generations side by side**:

| | Legacy | SNAP (Bank Indonesia standard) |
|---|---|---|
| Base | `https://api.durianpay.id/v1` | `https://api.durianpay.id/v1.0` (sandbox: `api-sandbox.durianpay.id`) |
| Auth | HTTP Basic — API key as username, blank password | B2B bearer token (TTL 900s) minted by signing `clientKey\|timestamp` with **RSA-SHA256** (merchant private key), then per-request **HMAC-SHA512** signature over `METHOD:path:token:lowerhex(sha256(minify(body))):timestamp` with client secret |
| Headers | `Authorization: Basic ...` | `X-TIMESTAMP, X-SIGNATURE, X-PARTNER-ID, X-EXTERNAL-ID (unique/day), CHANNEL-ID, Authorization: Bearer` |
| Coverage | Orders, all 7 charge methods, refunds, VA, customers, sub-accounts, legacy disbursements (submit/approve/fetch) | VA (create/update/inquiry), QRIS (generate/query/cancel/refund), e-wallet payments (create/status/cancel/refund), disbursements (bank transfer, e-wallet transfer, account inquiries, balance, status) |
| Regulation | Unregulated methods only option | BI-mandated for regulated methods |

**Company stance (from Ardi): route to SNAP wherever the payment method supports it; legacy only where there is no SNAP option.** Overlapping surfaces: VA, QRIS, e-wallet (payments); all disbursements. Legacy-only: cards, BNPL, online banking, retail store, orders/refunds/sub-accounts.

Endpoint inventory: 26 legacy ops (merged OpenAPI at `research/durianpay-legacy-openapi.json`, incl. 3 hand-added disbursement endpoints from guides) + 20 SNAP ops (inventory at `research/snap-endpoint-inventory.json`, 1 duplicate access-token page deduped → 19 unique).

## Reachability Risk
- None. `GET https://api.durianpay.id/v1/orders` → 401 with clean JSON (`DPAY_UNAUTHORIZED_ACCESS`) — expected for missing auth; no bot protection, no tier hints. Docs site (ReadMe.io) rate-limits scrapers but exposes full markdown mirrors + per-endpoint OpenAPI via llms.txt.
- Probe-safe endpoint used: GET /v1/orders (401 expected, PASS per decision matrix).

## Top Workflows
1. **Disburse money**: account inquiry → submit transfer (SNAP `transfer-interbank` / `emoney/topup`) → poll status → verify webhook signature. Legacy path: submit batch → approve → fetch.
2. **Accept a payment**: create order → charge (method-specific) → poll/check status → handle webhook → refund if needed. SNAP path for VA/QRIS/e-wallet.
3. **Payment ops/reconciliation**: list payments/settlements for a period, match against orders, export; check balances (master + sub-accounts).
4. **Integration testing**: sandbox simulation (even/odd account numbers, magic amounts), webhook verification, signature debugging (the #1 integration pain).
5. **SNAP onboarding**: generate RSA keypair, configure dashboard, mint B2B token, debug X-SIGNATURE mismatches (403s).

## Table Stakes (best competitor features)
- No Durianpay CLI/MCP exists; no Xendit/Midtrans CLI either. The bar is set by **Stripe CLI**: resource CRUD, `listen` (webhook forward), `trigger` (test events), `logs tail`, fixtures, profile switching.
- Best existing surface map: **abmid/dpay-sdk-go** (stale, 2023, legacy-only): orders, payments (charge/verify/cancel/MDR), promos, disbursements (submit/approve/validate/banks/balance), settlements, refunds, e-wallet accounts, VA + simulate, invoices. **ayatmaulana/durianpay-go-sdk** adds subscriptions, customers, static VA.
- **SNAP signature tooling**: only per-vendor demos (Midtrans SNAP-BI-Signature-Demo) and Durianpay's Postman pre-request scripts. No standalone tool. Biggest absorb-and-beat opportunity.

## Data Layer
- Primary entities: orders, payments, refunds, disbursements (+items), virtual_accounts, subaccounts.
- Sync cursor: created/updated timestamps via list endpoints (`from`/`to`/`skip`/`limit` params).
- FTS/search: customer email/name, payment ref IDs, disbursement recipient names.
- Static seed tables (offline, from docs): payment_method_limits, snap_response_codes (48), snap_service_codes, bank/e-wallet platform codes, webhook_events, sandbox_simulation_rules.

## Codebase Intelligence
- Source: abmid/dpay-sdk-go + ayatmaulana/durianpay-go-sdk (community Go SDKs, READMEs + package layout)
- Auth: legacy Basic (`Authorization: Basic base64(apiKey + ":")`); env convention: server key from dashboard, `dp_test_` / `dp_live_` prefixes (sandbox vs live keys differ)
- SNAP auth: client key, client secret, merchant RSA private key (PEM), partner ID, channel ID; token TTL 900s; public key upload to dashboard required
- Rate limiting: not publicly documented; webhook retries at 2/5/10/90/210 min
- Architecture: ID-prefixed resources (`dis_`, `pay_`, `ord_`); idempotency via `X-Idempotency-Key` on disbursement submit; disbursement completion signature = HMAC-SHA256(`dis_id|amount`, api_key)

## User Vision
- One CLI covering both API generations. SNAP preferred for any method it supports; legacy for the rest. Auth/header divergence handled transparently — the user configures both credential sets once, the CLI signs everything correctly per surface.
- Make the SNAP signature scheme a non-issue: built-in keygen, token lifecycle, signature debugging.

## Product Thesis
- Name: durianpay-pp-cli
- Why it should exist: There is **no CLI, no MCP server, and no maintained SDK** for Durianpay; nothing anywhere handles SNAP signing outside Postman pre-request scripts. A Go CLI that wraps both surfaces, auto-routes to SNAP per company policy, automates the dual-signature scheme (the #1 integration pain), and adds Stripe-CLI-grade dev tooling (webhook verify, simulation, signature debug) is instantly the best Durianpay tool in existence — and the only agent-native one.

## Build Priorities
1. Dual transport: generated legacy client (Basic) + hand-coded SNAP signing client (RSA-SHA256 token mint with 900s cache, HMAC-SHA512 per-request, X-EXTERNAL-ID generation) — `internal/snap/`.
2. SNAP command tree (19 endpoints) hand-wired over the signing client; legacy commands generated from the merged spec (27 ops).
3. Smart routing: `pay`/`payout`/`va`/`qris` umbrella commands that pick SNAP vs legacy per method, per company policy, with `--surface legacy|snap` override.
4. Local store: sync orders/payments/refunds/disbursements; seed static reference tables; FTS.
5. Dev tooling: signature debug (`snap sign --debug`), webhook verify (both schemes), sandbox simulation helpers, `explain <response-code>`.
