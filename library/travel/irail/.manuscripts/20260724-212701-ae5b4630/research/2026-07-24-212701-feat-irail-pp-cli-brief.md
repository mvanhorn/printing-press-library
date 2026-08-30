# iRail (Belgian Rail) CLI Brief

Run: `20260724-212701-ae5b4630` · Date: 2026-07-24 · Source resolved: **iRail open API** (`https://api.irail.be`)

## API Identity
- **Domain:** Belgian passenger rail (NMBS/SNCB) — live timetables, journey planning, delays, disruptions.
- **Users:** Belgian commuters and cross-border travellers; developers of transit apps/dashboards; agents answering "when is my next train / is it delayed / can I still make the transfer".
- **Data profile:** Small entity count, very high churn. ~716 stations (579 Belgian) are near-static reference data; departures/connections/disturbances are real-time and change by the minute. Responses are 6–120 KB JSON.
- **Auth:** **None.** All six read endpoints returned 200 without credentials (verified 2026-07-24). Officially published public API.
- **Base/paths:** `https://api.irail.be`. Legacy `/stations/` 303-redirects to `/v1/stations`. **Use `/v1/...` directly** to avoid a redirect on every call. TLS 1.1+ required.

## Reachability Risk
- **LOW.** `iRail/iRail` is actively maintained (134★, pushed 2026-06-07, not archived, 42 open issues). Zero open issues matching "blocked"; zero matching "rate limit". All six live probes returned 200 with real data.
- No tier/permission gating: the API is free and unmetered beyond rate limits, so there is no 4xx tier-hint body to quote.
- Probe-safe endpoint used: `GET /v1/stations` (no `x-pp-safe-probe` markers needed; all probes were read-only GETs).
- **Contrast:** `www.belgiantrain.be` (the URL originally supplied) returns **HTTP 403 behind a Cloudflare captcha** on every direct request — title `NMBS: 500 cloudflare captcha error`. Rejected as a source in Phase 0 in favour of iRail.

### Hard operational constraints (from docs, verbatim)
- **Rate limit: 3 requests/second per source IP, plus 5 burst** (so 8 in 1s or 15 in 3s). Exceeding returns **429**.
- **User-Agent:** format `<app>/<version> (<website>; <mail>)`. With a UA set, abuse gets an email first. **With no UA set, "the source IP address will be blocked without prior warnings."** The generated CLI must always send a UA.
- **Conditional GET supported:** send `If-None-Match`, receive `Etag` + `Cache-Control`, get **304** when unchanged.

## Documented-vs-reality drift (important)
The docs (`docs.irail.be`, aglio/API-Blueprint, **generated 16 Feb 2020**) declare `version 1.1`. The live API returns **`version 1.4`**. Six years of drift, and the live responses carry fields the docs never mention:

| Field (live, undocumented) | Meaning | Why it matters |
|---|---|---|
| `platforminfo.normal` | `"0"` = platform differs from the usual one | **Platform-change detection** — a stated user requirement |
| `canceled` | `"0"`/`"1"` on departures, arrivals, and per-stop | Cancellation detection |
| `left` / `arrived` | train already departed/arrived | Filter out stale board rows |
| `isExtra` / `isExtraStop` | unscheduled extra train/stop | Distinguishes added service |
| `occupancy{@id,name}` | `low`/`medium`/`high`/`unknown` | Crowding signal |
| `departureConnection` | stable departure URI | Durable join key across days |

Never author the spec from the docs alone; response shapes above were captured live.

## Endpoint surface (verified live 2026-07-24)
| Endpoint | Required | Optional | Notes |
|---|---|---|---|
| `GET /v1/stations` | — | `format`, `lang` | 716 stations, 122 KB |
| `GET /v1/liveboard` | `station` **or** `id` | `arrdep`(departure\|arrival), `alerts`, `time`(hhmm), `date`(ddmmyy), `format`, `lang` | Never pass name+id together |
| `GET /v1/connections` | `from`, `to` | `timesel`(departure\|arrival), `typeOfTransport`(automatic\|trains\|nointernationaltrains\|all), `results`*, `alerts`*, `time`, `date`, `format`, `lang` | *deprecated; alerts always included |
| `GET /v1/vehicle` | `id` | `date`, `alerts`, `format`, `lang` | **`date` is ignored — upstream bug, reported twice** |
| `GET /v1/composition` | `id` | `data`(`''`\|`all`), `format`, `lang` | `data=all` returns raw unfiltered NMBS fields |
| `GET /v1/disturbances` | — | `lineBreakCharacter`, `format`, `lang` | `type` is `disturbance` (4 live) or `planned` (28 live) |
| `GET /v1/logs` | — | — | **Returns `[]` in practice.** Low value; bulk archives at `gtfs.irail.be/logs/` |
| `POST /feedback/occupancy` | `connection`,`from`,`date`,`vehicle`,`occupancy` | — | Write endpoint; occupancy is a term URI (`.../terms/low\|medium\|high\|unknown`) |

**`format` defaults to `xml`** — the CLI must always force `format=json`.

## Top Workflows
1. **"When's my next train from X?"** — liveboard, filtered to non-departed, showing delay + platform + whether the platform changed.
2. **"Get me from A to B (now / at T / tomorrow)"** — connections with transfers, per-leg delay, and whether the transfer still holds once delays are applied.
3. **"Is anything broken on my route right now?"** — disturbances filtered to *actual* disruptions vs planned works, and correlated to the stations the user actually travels through.
4. **"Where is train IC1832 and how late is it?"** — vehicle trace across all stops with live delay.
5. **"Track my commute over time"** — repeat the same route daily and accumulate a delay history. **No existing tool does this.**

## Table Stakes (must match to be credible)
- Liveboard for a station, arrivals or departures, at a chosen time/date.
- Route planning A→B with transfers, arrive-by vs depart-at.
- Disturbances list.
- Station search with fuzzy matching.
- Saved/shortcut routes (commandtrein has this).
- Natural date input ("tomorrow", weekday names) — commandtrein already does this in Dutch.
- Telegraphic station codes (FR = Bruges, FBMZ = Brussels-South) — clirail has this.

## Competitive landscape
| Tool | Stack | Reach | Surface | Gap we exploit |
|---|---|---|---|---|
| **commandtrein** (`Kaya-Sem/commandtrein`) | Go + Cobra | 21★, active 2025-12 | root A→B, `timetable`, `issues`, `search`, `shortcut add/list`; `-a/-t/-d`, NL natural dates | No JSON/agent output, no persistence, no history, no MCP |
| **clirail** (`framagit.org/Midgard/clirail`) | Python | PyPI 1.7.3 | liveboard, routes, bare-invocation "timeliness analysis", telegraphic codes, fuzzy match | No JSON, no store; documented today-vs-tomorrow bug |
| **irail-mcp** (`HansF/irail-mcp`) | Python MCP | 0★, active 2026-02 | 5 tools: `search_stations`, `get_liveboard`, `find_connections`, `get_train_info`, `get_disturbances`; bundles offline `stations.json` | Text-formatted output (not typed), no composition, no history, no CLI |
| Raycast "NMBS Planner" | Raycast ext | — | GUI journey lookup | macOS-GUI only, not scriptable |
| `sncb-nmbs-train-search` | npm | — | search | Thin wrapper |
| `dedene-irail` | OpenClaw skill | — | agent skill | Skill only, no binary |

**Peer references from the printing-press library** (different networks, mined for feature parity only — not duplicates): `uk-train-goat` (UK rail: live boards + journey planning), `sncf-connect`/Navitia, `infotbm` (Bordeaux transit).

## Concrete user pain points (evidence-backed)
1. **"All data is returned as strings"** — [iRail/iRail open issue, 2025-01-08]. Verified: `"delay":"0"`, `"locationX":"4.421101"`, `"canceled":"0"`. Every consumer must hand-coerce. Hostile to agents and to `jq` arithmetic.
2. **`date` is `ddmmyy` and `time` is `hhmm`** — `300917` for 30 Sep 2017. Error-prone, ambiguous, and unlike every other date input the user touches.
3. **clirail's own documented bug:** a bare time is always "today", so planning a 7 AM trip at 11 PM silently gives yesterday's answer.
4. **`date` param on `/vehicle` is ignored** — reported twice (2025-09-03, 2026-05-20). Users silently get today.
5. **Responses are large** — [open issue: "Reduce HTTP response size (for embedded devices)"]. 122 KB just for stations; boards are 34 KB.
6. **Nobody keeps history.** Every tool is stateless request/response. There is no way to answer "is the 08:12 always late?" — the single most-asked commuter question.
7. **Far-past/far-future queries return HTTP 500**, not a clean error [docs, "Be aware"].

## Data Layer
- **Primary entities:** `stations` (716, static), `departures`/`liveboard rows`, `connections` (+`vias`, `stops`), `vehicles` (+stops), `compositions`, `disturbances`.
- **Reference data the API does *not* expose** — from `github.com/iRail/stations` (CC0-ish open data, 716 rows):
  - `stations.csv`: `telegraph-code` (**566 populated**), `taf-tap-code`, `alternative-{fr,nl,de,en}` names, `official_transfer_time` (**618 populated**), `avg_stop_times`, coordinates, `country-code`.
  - `facilities.csv` (691 rows): ticket vending machine, luggage lockers, free parking, taxi, bicycle spots, Blue-bike, bus/tram/metro links, wheelchair, ramp, disabled parking, elevated platform, escalators up/down, elevator, audio induction loop, **plus ticket-desk open/close hours for all seven weekdays**.
  - `stops.csv`, `shapes.geojson`, `embarkment_statistics.csv`.
- **Sync cursor:** stations/facilities are slow-moving → sync on demand + ETag. Live boards are not synced; they are captured as **observations** with a timestamp so history accumulates.
- **FTS/search:** station names across `name` + 4 language aliases + telegraph code + TAF/TAP code → one fuzzy resolver that beats every competitor's.

## Why this CLI instead of the incumbent
`commandtrein` is a good human TUI. `irail-mcp` is a thin agent wrapper. Neither is *agent-native* and neither remembers anything. This CLI is the only one that:
1. Emits **typed** JSON (numbers as numbers, booleans as booleans, timestamps as RFC3339) instead of iRail's all-strings payload.
2. Accepts **human dates** (`tomorrow`, `mon`, `2026-07-25`, `+2h`) and converts to `ddmmyy`/`hhmm` internally.
3. **Persists observations**, so delay history, transfer-risk and "is this train chronically late" become answerable.
4. Joins the **open stations/facilities datasets** the API withholds — transfer times, accessibility, telegraph codes.
5. Ships an **MCP surface** off the same Cobra tree.

## Product Thesis
- **Name:** `irail-pp-cli` (slug `irail`)
- **Why it should exist:** Belgian rail data is open, free, and unauthenticated, yet every existing tool throws the data away after printing it. The compounding value is in *keeping* it: once liveboard observations land in SQLite, the questions commuters actually ask — "should I leave earlier?", "will I make the transfer?", "is this train always late?" — become one command instead of impossible.

## Build Priorities
1. **Foundation:** typed client with mandatory UA, adaptive rate limiter (3 rps + burst), ETag/304 caching, all-strings→typed coercion, human-date parser, station resolver over the enriched dataset.
2. **Absorb:** liveboard, connections, vehicle, composition, disturbances, stations search, saved routes — matching commandtrein + clirail + irail-mcp feature-for-feature, with `--json`/`--agent`/`--select`.
3. **Transcend:** observation store + delay history, transfer-risk under live delay, platform-change and cancellation watch, accessibility-aware routing.

## Auth / User Vision (from briefing)
- **User vision:** journey planning + live disruptions/delays. Public data only.
- **Auth context:** no API key, no browser session, `AUTH_SESSION_AVAILABLE=false`. API needs none — key gate skipped.
