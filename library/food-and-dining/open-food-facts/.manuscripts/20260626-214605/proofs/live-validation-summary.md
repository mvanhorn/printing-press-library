# Open Food Facts Live Validation Summary

The first print was validated against Open Food Facts' public, keyless JSON APIs. No API key, account cookie, product edit, image upload, login/session flow, or write-side route is used.

## Verified Without Secrets

- `open-food-facts-pp-cli --help`
- `open-food-facts-pp-cli --version`
- `open-food-facts-pp-cli sources --agent`
- `open-food-facts-pp-cli doctor --agent`
- `open-food-facts-pp-cli product 3017620422003 --agent`
- `open-food-facts-pp-cli nutrition 3017620422003 --agent`
- `open-food-facts-pp-cli allergens 3017620422003 --agent`
- `open-food-facts-pp-cli compare 3017620422003 5449000000996 --agent`

## Search Endpoint Availability

- `open-food-facts-pp-cli search --category "breakfast cereals" --country "united-states" --page-size 2 --agent` was covered by unit tests, but the live `/api/v2/search` endpoint returned HTTP 503 during this validation window.
- `open-food-facts-pp-cli category "breakfast cereals" --page-size 2 --agent` shares the same bounded `/api/v2/search` request path and was covered by unit tests rather than repeated against the unavailable live search endpoint.
- A smaller live search retry with explicit `OPEN_FOOD_FACTS_USER_AGENT` and `OPEN_FOOD_FACTS_CONTACT_EMAIL` timed out while waiting for response headers.

## Source Proof

- Open Food Facts' API introduction confirms v3 as the current product-read API line, voluntary data-quality caveats, custom `User-Agent` expectations, and keyless read operations.
- The API guidance documents 15 product read requests per minute per IP address, 10 search requests per minute per IP address, and CSV/JSONL exports for bulk product datasets.
- The API feature table confirms structured search is available through `/api/v2/search` and not available in API v3.
- The authentication guidance confirms write operations need authentication and are out of scope for this read-only print.

## Validation Mode

Unit tests use an in-memory HTTP transport to prove request paths, query parameters, contact-bearing `User-Agent` behavior, category/search request shape, per-barcode comparison errors, upstream error summarization, and compact JSON parsing. Live smoke commands are bounded to small samples and use public read-only endpoints when available.
