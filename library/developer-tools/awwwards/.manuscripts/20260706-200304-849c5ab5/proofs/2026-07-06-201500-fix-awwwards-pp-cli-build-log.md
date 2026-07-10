Manifest transcendence rows: 5 planned, 0 built. Phase 3 will not pass until all 5 ship.

# Awwwards CLI Build Log

## Scope notes
- All 5 transcendence rows are hand-code: trends, context-pack, palette-match, elements-top, studio
- 4 hand-built flagship absorbed commands: latest, find, top, inspect
- Foundation: internal/awwwards parser, internal/store/awwwards_migrations.go typed tables, `mirror` feeder command (typed-table sync; framework `sync` untouched for generic resources)
- Honest adjustment: `trends --by` supports tag|color|tech (NOT font) — per-site font attribution does not exist anywhere on awwwards.com (fonts are filter pages only; confirmed on detail pages). Font browsing remains via `websites browse <Font>` / `find --font` over mirrored filter pages. Surfaced to user in Phase 3 summary.

## Phase 3 outcome
Manifest transcendence rows: 5 planned, 5 built. Gate PASSED (per-row Cobra resolution + dogfood novel_features_check planned=found=5, verdict PASS, 0 issues).

Built:
- internal/awwwards: parser (cards, detail scores/jury/palette/credits/tags, nominee-vote fallback markup), color (hex parse, RGB distance, hue families), tech vocabulary. 11 test functions incl. real-markup fixtures.
- internal/store/awwwards_migrations.go: typed tables aw_sites/aw_site_tags/aw_palette/aw_jury/aw_credits/aw_elements + upsert helpers + detail queues.
- Hand commands: mirror (typed-table feeder: pages/details/elements, element-parent detail chasing), latest, find (multi-filter AND intersection), top, inspect (auto live/local), trends (tag|color|tech, --vs deltas), context-pack, palette-match, elements-top (jury-or-votes score fallback with score_source labeling), studio.
- All local commands: pp:data-source annotations, missing-mirror guard w/ empty-JSON fallback, drain-first SQLite, NULL-safe scans, dogfood curtailment in mirror/latest.

Fixes during build:
- FlexID for element UUID card ids
- Overall score = official rubric weighting (chart-bar data-note unreliable)
- Credits regex anchored to markup (CSS block false match)
- SQLite COLLATE precedence: `tag = ? COLLATE NOCASE OR ...` silently matched nothing; fixed to `tag COLLATE NOCASE IN (?, ?)`
- Nominee/HM vote markup variant parsers (name outside anchor, <i>from</i> country)
- Removed dead generated helper writeNoop (RETRO CANDIDATE: template emits it unconditionally; dead in no-auth CLIs)
- root.Short aligned to narrative.headline (drift check)

Honest adjustments (surfaced to user):
- trends --by: tag|color|tech, font axis dropped (no per-site font data exists on awwwards.com)
- elements-top: ranks by official jury score when final, else average of captured votes (nominees/mentions render votes only); labeled via score_source
