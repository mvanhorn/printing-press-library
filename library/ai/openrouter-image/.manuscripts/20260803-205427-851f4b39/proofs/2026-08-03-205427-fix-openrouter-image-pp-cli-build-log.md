# Build Log: openrouter-image-pp-cli

Manifest transcendence rows: 6 planned, 6 built. Phase 3 gate PASSED (6/6 novel_features_check found, 0 missing).

## Priority 0 (foundation)
- [x] Data layer: synced `images` catalog table (38 image models verified live), custom `generation_ledger`, `image_endpoint_cache`, `image_catalog_snapshot` tables in `internal/store/openrouter_image_migrations.go`
- [x] sync/search/SQL paths verified: `sync --resources images` pulls 38 models in ~170ms
- [x] Custom store tables tested (`openrouter_image_migrations_test.go`: ledger round-trip, newest-first listing, endpoint cache, snapshot baseline)

## Priority 1 (absorb) — 14 features
- [x] Generated endpoint surface: `images create`, `images list-models`, `images list-model-endpoints`, `generation get/content/feedback`, `models get`, `key`, `credits` — all emit --json/--dry-run/--agent
- [x] Priority 1 Review Gate passed: generate/cost-estimate/models rank --help + --dry-run + --json all correct

## Priority 2 (transcend) — 6 hand-code features
- [x] `generate` (flagship, new top-level command): POST /images, saves base64 output to disk, writes ledger, unit-aware cost in response
- [x] `cost-estimate`: offline from endpoint pricing cache, live-fetch+populate on first use (public endpoint), unit-aware (image/token/megapixel)
- [x] `models rank`: capability filters (--image-to-image, --resolution) + budget (--max-cost) join catalog x pricing, cheapest-first, live-fetch+cache pricing
- [x] `regenerate`: replays stored params from ledger, --tweak prompt edit, writes new ledger row
- [x] `models diff`: baseline snapshot on first run, added/retired/changed diff after, canonical JSON compare
- [x] `batch`: CSV parse (prompt,model,n,resolution,quality,output), per-row estimate, hard --budget gate aborts before any spend, executes + ledger
- [x] `usage digest`: two-window ledger trend (current vs previous), top models, spend delta

## Tests
- `internal/cli/novel_helpers_test.go`: cheapestOutputUnit (5 cases), parseBatchCSV, safeName, extFromMediaType, normalizeAny
- `internal/store/openrouter_image_migrations_test.go`: 4 table tests
- All existing scaffold smoke tests pass

## Intentionally deferred
- (none)

## Skipped body fields
- provider.options.* deep per-provider objects in images create body: exposed as individual flags by the generator; the flagship `generate` command covers the common surface (model, prompt, n, resolution, aspect-ratio, size, quality, output-format, background, compression, seed, stream, reference, provider)

## Generator limitations found
- OpenAPI `x-flag-name` vendor extension is not read by the OpenAPI parser in v4.29.0 (`flag_name` is internal-YAML-only); resolved via evidence-backed skip decisions in the public-param-audit ledger
- novel feature command "models" stub skipped by generator (maps to generated command path); rank/diff were correctly attached as children of the generated models command
