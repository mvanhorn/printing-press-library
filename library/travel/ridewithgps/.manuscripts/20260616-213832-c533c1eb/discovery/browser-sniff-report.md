# RideWithGPS Browser-Sniff Discovery Report

**Run:** 20260616-213832-c533c1eb
**Mode:** authenticated enrichment (user logged in via fresh headed browser-use window)
**Goal:** confirm legacy endpoints absent from the official v1 OpenAPI spec (gear, file downloads)
**Backend:** browser-use CLI (headed), page-context authenticated probes
**PII note:** real route/trip IDs and privacy_code tokens observed in-session are NOT recorded here; only endpoint shapes.

## Confirmed legacy / web-route endpoints

All probed from the authenticated page context (cookie session). Status + content-type observed live:

| Method | Path | Status | Content-Type | Notes |
|--------|------|--------|--------------|-------|
| GET | `/routes/{id}.gpx` | 200 | `application/gpx+xml` | GPX with cues (course) |
| GET | `/routes/{id}.gpx?sub_format=track` | 200 | `application/gpx+xml` | GPX track variant (no cues) |
| GET | `/routes/{id}.tcx` | 200 | `application/tcx+xml` | TCX |
| GET | `/routes/{id}.fit` | 200 | `application/vnd.ant.fit` | binary FIT |
| GET | `/routes/{id}.kml` | 200 | `application/vnd.google-earth.kml+xml` | KML |
| GET | `/trips/{id}.{gpx,tcx,fit,kml}` | (by symmetry) | matching | trip exports; pyrwgps confirms `/trips/{id}.{gpx,tcx,kml}` |
| GET | `/gear.json` | 401 | `application/json` | legacy gear list; exists (returns permission error, not 404); needs `apikey`+`auth_token`, NOT cookie |

## Auth observations

- `GET /api/v1/users/current.json` with cookie session → **401** `{"errors":["Failed to authenticate the request"]}`.
  Confirms the v1 API does **not** accept browser cookies — it requires `x-rwgps-api-key` + `x-rwgps-auth-token` headers (or OAuth bearer).
- The legacy file-export routes (`/routes/{id}.gpx` etc.) **do** accept the browser cookie session. Per pyrwgps source, they also accept `?apikey=&auth_token=` query auth. The printed CLI will use its configured `apikey`+`auth_token`; this is verified live in Phase 5.
- `/gear.json` returned a permission error (not 404) under cookie auth → endpoint is real but expects `apikey`+`auth_token`.

## Replayability verdict

PASS — all discovered surfaces are plain replayable HTTP GETs (no resident-browser execution required). The printed CLI replays them via direct HTTP with the configured credentials. No clearance cookie / WAF; ridewithgps.com served everything cleanly.

## Generation impact

- **Primary spec source:** official v1 OpenAPI (`https://ridewithgps.com/api/v1/openapi.yaml`). These legacy endpoints are **not** merged into the generated typed surface.
- **Export (`export` transcendence command):** primary path synthesizes GPX/TCX/CSV from v1 `track_points`/`course_points` (header-auth, fully documented). Optional `--native <gpx|tcx|fit|kml>` passthrough fetches the legacy file-export route above for RWGPS's own formatting (cues).
- **Gear (`gear` transcendence command):** derive gear list + per-bike mileage from v1 Trip `gear` field aggregation (header-auth, robust). Legacy `/gear.json` noted as a possible direct-listing source but not the primary path.
