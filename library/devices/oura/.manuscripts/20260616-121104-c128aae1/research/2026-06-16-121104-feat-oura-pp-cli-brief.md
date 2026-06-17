# Oura Ring CLI Brief

## API Identity
- **Domain:** Personal health & wellness — wearable ring that tracks sleep, activity, readiness, heart rate, SpO2, stress, and more
- **Users:** Health-conscious individuals, biohackers, athletes, researchers, quantified-self practitioners; Oura has 5M+ users
- **Data profile:** Time-series health metrics synced nightly from the ring to Oura's cloud; data is read-mostly with rich scoring, contributor breakdowns, and raw sensor streams (HR beat-to-beat, sleep stage timelines)

## Reachability Risk
- **Low** — Official REST API, professionally maintained, no reports of blocking or 403 issues in community repos
- **Auth constraint (HIGH):** Personal Access Tokens deprecated December 2025. Auth is now OAuth2 only. Users must register an OAuth application at `https://cloud.ouraring.com/oauth/applications` to get a `client_id` + `client_secret`. CLI needs loopback OAuth flow (`http://localhost:PORT/callback`).
- **Sandbox available:** `https://api.ouraring.com/v2/sandbox/usercollection/*` — deterministic sample data without a real account. Excellent for testing.
- Probe-safe endpoint: `GET /v2/usercollection/personal_info` (requires auth; sandbox version available)

## Top Workflows
1. **Morning review** — Check sleep score, readiness score, and activity status before planning the day
2. **Sleep analysis** — Examine last night's sleep stages, HRV, respiratory rate, and latency vs personal averages
3. **Training load** — Review recent activity scores, calories, VO2 max trend, and cardiovascular age to inform workout intensity
4. **Trend spotting** — Compare sleep/readiness/stress over rolling windows to identify patterns (after travel, illness, high-stress weeks)
5. **Journal correlation** — Correlate enhanced tags (subjective notes) with objective scores to find what habits affect performance

## Table Stakes (from competing tools)
- `arzzen/oura` (bash): 37 API endpoints, interactive selector, CSV export, colorized output, token management
- `visionik/ouracli` (Python): All 11 data types, `--json`, `--markdown`, `--html`, `--dataframe`, `--ai-help` flag, flexible date parsing (`today`, `7 days`, `2 weeks`)
- `daveremy/oura-mcp` (MCP+CLI): 10 data types, daily summary aggregator, trends command, skills interface
- `hedgertronic/oura-ring` (Python lib): 12 endpoint methods, pandas DataFrame output, date range params
- MCP servers (10+ implementations): all expose sleep/readiness/activity/HR/stress/SpO2/workouts/sessions as discrete tools

**Minimum parity set:** every data type queryable, date range filtering, `--json` + `--select`, at least `today` / `yesterday` / `N days` date shortcuts.

## Data Layer
- **Primary entities:** daily_sleep, daily_readiness, daily_activity, daily_stress, daily_spo2, daily_cardiovascular_age, daily_resilience, vo2_max, sleep (detailed sessions), workouts, sessions, heartrate (time-series), enhanced_tags
- **Sync cursor:** `start_date`/`end_date` per endpoint; heartrate uses `start_datetime`/`end_datetime`
- **FTS/search:** Tag text search, activity type filter, date range filtering — all excellent for SQLite FTS5
- **High-gravity entities:** daily_sleep + daily_readiness (used in 90% of queries); heartrate is large but valuable for trend analysis
- **Volume concern:** heartrate can return thousands of samples per day; sync should batch by day and compress raw samples in store

## Codebase Intelligence
- Source: hedgertronic/oura-ring, Pinta365/oura_api, python-ouraring, arzzen/oura
- **Auth:** `Authorization: Bearer <token>` (Bearer scheme); canonical env var will be `OURA_ACCESS_TOKEN` (stored after OAuth flow) plus `OURA_CLIENT_ID` / `OURA_CLIENT_SECRET` for the OAuth app credentials
- **Data model:** Every endpoint returns `{ data: [...], next_token: "..." }` for lists, or a single document for single-doc endpoints. Pagination via `next_token` query param.
- **Rate limiting:** Not officially published. Community practice is to avoid rapid-fire requests; sync in batches with small delays.
- **Architecture:** Pure REST API. All resources are GET-only (no mutations). Tags have POST/PUT/DELETE. Webhooks are the only push mechanism.

## Auth Strategy
The CLI implements OAuth2 Authorization Code flow:
1. User runs `auth setup --client-id CLIENT_ID --client-secret CLIENT_SECRET` (one-time, stored in config)
2. User runs `auth login` → CLI starts local HTTP server on random port, opens browser to `https://cloud.ouraring.com/oauth/authorize?client_id=...&redirect_uri=http://localhost:PORT/callback&response_type=code&scope=...&state=...`
3. User approves in browser → callback fires → CLI exchanges code for access_token + refresh_token
4. `auth refresh` / auto-refresh when token nears expiry (access tokens valid 30 days per docs)

## Product Thesis
- **Name:** oura-pp-cli
- **Display name:** Oura Ring
- **Why it should exist:** Every existing Oura tool is a thin query wrapper — raw API round-trips with no local state. We sync everything to SQLite, enabling time-travel queries, offline analysis, cross-metric joins no API query can do, and local trend computation that doesn't burn API calls on every dashboard open.

## Build Priorities
1. OAuth2 auth flow (auth setup + auth login + auto-refresh) — blocking for everything else
2. Full sync to local SQLite (all 15+ data types, incremental)
3. Daily dashboard — sleep + readiness + activity + stress in one command
4. Deep sleep analysis — stage breakdown, HRV trend, compare to personal rolling average
5. Cross-metric correlation — tags vs scores, travel vs readiness, workout intensity vs next-day readiness
6. Trend commands — rolling averages, weekly/monthly comparisons, personal bests
7. Anomaly detection — flag days where readiness or sleep score is unusually low, surface likely contributors
