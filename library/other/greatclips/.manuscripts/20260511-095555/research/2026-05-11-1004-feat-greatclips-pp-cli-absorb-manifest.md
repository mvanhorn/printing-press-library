# GreatClips CLI Absorb Manifest

## Ecosystem audit
No existing CLI, MCP server, Claude skill, npm package, PyPI package, or
community wrapper found for GreatClips. The official Online Check-In
exists only as a Next.js SPA (`app.greatclips.com`) and a mobile app
(iOS/Android). No competitor catalog to match. **All shipping scope is
either table-stakes-from-the-website-itself or novel.**

## Table Stakes (mirror what the website does, but in a CLI)

| # | Feature | Source | Our Implementation | Added Value |
|---|---------|--------|--------------------|-------------|
| 1 | List salons near zip/city | greatclips.com web app | `salons near "98040"` | Pipeable, `--json`, distance-sorted |
| 2 | Show wait time for one salon | greatclips.com web app | `wait --salon 8991` | One-shot answer, no SPA load |
| 3 | List wait times across many salons | greatclips.com web app | `wait near "98040"` | JOINs salon meta with ICS wait in one row each |
| 4 | Show salon detail (address, phone, hours, cross-street) | greatclips.com web app | `salon 8991` | Includes "Next to Einstein" proximity, GPS coords |
| 5 | Show 14-day hours forecast incl. special hours | greatclips.com web app | `hours 8991` | Holiday detection; agent-friendly date filter |
| 6 | Read customer profile (name, phone, favorites) | greatclips.com web app | `profile` | `--json` for agents; surfaces favorite salonNumbers |
| 7 | Submit check-in (self + party 1-5) | greatclips.com web app | `checkin 8991 --party N` | Idempotent; refuses if already checked in; `--dry-run` |
| 8 | Show "where am I in line" for active check-in | greatclips.com web app | `status` | Polls every 30s with `--watch` |
| 9 | Cancel active check-in | greatclips.com web app | `cancel` | Confirms before submit |
| 10 | Resolve zip to lat/lng/city | greatclips.com web app | `geo "98040"` | Returns structured location info |
| 11 | Search salons by name/city term | greatclips.com web app | `salons search "Mercer"` | FTS over local SQLite |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|-------------------------|-------|
| T1 | Watch wait until threshold, then notify | `watch 8991 --until-under 10` | Requires daemon-style polling that the SPA can't run; emits OS notification or webhook | 10 |
| T2 | Auto-check-in when wait drops below threshold | `checkin 8991 --party 4 --when-under 15` | Combines poll + mutate; turns "I'll watch and click when it's short" into one command | 10 |
| T3 | Rank all nearby salons by current wait | `wait near "98040" --rank` | Web app shows distance OR wait, not joined-and-sorted; CLI sorts on `estimatedWaitMinutes` desc with `--reverse` to flip | 9 |
| T4 | Historical wait drift for a salon | `drift 8991 --hour-of-week` | Requires local timeseries from periodic `sync wait`; the API has no history endpoint. Answers "is Mercer Island always this slow at 5pm Tue?" | 9 |
| T5 | Family weekend planner | `plan --party 4 --days 7 --within 5mi` | Joins 14-day hours forecast + recent wait pattern + party size constraint; recommends the day+salon with the historical-shortest wait | 8 |
| T6 | "Wait now vs. typical" annotation | `wait near "98040" --vs-typical` | Compares current `estimatedWaitMinutes` to median for that hour-of-week from local snapshots; flags "shorter than usual" / "longer than usual" | 8 |
| T7 | Next-open salon | `next-open near "98040" --on 2026-05-25` | Reads upcoming-hours endpoint, filters out closed days/special hours, returns first opening time on that date | 7 |
| T8 | Family bookkeeping | `history` / `history --who "kid name"` | Tags each historical check-in with party member names from local config; "when did kid Y last get a haircut?" | 7 |
| T9 | Salon comparison snapshot | `compare 8991 2407` | Side-by-side wait + hours + distance; used to decide "Island Square or Coal Creek today?" | 7 |
| T10 | Bulk salons sync for an area | `sync --zip 98040 --radius 25` | Persists 50+ salons + hours forecasts locally so `salons` / `hours` work offline | 6 |
| T11 | Agent-shaped "what should I do" | `recommend --party 4` | One question -> structured answer: "Island Square in 11 minutes, drive 4 minutes, leave at 1:42 PM" | 8 |
| T12 | Check-in re-print / receipt | `status --copy` | Copies the position-in-line to clipboard or webhook for status pages | 5 |

## Stubs (none shipping)

Every transcendence feature listed above is shipping scope. No
"requires-paid-API" placeholders; no "future work" stubs. If any of T1-T12
cannot be implemented during Phase 3, return here for re-approval per the
shipping-scope rule.

## Data layer (mandatory before any command)

- `salons(salon_number PK, name, address1, address2, city, state, postal_code, country, latitude, longitude, phone, proximity, hours_display, hours_mon_open, hours_mon_close, hours_sat_open, hours_sat_close, hours_sun_open, hours_sun_close, marketing_name, status, last_synced_at)`
- `wait_snapshots(id, salon_number, captured_at, estimated_wait_minutes, state_code, state_name)` -- timeseries; FK to salons
- `salon_hours_forecast(id, salon_number, date, day_of_week, open_status, special_hours_reason, open_time, close_time, fetched_at)` -- 14-day rolling
- `check_ins(id, started_at, salon_number, party_size, first_name, last_name, phone, completed_at, cancelled_at, ended_status)` -- our local log of mutations we made
- `profile(id PK=1, first_name, last_name, email, phone, favorite_salon_numbers_csv, raw_json, fetched_at)`
- `kv(key PK, value)` -- for OAuth token cache, last-used salon, etc.

FTS5 over `salons(name, city, marketing_name, proximity, address1)` so
`salons search "QFC"` works (the proximity field is a goldmine the SPA hides).

## Auth strategy

- `auth login --chrome` (default): extract Auth0 token from the user's
  logged-in Chrome profile (the way the user just used Claude-in-Chrome).
  This is the *recommended* path because GreatClips's Auth0 tenant uses
  PKCE + same-site cookies that resist headless OAuth.
- `auth login --token <jwt>`: paste an existing Bearer token (for power
  users who already have one).
- `auth status`, `auth logout`.
- No password capture, no credential storage besides the JWT in
  XDG_CONFIG_HOME with chmod 600.

## Two-host client

The single biggest architectural decision: emit two client backends
(`webservicesClient` for `webservices.greatclips.com`, `stylewareClient`
for `www.stylewaretouch.net`), both authenticated with the same Bearer
token. Endpoint registration tags each operation with its host. The user
sees one CLI; under the hood, half the commands route to a different
service.
