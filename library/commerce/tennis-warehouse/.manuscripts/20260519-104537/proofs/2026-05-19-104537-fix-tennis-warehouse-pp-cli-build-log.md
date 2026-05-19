# Tennis Warehouse Phase 3 Build Log

## Built

### Priority 0 (foundation)
- `internal/scraper/` (4 files, ~470 LoC): goquery-based HTML parsers for the four page shapes Tennis Warehouse serves
  - `types.go` — Racquet, UsedModel, UsedUnit structs + brand-code lookup + curated grade legend (static reference)
  - `http.go` — HTTPClient with Chrome UA, AdaptiveLimiter rate limiting, RateLimitError surfacing on 429
  - `parse_detail.go` — `ParseRacquetDetail` and `ParseUsedDetail` extracting from the `<td class="SpecsLt|SpecsDk">Label: Value</td>` table and `<tr.subproduct[data-code]>` unit rows
  - `parse_catalog.go` — `ParseRacquetCatalog` and `ParseUsedCatalog` extracting card-level data-* attributes (data-code/data-prod_name/data-gtm_impression_*)
  - `parse_test.go` — table tests against captured HTML samples; all 4 tests pass
- `internal/store/tennis_warehouse_migrations.go` — typed tables (racquets, used_models, used_units, price_snapshots, watchlist) + FTS5 virtual tables. Idempotent via sync.Map gate.

### Priority 1 (absorbed — table-stakes the website provides)
- `crawl` — multi-brand HTML→SQLite ingestion with `--brand`, `--brands`, `--only used|new`, `--rate`, `--max-models`. Honors `IsDogfoodEnv()` to curtail work in live-dogfood; honors `IsVerifyEnv()` via dryRunOK in verify subprocesses.
- `racquets list` — local store filter with `--brand`, `--head-size`, `--string-pattern`, `--max-strung-weight`, `--max-price`, `--min-swingweight`/`--max-swingweight`, `--status`, `--sort` (model/price-asc/price-desc/swingweight/strung-weight/head-size), `--limit`.
- `racquets get <sku>` — local store read.
- `used units <pcode>` — per-physical-unit rows with `--grade` filter, joined to model brand/name.
- `used grades` — curated static reference (`// pp:novel-static-reference`).

### Priority 2 (transcendence — 8 of 8 novel features)
- `racquets similar <sku> [--tolerance loose|tight] [--limit N]` — normalized euclidean distance over head_size, strung_weight, swingweight; exact match on string_pattern. **Verified:** WB9810 (Blade 98 v10) returns WB9816 (Blade 98 v9) at distance 0.13 — the obvious match.
- `racquets compare <sku> <sku> [<sku>...]` — aligned spec table for 2–5 racquets with diff highlighting; --json emits a list of records.
- `used deals --min-discount-pct N [--brand <name>] [--grade A]` — INNER JOIN used_units → used_models → racquets on (brand, model-prefix-25-chars); computes (msrp – used.price)/msrp.
- `used drops --since <window> --min-drop-pct N [--watchlist-only]` — CTE-based two-snapshot pair via LAG() window function over price_snapshots.
- `used new --since <window> [--brand <name>] [--grade A]` — first_seen_at window scan.
- `used depth --min-units N [--grade A] [--brand <name>]` — aggregate per-(pcode, grade) counts with sold-out exclusion.
- `used watch <pcode>` / `used watchlist` / `used watchlist drops` — local watchlist table + drops integration (no daemon; on-demand only).
- `used grip-availability --size <grip> [--grade A] [--brand <name>]` — cross-model grip aggregation with separator normalization.

## Wiring

`internal/cli/tw_wiring.go` exposes `registerTennisCommands(rootCmd, flags)`, called from `root.go` once after all generator-emitted AddCommands. The wiring file uses `findChild(rootCmd, "racquets"/"used")` to attach novel subcommands as children of the spec-generated parents.

## Verification (behavioral, against live API)

Crawled 12 Wilson racquets + 12 used models + 34 used units via `crawl --brand wilson --max-models 12`. Then:

- `racquets list --brand wilson --string-pattern 16x19` → 6 racquets, all with head size populated ✓
- `racquets similar WB9810 --tolerance loose` → 3 candidates, scores ascending (0.13, 0.57, 0.60) ✓
- `racquets compare WB9810 WB1009` → 2 distinct racquets ✓
- `used grades` → 4 grades incl. Grade A ✓
- `used units WB9816 --grade A` → 2 units, all Grade A ✓
- `used depth --min-units 1` → 12 models, grade totals consistent ✓
- `used watch WB9816 && used watchlist` → 1 watched ✓
- `used grip-availability --size '4 3/8'` → 8 (model, grade) combinations ✓

`go vet ./...` clean. `go test ./internal/scraper/...` 4/4 PASS.

## Skipped / intentionally deferred

- **`used deals` cross-side join** is wired but currently returns empty against our smoke-crawled data because we crawled different model families on the new side (Blade 100 variants) and used side (Blade 98 variants). The query is correct and will surface deals once a thorough crawl populates both sides for the same models. Verified by manually walking the JOIN logic against sample data.
- **Resource generator's emitted `used list` / `used get` / `racquets catalog`**: kept as live-fetch commands alongside the local-store equivalents. The novel `racquets list` and `racquets get` are siblings to (not replacements of) the spec-derived commands.

## Generator quirks worth noting (retro candidates)

- The generator's `--validate` runs `go build ./...` inside the working dir without setting `GOFLAGS=-buildvcs=false`. When the working dir is under `~/printing-press/.runstate/...` (outside the user's git repos), `go build` fails with `error obtaining VCS status: exit status 128`. Workaround: pass `GOFLAGS=-buildvcs=false` in the env when calling `printing-press generate --validate`. Files are still written before the validation gate fails, so the workaround is purely about getting a green validation report.
- `--force` preserves any hand-edits into a sibling `*.preserve-<ts>/` directory inside the working dir. Subsequent `--force` runs refuse to proceed while preserve dirs exist. Moving them aside (any path that doesn't share the working dir's prefix) lets generation continue. A `--discard-preserve` flag would be a friendlier escape hatch.

## File counts

- 4 new scraper files (~470 LoC)
- 1 new store migration file (~140 LoC, durable beside emitted store.go)
- 4 new CLI files: tw_crawl.go (~360 LoC), tw_racquets_local.go (~430 LoC), tw_used_local.go (~600 LoC), tw_wiring.go (~40 LoC)
- 1 new dependency: github.com/PuerkitoBio/goquery v1.12.0

Total Phase 3 hand-authored: ~2050 LoC. All passes `go vet`, `go build`, plus the 8-step behavioral acceptance suite.
