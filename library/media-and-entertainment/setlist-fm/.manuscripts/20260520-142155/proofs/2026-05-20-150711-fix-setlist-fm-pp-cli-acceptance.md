# Acceptance Report: setlist-fm

Level: Full Dogfood
Tests: 39/39 passed
Failures: none

## Test Matrix

### Help checks (9/9)
- doctor, search, artist, predict, overdue, tour-shape, bingo, playlist, sync — all exit 0

### Live API tests (10/10)
- doctor + doctor --json: auth configured, API reachable
- search artists/venues/countries: return data
- artist get/setlists: return structured data
- All JSON output validates

### Sync test (1/1)
- sync artist "Radiohead" --max-pages 3: hydrated artists, venues, cities, countries, setlists, songs tables

### Store-based transcendence tests (17/17)
- predict: returns probability-ranked song list ✓
- predict --json: valid JSON ✓
- overdue: returns songs ranked by shows-since-last-played ✓
- overdue --json: valid JSON ✓
- song-stats: returns per-song stats (plays, first/last, gap, position) ✓
- song-stats --json: valid JSON ✓
- covers: lists cover songs with original artist ✓
- covers --json: valid JSON ✓
- encore: shows encore stats ✓
- encore --json: valid JSON ✓
- debut: lists one-time songs ✓
- venue-loyalty: ranks venues by show count ✓
- bingo: generates song grid ✓
- playlist --output csv: exports songs as CSV ✓
- since: returns setlists by date ✓
- tour-shape: shows tour analytics ✓
- tour-route: shows tour itinerary ✓

### Error path tests (2/2)
- Invalid MBID: exits non-zero ✓
- Nonexistent artist: exits non-zero ✓

## Fixes applied during dogfood: 2
1. Added `Accept: application/json` header to client (API returns XML by default)
2. Cleared stale XML cache from pre-header-fix session

## Printing Press issues: 1
1. Generator should detect APIs that default to XML and auto-add Accept: application/json header in client.go

## Gate: PASS
