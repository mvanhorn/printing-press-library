# Peekaboo CLI Build Log

Manifest transcendence rows: 6 planned, 0 built. Phase 3 will not pass until all 6 ship.

## Plan
- Auth: zero-config guest-token bootstrap (scrape window.__guest__ from page, persist via SaveTokens). Durable file internal/cli/peekaboo_ext.go + `bootstrap` command wired in root.go.
- Absorbed: 9 generated endpoint resources (locations, categories, places[list/detail], branches, deals, cards, brands, amenities) — generated, verify in place.
- Novel (6, all hand-code):
  1. directions <entityId> --city — Maps directions URL per branch (live, GET branches)
  2. nearest <entityId> --near <city|lat,long> — closest branch (live, haversine)
  3. wallet <bank> --city --category — card->merchants (live fan-out places->deals)
  4. top-deals --city --category — rank by discount (live fan-out)
  5. expiring --city --category --within — deals closing soon (live fan-out)
  6. open-now --city --category — entities with openNow=true (live, simple)

## Phase 3 result
Manifest transcendence rows: 6 planned, 6 built. Phase 3 gate PASS.

### Built
- Auth: bootstrap command + auto-bootstrap in novel commands (scrape window.__guest__, persist via SaveTokens). Zero-config verified live.
- 9 absorbed endpoint commands (locations, categories, places list/detail, branches, deals, cards, brands, amenities) — all verified live.
- 6 novel commands (directions, nearest, wallet, top-deals, expiring, open-now) — all verified live.

### Fixes applied during Phase 3
- listCityEntities: use sortType=trending + lat/long + categoryId (site's shape) so deal-bearing merchants surface first (CLI fix).
- spec: associatedDeals changed bool->string default "true" so the generated `deals list` actually sends it (generated CLI would otherwise drop a defaulted bool and return 0 deals). Regenerated. (Printing Press issue: generator drops defaulted bool body params — note for retro.)
- Re-added bootstrap AddCommand in root.go after regen (custom command not in research.json novel_features, so lost-registration merge did not re-inject it — note for retro).

### Deferred / notes
- Generated `deals list` offset param emits as string; harmless.
- open-now uses entity.openNow field (simple), not branch-timings parsing — more robust than the planned timings approach.
