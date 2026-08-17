Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Rental Car Spain Build Log

## Built (Priority 0 — foundation)
- `internal/carsource/` — hand-built HTML source layer (no generator endpoint mirror; both sources are server-rendered HTML):
  - `types.go` — Offer/Location/SearchQuery, plain-UA constant (WAF quirk).
  - `htmlutil.go` — x/net/html traversal helpers + robust EU/US price parser.
  - `suppliers.go` — DoYouSpain data-prv code → supplier name map + `--supplier` alias matching (incl. delpaso/recordgo/wiber presets).
  - `doyouspain.go` — 3-step flow (homepage cookie prime → POST /do/list → GET results), autocomplete resolver, **retry-once** on missing redirect token to smooth transient empties.
  - `parse_offers.go` — parses `<article data-prv>` offer rows.
  - `delpaso.go` — Laravel CSRF flow (GET / for _token → POST /offers), parses `list-car` groups; full coverage bundled.
- `internal/store/rentalcarspain.go` — durable hand-authored store extension: `saved_searches` + `price_snapshots` tables, lazy-init, snapshot record/list for drift/watch.

## Built (Priority 1 — absorbed, 14 features)
- `search` — headline DoYouSpain search; full-insurance default, `--base`, `--supplier/--class/--max-total/--max-per-day/--transmission/--driver-age/--sort/--currency/--limit`. Records a snapshot each run.
- `locations` — autocomplete resolver (name → code; Malaga Airport = MAL02).
- `delpaso` — live direct Delpaso quote (Málaga).
- Per-day + total price, deposit/excess where present, supplier score/reviews, free-cancellation flag: parsed into Offer and surfaced via `--json`.
- `doctor` — generated; reachability.

## Built (Priority 2 — transcendence, 6/6)
- `compare` — DoYouSpain cheapest Delpaso vs Delpaso direct, with delta. (Verified live: €39.98 vs €110.04, aggregator €70 cheaper.)
- `suppliers` — cheapest full-insurance offer per supplier, ranked (all 3 user companies present live).
- `dates` — cheapest-pickup-date sweep across a window (capped 21 days), per-date partial-failure preserved.
- `saved add/list/remove` — named searches in SQLite.
- `watch --target-price` — re-quote saved search, snapshot, typed exit 0/10 for cron. (Verified: exit 0 at/below target, exit 10 above.)
- `drift` — price history from local snapshots with direction/min/max.

## Verification (live, 2026-07-13)
- locations: 16 Málaga locations resolved.
- search: 327 offers parsed, Delpaso cheapest €39.98.
- suppliers: Delpaso/Niza/Drivalia/Europcar/OK Mobility/Record Go/Wiber all present.
- compare/watch/drift/dates: all exercised live end-to-end.
- `go vet ./...` clean; `go test ./...` green (carsource, store, cli).

## Intentionally deferred / out of scope (user-approved)
- Record Go and Wiber have NO standalone live client (Record Go geo-blocks non-EU egress; Wiber sits behind a Fastly JS challenge). Both are covered as suppliers within `search`/`suppliers`. Nothing stubbed.
- DoYouSpain per-offer deposit/excess and the explicit full-insurance upsell price are not separately isolated in the HTML parser yet; the quoted price is the offer's default (which the site presents with Full Insurance). Flagged as a parser refinement.

## Generator limitations found
- Internal spec `offers` resource emits a low-level `offers list` generated command requiring raw s/b session tokens; kept as documented low-level, superseded by top-level `search`.
