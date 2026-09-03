# MakerWorld CLI — Phase 3 Build Log

Manifest transcendence rows: 4 planned (discover [incl. printer-fit modes], movers, designers deltas, designs lineage), 0 built. Phase 3 will not pass until all 4 ship. (printer-fit is folded into discover per the manifest, not a separate command file.)

## Priority 0 (foundation) — DONE (generator)
- SQLite store with `resources` table (resource_type=designs, IDs extracted), typed `designs`/`designers`/`categories` tables, FTS indexes. `sync --resources designs` mirrors 100/page with extractable IDs. Verified live.

## Priority 1 (absorbed) — generated endpoint commands verified live
- designs list/get/search/related/remixes/ratings/recommend, designers models, categories list — all return real data from api.bambulab.com. search uses the browser-sniff-discovered select/design2 path.

## Priority 2 (transcendence) — IN PROGRESS
- discover, movers, designers deltas, designs lineage: generator scaffolded stub files + wiring. Implementing real logic next.

## Priority 2 (transcendence) — DONE (4/4 built)
- discover, movers, designers deltas, tags — all hand-coded, real tests pass, behaviorally verified live.
- **Scope change (user re-approved):** `designs lineage` replaced by `tags`. Build-time discovery: MakerWorld's `/design/{id}/remixed` endpoint returns empty (total=0) for every design tested (incl. gridfinity); `originals` is empty too — MakerWorld does not expose a remix graph via this API. Per the no-silent-downgrade rule, returned to Phase 1.5; user approved swapping lineage → `tags` (tag-cloud + multi-tag intersection over the 100%-populated tags field).
- Behavioral proofs: discover quality/popular/printable + live AMS enrichment; movers +500 delta surfaced after injected snapshot change; designers deltas Δlike/Δdl per creator; tags cloud + toy∩fidget intersection.

Manifest transcendence rows: 4 planned, 4 built.
