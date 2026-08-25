# Wahoo Cloud API CLI Brief

## API Identity
- **Domain:** Endurance-training data platform. Wahoo Fitness (ELEMNT GPS bike computers — BOLT/ROAM/ACE — KICKR trainers, TICKR/TRACKR sensors). The Cloud API is the sync backend behind the phone app; rides recorded on the ELEMNT upload via WiFi to the cloud, then fan out to Strava/TrainingPeaks/etc.
- **Server:** `https://api.wahooligan.com` (v1). Docs: https://cloud-api.wahooligan.com/. Spec source: `api-evangelist/wahoo` (OpenAPI 3.0.3, 28 ops, server + OAuth URLs verified against official).
- **Users:** Cyclists, triathletes, runners, and their coaches. The person who said "only an app" is the typical case — a rider who owns the device, has years of rides in Wahoo's cloud, and wants a scriptable / backup-able / analyzable interface the app doesn't give them.
- **Data profile:** Workouts (with embedded summaries: distance, duration, power, HR, cadence, work/kJ, ascent), GPS routes (geo + elevation), structured plans, cycling power zones (FTP + 7 zones), user profile. Time-series of training accumulating indefinitely.

## Reachability Risk
- **None (transport).** Stable documented cloud REST API. Confirmed live 2026-06-14: `GET /v1/user` → `401 {"error":"Invalid access token"}` (reachable, auth-required as expected); `GET /oauth/authorize` → `302` (OAuth endpoints live, match spec).
- **HIGH (access friction).** Auth is OAuth 2.0 Authorization Code + PKCE, and **Wahoo gates API use behind manual app approval** (register at developers.wahooligan.com/applications/new; pending → approved). The user has **no approved app / client_id yet**, so live smoke-testing (Phase 5) will be skipped — the CLI is built + verified against mocks, and ships a ready-to-use OAuth2 PKCE login for when an app is approved.
- Probe-safe endpoint used: `GET /v1/user` (read-only, returned 401).

## Auth
- **Type:** OAuth2 Authorization Code + PKCE. Bearer access tokens, **2h TTL (7200s)**, **single-use refresh rotation**, **10 unrevoked tokens/user cap from 2026-01-01**.
- `authorizationUrl: https://api.wahooligan.com/oauth/authorize`, `tokenUrl: https://api.wahooligan.com/oauth/token`, `refreshUrl: same`.
- **Scopes (granular):** email, user_read/write, workouts_read/write, offline_data (webhooks), plans_read/write, power_zones_read/write, routes_read/write.
- Generated CLI needs: `auth login` (browser PKCE flow), token store, auto-refresh (single-use rotation handling), `auth status`, and `permissions revoke` (maps to `DELETE /v1/permissions`).
- Env vars the CLI will read: `WAHOO_CLIENT_ID`, `WAHOO_CLIENT_SECRET` (confidential apps), `WAHOO_REDIRECT_URI`, plus a stored token.

## Rate Limits (fixed-window, 429)
- **Sandbox:** 25 / 5min, 100 / hr, **250 / day**. **Production:** 200 / 5min, 1000 / hr, 5000 / day.
- Excluded from limits: `/oauth/*`, refresh requests, **workout file downloads**.
- Implication: sync must be paced (adaptive limiter, surface 429 as typed error) — sandbox 250/day is tight for a big back-catalog sync. FIT downloads being free is a gift for the archive feature.

## Top Workflows
1. **Back up / archive my rides** — pull every workout + its FIT file locally. (Directly addresses the real-world "broken sync" pain that drove ELEMNT reverse-engineering.)
2. **Analyze my training** — volume trends, training load (Fitness/Fatigue/Form), FTP progression, PRs — none of which the API computes for you.
3. **Push routes & plans to the ELEMNT** — upload a GPS route (turn-by-turn nav) or a structured plan to the device.
4. **Query/filter my data offline** — find routes by distance/elevation/location, list recent workouts, scriptable for dashboards.
5. **Manage profile / power zones / app permissions.**

## Table Stakes (full CRUD surface — absorb all 28 ops)
- user get/update; workouts list/get/create/update/delete; workout_summary get; FIT upload + poll; plans list/get/create/update/delete + list-for-workout; routes list/get/create/update/delete; power_zones list/get/create/update/delete; permissions revoke.
- Pagination: `page`/`per_page` (max 100). Workouts list returns rich envelope (total/order/sort); plans/routes return bare arrays.

## Data Layer
- **Primary entities (local SQLite):** workouts (+ embedded workout_summary), routes, plans, power_zones, user.
- **High-gravity entity:** workouts — they accrue forever and carry the metrics every analysis feature needs.
- **Sync cursor:** `updated_at` / `starts` on workouts; paginate `/v1/workouts`.
- **FTS/search:** workout name, route name/description, plan name/description.
- **Data-quality gotcha:** WorkoutSummary metrics (`distance_accum`, `power_bike_avg`, `work_accum`, `duration_*_accum`, etc.) are **string-typed** in the API. Every analysis feature must parse strings → numbers (NULL-safe) before math. This is the #1 correctness trap.

## Codebase Intelligence
- Reference Go wrapper exists: `james-millner/go-wahoo-cloud-api` (evaluates the API, stores FIT files when cycling) — confirms the archive workflow is the real-world draw and the OAuth flow is implementable in Go.
- Webhooks (AsyncAPI): `workout_summary` event, requires `offline_data` scope, retry at 30m/4h/24h/72h. A CLI can't passively receive webhooks without a public endpoint; treat as a possible local-dev receiver/verify helper (low priority), not core.

## User Vision
- Captured from briefing: user owns an **ELEMNT BOLT**, perceives "only an app." Chose the Cloud API path (data/automation) over direct BLE. Wants the app-equivalent surface they can script.

## Product Thesis
- **Name:** `wahoo` (binary `wahoo-pp-cli`).
- **Thesis:** Every Wahoo Cloud API capability, plus the local SQLite training database and offline analysis Wahoo's app never gives you — back up every ride and FIT file, compute Fitness/Fatigue/Form and FTP progression, and push routes/plans to your ELEMNT, all scriptable and agent-native.
- **Why install over the app/incumbent:** The app shows you one ride at a time and can't export or analyze. Strava/TrainingPeaks lock analysis behind paid tiers and don't back up your raw FIT files. This CLI owns your data locally and does the math.

## Build Priorities
1. **P0 foundation:** OAuth2 PKCE auth (login/status/refresh), SQLite store for all entities, `sync` (paced for rate limits), `search`, `sql`, `doctor`.
2. **P1 absorb:** all 28 endpoints as typed commands (user, workouts, workout_summary, uploads, plans, routes, power_zones, permissions), with `--json/--select/--dry-run`, multipart FIT/route upload handling.
3. **P2 transcend:** FIT-file archive, training-load (CTL/ATL/TSB), volume trends, FTP progression, personal bests, recent-window digest, offline route finder. (Final set decided by the Phase 1.5 novel-features subagent + gate.)
4. **P3 polish:** flag descriptions, README/SKILL, tests for analysis logic (string-metric parsing, load math).
