# BMW CarData CLI Brief

## API Identity
- **Domain:** Personal vehicle telematics data access (BMW Group EU Data Act compliance program). BMW/MINI/Rolls-Royce/Toyota Supra owners access their OWN vehicle data over a documented REST API + MQTT stream.
- **Official product:** BMW CarData — *"free for personal use only"*. Launched ~2024 (replaced the disabled `bimmerconnected` path).
- **Specs found (authoritative):** OpenAPI 3.1.0 `CARDATA API` (REST, 11 ops) + OpenAPI 3.0.1 `Customer OAuth2.0 Login - Device Code Flow`. Located as static SPA assets:
  - `https://bmw-cardata.bmwgroup.com/customer/public/assets/swagger/swagger-customer-api-v1.json` (saved to `$DISCOVERY_DIR/`)
  - `…/swagger-device-code-flow.json`
- **Hosts:** REST `https://api-cardata.bmwgroup.com` (header `x-version: v1`, `Authorization: Bearer <opaque GCDM token>`). OAuth `https://customer.bmwgroup.com/gcdm/oauth/{device/code,token}`. Stream `customer.streaming-cardata.bmwgroup.com:9000` (MQTT v5 over TLS).
- **Users:** EU BMW/MINI/Roll-Royce/Supra owners with (a) active SIM, (b) ConnectedDrive contract, (c) vehicle mapped to account as PRIMARY user; tinkerers + Home Assistant users.
- **Data profile:** VSS-style telematic descriptors (`vehicle.drivetrain.batteryManagement.*`, `vehicle.cabin.infotainment.navigation.currentLocation.*`), vehicles, containers (user-defined descriptor bundles), charging sessions, tyre diagnosis, customer archive (.zip/XML).

## Reachability Risk
- **Low.** API host `api-cardata.bmwgroup.com` returns **403 application/json** without a valid bearer token — **expected** (auth-required data API), not a bot wall. OAuth host `customer.bmwgroup.com/gcdm/oauth/device/code` reachable (returns 400/4xx on malformed body, 200 on valid device-code request). All spec/doc/Swagger assets publicly fetchable (200). No Cloudflare/WAF challenge observed.
- **Rate limit (critical):** ~50 calls/day per client, **resets midnight UTC** (from `kvanbiesen/bmw-cardata-ha` bootstrap code). Commands must be conservative + cache locally.
- Tier/permission hints: none (single consumer tier; gating is by OAuth scope `cardata:api:read` / `cardata:streaming:read`).
- Probe-safe endpoint used: `GET https://api-cardata.bmwgroup.com/customers/vehicles/mappings` → 403 w/o token (expected).

## Auth Model (key design decision)
- **OAuth2 Device Authorization Grant + PKCE (S256).** Not authorization-code. Requires a **client_id** the user generates once in the BMW portal ("Create CarData Client" — note: ID disappears on reload, must save it) + subscribes scopes (`cardata:api:read`, optionally `cardata:streaming:read`).
- **Flow:** `POST /gcdm/oauth/device/code` (form: client_id, response_type=device_code, scope, code_challenge, code_challenge_method=S256) → `{user_code, device_code, interval, expires_in, verification_uri, verification_uri_complete}`. User opens `verification_uri_complete`, logs in, approves. Poll `POST /gcdm/oauth/token` (form: client_id, grant_type=urn:ietf:params:oauth:grant-type:device_code, device_code, code_verifier). **Status codes: 200=success, 403 authorization_pending=keep waiting, 403 access_denied=denied, 400 slow_down=+5s, expired_token.** Refresh via `grant_type=refresh_token`.
- **Modeling for generator:** runtime auth = `bearer_token` (the opaque GCDM access token), env `BMW_CARDATA_ACCESS_TOKEN` (manual override). The device-code flow is a **hand-coded `auth login`** transcendence command that stores `{access_token, refresh_token, id_token, expires_at, client_id}` in config and auto-refreshes. Env `BMW_CARDATA_CLIENT_ID` for the client id.

## Top Workflows
1. **Onboard once:** `auth login` → device-code flow → token stored. (`doctor` to verify.)
2. **See my vehicles:** list mappings + basic data (brand, model, battery capacity `hvsMaxEnergyAbsolute`, SIM status, propulsion/charging modes).
3. **Define a container** of wanted descriptors (e.g. "HV Battery" bundle) → **fetch live telematic data** (SoC, range, location, charging status, windows, fuel).
4. **Review charging history** sessions (energy, duration, location, preconditioning).
5. **Tyre diagnosis** (smart maintenance) + import **Customer Archive .zip** for historical data.

## Table Stakes (from spec + competitors — must match)
- OAuth device-code login + token refresh (all community impls)
- `GET /customers/vehicles/mappings`, `GET /customers/vehicles/{vin}/basicData`
- Container CRUD: list / **create** / get / delete (tjamet CLI lacks **create**)
- `GET /customers/vehicles/{vin}/telematicData?containerId=` (core)
- `GET /customers/vehicles/{vin}/image`
- `GET /customers/vehicles/{vin}/chargingHistory` (paginated, `next_token`)
- `GET /customers/vehicles/{vin}/smartMaintenanceTyreDiagnosis`
- `GET /customers/vehicles/{vin}/locationBasedChargingSettings`
- **Customer Archive .zip reader** (XML `KeyList.xml` → structured) — tjamet has `read-archive`; portal downloadable bundle
- **MQTT streaming** of live telematic data (3+ bridges exist: dj0abr C++, whi-tw Python, bausi2k Python)

## Data Layer
- **Primary entities:** vehicles (mappings+basicData), containers (config), telematic snapshots (VSS descriptor → {value,unit,timestamp} time-series), charging sessions, tyre diagnosis, archive imports.
- **Sync cursor:** telematic snapshots by `timestamp`; charging history `next_token`; rate-limit-aware (≤~24 polls/day).
- **FTS/search:** over VSS descriptor catalogue + vehicle fields; descriptor-path lookup (`vehicle.drivetrain.*`).

## Codebase Intelligence
- **`kvanbiesen/bmw-cardata-ha`** (Python, ~19K LOC, HA integration) — most complete: device flow, container CRUD + auto-create HV battery container, telematic polling w/ rate-limit handling (50/day, midnight UTC reset), MQTT stream, derived sensors (range, is_moving, fuel), `magic_soc`/predicted-SoC + DC-efficiency learning, charging history, tyre diagnosis, external power injection. **Ground truth for behavior + rate limits + descriptor semantics.**
- **`whi-tw/bmw-cardata-streaming-poc`** (Python) — device flow + PKCE, token refresh/persistence, MQTT v5 clean-session, data-catalogue-driven human-readable messages, credentials-only mode.
- **`dj0abr/bmw-mqtt-bridge`** (C++, 46★) — CarData MQTT → local Mosquitto bridge. Notes BMW *disabled bimmerconnected*.
- **`tjamet/bmw-cardata`** (Go, Apache-2.0, 1★) — typed Go client + minimal flag CLI (11 cmds incl. `read-archive`, `stream-telematic-data`); **no container create, no tyre diagnosis, no local store, no analytics**.
- **No MCP server exists** for BMW CarData (opportunity). **No full-featured CLI exists** (only HA integrations + MQTT bridges + one minimal Go CLI).

## User Vision
- (none provided — user chose "Let's go")

## Product Thesis
- **Name:** `bmw-cardata-pp-cli` (display: "BMW CarData"). Module `github.com/mvanhorn/bmw-cardata-pp-cli`.
- **Thesis:** *Your BMW's telemetry, on the command line.* OAuth device-code login, live telematic snapshots, charging history, tyre diagnosis, Customer-Archive import, and a local SQLite store that turns raw VSS data into trends, SoC/range analytics, and fleet insight across VINs — none of which any existing tool (HA-bound Python, MQTT bridges, or the minimal Go CLI) provides. Agent-native (`--json`/`--select`/`--dry-run`), offline, scriptable, MCP-exposed.

## Build Priorities
1. **P0 foundation:** bearer-token client w/ `x-version: v1` header + auto-refresh; data layer for vehicles/containers/telematic-snapshots/charging-sessions; sync/search/SQL path; descriptor catalogue.
2. **P1 absorb (match all):** mappings, basicData, telematicData, container CRUD (incl. create), image, chargingHistory, tyreDiagnosis, locationBasedChargingSettings, archive .zip reader, MQTT stream.
3. **P2 transcend (beat all):** `auth login` device-code flow; telematic **snapshot history** + SoC/range/**trends**; charging-cost/efficiency analytics; multi-VIN **fleet** view; descriptor-catalogue search + container templates; snapshot **diff**; rate-limit-aware scheduler. Min 5.

## Reachability Gate
- Decision: PASS
- OAuth host `customer.bmwgroup.com/gcdm/oauth/device/code` → HTTP 400 `{"error":"invalid_request",...}` (reachable; structured OAuth error; no auth needed to prove reachability).
- REST host `api-cardata.bmwgroup.com/customers/vehicles/mappings` (no token) → HTTP 403 `application/json` `{"exveErrorId":"TP-403","exveErrorMsg":"Bad Request. Forbidden",...}` — BMW's `CustomerErrorResponse`; expected auth-required response, NOT a bot wall.
- Bot-protection scan: no Cloudflare/Datadome/PerimeterX/captcha/Vercel evidence.
- No 403 issues reported by community impls (they work once authed).
