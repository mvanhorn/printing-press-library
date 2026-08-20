# AllTrails Community And API Inventory

Checked: 2026-05-26

## Summary

AllTrails has no obvious public developer API or mature official CLI. The useful public evidence points to:

- official file export/import-adjacent surfaces,
- private web/mobile JSON endpoints used by the site,
- a known route-geometry endpoint: `GET /api/alltrails/v3/trails/{trail_id}?detail=offline`,
- strong bot-protection and ToS constraints around automated scraping.

The build should be read-first and authorization-first: browser-account capture for the user's own session, sanitized fixtures, dry-run planners, and explicit barriers before any account mutation.

## Community Findings

### `cdown/alltrailsgpx`

- Repo proof: `alltrails/proofs/gh-repo-view-alltrailsgpx-2026-05-26.json`
- README proof: `alltrails/proofs/gh-readme-alltrailsgpx-2026-05-26.md`
- Signal: converts saved AllTrails route API responses into GPX.
- Useful endpoint pattern: `https://www.alltrails.com/api/alltrails/v3/trails/{route_id}`.
- Limitation: requires the user to manually save a browser network response; it is not an authenticated sync client.

### `JoooostB/alltrails-to-gpx`

- Repo proof: `alltrails/proofs/gh-repo-view-alltrails-to-gpx-2026-05-26.json`
- Client proof: `alltrails/proofs/gh-file-alltrails-to-gpx-client-go-2026-05-26.txt`
- Design proof: `alltrails/proofs/gh-file-alltrails-to-gpx-technical-design-2026-05-26.md`
- Signal: confirms `GET /api/alltrails/v3/trails/{trail_id}?detail=offline`.
- Signal: numeric trail ID can be extracted from page HTML / embedded server state.
- Risk note: this repo explicitly uses Chrome TLS fingerprint impersonation to bypass DataDome. Do not copy that into this project without explicit legal/product approval; prefer real logged-in browser capture and sanctioned account usage.

### `dbrown540/alltrails-mcp-client`

- Repo proof: `alltrails/proofs/gh-repo-view-alltrails-mcp-client-2026-05-26.json`
- README proof: `alltrails/proofs/gh-readme-alltrails-mcp-client-2026-05-26.md`
- Signal: MCP-style AllTrails search/detail wrapper exists, but it is small and not a collision with a PP package.
- Risk note: README warns about rate limits/CAPTCHA.

### `srinath1510/alltrails-mcp-server`

- Repo proof: `alltrails/proofs/gh-repo-view-alltrails-mcp-server-2026-05-26.json`
- README proof: `alltrails/proofs/gh-readme-alltrails-mcp-server-2026-05-26.md`
- Signal: prior AllTrails MCP work was deprecated after a ToS objection.
- Current handling: record as policy precedent, not a hard blocker, because the account owner separately reported AllTrails approval for this CLI/API direction. The code still needs ToS docs and barriers.

### `Asherlc/dofek` provider note

- Proof: `alltrails/proofs/gh-file-dofek-alltrails-md-2026-05-26.md`
- Signal: official export workflows exist for activity/custom route/trail files, including GPX, FIT, TCX, GeoJSON, JSON, CSV, KML, and KMZ.
- Signal: AllTrails documents AI assistant integrations for trail search/detail/weather/map display, but that appears consumer-facing rather than a public developer API contract.

## Package Searches

- npm search for `alltrails` returned `[]`.
  - Proof: `alltrails/proofs/npm-search-alltrails-2026-05-26.json`
- PyPI checks for `alltrails` and `py-alltrails` found no matching distribution.
  - Proofs:
    - `alltrails/proofs/pypi-alltrails-versions-2026-05-26.txt`
    - `alltrails/proofs/pypi-py-alltrails-versions-2026-05-26.txt`
- GitHub repo search for `AllTrails API v3` returned `[]`; code search only found small/adjacent projects.
  - Proofs:
    - `alltrails/proofs/gh-repo-search-alltrails-api-v3-2026-05-26.json`
    - `alltrails/proofs/gh-code-search-api-alltrails-v3-2026-05-26.json`

## Site Policy And Access Signals

- `robots.txt` is accessible and disallows `/api/`, `/api-v4/`, `/api-v5/`, `/*/api/`, `/*/api-v4/`, `/*/api-v5/`, `/members/`, and `/explore/map/`.
  - Proof: `alltrails/proofs/alltrails-robots-2026-05-26.txt`
- Direct curl to public AllTrails pages and sitemap/support articles frequently returns 403, while browser access is expected to work.
  - Proofs:
    - `alltrails/proofs/alltrails-yosemite-public-2026-05-26.exit`
    - `alltrails/proofs/alltrails-yosemite-jina-2026-05-26.txt`
    - `alltrails/proofs/alltrails-sitemap-index-2026-05-26.exit`
    - `alltrails/proofs/alltrails-support-downloading-files-2026-05-26.exit`
- AllTrails terms page was reachable by direct curl.
  - Proof: `alltrails/proofs/alltrails-terms-2026-05-26.html`

## Initial Route Families

Verified seed:

- `GET /api/alltrails/v3/trails/{trail_id}?detail=offline` — trail detail / geometry payload; known useful for GPX conversion.

Browser-capture targets:

- public discovery: explore, trails, parks, regions, lists, search, maps;
- trail detail: trail page state, reviews, photos, weather, directions, nearby trails, conditions;
- authenticated account: `me`, profile, saved/favorites, completed trails, custom lists, activities/recordings, routes, downloads/offline maps;
- social: friends/followers, feed, comments, likes/upvotes, reviews;
- subscription-gated: GPX/export, offline maps, 3D preview, advanced weather/maps, Garmin/export-like handoffs;
- write-plan only: save/unsave, list edits, review/comment/photo upload, activity upload/import, profile edits, subscription/payment settings.

## Implementation Barriers

- Default all write/action commands to `--dry-run`.
- Require both a clear flag and env barrier for live writes, for example `--confirm` plus `ALLTRAILS_PP_ALLOW_WRITES=1`.
- Never persist raw cookies, Authorization headers, refresh tokens, request bodies, localStorage, or private IDs in proofs.
- Do not programmatically solve CAPTCHA or implement stealth/bot-bypass code.
- Rate limit browser reads and expose `--delay` / `--max-pages` on paginated capture.
- Keep route maps honest: label `official_export`, `browser_observed`, `community_inferred`, `subscription_gated`, `mutation_dry_run_only`, and `unverified`.

## Dedicated Browser Lane

The default Chrome CDP lane was repeatedly prompting for remote-debugging Allow. A dedicated Chrome profile was launched for AllTrails:

- profile: `<local-browser-profile>/alltrails-cdp`
- port: `9227`
- URL: `https://www.alltrails.com/login`

Use explicit CDP attachment to `127.0.0.1:9227` only. Do not use harness auto-detect for this slice.
