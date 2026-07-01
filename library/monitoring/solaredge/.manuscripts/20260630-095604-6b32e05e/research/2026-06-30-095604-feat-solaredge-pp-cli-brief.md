# SolarEdge CLI Brief

## API Identity
- Domain: Solar PV monitoring — site energy/power production, consumption/export/import meters, battery (storage) telemetry, equipment inventory (inverters, optimizers, gateways, sensors), for residential and commercial SolarEdge installations.
- Users: System owners checking their own production, installers/integrators managing fleets of sites, home-automation enthusiasts (Home Assistant, Node-RED, MQTT bridges) pulling data into other systems.
- Data profile: time-series energy/power data (15-min to yearly resolution), site metadata, equipment inventory, battery telemetry. Strong fit for local SQLite history — the API itself has no historical comparison or anomaly features.

## Reachability Risk
- None. Verified live: `curl https://monitoringapi.solaredge.com/version/current` → HTTP 401 with a structured JSON error body (`{"type":"https://se-api.stoplight.io/docs/monitoring","title":"Unauthorized - Authentication Failed",...}`), confirmed via plain stdlib HTTP, no Cloudflare/browser-fingerprint gating on the API host. (Note: the *marketing-site* PDF mirrors at solaredge.com and knowledge-center.solaredge.com are Cloudflare-protected and returned 403 to curl; this only affects documentation hosting, not the API itself. A working PDF mirror was found at solar.ece.ksu.edu.)
- Tier/permission hints from 4xx body: "API key missing in the request" / "Invalid API key" — confirms `api_key` query param auth, RFC7807-style problem-details envelope.
- Probe-safe endpoint used: `GET /version/current` (no auth, no params, idempotent).

## Top Workflows
1. Check current site power flow / "how much am I producing right now" (`currentPowerFlow`, `overview`)
2. Track daily/monthly/yearly energy production vs consumption/export/import (`energyDetails`, `powerDetails` with meters=Production,Consumption,SelfConsumption,FeedIn,Purchased)
3. Monitor battery state of charge and charge/discharge history (`storageData`, `currentPowerFlow` STORAGE element)
4. Check equipment health/inventory — inverter firmware, optimizer counts, connected gateways/sensors (`Inventory`, `equipment/{siteId}/list`)
5. Multi-site fleet management for installers — bulk overview/energy across many sites, sorted by underperformers (`accounts/list` + bulk `/sites/{ids}/...`)

## Table Stakes
- Every read-only endpoint from the official API (every existing wrapper across Python/Node/Go/Rust explicitly claims "implements all documented endpoints" — no wrapper has found undocumented endpoints)
- JSON/XML/CSV output format selection
- Env var API key management (`SOLAREDGE_API_KEY` is the de facto convention used by `solaredge-interface`)
- Bulk/multi-site query support for fleet-style endpoints
- CLI form exists already (`solaredge-interface`, Python+Click) but is a thin 1:1 wrapper with no persistence layer, no anomaly detection, and no agent-native (`--json`/`--select`) ergonomics beyond basic CSV/JSON toggling

## Data Layer
- Primary entities: `sites` (metadata), `equipment` (inverters/SMIs/batteries/meters/gateways/sensors inventory), `storage_telemetry` (per-battery time series), `power_series` / `energy_series` (per-meter time series), `inverter_telemetry` (per-serial technical data)
- Sync cursor: per-site `dataPeriod.endDate` (start/end of available production data) drives incremental sync; equipment inventory and site details are low-churn and can be re-pulled on a longer cadence
- FTS/search: site list searchable by name/notes/address/city/zip (server-side `searchText`); local FTS over synced site + equipment names useful for multi-site installer accounts

## Rate Limit Risk
- Hard daily limit: 300 requests per (account-token, siteId) pair from the same source IP per day. Calls with no specific ID (Site List, Account List) count toward the same total budget.
- Concurrency limit: max 3 concurrent calls from the same source IP; additional concurrent calls return HTTP 429.
- Real-world friction confirmed via Home Assistant issue #59574: users hitting 429s from default polling intervals, especially with multiple integrations/dashboards pulling the same account. This is exactly the kind of budget a local sync+cache layer should track and warn about before exhaustion — a feature no existing wrapper provides.
- 403 is overloaded: used both for permission violations (no access to a site ID in a bulk call) and period-length violations (e.g., requesting more than 1 month of `power` data). The CLI should distinguish these in error messages where possible by inspecting the response body.

## Reachability Gate
- Decision: PASS
- Check: `GET https://monitoringapi.solaredge.com/version/current` (no auth, no params, idempotent) → HTTP 401, structured JSON error body (`"API key missing in the request"`)
- Per the decision matrix: 401 with no key provided is expected PASS behavior when the API needs auth and the user declined the key gate. No bot-protection, no Cloudflare/browser-fingerprint gating on the API host.

## Auth
- Type: `api_key` (query parameter `api_key`)
- Canonical env var: `SOLAREDGE_API_KEY` (matches the one existing CLI competitor's convention; no OAuth/bearer/cookie shape involved)
- Two key scopes available to users: Account-level key (admin, sees multiple sites) and Site-level key (single site, generated from Site Admin → Site Details → API Access). Either works against the same endpoints; account-level keys unlock `/accounts/list` and multi-site bulk calls.

## Product Thesis
- Name: SolarEdge (binary: `solaredge-pp-cli`)
- Why it should exist: every existing tool (Python `solaredge`/`py-solaredge`/`solaredge-interface`, Node `solaredge`/`solarmon`, Go `clambin/solaredge`, Rust `se_ms_api`) is a 1:1 stateless API wrapper. None persist history locally, none warn about the 300-req/day budget before it's exhausted, none compare today's production against the site's own historical baseline to flag underperformance, and none give an agent a single `--json --select` call to answer "is my system OK right now" without re-deriving it from `currentPowerFlow` + `overview` + `Inventory` by hand. A local SQLite mirror with sync makes "what changed since yesterday" and "is panel X underperforming vs its own history" answerable offline, which the raw API fundamentally cannot do in one call.

## Build Priorities
1. Data layer + sync for sites, equipment inventory, energy/power series, storage telemetry (Priority 0)
2. Absorb every endpoint from the 24-endpoint surface above with agent-native output (Priority 1)
3. Transcendence: local-history-driven underperformance/anomaly detection, rate-limit-budget-aware sync, fleet rollup for multi-site accounts, "what changed since X" digest (Priority 2 — finalized via the novel-features subagent in Phase 1.5c.5)
