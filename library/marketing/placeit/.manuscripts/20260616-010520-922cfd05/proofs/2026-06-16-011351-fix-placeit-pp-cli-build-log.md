# Placeit CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 gate PASS.

## Architecture
- Catalog core (search/sync/template/facets/industries/open) hand-built on Algolia client (internal/algolia).
- Catalog cached in generic resources table (resource_type='template'); search live by default, --data-source local for offline.
- Auth endpoints (account/bookmarks/campaigns) generated from spec (cookie auth, browser-chrome/Surf transport — clears Cloudflare; verified live).

## Built (behavioral acceptance against real Algolia + 1000-row mirror)
- search: live Algolia, filters (category/type/tag/sort/free/printify), local fallback. VERIFIED.
- sync: mirrors catalog into SQLite (1000 mockups synced). VERIFIED.
- template/facets/industries/open: live catalog lookups. VERIFIED.
- top: ranks by real purchases (381/361/334). VERIFIED.
- pod: is_printify filter (returns real Printify mockups). VERIFIED.
- kit: matched streamer kit, 5/6 slots covered, flags missing emote. VERIFIED.
- industry-map: taxonomy join, Coffee Shop=142 templates. VERIFIED.
- watch: saved search + diff (first_run true->false). VERIFIED.
- gaps: device x color pivot, 175 gaps. VERIFIED.
- rank: purchase percentile within category/device cohort. VERIFIED.

## Notes
- is_printify is filterable but NOT an Algolia facet (facets --facet is_printify is empty by design).
- account returns anonymous "Visitor" without login; real subscription data with 'auth login --chrome'.

## Deferred
- Editor render/download: async browser-only; shipped as 'open' deep-link resolver (honest).
- Brand-kit (absorbed row 16): explicit stub — needs editor session, out of v1 scope.
