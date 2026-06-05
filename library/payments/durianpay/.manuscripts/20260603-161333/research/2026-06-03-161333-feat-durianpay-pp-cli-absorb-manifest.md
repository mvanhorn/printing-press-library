# Durianpay Absorb Manifest

Sources searched: github.com/durianpay (10 official repos), npm (dpay-node-sdk, web/RN SDKs), PyPI (none), abmid/dpay-sdk-go, ayatmaulana/durianpay-go-sdk, zerosdev/durianpay-php-client, Stripe CLI (canonical payment-gateway CLI), Durianpay Postman workspace (Merchant APIs v1 + SNAP collection with signing pre-request scripts), Midtrans/SNAP-BI-Signature-Demo. No Durianpay MCP server, CLI, or maintained SDK exists; nothing anywhere handles SNAP outside Postman scripts.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Create order | abmid/dpay-sdk-go orders, Postman v1 | (generated endpoint) orders POST /orders | --json/--dry-run/--select, agent-native |
| 2 | List/fetch orders | abmid/dpay-sdk-go | (generated endpoint) orders GET /orders, /orders/{id} | offline cache via sync |
| 3 | Charge payment (all 7 method types: VA/EWALLET/QRIS/CARD/BNPL/ONLINE_BANKING/RETAIL_STORE) | dpay SDKs + Postman v1 | (generated endpoint) payments POST /payments/charge | plus smart `pay` routing (transcendence #3) |
| 4 | Payment list/get/status/cancel/verify | abmid/dpay-sdk-go payment | (generated endpoint) payments GET/PUT/POST /payments/* | typed exit codes, offline list |
| 5 | Refund create/list/get/by-payment | abmid/dpay-sdk-go refund | (generated endpoint) refunds * | refund-audit join on top |
| 6 | Legacy VA create/list/get/patch-expiry | abmid/dpay-sdk-go virtualaccount | (generated endpoint) payments-va * | — |
| 7 | Customer update | ayatmaulana sdk customer | (generated endpoint) customers PATCH /customers/{id} | — |
| 8 | Sub-account register/list/update/fees/balance | Postman v1 | (generated endpoint) merchants-subaccount * | — |
| 9 | Disbursement submit (X-Idempotency-Key, force_disburse, skip_validation) / approve / fetch | abmid/dpay-sdk-go disbursement | (generated endpoint) disbursements * (paths hand-added to spec from guides) | idempotency-first examples |
| 10 | SNAP B2B token mint, 900s cache, auto-refresh | Durianpay SNAP Postman pre-request scripts | durianpay-pp-cli snap token | cached on disk, auto-refresh before expiry — beats Postman's per-session mint |
| 11 | SNAP balance inquiry | SNAP Postman | durianpay-pp-cli snap balance | one command, signing handled |
| 12 | SNAP bank account inquiry | SNAP Postman | durianpay-pp-cli snap inquiry-bank | — |
| 13 | SNAP e-wallet account inquiry | SNAP Postman | durianpay-pp-cli snap inquiry-ewallet | — |
| 14 | SNAP bank transfer (transfer-interbank) | SNAP Postman | durianpay-pp-cli snap transfer | auto X-EXTERNAL-ID, signed |
| 15 | SNAP transfer status inquiry | SNAP Postman | durianpay-pp-cli snap transfer-status | — |
| 16 | SNAP e-wallet transfer (emoney/topup) + status | SNAP Postman | durianpay-pp-cli snap ewallet-transfer / ewallet-transfer-status | — |
| 17 | SNAP eWallet payment create/status/cancel/refund | SNAP Postman | durianpay-pp-cli snap ewallet-pay / ewallet-status / ewallet-cancel / ewallet-refund | — |
| 18 | SNAP QRIS generate/query/cancel/refund | SNAP Postman | durianpay-pp-cli snap qris-generate / qris-query / qris-cancel / qris-refund | — |
| 19 | SNAP VA create/update/inquiry | SNAP Postman | durianpay-pp-cli snap va-create / va-update / va-inquiry | — |
| 20 | Sandbox/live profile switching (dp_test_/dp_live_ keys, per-mode SNAP creds) | ayatmaulana ClientConfig pattern | durianpay-pp-cli env sandbox/live (rewrites base_url, reports which credential env vars to flip) | one flag flips every credential |
| 21 | Webhook signature verification (SNAP RSA-2048 notify + legacy) | docs/handling-webhooks + Midtrans SNAP-BI demo | durianpay-pp-cli webhook verify | works offline from payload+headers |
| 22 | Local store sync/search/SQL over core entities | printing-press framework (beats every SDK: none have storage) | (behavior in durianpay-pp-cli sync) plus search/analytics/export over orders, payments, payments-va, refunds; CLI-initiated payouts recorded as local disbursement rows | FTS, SQL joins, offline |

Note: SNAP commands (#10-19) are hand-built over a hand-coded signing client (`internal/snap/`) because SNAP requires per-request HMAC-SHA512 signatures no generated client can produce. They are first-class shipping scope, not stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | SNAP signature debugger | snap sign --method POST --path /v1.0/transfer-interbank --body @req.json --debug | 9/10 | hand-code | Builds string-to-sign locally (METHOD:path:token:sha256(minify(body)):timestamp), computes both signatures, prints every intermediate — localizes 403s without an API call. #1 integration pain; only Postman scripts exist today | Use this command to construct and inspect SNAP request signatures and diagnose 403 signature mismatches. Do NOT use it to verify an inbound webhook signature; use 'webhook verify' instead. |
| 2 | SNAP keygen + token lifecycle | snap keygen / snap token --status | 8/10 | hand-code | Local RSA-2048 keypair generation with dashboard-ready public key output; cached-token status (minted-at/expires-in vs 900s TTL) | none |
| 3 | Smart payment/payout routing | pay --method qris ... / payout ... | 9/10 | hand-code | Method→surface policy table dispatches SNAP where supported, legacy otherwise (company stance), --surface override. Requires both transports in one binary | Use this command to charge/accept a payment and let the CLI pick the correct surface. Do NOT use it to send money to a recipient; use 'payout' for disbursements. For an explicit single-surface call use the generated 'payments charge' or 'snap ...' commands. |
| 4 | SNAP response-code explainer | explain 4012401 | 8/10 | hand-code | Seeded 48-row snap_response_codes table; meaning, HTTP status, service, likely fix — offline | none |
| 5 | Sandbox simulation helper | sandbox simulate --scenario success --method transfer | 7/10 | hand-code | Seeded sandbox_simulation_rules table emits the magic values (even/odd account numbers, magic amounts) for a target outcome | none |
| 6 | Reconcile orders vs payments | reconcile --since 7d | 8/10 | hand-code | Local SQLite join across synced orders+payments flags charged-but-unsettled, paid-with-no-order, amount mismatches | Use this command to match orders against payments and surface settlement gaps. Do NOT use it to reconcile refunds against payments; use 'refund-audit' instead. |
| 7 | Refund audit | refund-audit --since 30d | 7/10 | hand-code | Local join refunds×payments flags over-refunds, refunds on unsettled payments, double-refunds | Use this command to validate refunds against their source payments. Do NOT use it for order-vs-payment settlement gaps; use 'reconcile' instead. |
| 8 | Stuck-disbursement detector | stuck --older-than 90m | 7/10 | hand-code | Local query over synced disbursements bucketed against the 2/5/10/90/210-min webhook retry ladder | none |
| 9 | Disbursement completion-signature verifier | disbursements verify-completion --id dis_x --amount 50000.00 --signature <sig> | 8/10 | hand-code | Recomputes HMAC-SHA256(dis_id\|amount, api_key) locally — scheme documented only in integration guides | Use this command to verify a disbursement-completion callback signature. Do NOT use it for general legacy/SNAP webhook payload verification; use 'webhook verify' instead. |

Killed candidates (audit): route explainer (folds into pay --dry-run), balance sweep (thin fan-out), ID-prefix resolver (wrapper), limits lookup (overlaps explain/sandbox), external-id guard (sub-helper of sign), export (duplicates framework search/sql --csv). Full trail: research/2026-06-03-161333-novel-features-brainstorm.md
