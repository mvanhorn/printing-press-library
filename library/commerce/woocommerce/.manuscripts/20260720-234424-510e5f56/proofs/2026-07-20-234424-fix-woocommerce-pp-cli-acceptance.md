# WooCommerce CLI — Phase 5 Acceptance (live, read-only)

**Target:** `https://www.heatwave.com.mx/wp-json` — a real production store (537 orders, 40 products).
**Level:** read-only by operator instruction. The full `dogfood --live` matrix was NOT run: of 252
commands, 78 mutate, including `orders send-email` and `orders send-order-details`, which would have
sent real messages to real customers. No machine acceptance marker was produced as a result.

**Credentials:** read/write key pair already stored by the operator at
`~/.local/share/woocommerce-pp-cli/credentials.toml` (mode 600). Only read paths were exercised.

## Results

- **Read surface: 28/28 PASS.** Every admin read family (orders, products, customers, coupons,
  categories, tags, reviews, refunds, all five report families, system status, payment gateways,
  shipping zones/methods, taxes, tax classes, webhooks, settings, store data, product attributes)
  and every public Store API family returned valid JSON.
- **Auth boundary correct.** `curl` without credentials on `wc/v3/orders` → 401; the CLI with stored
  credentials → 537 orders. Public `wc/store/v1` works with no credentials at all.
- **stdout purity verified.** Cache-freshness `sync_*` progress events go to stderr; stdout is a
  single valid JSON document (`meta` + `results`). Agent pipelines are safe.
- **All 8 novel commands produced correct, non-empty output on real data.**

## Defect found and fixed during this phase — silent under-fetch

`sync` **never advanced past page one.** Root cause: the spec declared `pagination.type: offset` for
every list endpoint, so the generated runtime computed an *offset* (`0 + limit`) and sent it as the
value of a parameter that WooCommerce defines as a 1-based *page number*. It requested `?page=10`
instead of `?page=2`; on a 40-product catalog page 10 is empty, so enumeration stopped after one page.

Observed before the fix: 10 of 40 products, and exactly 100 of 537 orders at `per_page=100` — the
arithmetic matched the hypothesis precisely. Nothing warned the user the mirror was partial, so every
analytics command silently computed on a fraction of the store.

Fixed in two places: `cursorType` corrected from `offset` to `page` across all 24 paginated resources
in the generated runtime, and `pagination.type` corrected in the source spec so a reprint is right.
After the fix: **orders 537/537, products 40/40, coupons 1/1 — complete.**

This is the highest-value defect of the whole run, and it was only findable against a store with more
than one page of data.

## Verified analytics output (complete mirror)

| Command | Result on real data |
|---|---|
| `orders triage` | 5 stuck processing, 33 aging on-hold, 0 recent failures across 537 orders |
| `customers ltv` | 306 distinct customers, 18 repeat (5.9%), avg LTV $5,687.07 MXN, cohorts from 2022-11 to 2026-07 |
| `revenue explain` | decomposition identity held exactly (order_count + basket_size = revenue delta) |
| `refund-rate` | 53 orders in window, 17 products, no refunds recorded — reported honestly with a note |
| `catalog audit` | 28 findings over 40 products: 16 missing SKU, 9 published-but-out-of-stock, 3 zero price |
| `stock velocity` | 1 product with demand in 90d; null cover reported honestly where stock is unknown |
| `catalog watch` | 35 products snapshotted from the public Store API, MXN, prices normalized correctly |
| `catalog diff` | `membership_comparable: true`, no changes between same-day snapshots — correct |

## Business findings surfaced for the operator

- **Card gateway "Pago con tarjeta de crédito o débito": 44.2% failure rate over 124 orders, up from
  25% in the earlier half — flagged `worsening`.** This is exactly the signal `orders triage` exists
  to surface, and it is not visible anywhere in the WooCommerce admin.
- 33 orders aging in `on-hold`.
- 16 of 40 products have no SKU; 3 are published with a price of zero.

## Gate

**PASS (read-only scope).** No machine `phase5-acceptance.json` was written because the runner's full
matrix would mutate a production store; promotion therefore requires an explicit operator decision
rather than an automated marker.
