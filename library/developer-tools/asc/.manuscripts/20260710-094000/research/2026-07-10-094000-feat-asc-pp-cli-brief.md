# App Store Connect CLI Brief

## API Identity
- Domain: Apple App Store Connect API v4.4 (`api.appstoreconnect.apple.com`), read slice.
- Users: an indie/solo dev shipping MANY apps (6+ under one or more teams) who lives in the ship + measure loop.
- Data profile: apps, TestFlight builds, App Store versions + review state, customer reviews, beta groups/testers, sales/downloads.

## Reachability Risk
- None. `GET /v1/apps` unauth → 401 (reachable, auth required as expected). Official Apple API, official OpenAPI spec.

## Top Workflows
1. "Across all my apps, what needs my attention right now?" — review state, failed builds, metadata rejections.
2. "Which app is getting traction?" — downloads + rating movement, ranked.
3. "What's in flight?" — every build/submission and how long it's been stuck in review.
4. "What are people saying?" — newest written reviews across the fleet.
5. Per-app drill: builds, versions, reviews, testers.

## Table Stakes (from existing tools — all single-app / CI-focused)
- list apps / show app (spaceship, ittybittyapps, kyledecot, rorkai, pofky-mcp)
- list builds + processingState (fastlane pilot)
- list versions + appStoreState / review status (spaceship get_live/edit_version; pofky review_status)
- list customer reviews w/ rating/territory/sort (pofky list_reviews)
- list beta groups / testers (fastlane, ittybittyapps)
- download sales report (appstoreconnect PyPI, rorkai)

## Data Layer
- Primary entities: apps, builds, appStoreVersions, customerReviews, betaGroups, betaTesters, salesRows.
- Cache: light SQLite for the rate-limited async/gzip reports (sales). Reviews/builds/versions live-fetch or cache-refresh.
- Rate limit: ~3600/hr (header `x-rate-limit`), undocumented ~300-350/min → 429. AdaptiveLimiter required.

## Auth
- JWT ES256. Header `{alg:ES256,kid:<keyId>,typ:JWT}`, payload `{iss:<issuerId>,iat,exp≤1200s,aud:"appstoreconnect-v1"}`.
- Env: `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_PRIVATE_KEY_PATH` (matches pofky/asc-mcp convention). `.p8` file, per-team.

## Gotchas
- **salesReports**: ALL filters required — `frequency`(DAILY/WEEKLY/MONTHLY/YEARLY), `reportType`(SALES/INSTALLS…), `reportSubType`(SUMMARY…), `vendorNumber`, `version`, `reportDate`. Returns **gzip TSV**, `Accept: application/a-gzip`, empty→404. Needs `ASC_VENDOR_NUMBER`.
- customerReviews: cursor pagination (limit≤200), `filter[territory]`, sort createdDate/rating.
- appStoreState enum: PREPARE_FOR_SUBMISSION, WAITING_FOR_REVIEW, IN_REVIEW, PENDING_APPLE_RELEASE, READY_FOR_SALE, REJECTED, METADATA_REJECTED, DEVELOPER_REJECTED, INVALID_BINARY, … — drive the action-needed flags.
- builds.processingState: PROCESSING/VALID/INVALID/FAILED.

## User Vision (Justin)
Cross-app cockpit as the measurement layer under "which of my apps deserves real users." Read-only, all-apps-at-once, one glance. Multi-team: v1 = one team via env; profile switch is a fast follow.

## Product Thesis
- Name: `asc` — the App Store Connect cockpit.
- Why it should exist: every existing ASC tool is single-app and CI-shaped. None gives a solo multi-app dev a fleet-wide, at-a-glance read of review state, builds, downloads, and ratings. The differentiator is the cross-app rollup, not the endpoint wrap.

## Build Priorities
1. Data layer + generated read commands (apps/builds/versions/reviews/beta) + JWT auth.
2. Absorbed table-stakes read commands with `--json`/`--select`.
3. Transcendence: `cockpit`, `pipeline` (SLA aging), `traction`, `reviews recent`, `blockers`.
