# Withings CLI Brief

## API Identity
- **Domain:** Personal connected-health data (Withings devices: scales, ScanWatch, Sleep mat, BPM, Thermo). Weight/body-composition, activity/steps, sleep, heart rate, ECG/AFib, BP, SpO2, temperature, workouts, goals, devices.
- **Users:** Quantified-self users, athletes, patients sharing data with clinicians, home-automation/dashboard builders, and (new wave) people piping health data into LLMs for trend Q&A.
- **Data profile:** Time-series measurements keyed by user + device. Mantissa/exponent encoded values. Two date idioms (epoch ints vs `YYYY-MM-DD`). Incremental sync via `lastupdate` cursor. Webhooks (Notify) preferred over polling.

## Source Priority (combo CLI — confirmed)
- **Primary:** `withings-official-oauth2` — `wbsapi.withings.net`, OAuth2 bearer, **durable**. Covers all core data.
- **Secondary:** `withings-web-scalews` — `scalews.withings.com/cgi-bin`, cookie (`session_token`) import, **fragile**, web-only endpoints (`aggregate`, `plan`, `target`, `feature`, `timeline` feed, `subcategory`).
- **Economics:** both free. Primary uses OAuth2 only; secondary commands gated on a Chrome cookie import and clearly labeled fragile.
- **Inversion risk:** secondary is undocumented; must NOT overshadow the primary. Secondary contract requires a DevTools HAR to recover exact actions/params (sniff hit `503 Invalid Params` on guesses; SPA in-memory caching defeated live body capture).

## Reachability Risk
- **Primary (official API): LOW.** Documented, 120 req/min, every maintained tool uses it, no captcha/Cloudflare on data endpoints. Sharp edges: single-use rotating refresh tokens, 3h access tokens, ~30s auth codes, occasional 503 on token endpoint, newer `nonce`+`signature` model for `requesttoken`.
- **Secondary (web scalews): MEDIUM/HIGH.** Cookie session authenticates (sniff confirmed — got param errors, not auth errors) but undocumented, short-lived session, actively security-hardened (Spaceraccoon IDOR patched in days), ToS-adjacent.
- Probe-safe endpoint: official `POST /v2/user action=getdevice` (read-only, bearer).

## Top Workflows
1. **Sync weight/body-composition history** → CSV/JSON/local DB (export the built-in app can't automate well).
2. **Push to other platforms** (Garmin/TrainerRoad/Strava/Apple Health) — withings-sync's whole reason to exist.
3. **Trend analysis** — weight/fat trend, sleep debt, HR/HRV over time, BP logs for a doctor.
4. **ECG/AFib review** — list recordings, pull raw signal, AFib classification.
5. **LLM-assisted health Q&A** — pull metrics into an agent (the MCP-server wave).

## Table Stakes (must match)
- OAuth2 login with **persisted auto-rotating refresh token** (withings-sync, aiowithings).
- Incremental `--since`/`lastupdate` sync.
- Per-measure-type selection via type codes (1=weight, 6=fat ratio, 9/10=BP, 11=HR, 54=SpO2, …).
- CSV + JSON export (FIT is a stretch goal).
- Sub-commands per service: measure / activity / sleep / heart / workouts / devices / goals / notify.
- Graceful 401→refresh and 503/429 backoff.
- Mantissa/exponent scaling to real floats.

## Data Layer
- **Primary entities:** measure_groups (+ measures), activities (daily), workouts, sleep_summaries, sleep_series, heart_records (ECG/BP), devices, goals.
- **Sync cursor:** `lastupdate` per resource (epoch). Store rotated refresh token securely.
- **FTS/search:** across measurement types, dates, devices, workout categories.

## Codebase Intelligence
- Official API is action-based form-POST RPC on `wbsapi.withings.net`; `{status, body}` envelope; HTTP always 200, logical status in `status` int (0 ok, 401, 503, 601 rate-limit, 2554 not-implemented).
- Auth: `Authorization: Bearer <access_token>` on all data calls; `requesttoken` uses `client_id`+`grant_type`+(`nonce`+`signature` | legacy `client_secret`).
- Reference wrappers: jaroslawhartman/withings-sync (Py, ~670★, the reference CLI), vangorra/python_withings_api (~110★), joostlek/aiowithings (powers Home Assistant), tgrangeray/withings-go (Go), akutishevsky/withings-mcp + Schimmilab/withings-mcp-server (MCP).

## User Vision
- User explicitly wants **both** tiers: durable official OAuth2 core + the web-only extras (goals/targets/plans/timeline feed) as a clearly-labeled fragile cookie-import tier.

## Product Thesis
- **Name:** `withings-pp-cli` (binary), slug `withings`.
- **Why it should exist:** Every maintained Withings tool is either a single-purpose sync bridge (withings-sync → Garmin) or a language SDK. None is an **offline-first, agent-native CLI** with a local SQLite mirror, `--json`/`--select`/`--csv`, typed exit codes, incremental sync, trend/ECG/sleep analysis commands, AND the web-only timeline/goals extras. This is the GOAT Withings CLI: it absorbs the SDKs' coverage, matches withings-sync's export UX, and transcends with local-join analytics no single API call provides.

## Build Priorities
1. **P0 foundation:** SQLite data layer for all entities; OAuth2 login + auto-rotating refresh token; `sync` (incremental via lastupdate); `search`/`sql`.
2. **P1 absorb:** measure/activity/sleep/heart/workouts/devices/goals/notify commands with `--json`/`--select`/`--csv`/`--dry-run`; CSV export; type-code selection; mantissa scaling.
3. **P2 transcend:** trend analytics, sleep-debt, body-comp deltas, BP/HR doctor report, ECG/AFib review, "since" digest — all local-join over SQLite.
4. **P3 web tier:** scalews cookie-import secondary commands (aggregate/plan/target/feature/timeline) — needs HAR to finalize contract.
