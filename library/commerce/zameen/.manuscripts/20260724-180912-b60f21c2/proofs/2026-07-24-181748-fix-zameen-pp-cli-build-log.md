# Zameen CLI Build Log

Manifest transcendence rows: 5 planned, 5 built. Phase 3 will not pass until all 5 ship.

## Built

### Core (hand-authored `internal/zameen` client + commands)
- `internal/zameen/zameen.go` — location/category resolution, `window.state` hit schema, `hit -> types.Listing` mapping, Marla conversion (`area / 20.9032`, derived exactly from data), flexString for string-or-number ids, client-side filter predicates.
- `internal/zameen/client.go` — HTTP client with `cliutil.AdaptiveLimiter` per-source rate limiting + `*cliutil.RateLimitError` on 429; `window.state` brace-balanced extraction; `Page` (raw single page) and `Search` (scan-and-filter with `MaxScanPages`, IsDogfoodEnv curtailment, dedup of Zameen featured+organic copies).
- `find` — live parametric search (price/beds/baths/area/type/verified filters + sort), scan-and-filter caps, `--json/--select/--csv/--agent`. Verify-friendly RunE.
- `listings <category> <location> <page>` — raw single-page primitive (rewrote the generated html-endpoint scaffold, which could not parse `window.state`).
- `pull` — mirror a search into local SQLite via `store.Upsert` keyed by `external_id` (auto-dedup).
- `get <id>` / `open <id>` — store-first detail; print-by-default URL with `--launch` + IsVerifyEnv guard.

### Transcendence (5/5, all hand-code, all scored >=5/10)
1. `watch <name>` — saved-search cross-run diff (new listings + price drops). Snapshot tables in hand-authored `internal/store/zameen_watch.go`; `--list`; first-run baseline. **Verified live**: baseline recorded 9, re-run diffed 0/0 correctly.
2. `comps --city --area` — area medians + price-per-Marla + inventory from store. **Verified live**: 21 areas, DHA Defence 13 listings median 119M / 5.95M-per-Marla.
3. `deals --city --area --type` — below-market ranking vs area median price-per-Marla. **Verified live**: 25 below-market rows ranked by pct-below.
4. `aging --city --days` — days-on-market from `updated_at`. **Verified live**.
5. `agencies --city` — agency leaderboard (count + median). **Verified live**.

## Store
- Listings stored as `resource_type="listings"`, id=`external_id` (via `Upsert`, not `UpsertBatch` — the generic ID heuristic does not recognize `external_id`).
- `watch_searches` + `watch_snapshots` tables (hand-authored migration file).

## Deferred / intentionally out of scope
- Favorites / saved-alerts (login-gated) — v1 is read-only, no auth.
- New-projects and agency-detail surfaces — secondary, not built (killed at absorb gate as scope creep).
- Location autocomplete (name->slug for arbitrary areas) — handled client-side by fetching city-level and filtering by area substring, which sidesteps the area-ID lookup.

## Generator issues found (retro candidates)
- **sync_hint tests contradict impl**: generator emits `const syncHintsEnabled = false` (correct here — this CLI has no framework `sync` command; hint text even points at a nonexistent `sync`) but still emits `sync_hint_test.go` asserting hints fire → 5 test failures on a fresh tree. Worked around locally by guarding the 5 tests to skip when the feature is disabled. Generator should skip these tests when `syncHintsEnabled=false`.
- **html_extract cannot parse `window.state = {…}`**: the embedded-json mode targets `<script>` JSON blobs, not JS assignments, so the entire listing surface had to be hand-built. Reasonable for a website-itself CLI, but worth a generator note.

## Quality gates (pre-shipcheck)
- `go build ./...` clean; `go vet ./...` clean.
- `go test` green: internal/cli, internal/zameen (8 tests), internal/store, internal/mcp (+bound, +cobratree), internal/client.
- Phase 3 Completion Gate: 9/9 approved commands resolve; dogfood novel_features_check planned=5 found=5 missing=0.
