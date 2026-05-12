# WHOOP API Research Brief — whoop-pp-cli reprint

**Date:** 2026-05-11
**API:** WHOOP (developer.whoop.com) — wearable health tracker
**Base URL:** `https://api.prod.whoop.com/developer` (v2)
**Customer:** Non-programmer power-user, 2+ years on WHOOP, wants local analytics and trend/correlation queries via Claude Code.

---

## Spec Source

**Primary (recommended):** `https://raw.githubusercontent.com/pelo-tech/whoop-api-spec/master/whoop-api-spec.yaml` — unofficial OpenAPI 3.0 spec (`pelo-tech/whoop-api-spec`, 2.0.x). Servers it points at include `https://api.prod.whoop.com`. Note the spec is older v1-flavored; **must be adjusted to v2 paths** (`/developer/v2/...`) and UUID IDs.

**Authoritative reference (HTML, no downloadable YAML):** `https://developer.whoop.com/api` — official Redocly-rendered docs. WHOOP does NOT publish a downloadable OpenAPI file at any of the candidate URLs we probed (`/api/openapi.yaml`, `/api/openapi.json`, `/static/openapi.yaml`, `api.prod.whoop.com/openapi.json`, `api.apis.guru/v2/specs/whoop/`). All 404.

**Strategy:** use pelo-tech YAML as scaffolding, then patch endpoints/IDs from the authoritative HTML docs + v1→v2 migration page. Source-of-truth endpoints:

| Resource | v2 Endpoint | Auth scope |
|---|---|---|
| Cycle list | `GET /v2/cycle` | `read:cycles` |
| Cycle by id | `GET /v2/cycle/{cycleId}` | `read:cycles` |
| Sleep for cycle | `GET /v2/cycle/{cycleId}/sleep` | `read:sleep` |
| Recovery list | `GET /v2/recovery` | `read:recovery` |
| Recovery for cycle | `GET /v2/cycle/{cycleId}/recovery` | `read:recovery` |
| Sleep list | `GET /v2/activity/sleep` | `read:sleep` |
| Sleep by id | `GET /v2/activity/sleep/{sleepId}` (UUID) | `read:sleep` |
| Workout list | `GET /v2/activity/workout` | `read:workout` |
| Workout by id | `GET /v2/activity/workout/{workoutId}` (UUID) | `read:workout` |
| User profile | `GET /v1/user/profile/basic` | `read:profile` |
| Body measurement | `GET /v1/user/measurement/body` | `read:body_measurement` |
| Revoke OAuth | `DELETE /v1/user/access` | (auth-only) |
| V1→V2 id mapping | `GET /v1/activity-mapping/{v1Id}` | resource scope |

OAuth endpoints (not under `/developer`):
- Authorize: `https://api.prod.whoop.com/oauth/oauth2/auth`
- Token: `https://api.prod.whoop.com/oauth/oauth2/token`

---

## Pagination (critical)

- Every list endpoint enforces `limit ≤ 25`, **default `10`**.
- Cursor field on response: `next_token` (string).
- Cursor on request: query param `nextToken=<value>`.
- Stop condition: `next_token` empty/missing.
- Time filters: `start`, `end` (ISO-8601, UTC `Z`). `start` defaults to 24h ago, `end` defaults to now.
- Pages are sorted descending by `start` time.
- **Implication for sync:** must auto-page in client code with `for nextToken != ""`. Prior CLI's bug was passing `limit=50/100` directly → 400. Generated client must clamp `limit` to 25 and paginate internally.

---

## Authentication

**OAuth 2.0 Authorization Code with PKCE** (no static API key). Scopes are space-delimited.

Required scopes:
- `read:recovery`, `read:cycles`, `read:workout`, `read:sleep`, `read:profile`, `read:body_measurement`
- `offline` — required to receive a refresh token

Token endpoint returns `{ access_token, refresh_token, expires_in, scope, token_type }`. Refresh: `POST /oauth/oauth2/token` with `grant_type=refresh_token`.

**Auth approach recommendation: full OAuth2 PKCE flow with auto-refresh.**

User already has:
- WHOOP OAuth app registered (`WHOOP_CLIENT_ID`, `WHOOP_CLIENT_SECRET` in env)
- Redirect URI `http://localhost:8085/callback` pre-registered

Recommended CLI auth surface:
- `auth login --client-id ... --client-secret ... --port 8085` → browser-based PKCE flow; spins local HTTP listener; persists `{ access_token, refresh_token, expires_at, scopes }` to `~/.config/whoop-pp-cli/tokens.json` (XDG-aware on Linux, `%APPDATA%` on Windows).
- `auth status` → token expiry + scopes
- `auth refresh` → manual refresh
- `auth logout` → revoke + delete local tokens (calls `DELETE /v1/user/access`)
- Bearer fallback: `WHOOP_ACCESS_TOKEN` env var still honored for non-interactive contexts (CI, agents). Greg's prior CLI used `WHOOP_OAUTH` — keep both for back-compat but document `WHOOP_ACCESS_TOKEN` as canonical.

Every API call: auto-refresh if `expires_at - now < 60s`; on `401`, single refresh-and-retry; on second 401, exit code 4 with guidance.

---

## High-gravity response fields (from official docs + spec)

These are the fields competitors uniformly extract — they map to first-class SQLite columns, not JSON blobs.

**Cycle:** `id` (int), `user_id`, `start`, `end`, `timezone_offset`, `score_state`, `score.strain`, `score.kilojoule`, `score.average_heart_rate`, `score.max_heart_rate`.

**Recovery:** `cycle_id`, `sleep_id` (UUID v2), `user_id`, `created_at`, `updated_at`, `score_state`, `score.user_calibrating`, `score.recovery_score`, `score.resting_heart_rate`, `score.hrv_rmssd_milli`, `score.spo2_percentage`, `score.skin_temp_celsius`.

**Sleep:** `id` (UUID), `cycle_id`, `v1_id`, `user_id`, `start`, `end`, `timezone_offset`, `nap` (bool), `score_state`, `score.stage_summary.{total_in_bed_time_milli, total_awake_time_milli, total_light_sleep_time_milli, total_slow_wave_sleep_time_milli, total_rem_sleep_time_milli, sleep_cycle_count, disturbance_count}`, `score.sleep_needed.{baseline_milli, need_from_sleep_debt_milli, need_from_recent_strain_milli, need_from_recent_nap_milli}`, `score.respiratory_rate`, `score.sleep_performance_percentage`, `score.sleep_consistency_percentage`, `score.sleep_efficiency_percentage`.

**Workout:** `id` (UUID), `v1_id`, `user_id`, `start`, `end`, `timezone_offset`, `sport_id` (int), `sport_name` (string), `score_state`, `score.strain`, `score.average_heart_rate`, `score.max_heart_rate`, `score.kilojoule`, `score.percent_recorded`, `score.distance_meter`, `score.altitude_gain_meter`, `score.altitude_change_meter`, `score.zone_durations.{zone_zero_milli ... zone_five_milli}`.

**User profile:** `user_id`, `email`, `first_name`, `last_name`.
**Body measurement:** `height_meter`, `weight_kilogram`, `max_heart_rate`.

**Sport ID mapping** (partial, well-known): 0 Running, 1 Cycling, 16 Baseball, 17 Basketball, 18 Rowing, 24 Ice Hockey, 30 Soccer, 33 Swimming, 34 Tennis, 39 Boxing, 43 Pilates, 44 Yoga, 48 Functional Fitness, 52 Hiking, 63 Walking, 96 HIIT, -1 Activity/Unknown. Generate `data/sports.go` lookup table.

---

## Risk and reachability concerns

1. **No official spec download.** Mitigation: pelo-tech yaml + scaffold from docs; generate via `--docs+spec-hybrid` mode if the press supports it. Otherwise patch the yaml with v2 paths before feeding the press.
2. **`recovery.sleep_id` is UUID in v2 webhooks** but in REST responses still tied to a cycle. Treat `sleep_id` as the canonical join key for sleep↔recovery.
3. **`limit ≤ 25` enforced server-side.** Press-generated paginator MUST cap or 400 every list call (Greg's bug).
4. **OAuth redirect URI fixed.** Default to `localhost:8085`. Pass `--port` if user re-registers.
5. **Rate limits not publicly documented**; observed behavior: HTTP 429 on aggressive backfills. Implement exponential backoff with jitter; expose `--concurrency` (default 1) and `--rate-limit` knobs in `sync`.
6. **User profile uses `v1` path** even in v2 era — not consolidated. Keep `/v1/user/...` endpoints as-is.
7. **Partner endpoints (lab requisitions, diagnostic reports)** exist in Greg's prior CLI but are only available to "trusted WHOOP partners." Skip from default surface; expose behind `--include-partner` if archetype permits.

---

## Skipped gates

- **browser-sniff gate:** `skip-silent`, reason `documented-rest-api-no-browser-needed`. WHOOP has a public, complete Redoc-rendered API page covering all endpoints; no browser DevTools sniffing required.
- **crowd-sniff gate:** `skip-silent`, reason `spec-complete-or-redundant`. We have pelo-tech OpenAPI yaml + 7 SDK wrappers + 9 MCP servers covering the same endpoint surface. No further crowd discovery would surface new resources.

---

## Generation guidance for downstream phases

1. Feed pelo-tech yaml to the press but **patch base path** to `/developer/v2/...` and **swap long IDs for UUIDs** for `sleepId` and `workoutId`. Manual patch list:
   - `/v1/activity/sleep` → `/v2/activity/sleep`
   - `/v1/activity/workout` → `/v2/activity/workout`
   - `/v1/cycle` → `/v2/cycle`
   - `/v1/recovery` → `/v2/recovery`
   - Add `/v2/cycle/{cycleId}/sleep` and `/v2/cycle/{cycleId}/recovery`
   - Keep `/v1/user/profile/basic`, `/v1/user/measurement/body`, `/v1/user/access`
   - Keep `/v1/activity-mapping/{v1Id}` for migration tool
2. Generated `Client.Get*Collection()` MUST iterate `nextToken` and clamp `limit` to 25.
3. Domain archetype: **`personal-quantified-self`** (sibling of fitness/health). Generated commands should include: `sync` (all resources, incremental cursor by `created_at`), `search`, `sql`, `stats daily`, `stats weekly`, plus the 6 novel analytics (see absorb manifest).
4. SQLite schema: tables `cycles`, `sleeps`, `recoveries`, `workouts`, `user_profile`, `body_measurements`, `sync_cursors`. Indices on `start`, `end`, `cycle_id`, `sleep_id`, `sport_id`. FTS5 over `sport_name` + `score_state` notes.
5. MCP server (`whoop-pp-mcp`) tools should mirror commands 1:1; analytics commands (`recovery-trend`, `strain-vs-sleep-debt`, etc.) are the highest-value MCP tools.
