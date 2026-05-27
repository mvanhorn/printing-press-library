# Zaptec CLI Brief

## API Identity
- **Domain:** EV charging. Zaptec makes the Zaptec Go / Go2 / Pro home & commercial AC chargers. The cloud REST API (ZapCloud) controls and monitors chargers, installations, and charging sessions.
- **Base URL:** `https://api.zaptec.com`
- **Spec:** Official OpenAPI 3.0.4 at `https://api.zaptec.com/swagger/v1/swagger.json` (16 endpoints).
- **Users:** Home EV owners (1 charger), landlords / housing co-ops (installations with many chargers), fleet/commercial operators, and integrators (Tibber, Monta, evcc, Home Assistant).
- **Data profile:** Chargers (state, settings), installations (hierarchy, current limits), charging sessions (energy kWh, cost, duration, timestamps), firmware. Numeric observation IDs and command IDs that need decoding via `/api/constants`.

## Reachability Risk
- **None.** `GET /api/constants` returns HTTP 200 over plain stdlib HTTP (`probe-reachability` → `standard_http`, confidence 0.95). Authenticated endpoints expected to 401 without a token; that is the normal auth path, not a block. No bot protection, no Cloudflare/WAF.

## Auth
- **OAuth2 Resource Owner Password Credentials (ROPC / `password` grant).**
- Token endpoint: `https://api.zaptec.com/oauth/token` (form-encoded: `grant_type=password`, `username`, `password`). Returns `access_token` + `expires_in`.
- Send as `Authorization: Bearer <token>` on all `/api/*` calls.
- Guideline: cache the token + expiry, reuse until expiry — do NOT re-auth per call.
- Env vars for CLI: `ZAPTEC_USERNAME`, `ZAPTEC_PASSWORD` → `auth login` harvests a bearer token into local config; token auto-refreshes when expired.
- `GET /api/constants` is effectively public (no auth) — useful for `doctor` reachability and for the offline constants cache.

## Rate Limits & Constraints
- **10 requests/second per account.** Returns `429 Too Many Requests`. Implement queue + exponential backoff.
- Pagination: **max 100 records per page** (`PageSize`, `PageIndex`).
- **15-minute minimum interval** between installation available-current updates (load-balancing). The CLI must warn / guard against rapid re-sends.
- Real-time updates: Azure Service Bus / AMQP subscription via `/api/installation/{id}/messagingConnectionDetails` (push, not polling). Out of scope for a single-binary CLI runtime; the CLI uses REST polling.

## Top Workflows
1. **Quick control of one charger** — `pause` / `resume` / `start` / `stop` / `restart` a charger from the terminal without opening the portal app.
2. **Fleet/installation overview** — list all chargers across installations, see which are charging / connected / finished / offline at a glance.
3. **Charge session history & energy/cost analytics** — "how much did I charge this week/month, how many kWh, what did it cost" — sync sessions locally and aggregate.
4. **Live state inspection** — decode a charger's raw observation IDs into human-readable readings (current per phase, voltage, power, total energy, operation mode, temperature).
5. **Load-balancing / available-current control** — adjust the installation's max available current (with the 15-min guard) for dynamic load balancing.
6. **Firmware status** — see firmware versions across the fleet and trigger upgrades.

## Endpoint Surface (16)
- `GET /api/chargers` · `GET /api/chargers/{id}` · `GET /api/chargers/{id}/state` · `POST /api/chargers/{id}/update` · `POST /api/chargers/{id}/sendCommand/{commandId}`
- `GET /api/installation` · `GET /api/installation/{id}` · `POST /api/installation/{id}/update` · `GET /api/installation/{id}/hierarchy` · `GET /api/installation/{id}/messagingConnectionDetails`
- `GET /api/chargehistory` · `GET/POST /api/chargehistory/installationreport`
- `GET /api/session/{id}` · `POST /api/session/{id}/priority`
- `GET /api/chargerFirmware/installation/{installationId}`
- `GET /api/constants` · `GET /api/userGroups/{id}/messagingConnectionDetails`

### Command IDs (for sendCommand) — baked from /api/constants
- StartCharging=501, StopCharging=502, StopChargingFinal=506, ResumeCharging=507
- RestartCharger=102, RestartMcu=103, RestartApplication=107
- UpgradeFirmware=200, UpgradeFirmwareForced=201
- UnlockConnector=708, DeauthorizeAndStop (deauthorize), UpdateSettings=104
- ChargerOperationModes: 0 Unknown, 1 Disconnected, 2 Connected_Requesting, 3 Connected_Charging, 5 Connected_Finished

## Data Layer
- **Primary entities:** chargers, installations, charge_sessions, firmware. `constants` cached offline (commands, observations, settings, operation modes).
- **Sync cursor:** charge history by `From`/`To` timestamps; chargers/installations full refresh (small N).
- **FTS/search:** charger name, installation name, session id — for `search` and offline lookups.
- **Why SQLite matters:** session history lives only as paginated API responses. Persisting it locally unlocks energy/cost trends, per-month rollups, per-charger totals, and "what changed" diffs that no single API call provides.

## Competitive Landscape
- **`custom-components/zaptec` (Home Assistant, Python)** — the main incumbent. Exposes: charger status/mode, current/voltage/power, session energy totals, start/stop/pause/resume, authorize/deauthorize, cable-lock switch, available-current number, per-phase `limit_current`, 3→1 phase switch, status-light brightness, installation max-current limiter. Polling-based; ships a standalone-capable Python API lib.
- **`evcc-io/evcc` (Go)** — has a read-oriented Zaptec charger driver (state + current control) inside a larger EV charge controller. Not a standalone Zaptec tool.
- **No dedicated Zaptec CLI or MCP server exists.** The field is wide open.

## Why this CLI instead of the incumbent
- The HA integration needs a running Home Assistant. There is **no way to control a Zaptec charger from a terminal / script / cron today.**
- Agent-native: `--json`, `--select`, typed exit codes, `--dry-run` on every mutation, scriptable in CI.
- Offline local store: energy/cost analytics and fleet history the portal app does not surface.
- Human-friendly command + observation decoding baked in (no magic numbers).

## Product Thesis
- **Name:** `zaptec` (binary `zaptec-pp-cli`)
- **Display name:** Zaptec
- **Why it should exist:** Control and monitor your Zaptec Go charger from the terminal — pause/resume charging, watch live state in plain English, and track energy & cost history in a local database, all scriptable. The only Zaptec CLI that exists.

## Build Priorities
1. **Foundation:** OAuth2 password-grant `auth login` + token cache/refresh; local SQLite store for chargers/installations/sessions; sync + search + sql.
2. **Absorbed (match the HA integration):** list/get chargers & installations, decode state, pause/resume/start/stop/restart/unlock/authorize, available-current update (with 15-min guard), charge history, firmware status, installation hierarchy.
3. **Transcend:** energy/cost analytics, fleet "what's charging now" overview, session rollups, state-change diffs, plain-English state — see absorb manifest.
