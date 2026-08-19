# foodpanda Browser-Sniff / Discovery Report

Run: `20260808-044349-820894b7` · Target: `https://www.foodpanda.pk/`
Backend: `browser-use` 0.13.6 (browser-harness 0.1.6) attached to the user's running Chrome,
plus direct `curl` replay verification. Gate decision: `pre-approved` (Phase 0 website choice).

## Reachability

| Probe | Result |
|---|---|
| bare `curl` (no headers) | **403** PerimeterX `px-captcha`, deterministic (3/3, identical 4599 B) |
| `curl` + full browser header set | **200**, 995 KB real SSR HTML, zero PX markers |
| `probe-reachability` | `mode: standard_http`, confidence 0.95 (stdlib 200, surf-chrome 200) |
| browser-use rapid programmatic navigation | PX interstitial → cleared after a human solved press-and-hold |

**Runtime verdict: `standard_http`.** No clearance cookie, no Surf requirement, no resident
browser. The single mandatory mitigation is a fixed browser-like header set on every request.

## Public surface (no credentials)

Headers: `X-FP-API-KEY: volo`, `x-disco-client-id: web`, `perseus-client-id`,
`perseus-session-id`, browser UA. **perseus IDs are client-generated, not credentials** —
synthesized values (`<unix_ms>.<18 digits>.<8 alnum>`) returned 200.

| Endpoint | Verified |
|---|---|
| `GET disco.deliveryhero.io/listing/api/v1/pandora/vendors` | 200; 80+ fields/vendor; `aggregations` facets (22 cuisines) |
| `GET disco.deliveryhero.io/listing/api/v1/pandora/search?query=` | 200; **discriminates** (biryani 63 / pizza 19 / sushi 5) |
| `GET {cc}.fd-api.com/api/v5/vendors/{code}?include=menus` | 200; menus→categories→products→variations w/ prices, deals |
| `GET reviews-api-{cc}.fd-api.com/reviews/vendor/{code}` | 200; per-topic ratings, vendor replies, `pageKey` pagination |
| `GET /city/{city}/area/{area}` (SSR) | 200; carries lat/lng + ~48 vendor links → native geocoding |
| `GET /restaurant/{code}/{slug}` (JSON-LD) | 200; aggregateRating, hours, `areaServed.geoRadius`, `reviews[]`×30 |

Multi-market verified on the same endpoints: **pk, bd (235), sg (789), my (2854), hk (1438)**.
`th` returned a transient 530. Vendor codes are market-scoped (`pk2v` 404s on `sg.fd-api.com`;
SG code `fqow` returns 200 there).

## Authenticated surface

**Auth mechanism (verified):** cookie `token` is a JWT (657 chars, `eyJ…`, 3 segments).

```
Authorization: Bearer {token}   +   X-FP-API-KEY: volo
```

- With composed header → `GET /api/v5/customers/addresses` **200**, 34,748 B of real data.
- Without it → **400** `ApiOAuthFailedException`.
- Cookies present when logged in: `token`, `refresh_token`, `device_token`, `user-address-model`.

Spec shape:

```yaml
auth:
  type: composed
  header: Authorization
  format: "Bearer {token}"
  cookie_domain: www.foodpanda.pk
  cookies: [token]
```

| Endpoint | Status |
|---|---|
| `GET /api/v5/customers/addresses` | **verified 200** (saved addresses w/ coordinates) |
| `GET /api/v5/orders/reorder` | observed in Performance API, not individually re-verified |
| `GET /api/v5/rewards/stamp-cards/suggestions` | observed, not re-verified |
| `GET /api/v5/subscription/plans-v2` | observed, not re-verified |
| `GET /api/v5/timepicker-service/timeslotsV2` | observed, not re-verified |
| `POST /api/v5/cart/calculate`, `/payment/intent` | observed; **out of scope** (no ordering) |
| `POST /graphql` | **safelisted** — `403 QUERY_NOT_IN_SAFELIST`, introspection disabled |

**No web order-history route exists.** `/orders`, `/account/orders` both return 404; foodpanda
keeps order history app-side. `/account` and `/favorites` render. Reorder data is reachable only
through `/api/v5/orders/reorder`.

## Commission investigation (explicit user request)

Exhaustive recursive key-path search over 6 payloads — **1,881 distinct key paths**
(vendor 439, disco 187, sgv 596, darkv 267, filt 185, s2 207) — matching
`commis|margin|payout|revenue|billing|contract|take_rate|gmv|settle|invoice|rate_card`.

**Only hits: `ncr_pricing_model` (value `"cpc"`) and `ncr_token`.**
`nonCommissionRevenueInfo` appears in the page's embedded Apollo cache with value `null`.

**NCR = Non-Commission Revenue = foodpanda's advertising product** (CPC sponsored placement).
It is by definition the revenue stream *other than* commission. Merchant commission rate is
**not present in any consumer surface**, and GraphQL safelisting blocks fishing for hidden
fields. Commission is a B2B contract term living in partner systems
(`partner.foodpanda.pk`, Partner API) — reachable only via an onboarded vendor account for a
restaurant the operator represents.

**Exposed commercial-posture proxies** (built into the CLI instead):
`ncr_pricing_model`, `ncr_token` presence, `is_promoted`, `is_premium`, `premium_position`,
`vendor_points` (10,826–15,394 observed), `is_preferred_partner`, `delivery_provider`,
`has_delivery_provider`, `service_fee_percentage_amount`, `vat_percentage_amount`,
`loyalty_percentage_amount`, `tags[].code` (`FEAT`/`DEAL`), `discounts`, `deals`.

## Replayability verdict

**PASS.** Every shipped surface replays through plain HTTP with a browser header set. The one
browser-dependent element is the `token` cookie for authenticated reads, which is imported once
via `auth login --chrome` and then replayed as a header. No resident browser transport.

## Landmines

- `q=` on the vendors endpoint is **silently ignored** (biryani / pizza / `zzzqqqnonsense` all
  returned the same 102). Text search MUST use `/search?query=`.
- `/search` rejects `q`/`search`/`keyword`/`term` with 400; only `query` is accepted.
- Search is fuzzy and **never truly empty** (`zzzqqqnonsense` → 18 results).
- JSON-LD reviews use the non-standard key `reviews` (plural), not `review`.
- `vertical=darkstores` returns a store but `menus: 0` — pandamart catalog is a separate
  q-commerce surface, not reachable via `/api/v5/vendors`.
- A 404 on `pk.fd-api.com` returns no CORS headers, surfacing in-browser as
  `TypeError: Failed to fetch`. Do not read that as "blocked" — it means the path doesn't exist.
