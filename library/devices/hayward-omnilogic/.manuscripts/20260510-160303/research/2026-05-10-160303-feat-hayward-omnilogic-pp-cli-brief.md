# Hayward OmniLogic CLI Brief

## API Identity
- **Domain:** Residential pool/spa automation. The OmniLogic controller is a network-connected device next to a pool/spa that drives pumps, heaters, chlorinators, ColorLogic lights, valves, and chemistry sensors. The cloud API is what the OmniLogic mobile app and the web app at `haywardomnilogic.com` talk to.
- **Users:** Pool owners managing one site; pool-service businesses managing many sites; pool integrators (HA enthusiasts) wiring OmniLogic into home automation.
- **Data profile:** Mostly real-time telemetry over a fixed equipment inventory. Equipment changes rarely (once installed it's stable for years); chemistry, temps, pump speed change continuously; alarms are sparse but high-signal. No historical/trend data is provided by the API — the cloud returns "now" only.
- **Target surface:** The cloud partner API (`api.haywardomnilogic.com` / `services-gamma.haywardcloud.net` / `www.haywardomnilogic.com/HAAPI/HomeAutomation/API.ashx`), not the consumer web app at `haywardomnilogic.com` or the LAN-only UDP path.

## Reachability Risk
- **Low.** Three-year stable surface on the `.ashx` operations endpoint. Only major change in that span: Oct 2025 forced migration from username to email-based auth (well documented in `home-assistant/core#148146`). Servers are up (IIS + AWS ALB, 200/404 depending on path). No 403/blocked/rate-limit complaints on `djtimca/omnilogic-api`'s issue tracker.
- **Auth changed Oct 2025**: any wrapper before `omnilogic-api` 0.5.6 is broken. We must target the v2 auth flow from day one.
- **Polling guidance**: community uses 30–60s as the safe poll interval. No published rate limits.

## Top Workflows
1. **Pre-heat for swim time** — set heater enable + setpoint a few hours before swim.
2. **Check water chemistry before guests** — pH, ORP, salt, temp on demand. Today this requires opening the app; agents and CLI users want a one-liner.
3. **Diagnose "pump isn't running"** — correlate alarms + relay state + schedule + heater demand into one answer. No wrapper currently packages this.
4. **Trigger a ColorLogic show / theme for a party** — set show, speed, brightness on a body of water.
5. **Manual superchlorination** — fire a one-shot superchlor cycle before/after heavy bather load or rain.
6. **Weekly chemistry log** — snapshot pH/ORP/salt/temp once a day, render a weekly CSV for service-record / HOA / insurance.
7. **Multi-site sweep for service businesses** — every morning, list alarms across all sites in the account, decide which trucks roll where.
8. **Schedule audit** — read current pump/heater schedules from MSP config and verify no one has changed them since last snapshot.

## Table Stakes (everything the existing wrappers expose)
Single source of truth: `djtimca/omnilogic-api` 0.6.1. Every operation it has, we ship.
- `Login` (REST JSON, v2 endpoint, email + password → token + refreshToken + userID)
- `GetSiteList` → list of `MspSystemID` + `BackyardName`
- `GetMspConfigFile` → full equipment inventory tree (System, Backyard, Relays, BodyOfWater[Filters, Pumps, Heaters, Chlorinator, Lights, CSAD, Valves])
- `GetAlarmList` → active alarms keyed by equipment
- `GetTelemetryData` → live state mirror of MSP config (chemistry, temps, pump speeds, light state, alarm flags)
- `SetHeaterEnable` (on/off) and `SetUIHeaterCmd` (setpoint)
- `SetUIEquipmentCmd` (generic on/off + timed run — pumps, VSPs, valves, relays)
- `SetUISpilloverCmd` (spillover speed/timing)
- `SetUISuperCHLORCmd` (superchlorination on/off)
- `SetStandAloneLightShow` and `…V2` (ColorLogic show + speed + brightness)
- `SetCHLORParams` (chlorinator configuration)
- HA-component sensors layer: derived per-equipment readings (pump-speed-percent, chlor-output-percent, salt level, pH, ORP, heater-enabled boolean, etc.) — we beat this with structured `--json` output.

## Data Layer
- **Primary entities:**
  - `sites` (`MspSystemID`, `BackyardName`) — root keys.
  - `msp_config_snapshots` (per site, versioned XML blob + parsed equipment tree).
  - `bodies_of_water` (`bow_id`, `site_id`, `name`, `type=Pool|Spa`, `shared_equipment_id`).
  - `equipment` (rows for every filter, pump, heater, chlorinator, light, relay, valve, CSAD sensor; `equipment_id` stable across syncs; FK to `bow_id`).
  - `telemetry_samples` — append-only time-series: `site_id`, `bow_id`, `equipment_id`, `metric`, `value`, `ts`. Powers weekly chemistry export, trend detection, and "last time pump ran at X%" queries.
  - `alarms` — event log: `alarm_id`, `equipment_id`, `severity`, `first_seen`, `last_seen`, `cleared_at`. The cloud only returns the current set; we persist transitions.
  - `command_log` — every `Set*` you issue: who/when/what/result. Hayward's app doesn't show this; service-business users want an audit trail.
  - `auth_tokens` — cached `token`, `refreshToken`, `expires_at`, `user_id` so the CLI doesn't re-login on every call.
- **Sync cursor:** Append-only on telemetry/alarms (no upstream cursor); diff on MSP config (rare changes). `sync --full` walks all sites; `sync --site <id>` for one.
- **FTS/search:** FTS5 over alarms (message + equipment name), equipment (name + function + type), command_log (description), telemetry_samples_summary (anomaly labels). Backed by the standard `printing-press` store pattern.

## Codebase Intelligence
- Source: research of `djtimca/omnilogic-api` (Python wrapper, the canonical reference for the cloud API).
- Auth: Two-stage. Stage 1 is REST JSON `POST https://services-gamma.haywardcloud.net/auth-service/v2/login` with header `X-HAYWARD-APP-ID: tzwqg83jvkyurxblidnepmachs` and body `{"email","password"}`, returning `{token, refreshToken, userID}`. Stage 2 is XML-envelope `POST https://www.haywardomnilogic.com/HAAPI/HomeAutomation/API.ashx` with a flat `<Request><Name>Op</Name><Parameters><Parameter name= dataType=>…</Parameter></Parameters></Request>` body. Not real SOAP — XML-RPC-shaped custom envelope. Response is XML, parsed via `xmltodict` in Python; we'll use `encoding/xml` in Go.
- Data model: `MspSystemID` (per-site) → `BodyOfWater` (Pool|Spa, several per site) → equipment (`Pump`, `Heater`, `Chlorinator`, `ColorLogic-Light`, `Valve`, `CSAD`, `Filter`). Equipment IDs are stable; telemetry mirrors the config tree with current values overlaid.
- Rate limiting: No documented limits; community polls every 30–60s. We'll default to 30s and surface `--poll-interval`.
- Architecture: All cloud ops are POSTs to one URL with a different `<Name>`. The endpoint catalog is operation-by-name, not REST-by-path. This shapes the spec: each operation becomes an internal-YAML endpoint with a custom body emitter (an XML envelope helper, not the default JSON encoder).

## User Vision
- The user has a logged-in Chrome session at `haywardomnilogic.com` (kept for cross-checks if needed but not used as the CLI runtime — partner API is more stable) and credentials (`HAYWARD_USER`, `HAYWARD_PW`) in `/Users/zehner/dev/cli-printing-press/.env`. `HAYWARD_USER` is presumed to be an email post-Oct-2025; the spec will use those env var names verbatim.
- Implied vision: a fully functional CLI for their own pool (single site) that also handles the multi-site case gracefully. Agent-native output so an LLM can answer "is the pool ready for guests" without a UI.

## Product Thesis
- **Name:** `hayward-omnilogic-pp-cli` (slug `hayward-omnilogic`).
- **Why it should exist:**
  - **There is no CLI today.** Zero npm/PyPI/Go CLIs, zero MCP servers, zero Claude plugins. Greenfield.
  - **The wrappers leave gaps users complain about.** HA's in-tree component has no heater control, no ColorLogic, no schedule visibility, no historical data, no diagnostics. We absorb every wrapper feature AND add the missing surface.
  - **Local store unlocks features the cloud API can't expose.** Weekly chemistry export, chemistry drift alerts, schedule-vs-actual diff, "why isn't my pump running" diagnostics, multi-site morning alarm sweep — all require correlating data across calls and across time. The cloud only returns "now"; SQLite makes them trivial.
  - **Agent-native.** `is the pool ready for guests` is a single MCP call (sees chemistry + temp + alarms in one shot). The Hayward app can't do this.
  - **Stable substrate.** The partner API has been stable for 3+ years; the Oct-2025 break is the exception. We ship against it with confidence.

## Build Priorities
1. **Foundation (Priority 0):** internal YAML spec for the OmniLogic operations; auth (two-stage with token caching); custom XML-envelope client; SQLite store with telemetry time-series and command-log audit trail.
2. **Absorb (Priority 1):** every operation in `djtimca/omnilogic-api` 0.6.1 (sites, MSP config, alarms, telemetry, all `Set*` operations, light shows v1+v2, chlorinator params, spillover). Add `--json`, `--dry-run`, idempotent retries, structured exit codes.
3. **Transcend (Priority 2 — the differentiators):**
   - **`status`** — single-call "is the pool ready?" view (chemistry green/yellow/red, temp at setpoint, no active alarms, pump running). Combines `GetTelemetryData` + `GetAlarmList` + thresholds.
   - **`chemistry log`** — local-store-backed weekly/monthly CSV/JSON of pH, ORP, salt, temp; powers HOA/service-record exports.
   - **`chemistry drift`** — detect when pH/ORP/salt drifts beyond bounds compared to a recent baseline; the cloud has thresholds for alarms, not for trends.
   - **`why-not-running`** — diagnostic: given equipment that should be on (per schedule), check current state, alarms, and recent commands to explain why it isn't.
   - **`schedule diff`** — compare today's MSP-config schedules against yesterday's snapshot; flag any change a tech might have made.
   - **`sweep`** — multi-site morning alarm sweep for service businesses; emits a "trucks should roll to these sites today" report.
   - **`preheat`** — one-shot: enable heater + set setpoint + report ETA based on current water temp and historical heat rate from telemetry.
4. **Polish (Priority 3):** flag descriptions, error messages, README/SKILL prose, MCP tool descriptions, tests for the XML envelope helper and the diagnostic logic.

## Auth & Env Var Plan
- `auth.type: composed` (two-stage: REST JSON login + XML-envelope operations).
- Stage 1: REST JSON `POST` to the v2 auth endpoint.
- Stage 2: every operation appends the cached `Token` + `MspSystemID` to the XML body.
- Env vars: `HAYWARD_USER` (email post-Oct-2025) and `HAYWARD_PW`. These match the user's existing `.env` exactly; no aliases.
- Token cache: `$XDG_STATE_HOME/hayward-omnilogic/auth.json` (or `~/.local/state/...`). Refresh-token flow on token expiry.

## Out of Scope (this run)
- The LAN-only UDP path (`cryptk/python-omnilogic-local`). Different protocol, different deployment story; if the user wants it, that's a v2 add-on.
- Web-app-only surfaces (account/billing, dealer locator, marketing). Their logged-in Chrome session covers these in a browser; not worth a CLI.
- Schedule CRUD beyond reading. The cloud API surface for schedule mutation is undocumented in the wrappers; we'll surface reading + diffing, leave writes for a follow-up after verifying with traffic capture.

## Sources
- `djtimca/omnilogic-api` (canonical wrapper, 0.6.1 implements Oct-2025 v2 auth).
- `home-assistant/core` `omnilogic` integration tree.
- `home-assistant/core#148146` (Oct-2025 auth migration tracking issue).
- `cryptk/python-omnilogic-local` (LAN-only path; out of scope but referenced for entity shape).
- `openHAB haywardomnilogic` binding (cross-check on operation names + parameters).
- Trouble Free Pool forum + HA community thread for pain-point inventory.
