Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

# Build Log — lightroom-classic-pp-cli

## What was built
- internal/lrcat package (hand-authored, tested): read-only catalog access (mode=ro + PRAGMA query_only), path discovery (flag > LIGHTROOM_CLASSIC_CATALOG > newest under ~/Pictures), criteria search with APEX aperture/shutter conversion, listings (collections/keywords/cameras/lenses with first/last-seen), day coverage + streaks, pick-of-day ladder (pick=1 → rating → touchTime), on-this-day, project progress, stats histograms, keeper funnel, unedited backlog, catalog health sweep.
- All 8 transcendence commands implemented: streaks, pick-of-day, on-this-day, project, stats, funnel, backlog, doctor (health sweep wired into framework doctor).
- Absorbed surface: photos (promoted criteria search, alias find), collections, keywords, cameras, lenses (top-level, mirrored under hidden catalog group), path (id/filename → absolute path + exists check), --json/--select/--csv on everything, human EXIF units.
- root.go: --catalog persistent flag; sync/tail hidden (local-sqlite CLI has no API to sync from).
- Read-only enforcement covered by test TestReadOnlyEnforced (UPDATE fails at SQLite layer).

## Intentionally deferred / notes
- Generated HTTP client remains for framework probes but no user-facing command calls it (BaseURL empty).
- catalog_*.go generated endpoint files rewritten as thin wrappers over the same lrcat-backed implementations (local-sqlite CLI; the HTTP path was dead on arrival).
- Phase 3 per-row gate: 14/14 approved command paths resolve with correct Usage lines.

## Generator limitations found (retro candidates)
- generate emits HTTP-backed endpoint commands + sync/tail/search even for source: local-sqlite specs; a local-data mode that skips client/sync emission would remove hand-rewriting.
- doctor's health check assumes base_url; a spec hook for a local health probe would avoid editing generated doctor.go.
- novel feature named "doctor" collides with framework doctor (warning at generate time); the collision-merge had to be done by hand.
