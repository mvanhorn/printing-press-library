# Roadside America — Build Log

Manifest transcendence rows: 5 planned, 5 built. Phase 3 completion gate passed (dogfood novel_features_check: planned 5 / found 5 / 0 missing).

## Built
- **Domain package `internal/roadside/`** (hand-authored, tested — 12 table-driven tests):
  - `parse.go` — `ParseAttrList` (the `<ul class="attrlist">` fragment from attractionsByState.php / nearbyAttractions.php) and `ParseDetail` (/tip/<id>, /story/<id>): name, street, city/state, distance, writeup, directions, source URL. Strips `<script>/<style>` and anchors the editorial blurb on `fieldReviewListIcon"></div></a><p>…</p>`.
  - `classify.go` — local superlative/keyword categories (biggest, smallest, tallest, weird-food, muffler-men, animals, signs, statues, museums) with alias normalization.
  - `geocode.go` — keyless OSM/Nominatim geocoding (pure URL builder + response parser + rate-limited `Geocoder`).
  - `roadside.go` — types, state code table, `MilesToDelta`, URL builders, source label.
- **`internal/cli/roadside_shared.go`** — polite client usage, fetch+parse helpers, SQLite cache (attractions + details), fresh-on-read detail TTL (30d), geocode cache, place-or-latlng resolution, output rendering with source attribution.
- **Hand-built top-level commands:** `near` (place|lat,lng + --radius), `state <ST>`, `show <id|name>`, `search <query>` (offline FTS + substring fallback), `sql <SELECT>` (read-only guard).
- **Filled novel stubs:** `category` (+`--list`), `stats`, `random` (live-capable bootstrap), `trip` (multi-stop, per-stop partial-failure handling), `compare`.
- Registered the 5 new commands in `root.go`; set `--rate-limit` default to 0.34 req/s (~1 req/3s) for the polite source policy.

## Verified live (end-to-end against roadsideamerica.com)
- `state TX` → 893 attractions parsed; `near 30.27,-97.74` → distance-sorted; `near "Austin, TX"` → geocoded via Nominatim.
- `show 2055` → "A 200-foot-long alligator-shaped building…" (clean editorial writeup, after fixing a JS/script-leak bug).
- `category biggest`, `stats` (894 cached, by-state/by-category), `compare TX RI`, `trip Austin/Waco` (60 unique), `search alligator`, `sql` group-by — all correct.
- Exit codes: bad state → 2, missing args → 2, help → 0; every command `--dry-run` short-circuits with rc 0.

## Decisions / deferred
- **Theme slugs dropped:** `attractionsByTheme.php` works but its slug vocabulary is internal/undocumented (display-name guesses 404; `themes.php` 404s). `category` is local classification instead — serves the user's examples and needs no undocumented endpoint.
- **No per-attraction coordinates** on the site → `--radius` is enforced via the site's own "X mi. away" distances, not client-side haversine.
- **Geocoding via keyless OSM Nominatim** (cached, rate-limited, disclosed) for `near "City, ST"` / `trip`; `near <lat,lng>` needs no geocoder.

## Generator notes (for retro)
- `sql` and `search` framework commands were NOT auto-emitted for an all-HTML (`response_format: html`) spec; hand-built both.
- Novel-feature stubs were emitted from research.json and wired into root.go (good); filled in place (body-drift, regen-mergeable).
- Generated `--rate-limit` default is 0 (disabled); a scrape CLI wants a polite non-zero default — set to 0.34.
