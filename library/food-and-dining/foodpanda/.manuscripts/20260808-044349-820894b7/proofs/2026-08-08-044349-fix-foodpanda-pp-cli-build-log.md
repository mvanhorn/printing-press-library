# foodpanda CLI — Phase 3 Build Log

Manifest transcendence rows: 9 planned, 9 built. Phase 3 gate satisfied.

| # | Command | Status | Behavioral evidence |
|---|---------|--------|---------------------|
| 1 | `home` | built | Fails cleanly with authErr + `auth login --chrome` hint when no session |
| 2 | `dish` | built | `biryani` -> 60 matches, cheapest real biryani PKR 126; nonsense -> 0 |
| 3 | `menu-diff` | built | First run records baseline and fabricates no drift; second run reports no change |
| 4 | `posture` | built | 10 ad-buyers / 192 scanned; commission disclaimer present in output |
| 5 | `coverage` | built | Lahore vs Karachi sets verified disjoint |
| 6 | `fees` | built | Discloses "only 2 of 192 vendors delivery-fee priced" |
| 7 | `digest` | built | Pizza Bake split: food 2.83 vs rider 4.64 (site shows one blended 4.6) |
| 8 | `market-compare` | built | pk 119 / sg 66 / my 160 with avg rating + ad share |
| 9 | `find` | built | `sushi` -> 1 strong of 74; `zzzqqqnonsense` -> 0 strong of 51 |

## Bugs found and fixed during build (all fix-before-ship)

1. **Every sweep command was dead.** `minimum_delivery_time` arrives as JSON float
   `25.0`; decoding into `int` failed with "cannot unmarshal number 25.0". All numeric
   vendor fields are now float64 with casts at use sites. Caught only by behavioral
   testing -- the commands compiled and had valid help.
2. **`dish` searched the wrong vendors.** It seeded from the plain listing, which is not
   relevance-ordered, so `dish biryani` scanned 817 products from upscale cafes and
   returned 0. Now seeds from the search endpoint and ranks candidates by name match.
3. **`dish` ranked side dishes above the dish.** A raita in a "Biryani" category matched
   on category and, sorted by price, outranked every real biryani. Added `matched_on`
   provenance, name-hits-first ranking, and a strict `--name-only` mode.
4. **Delivery fee silently reported as 0.** foodpanda prices delivery in a separate
   Dynamic Pricing Service (`delivery_fee_source: "dps"`) that stays silent without a
   session: only 2 of 192 Lahore vendors returned a fee. Printing a bare 0 reads as free
   delivery. `home` and `fees` now report `delivery_fee_priced_count` / total and warn.
5. **Novel command name collision.** `search` is a framework command; the novel live
   search was renamed `find` with a Long-field redirect between the two.
6. **Wrong spec examples.** `menu get pk2v` / `reviews list pk2v` were the un-promoted
   forms; corrected to `menu pk2v` / `reviews pk2v`.
7. **Missing perseus headers.** `pk.fd-api.com` rejects requests without
   `perseus-client-id` / `perseus-session-id`; added to spec `required_headers`.

8. **Delivery fees were silently zeroed by our own headers.** The root cause of #4 was
   the opposite of the first diagnosis. Verified matrix (2026-08-08):

   | Endpoint | Without perseus | With perseus |
   |---|---|---|
   | disco vendors / search | 200, **real fees** | 200, **fees all 0** |
   | fd-api vendor detail | 400 "perseus headers are absent" | 200 |
   | reviews-api, customers/addresses | 200 | 200 |

   The spec sent perseus globally, so every listing fee came back 0. perseus is now
   attached ONLY to the menu endpoint (spec endpoint-level `required_headers`, plus
   `fpPerseusHeaders()` for the hand-written menu fetch).

9. **Listing fee is a floor, not a price.** With perseus removed the listing returns a
   flat 99 for every Lahore vendor. The true per-vendor fee lives on the vendor-detail
   endpoint and requires perseus + Authorization (n3qk: 0 unauthenticated -> 229 with a
   session). `home` now resolves real fees per returned vendor concurrently and tags
   each row `fee_source` = `vendor-detail` | `listing-floor` | `unpriced` | `lookup-failed`.
   Verified live against the operator's saved address: Savour Foods 49, KFC 70, others
   229 -- 15/15 resolved, fees genuinely varying.

## Known limitation (documented, not a bug)

Per-vendor delivery fees require an authenticated session. Without one, `home` falls back
to foodpanda's flat listing floor and labels every row `fee_source: listing-floor`, plus a
note naming `auth login --chrome`. It never presents an unpriced vendor as free delivery.

## Deferred / out of scope

- Cart, checkout, ordering (user scoped to reads).
- Merchant commission rates: not present in any consumer surface (1,881 key paths
  searched; GraphQL safelisted). `posture` ships the observable ad/placement proxies.
- pandamart grocery catalog: separate q-commerce surface, `menus: 0` on darkstores.
