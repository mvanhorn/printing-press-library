# Google Play CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 passes — all 7 ship.

## What was built

### Priority 0 — Foundation (hand-built)
- `internal/gplay/` — the entire Play public-store client, live-verified against play.google.com on 2026-06-11:
  - `client.go`: standard HTTP transport, batchexecute envelope + `)]}'` length-prefixed double-encoded framing (handles rt=c chunking via a streaming JSON decoder), AF_initDataCallback ds-key extraction, ~1 req/s adaptive throttle, exponential backoff on 429/503, and `PlayGatewayError`-in-200 detection surfaced as a typed `*cliutil.RateLimitError`.
  - `parse.go`: panic-safe positional `node` navigator (array index + map key + negative index) so shifted Play layouts degrade to empty fields, never a crash.
  - Feature methods: `AppDetails` (ds:5), `TopCharts` (vyAe2, exact embedded field-mask payload), `Search` (ds:4 + qnKhOb pagination), `Reviews` (oCPfdb + continuation), `Suggest` (IJ4APc), `Similar` (details -> ag2B9c cluster grid), `Permissions` (xdSrCf), `DataSafety` (ds:3 with the "138" object key), `Developer` (/dev numeric + /developer name).
- `internal/store/gplay_store.go` — snapshot layer (chart_snapshots, app_snapshots, keyword_ranks, app_reviews) with lazy migrations and time-series query helpers. This is what makes the transcendence commands possible.

### Priority 1 — Absorbed (9 hand-built live commands)
app, top, search-store, suggest, reviews, similar, developer, permissions, datasafety. Plus the generator-emitted `categories list` (html_extract links). All emit through the generated output pipeline (--json/--select/--compact/--csv) and carry verify-friendly RunE (help-only branch, dry-run short-circuit, usageErr on missing input).

### Priority 2 — Transcendence (7 hand-built)
movers, rank-history, watch-listing, keyword-rank, keyword-history, review-digest, compare. Built by replacing the generator's stub files in place (function names preserved so root.go wiring resolves). All annotated `// pp:data-source local|live`.

## Verification done in-session (live)
- Every absorbed surface returned correct real data (app fields incl. exact realInstalls 92.5M + 5-bucket histogram; charts with real appIds; search; 7 permissions; datasafety shared/collected; developer 2 apps; similar 6 games).
- All 7 transcendence commands exercised against a real snapshot DB: movers diff, rank-history series, watch-listing diff, keyword-rank (rank 19 found), keyword-history, review-digest (60 reviews, mean 3.58), compare (2 items 0 failures).
- Unit tests: gplay (12 tests: normalizers, cleanText, devIDFromURL, node nav, batchexecute framing incl. rt=c + null, AF extraction, chart parse) + cli (computeMovers, diffListings, digestReviews, tokenize, round2, command construction + dry-run + required-input) + store. All pass.

## Intentionally deferred / honest gaps
- review-digest complaint-term frequency is mechanical token counting (no NLP, by design); a few generic words slip past the stopword list. The feature is honest: pipe to an LLM for prose.
- Parser fragility is inherent to scraping Play (positional protojson). The node navigator and fallback paths mitigate; index paths can shift on a store redesign (~yearly).

## Generator limitations found (retro candidates)
- Generated typed endpoints cannot parse Play's positional-protojson responses, so the entire client + 16 commands had to be hand-built; only `categories` (html_extract) used the generated path. Not a bug, but a from-website CLI of this shape gets little leverage from typed-endpoint emission.
