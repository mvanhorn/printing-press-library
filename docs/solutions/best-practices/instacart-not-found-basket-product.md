# Instacart: notFoundBasketProduct on UpdateCartItemsMutation

_Finding date: 2026-04-19_

## Summary

`instacart-pp-cli add <retailer> <query>` occasionally fails with:

```
error: Instacart rejected the add: notFoundBasketProduct
```

Running `instacart-pp-cli search <query> --store <retailer>` immediately after returns valid candidates, and `instacart-pp-cli add --item-id <id> <retailer>` against a candidate from that search succeeds against the same cart. The add flow's single-attempt live resolver was losing the bet on which ranked candidate the active cart would accept.

The fix: walk the next ranked candidates on `notFoundBasketProduct` instead of giving up after the first miss. See the PR referenced at the bottom of this note.

## Why this happens

Three drivers, all observable in a single session:

### 1. Autosuggest nondeterminism

`library/commerce/instacart/internal/gql/search.go:63` mints a fresh `autosuggestionSessionId` UUID on every call. Instacart's server sometimes reorders suggestions between calls and the client-side `rerank` can only promote based on name-token overlap. The top pick on any given call is not stable. Some reorderings float a catalog-present-but-cart-unavailable item into position 0.

### 2. Shop / location drift

`ensureInventoryToken` in the same file caches `retailerInventorySessionToken` for six hours, pinned to whichever shop served the bootstrap. The user's active cart may have been created against a different Costco warehouse. In a real session observed 2026-04-19, cart `757109404` contained `items_18148-25502282` (Dino Nuggets) alongside five `items_1576-...` items. The cart has historically accepted items from both location prefixes, but not every `items_1576` catalog entry is stocked at every shop the cart routes through.

Decoded token format (see `parseTokenLocation`):

```
v1.<hash>.<customerId>-<zip>-<signed>-<sep>-<retailerId>-<retailerLocationId>-<sep>-<sep>
```

The zoneId is not encoded in the token; it comes from config. So the "same" retailer can mean different physical shops across cart operations.

### 3. Transient stock-out

The `Items` lookup returns `{id, name, ...}` without a reliable "addable right now at this shop" flag. A momentary inventory gap passes local checks and fails only at mutation time. Retrying a few candidates generally avoids the gap without any extra round trip.

## What does not work: location-prefix filtering

A tempting fix is to require retrieved candidates to match the location prefix of items already in the active cart. Do not do this. The observed cart mixed `1576` and `18148` prefixes successfully, so a prefix filter would drop valid adds. Instacart's backend is authoritative on what a cart accepts; the client should not second-guess it with a local invariant.

## What works: retry the next candidate

`tryAddCandidates` in `library/commerce/instacart/internal/cli/add_retry.go` walks the ranked candidate slice, retrying only on `notFoundBasketProduct`, up to `retryMaxAttempts` (currently 3). The first rejection is by far the most common, the second is rare, and exhaustion is rarer still. Three attempts caps round-trip latency and avoids looking like hammering if Instacart tightens basket validation later.

Other `ErrorType` values surface immediately. Widening the allowlist should happen only when a second confirmed symptom arrives.

## Correctness bonus: write-back now tracks reality

Before this fix, `writeBackPurchasedItem` fired for the original top pick, so a rejected-then-replaced candidate could still end up with a stronger history signal than the item the user actually bought. After the fix, write-back keys off the winning candidate, so the history-first resolver gets smarter about the user's actual purchase rather than what autosuggest happened to rank first.

## History-first fallback

A history hit that fails with `notFoundBasketProduct` now falls back to the live resolver and runs the full retry loop. The successful add reports `resolved_via: "history->live"` and the JSON envelope's `attempts` array shows both the rejected history item and any live candidates that were skipped before the winner.

## JSON envelope changes

Successful add:

```json
{
  "added": {...},
  "cart_id": "...",
  "resolved_via": "live",          // or "history", "history->live", "item-id"
  "result": {...},
  "retry_count": 1,                // number of rejected candidates
  "attempts": [                    // only when retry_count > 0
    {"item_id": "...", "name": "...", "error_type": "notFoundBasketProduct"}
  ]
}
```

Exhaustion (exit 5):

```json
{
  "error": "notFoundBasketProduct",
  "retailer": "costco",
  "query": "strawberries",
  "attempts": [...],
  "hint": "try: instacart search \"strawberries\" --store costco, then instacart add --item-id <id> costco. Or retry with --no-history."
}
```

## Similar services to watch for

Any cart API that tolerates multi-location stock behind a single cart id is a candidate for the same symptom:

- Amazon Fresh: Prime Pantry and Fresh have shown similar cross-zone basket rejections historically.
- Uber Eats: per-store basket validation can reject items the catalog lookup returns.
- DoorDash: same shape likely.

When adding a similar CLI, always assume the cart endpoint, not the product catalog, is the authority on what can be added. Retry ranked candidates; do not try to filter before the server answers.

## Related

- Plan: `docs/plans/2026-04-19-005-fix-instacart-add-notfoundbasketproduct-plan.md`
- Code: `library/commerce/instacart/internal/cli/add_retry.go`, `add.go` (`newAddCmd`)
- Tests: `library/commerce/instacart/internal/cli/add_retry_test.go`, `add_emit_test.go`
- Prior note: `docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md` (same CLI, different subsystem)
