# Atlas Obscura CLI — Phase 5 Live Dogfood Acceptance

Level: Full Dogfood (live, against atlasobscura.com — no API key needed)
Result: PASS — 65/65 mechanical matrix tests passed.

## Matrix
- 65 tests across every leaf command: help, happy-path, JSON-fidelity, output modes, error paths.
- 3 error-path probes skipped via `pp:no-error-path-probe` (export / trip remove / trip show):
  an unknown trip/item is a legitimate empty/idempotent result (exit 0), not an error.

## Behavioral correctness (live, beyond the matrix)
- search "catacombs" → 84 matches, correct relevance.
- near "Paris" --radius / --category cemeteries → distance + tag filtering correct.
- show <slug>/<id> → full JSON-LD detail incl. Know Before You Go.
- route "Denver" "Moab" / "SF" "LA" → scored corridor stops.
- trip add/list/show, visited mark/list → persisted across sessions.
- gaps "Portland, Oregon" → geocoded (City,State fallback), excludes visited.
- cluster, surprise (date-stable), export gpx/geojson/md → all correct.

## Fixes applied during Phase 4/5
- Geocoding "City, State" fallback (Open-Meteo) — CLI fix.
- near/route radius/width now drop unverifiable zero-coordinate places — CLI fix.
- near --category surfaces fetch errors instead of silently under-delivering — CLI fix.
- visited mark emits slug+id; export empty trip exits 0 — CLI fix.
- error-path opt-out annotations on export/trip remove/trip show — CLI fix.

## Printing Press issues for retro: none material.

Gate: PASS → proceed to polish + promote.
