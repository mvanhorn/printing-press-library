Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# Atlas Obscura CLI — Phase 3 Build Log

## Architecture
- `internal/cli/ao_source.go` — hand-authored AO source layer: discovered HTTP contract
  (search/near JSON via GetWithHeaders + Accept/X-Requested-With; place short JSON; place
  full HTML JSON-LD + categories + KBYG), Open-Meteo geocoding, interestingness score,
  haversine, local-store trip/visited tables.
- `internal/cli/ao_output.go` — agent-native emit helpers.
- Headline commands (hand-built; AO has no clean spec): `search`, `near`, `show`.
- Transcendence (all 7 built): `route`, `trip` (add/remove/list/show), `visited` (mark/list),
  `gaps`, `cluster`, `surprise`, `export` (gpx/geojson/md).
- Generated typed commands retained: `places get`, `categories places`, `destinations places`,
  plus framework `sync`/`doctor`/`profile`/`feedback`/`import`/`workflow`/`which`/`api`.

## Built & live-verified
- search "catacombs" → 84 matches, "The Catacombs of San Sebastian" (Rome), score 8.
- near "Paris" --radius 1 → "Paris Point Zero" 0.03mi; geocoded via Open-Meteo.
- near "Paris" --category cemeteries → client-side tag filter works (Victor Noir, etc.).
- show <slug>/<id> → full JSON-LD (description, 8 categories, KBYG, coords); --short = JSON.
- route "San Francisco" "Los Angeles" → 347mi corridor, scored stops incl. mid-route.
- trip add/list/show → persists across sessions.
- visited mark/list → local state.
- gaps "Paris" → correctly EXCLUDES visited Eiffel apartment (cache × visited join).
- cluster "Edinburgh" → greedy walkable clusters.
- surprise --near Tokyo --exclude-visited → date-stable pick (score 8).
- export → gpx/geojson/md, offline from local store.

## Contract notes (community-sourced, undocumented)
- search: GET /search?q=&kind=keyword&page= (Accept:json + X-Requested-With).
- near:   GET /search?lat=&lng=&page= (q ignored when lat/lng present).
- show:   GET /places/<id-or-slug>; JSON headers → short JSON; Accept:text/html → full page.
- Client defaults Accept:application/json; place-full forces Accept:text/html via GetWithHeadersNoCache.
- No category facet (browser-sniff confirmed) → client-side tag filter.
- No native geocoder → Open-Meteo (no auth).

## Deferred / honest gaps
- Interestingness score is a documented heuristic from result fields, not an official AO ranking.
- AO has no structured opening hours; practical info surfaced via "Know Before You Go" text.
- route corridor is a straight-line approximation, not turn-by-turn routing.

## Stubs: none.
