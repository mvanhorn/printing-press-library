# RideWithGPS CLI Build Log

Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

## Generation
- Generated from enriched official v1 OpenAPI (29 ops, 8 resources). All gates green.
- Spec enrichment: .json path fixes (users/current, auth_tokens); two-header auth (x-rwgps-api-key + x-rwgps-auth-token, uniform AND security); x-pp-resource clean grouping; x-display-name.
- Auth verified: both env vars in config.go, both headers conditional in client.go (login-safe).

## Phase 3 build (transcendence)

### Built (7/7 transcendence rows)
- stats (local): trips table aggregation by week/month/year/activity-type. NULL-safe.
- records (local): top-N best efforts (distance/elevation/speed/power/duration) from trips table.
- dedup (local): Union-Find clustering by distance + start/end haversine; --apply deletes extras.
- audit (local): stale/private/incomplete checks over routes. (Adjusted from cue-sheet/untagged — those need detail, not in summary mirror.)
- export (live): GPX/TCX/CSV synthesized from track_points; --native passthrough to legacy /{routes,trips}/{id}.{gpx,tcx,fit,kml}.
- gear (live): per-bike mileage via bounded fan-out over trip details (gear embedded in detail); --due-km flagging; partial-failure accounting.
- event-routes (live): legacy /events/{id}.json embedded routes; optional --export reusing export path.
- Shared helpers in rwgps_novel.go (haversine, GPX/TCX/CSV builders, detail unwrap, event-routes extract) + rwgps_novel_test.go.
- All hand-authored (no generated header) → regen-merge preserves. event-routes wired additively in root.go.
- Phase 3 gate: go test ok, go vet clean, dogfood novel_features_check planned=7 found=7.

## Phase 5 live dogfood + fixes (136/136)
Live full dogfood against the user's real account: 131→135→136 across 2 fix loops.
- tail: real bug — polled `/`+resource (web HTML) instead of `/api/v1/<resource>.json`; fixed via syncResourcePath() + resource validation. GENERATOR retro candidate.
- sync-json: added --since happy-args fixture.
- POI list/get: org-account-only (403); tier-gated (pp:requires-tier=org) + documented.
(4 generated-file hand-edits; reprint-review items — see acceptance report.)

## Phase 4.95 code review fix
- export/event-routes: path-traversal guard on --id (reject `/`,`\`,`..`).

## Phase 4.8/4.9 docs fix
- Removed fabricated `auth login` from narrative (real flow: auth-tokens + env vars). "RideWithGPS"→"Ride with GPS" prose.

## Phase 5.5 polish (by-slug, after promote)
- Polish working-dir-path mode malfunctioned (hallucinated unrelated "redfin"); worked around by promoting first then polishing by slug.
- Result: scorecard 92→93 (Grade A), verify 100%, publish-validate 11/11, verify-skill clean, gosec clean (hand-authored), output review PASS. ship.
- Retro flagged: 29 gosec findings + type-fidelity RawMessage shortfall, all in generated files → generator/template fixes.
