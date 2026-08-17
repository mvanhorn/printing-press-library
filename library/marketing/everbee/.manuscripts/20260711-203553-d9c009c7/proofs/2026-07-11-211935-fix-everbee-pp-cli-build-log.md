# EverBee CLI Build Log (v4.28 reprint)

Manifest transcendence rows: 7 planned, **7 built**. Phase 3 completion gate: PASS
(all 16 approved command paths resolve as real Cobra leaves; dogfood
`novel_features_check` reports planned 7 / found 7 / missing none).

## What was built

**`internal/research`** (new hand-authored package, 6 files + 2 test files):

- `types.go` — Provenance, EvidenceSet, KeywordRow/ProductRow (relevance-annotated), Verdict, PriceBand.
- `relevance.go` — the #1492 core. Token-overlap relevance with trivial plural stemming, product-type
  classification (physical/digital/apparel, SVG-PNG exclusion via `listing_type` **and** tag/title
  fallback for upstream mislabels), saturation, opportunity score, and confidence. Thresholds are
  named constants, not magic numbers, because a "low competition" label has to be defensible.
- `fetch.go` — the corrected endpoints. Seeded keyword POST + seeded product GET, `searched_keyword`
  seed-metrics parsing (object *and* array forms), integer-exact listing IDs (`json.Number`, never
  float64), and typed plan-cap detection.
- `pipeline.go` — Niche() assembles a Verdict; degrades honestly when one leg fails; ChildSeeds()
  and Normalize() for batch runs.
- `stats.go` — competitor sampling (median price, review/sales density, listing-age quartiles, shop roll-up).
- `tags.go` — tag/title consensus and seasonality classification.
- `baseline.go` — drift baselines in their own SQLite table.

**Commands** (7 transcendence + 1 hand-authored absorbed):

| Command | #1492 | Verified live |
|---|---|---|
| `research niche` | F1/F2/F7/F8 | "dad shirt": opportunity 9/100, confidence 1.0 backed by 40/40 relevant rows, demand 2565 vs competition 698326, explicitly refuses the low-competition label |
| `research subniches` | F9/F10/F14 | ranks "dadfully shirt" (167 competitors) at 100 vs "dadness shirt" (805k competitors) at 13.8 — at equal demand |
| `research competitors` | F12 | median $25.46, review density 6934, listing age p25=3 / median=8.5 / p75=14 |
| `research tags` | F13 | consensus "custom dad shirt" 8/20, "fathers day shirt" 8/20 |
| `research drift` | F14 | no-baseline reported explicitly; `--save-baseline` then diffs with both timestamps |
| `research listing` | F6/F11 | listing 4515173344 → `resolved: true, has_data: false` + the sync command that fills the gap |
| `selftest` | F16 | 6/6 semantic checks pass, 20/20 keyword + 20/20 product relevance, typed exits 0/3/4 |
| `products search` | absorbed #2 | seeded product search; hand-authored (see below) |

## Deliberate deviations

- **`products search` is hand-authored, not spec-emitted.** The generator elects a resource's
  shortest endpoint path as its canonical bulk-sync target. `GET /product_analytics` (the seeded
  search) is shorter than the browse-feed path, so declaring it in the spec made a bare `sync` call
  it with no `search_term` — which EverBee answers with 400. A seeded search is query-scoped, not an
  enumerable collection. `sync` now mirrors the browse feed; the search lives in
  `internal/cli/products_search.go`. **Generator retro candidate: endpoint election ignores
  `required: true` params.**
- **`folders` dropped from the spec.** `/folders` requires `?type=`, and bulk `sync` does not send
  endpoint param defaults, so it was a guaranteed 400 on a bare `sync`. Folders are saved-UI
  groupings, not research data; no novel feature depends on them.
- **`http_transport: standard`.** `--spec-source browser-sniffed` made the generator ship
  browser-impersonation headers (`Accept: text/html`, `Sec-Fetch-Dest: document`). EverBee's Rails
  API then tried to render an HTML view for JSON endpoints and returned 500 on `/users/show`.
  `probe-reachability` classifies the API `standard_http` (0.95).
- **Single-endpoint resources are promoted to bare commands** by the generator, so the browse feed is
  `products` (not `products list`) and the plan readout is `account` (not `account show`). Manifest
  path labels were corrected to match; no capability changed.
- **Seasonality honestly reports `unknown`.** EverBee returns `trend: null` on every keyword row as
  of the 2026-07-11 capture. Rather than fabricate a series, `research tags` reports
  `verdict: unknown` with the reason. The code parses trend when EverBee does supply it.
- **#1492 F15 (raw safe-GET discovery) not shipped.** The novel-features brainstorm killed it as
  maintainer tooling rather than a weekly command; the user approved the manifest without it.

## Tests

`internal/research` carries table-driven tests including the #1492 regression as data: the
default-feed payload (junk `gifttings` keywords, African-mask listings) is fed through the pipeline
and asserted to produce **confidence 0, opportunity 0, and an explicit no-evidence warning** — the
exact case where the published CLI reported confidence 1.0.

`go build ./...`, `go vet ./...`, and `go test ./internal/...` all pass.
