Manifest transcendence rows: 8 planned, 8 built. Phase 3 completion gate PASSED.

# WooCommerce CLI — Phase 3 Build Log

## Generated baseline (Phase 2)
- Spec: 31 resources / 166 endpoints, derived from the live WP-REST route index.
- All quality gates green on a clean generate: go mod tidy, govulncheck, go vet, go build, binary, --help, version, doctor.
- Cloudflare MCP pattern auto-applied (166 endpoints > 50): code orchestration, hidden endpoint mirrors, stdio+http transport.
- Learn loop on by default, seeded with order-status / product-type / stock-status / discount-type vocabulary.
- All 31 resource command families verified to resolve with their subcommands.

## Generator defects encountered
1. **Duplicate switch case in sync.go.** A resource containing two syncable collection-list endpoints
   (`variations`: per-product `list` + store-wide `list-all`) made the generator emit `case "variations"`
   twice in `syncResourceSinceParam` and `syncResourceSinceParamFormat`, breaking the build. Worked around
   by splitting the store-wide listing into its own `all-variations` resource. Retro candidate: the
   generator should dedupe switch cases by resource name.
2. **`--force` merge resurrected a broken file.** After fixing the spec, `generate --force` preserved the
   previous run's broken `sync.go` through the AST merge ("merged 9 preserved files"), reproducing the
   original compile error even though the fresh emission was green. Required wiping the output directory.
   Retro candidate: a preserved file that fails to compile should not win over a green fresh emission.
3. **`example.com` base_url rejected.** Correct behaviour (RFC 2606 guard), but worth noting it fires
   before any other validation, so a spec authored with a placeholder host fails fast with no other signal.

## Novel-feature scaffolds emitted
The generator produced Cobra scaffolds for all 8 transcendence commands with correct names, flags,
`mcp:read-only` annotations, and examples — RunE bodies are TODO. Only 3 are wired into root.go
(`refund-rate`, `revenue`, `stock`); the other 5 collide with generated resource parents
(`catalog`, `customers`, `orders`) and need local-variable-capture wiring.

## Local store ground truth (verified by a real zero-credential sync)
Typed tables, all carrying `id`, `data` (full JSON), `synced_at`:
- `orders`      + number, status, currency, total, total_tax, customer_id, payment_method_title, date_created, date_paid, date_completed
- `products`    + name, slug, permalink, type, status, sku, price, regular_price, sale_price, stock_status, stock_quantity, total_sales, date_created, date_modified
- `customers`   + email, first_name, last_name, username, date_created, is_paying_customer
- `catalog`     + name, slug, permalink, sku, type, on_sale, average_rating, review_count, is_in_stock
- `coupons`     + code, amount, discount_type, usage_count, usage_limit, date_expires
- `variations`  + parent_id
- `order_refunds` + parent_id
Generic `resources` table is keyed by flat `resource_type` (verified: `catalog`, not `products_catalog`).

## Build progress — 8/8 built

- [x] `orders triage` — status/age buckets + per-gateway failure rate vs. the trailing half of synced history
- [x] `stock velocity` — line-item demand over a window ÷ current stock → days-of-cover + stockout date
- [x] `revenue explain` — period delta decomposed; order-count and basket-size contributions sum exactly to the delta (asserted in test)
- [x] `refund-rate` — refunds joined to parent-order line items; falls back to proportional attribution and labels which it used
- [x] `customers ltv` — per-customer LTV, order cadence, first-order-month cohorts, guests grouped separately
- [x] `catalog audit` — 8 deterministic defect rules over products + variations, zero-count rules still emitted for stable shape
- [x] `catalog watch` — public Store API snapshot of ANY store, no credentials, minor units normalized on write
- [x] `catalog diff` — snapshot comparison, no API call at read time

## Codex delegation outcome

Codex (gpt-5.6-sol) produced the spec converter successfully (30KB Python → 166-endpoint spec).
It then **stalled on the Phase 3 command batch**: ~25 minutes of exploration with zero file writes and
a flat log, so the circuit breaker fired and Claude implemented all 8 commands directly. No partial
writes were left behind (working tree was clean at the kill), so no rollback was needed.

## Supporting code written by hand

- `internal/cli/woo_analytics.go` — shared date/money parsing, drain-first order loader, revenue-bearing
  status set, missing-mirror guard. WooCommerce emits money as decimal STRINGS and dates in three
  different layouts, so both need tolerant parsing rather than a struct decode.
- `internal/store/woocommerce_migrations.go` — hand-authored `catalog_snapshots` schema in its own file
  so `generate --force` preserves it. Single-transaction insert that deliberately avoids `store.Upsert`
  (SQLite WAL permits one writer).
- `internal/source/storeapi/client.go` — sibling client for arbitrary public stores, with an adaptive
  rate limiter that surfaces a typed `*cliutil.RateLimitError` instead of returning an empty page.

## Additional generator defects found and handled

4. **Data race in generated `internal/store/learnings.go`** (FIXED IN PLACE). `RegisterQuerySynonyms`
   writes the package-level `querySynonyms` map while `compileQuerySynonyms` iterates it. The generated
   test `TestPlaybookInit_ConcurrentSafe` calls the registration path from several goroutines, producing
   a fatal "concurrent map iteration and map write", reproducible under `-race`. Fixed with an RWMutex.
   Retro candidate: this affects any printed CLI whose spec declares `learn.synonyms`.
5. **Dead single-token auth command for pair-credential specs** (FIXED IN PLACE). The generator emitted
   `newAuthSetTokenCmd` but correctly registered `auth set-credentials` for this two-env-var Basic auth
   spec, leaving an unregistered command that dogfood flagged. Removed the dead function and repointed
   two `doctor.go` hints and one `client.go` credential error that still told users to run the
   nonexistent `auth set-token`. Retro candidate: skip emitting the single-token command when the auth
   shape is a credential pair.
6. **Generated credential tests assume a single credential** (NOT FIXED — generator-reserved package).
   Three tests in `internal/cliutil/credentials_test.go` fail because they set one legacy credential and
   expect a non-empty `AuthHeader()`, but Basic auth needs both key and secret. `internal/cliutil/` is
   generator-reserved, so per the Phase 4.95 scope rule this routes to retro rather than being patched.


## Phase 5 defect: sync never paginated (fixed)

`pagination.type: offset` in the spec caused the runtime to send an offset value through the `page`
parameter (`?page=10` instead of `?page=2`), ending enumeration after one page. Mirror held 10 of 40
products and 100 of 537 orders with no warning. Corrected to `type: page` in both the generated
runtime (24 resources) and the source spec. Verified complete afterwards: 537/537 and 40/40.

Retro candidate: the profiler should not pair `cursorType: offset` with `cursorParam: page`; those two
are contradictory, and the mismatch is silently wrong rather than an error.

## Multi-store defect: cross-tenant credential leak (fixed)

Found while wiring a second client store. `config.Load` unconditionally overwrote any credentials the
selected config file carried with the single global `credentials.toml`:

```go
creds, ok, err := cliutil.LoadCredentials()
if ok && creds.HasValues() {
    cfg.clearCredentialFields()   // discards the config file's own credentials
    cfg.applyCredentials(creds)
}
```

Two consequences, both silent:

1. **Per-store credentials were ignored.** A config file with its own `consumer_key`/`consumer_secret`
   had them cleared, so every config resolved to the same global pair.
2. **Cross-tenant leak.** Running the CLI against store B while store A's keys sat in the global file
   sent **store A's credentials to store B's server**, where B's operator could log them. Confirmed via
   `doctor --config <store-B>` reporting `Auth: configured` with `credentials_location: credentials file`
   while `base_url` pointed at store B.

This matters most for the exact audience the CLI targets: agencies running many client stores.

Fixed so a config file carrying its own credentials wins, and the shared credentials file is consulted
only as a fallback. Verified both directions: a 0600 config with its own keys reports
`credentials_location: config file`; a config without keys still falls back to the shared file.

Note the pre-existing permission gate is a real feature and interacts here: a config file with loose
permissions has its credentials discarded before this precedence check, so per-store config files must
be `chmod 600`.

Retro candidate: credential resolution should be scoped to the active config/base_url rather than a
process-global file. Three `internal/config` permission tests also fail identically with and without
this change because they read the real user's global credentials file instead of an isolated HOME.
