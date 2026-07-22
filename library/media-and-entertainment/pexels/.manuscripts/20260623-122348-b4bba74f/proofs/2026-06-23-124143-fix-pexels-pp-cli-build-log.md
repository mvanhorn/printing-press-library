# Pexels CLI Build Log

Manifest transcendence rows: 4 planned (hand-code: quota forecast, resolve, download, attribution export), 0 built. Phase 3 will not pass until all 4 ship. (analytics group-by + offline search are framework spec-emits — tracked separately.)

## Build progress

Manifest transcendence rows: 4 planned, 4 built (quota forecast, resolve, download, attribution export). Plus framework-backed: search (FTS), analytics (group-by). novel_features_check: 6/6.

## Built
- internal/pexels/client.go — sibling Pexels client (AdaptiveLimiter, RateLimitError on 429, raw-key auth, User-Agent, rate-ledger persistence)
- internal/pexels/rateledger.go — rate snapshot persistence
- internal/pexels/sizes.go (+test) — pure resolution pickers (PickPhotoSize / PickVideoFile)
- internal/store/pexels_downloads.go (+test) — downloads ledger (Ensure/Insert/Exists/All, NULL-safe scans)
- internal/cli/quota_forecast.go — quota forecast (auto data-source)
- internal/cli/resolve.go — best-fit resolution picker (live)
- internal/cli/download.go — dedup + rate-aware checkpointed bulk download (live)
- internal/cli/attribution_export.go — SOURCES.md / sidecar export (local)
- All 9 endpoint commands + framework generated.

## Verified (independent live behavioral checks)
- resolve 2014422 (target 1280x720) -> picked large2x 1880x1300, no upscale, attribution correct
- download "mountain lake" -> 2 valid JPEGs + 2 sidecars
- dedup re-run -> downloaded 0, skipped 2
- attribution export -> SOURCES.md with real photographer credits + Pexels links
- quota forecast -> remaining 24671, fits true
- Phase 3 gate: 15/15 command paths resolve; novel_features_check 6/6

## Deferred / notes
- Sync pagination uses offset-style page increment; verify in shipcheck.
- No stubs shipped.

## Shipcheck fixes (generated-file edits; retro candidates — generator gaps)
- internal/cli/sync.go determinePaginationDefaults(): generic offset/`limit`=100 defaults caused HTTP 401 ("Missing API key") on Pexels because an unknown `limit` query param trips request validation. Fixed to page/`per_page`=80. Root cause: profiler emits generic offset defaults; should derive page/per_page from spec pagination + params.
- internal/cli/search.go extractSearchResults(): generic wrapper keys (data/results/items) missed Pexels `photos`/`videos`/`media` envelopes, so live framework search returned empty. Added Pexels envelope keys. Root cause: extraction key list should include spec response_path values.

## Phase 5 dogfood fixes (CLI-side, verified; full dogfood 80/80 PASS)
- download/resolve: added --query/--id flag alternatives (positional-or-flag) — agent + verifier friendly.
- workflow archive: cap pages-per-resource to 2 under IsDogfoodEnv (avoid full-archive timeout).
- client.go: maxRetries=0 under dogfood/verify (fail-fast on 429 to respect per-command timeouts; production keeps 3 retries). Retro candidate: generator could ship this by default.
- pp:happy-args annotations on the 4 novel commands.

## Key API finding
Pexels gates filtered search (orientation/size/color/locale) + all collections endpoints behind auth; basic query/curated/by-id/video search are keyless. Collections endpoints have a ~6-call burst limit. CLI handles 429 correctly.
