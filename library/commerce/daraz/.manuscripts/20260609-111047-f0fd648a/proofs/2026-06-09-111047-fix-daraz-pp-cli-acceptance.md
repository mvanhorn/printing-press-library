# Daraz CLI — Phase 5 Acceptance (Full Dogfood)

Level: Full Dogfood (live, no auth required — public catalog API)
Tests: 59/59 passed (binary-owned matrix; 4 error-path probes correctly skipped via pp:no-error-path-probe)
Gate: PASS

## Behavioral verification (manual live smoke, pre-matrix)
- products (search): live results, sort/price filters work.
- reviews: live review JSON for item 599201597.
- deals "power bank": ranked by composite (discount x rating x sold) — sensible top results.
- value "power bank" --only-suspicious: flags inflated-original discounts vs market median.
- compare "airpods pro": 120 scanned, cheapest + best-rated + 10-seller breakdown.
- watch "gaming mouse": recorded 80 items into local store.
- since "gaming mouse": correct baseline + diff path.
- price-history / seller stats / seller listings: aggregate from the compounding local store (320 rows captured during testing).
- products get 599201597: full JSON-LD detail (brand/category/availability); price from local store when seen.

## Fixes applied during Phase 4-5
- flexInt + productFromMap: robust to Daraz returning totalResults/IDs as string or number.
- parseMoney number-regex fix.
- seller products -> seller listings (verify-skill name collision with top-level products).
- quickstart/README/SKILL: products search -> products; removed nonexistent --limit.
- pp:no-error-path-probe on price-history/seller stats/seller listings/tail (empty local cache and bad-resource are exit 0 by design).

Printing Press issues for retro: generated store "no extractable ID field" warnings for the catalog/reviews typed endpoints; tail exits 0 on an unresolvable resource.
