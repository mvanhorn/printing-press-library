# OfferUp CLI — Phase 3 Build Log

**Run:** 20260531-200239 · **Binary:** `offerup-pp-cli`

## What was built

### Durable hand-authored data layer (`internal/offerup/`)
- `offerup.go` — plain-HTTP client (Chrome UA), `__NEXT_DATA__` extraction, clean `Listing`/`ListingDetail`/`Seller` structs, ad-tile filtering (`tileType==LISTING`), Apollo ROOT_QUERY listing + User seller resolution, schema.org condition-label fallback (numeric code → "Used"/"New"/…), `ou.location` cookie, `cliutil.AdaptiveLimiter` pacing + `*cliutil.RateLimitError` on 429. Reads `OFFERUP_BASE_URL` (default https://offerup.com) so verify/dogfood mock-routing works.
- `store.go` — SQLite tables `offerup_listings` (composite key query@location), `offerup_price_snapshots` (price history), `offerup_sellers`. Preserves `first_seen`, appends snapshots on price change. Uses `database/sql` on the local file (novel-command carve-out). NULL-safe scans throughout.
- `stats.go` — median/percentile/firm-ratio aggregation.
- `offerup_test.go` + `store_test.go` — table-driven tests: ParsePrice, listing extraction, price stats, search-against-mock (ad filtering + price/firm filters), location cookie, store round-trip (new-count, new-since window, drop detection, seller inventory). `go test ./...` → exit 0.

### Commands (P1 absorbed)
- `listings search <query>` — clean listings; flags `--zip/--lat/--lon/--city/--state/--category/--limit/--price-min/--price-max/--firm/--local`. Live + caches to store. (`pp:endpoint listings.search`, read-only.)
- `listings get <listing-id>` — full detail (description, photos, condition, seller); caches detail + seller. (`pp:endpoint listings.get`, read-only.)

### Commands (P2 transcendence — all hand-code, all approved at Phase 1.5)
- `price-check <query>` — median/p25/p75/min/max/mean + firm% over the live result set.
- `deals <query> --below N` — listings ≥N% under the local median, steepest first.
- `new-since <query> --since D` — listings first-seen within the window (self-populating).
- `price-drops <query> --since D` — cross-snapshot price-cut detection (empty until ≥2 observations).
- `seller-scan <seller-id>` — store-read: seller reputation + locally-known inventory + median asking.
- `digest <query> --since D` — one-call composite (new + drops + deals + stats).

All commands: bare→help; `--dry-run`→exit 0 (no IO); `cliutil.IsVerifyEnv()`→valid empty result (no network/store write); else live. All read-only MCP-annotated. All output via `printJSONFiltered` (--json/--select/--compact/--csv/--quiet).

## Live behavioral acceptance (real OfferUp)
- `listings search "iphone" --zip 85001 --limit 4` → clean array, no ads, flat fields ✓
- `listings get <id>` → title/price/description(392ch)/5 photos/location/seller(name+BUSINESS badge)/condition "Used" ✓
- `price-check "iphone" --zip 85001` → 44 listings, median $362.50, p25 $80, p75 $600 ✓
- `deals --below 25` → 17 deals, top $5 box at 98.6% below median ✓
- `new-since --since 1h` → 44 (all new on fresh store) ✓
- `price-drops` → 0 (honest empty, first run) ✓
- `digest` → newCount 44, dropCount 0, dealCount 18 ✓
- `seller-scan 161842229` → seller + inventory 1 + median asking, after `listings get` populated owner ✓

## Phase 3 completion gate
- Per-row Cobra resolution: all 8 commands resolve with correct Usage spec lines ✓
- `dogfood --json .novel_features_check` → planned 6, found 6, missing [], skipped false → **GATE PASS** ✓
- `go build ./...` 0, `go vet ./...` 0, `go test ./...` 0 ✓

## Intentionally deferred / scope notes
- **`deals --markdowns`** (seller-declared original-vs-current cuts): dropped — OfferUp search tiles do not expose `originalPrice` (only item-detail does), so it would require N item fetches per search. The core `deals` (below-median) is delivered. research.json description corrected.
- **`sellers get` / `categories list`** standalone commands: not built. Seller profile rides on `listings get` + `seller-scan`; category browse is `listings search --category <cid>`. Manifest rows normalized to `(behavior in ...)` dispositions.
- **Auth-gated features** (saved/messages/posting/offers): out of v1 scope per the unauthenticated-first directive.

## Generator notes
- Generated baseline `listings search`/`get` (html embedded-json) returned raw `looseTiles` incl. ad tiles + nested `.listing`; reimplemented against the clean `internal/offerup` client. Possible retro candidate: html embedded-json mode can't filter/flatten a tile array, so Next.js feed pages that wrap items need hand extraction.
