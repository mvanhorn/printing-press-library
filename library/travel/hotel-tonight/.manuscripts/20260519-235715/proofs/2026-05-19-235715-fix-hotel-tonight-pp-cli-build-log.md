# HotelTonight CLI — Build Log

## Generated (Priority 0/1)
- `printing-press generate --spec hotel-tonight-spec.yaml` — 8/8 gates pass.
- Internal YAML spec: `auth: none` + `required_headers` baking the `X-App-*` mobile headers into every request (the API gates on app-identifying headers, not a token).
- Endpoint-mirror commands: `inventory` (promoted search), `markets list/get/nearby`.
- Confirmed live: all base commands return correct data header-only over plain HTTP.

## Hand-built (Priority 2 — 6 novel features + bonus)
All in `internal/cli/ht_*.go` (hand-authored, survive `generate --force`; AddCommand wiring in root.go re-injected by regen-merge):
- `ht_core.go` — typed inventory/hotel/market structs (json.Number for cross-market numeric type variance), `/v6/inventory` fetch, date/category/discount helpers, deal flattening.
- `ht_pricedb.go` — price-history layer on the generated SQLite store (`ht_snapshots` table); NULL-safe scans throughout; percentile/verdict math.
- `ht_output.go` — shared JSON-or-table emit + fetch-and-record.
- `ht_featureddeal.go` — Daily Drop extraction from the SSR results page (brace-counted embedded `featured_deal` JSON; hotel name resolved by id).
- Commands: `deals` (rich search + `--category` tier filter), `watch`, `history`, `verdict`, `compare-neighborhoods`, `datescan`, `daily-drop`.

### Novel features (all verified live)
| Command | Status |
|---------|--------|
| `watch --below` | flags rooms below threshold + dropped-since-last-run (store diff) |
| `history <hotel>` | time-series from local store |
| `verdict <hotel>` | cheap/typical/expensive percentile classification |
| `compare-neighborhoods --metro` | neighborhood rollup ranked by price/discount |
| `datescan` | tonight/tomorrow/weekend fan-out, ranked |
| `daily-drop` | reveals the hidden Daily Drop hotel + real price via SSR; `--history` longitudinal record |

### User-requested addition (Phase Gate 1.5)
- `--category {basic,solid,hip,luxe,charming,crashpad}` filter on the deal-listing commands. Client-side filter on the live `hotel.category` field.

## Notable engineering decisions
- **json.Number for all upstream numerics.** HotelTonight types `deal_id`/`id`/lat/lng/prices inconsistently across markets (string in SF, number in Austin). json.Number + numF/numI helpers de-risk unmarshaling.
- **Daily Drop is not in the anonymous `/v6/inventory` feed** — the app hides it behind an account-gated "slide to unlock" (`/v6/featured_deal` 404s anonymously). The full deal is embedded in the SSR results page; `daily-drop` reads it there (replayable HTTP+HTML, no auth, no runtime browser).
- **Snapshot-on-fetch:** every live deal fetch records to `ht_snapshots`, so history/verdict/watch accrue data naturally.

## Tests
- `ht_core_test.go`, `ht_pricedb_test.go`, `ht_featureddeal_test.go` — table-driven coverage of pctOff, category parse/match, date resolution, sort, neighborhood rollup, percentile/classify, and the Daily Drop SSR parser (incl. nested-array brace-count + unavailable/missing cases). `go test ./internal/cli/` passes.

## Deferred / intentional
- No hotel-detail command — no public hotel-detail endpoint exists (404); inventory embeds full hotel data.
- No authenticated/booking flows — out of scope per user briefing (anonymous only).

## Gate results
- `go build`, `go vet`, `go test ./internal/cli/`: pass.
- Phase 3 Completion Gate: all 6 transcendence commands resolve as leaf commands; dogfood `novel_features_check` planned=6 found=6, no missing.
