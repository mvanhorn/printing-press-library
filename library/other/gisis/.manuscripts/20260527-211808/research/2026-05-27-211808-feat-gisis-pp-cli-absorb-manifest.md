# GISIS Absorb Manifest — gisis-pp-cli v1

Greenfield API. No GISIS-specific CLI or MCP exists. Adjacent reference: `rhinonix/equasis-cli`. Vessel MCP reference: `vessel-api/vesselapi-mcp` (commercial AIS, not authoritative registry).

## Scope rules for v1 (per execution log Phase 1a)

In scope:
- Ship Particulars module — GET `/Public/SHIPS/ShipDetails.aspx?IMONumber=<imo>` only
- Cookie auth via press-auth companion
- All 7 transcendence features below

Explicitly deferred to v0.2 via `/printing-press-amend`:
- Ship search by name (requires `__VIEWSTATE` postback to `/Public/SHIPS/Default.aspx`)
- Marine Casualties and Incidents module (requires separate authenticated HAR capture)
- Other 25 GISIS public modules
- Companies module (independent module accessed differently than Ship Particulars)

## Absorbed (match or beat every feature competing tools have)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Ship lookup by IMO number | equasis-cli `vessel /imo` + VesselAPI MCP `vessel by IMO` | gisis-pp-cli ship get <imo> | Authoritative IMO source; JSON/CSV/table; MCP-callable; HTML-parses GISIS Ship Particulars page |
| 2 | JSON / CSV / table output | equasis-cli | (behavior in gisis-pp-cli ship get) | Standard press --json/--csv/--select/--compact across all commands |
| 3 | Persistent credential store | equasis-cli credentials.json | (behavior in gisis-pp-cli auth) | press-auth companion (keychain-backed cookie store), one-command setup; survives Cloudflare Turnstile |
| 4 | Polite throttle baseline | equasis-cli 1s + backoff | (behavior in gisis-pp-cli client) | 1 req/2-3s default (per execution log), AdaptiveLimiter from press cliutil; 429/503 exponential backoff |
| 5 | Local SQLite cache | equasis-cli | (generated endpoint) ship get | Press default; ships accumulate as queried; FTS on name/owner/operator |
| 6 | MCP tool exposure | VesselAPI MCP `vessel` tools | (behavior in cobratree mirror) | Auto-generated; `gisis_ship_get` available to orchestrator + Claude Desktop |
| 7 | Health check (auth + reachability) | equasis-cli implicit | gisis-pp-cli doctor | Verifies cookies present, session live (probe-ping returns non-WebLogin) |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Batch IMO lookup with throttled persistence | `ship batch --imos 9966233,9123456` or `ship batch --file imos.txt` | hand-code | GISIS has no bulk-fetch endpoint; this orchestrates per-IMO calls under the 1-req/2-3s throttle and persists each to SQLite so the orchestrator can resume nightly tests against a 50-vessel watchlist without re-hitting the rate limit | Use this command to resolve many IMO numbers at once with the configured throttle. Do NOT use this command for a single IMO; use 'ship get' instead. |
| 2 | Local cache browse with FTS | `ship list [--flag] [--owner] [--type] [--name-like] [--limit]` | hand-code | GISIS has no "my recent lookups" view; this surfaces the journalist's accumulated cache, FTS on name/owner/operator | Use this command to browse vessels you have already fetched. Do NOT use this command to fetch a vessel by IMO from GISIS; use 'ship get' instead. |
| 3 | Watchlist with selective refresh | `ship pin <imo> [--label X]` / `ship unpin <imo>` / `ship refresh [--pinned] [--older-than 30d]` | hand-code | GISIS has no per-user watchlist; pinning + scoped re-fetch replaces sync semantics for a lookup-only API | none |
| 4 | Cross-snapshot ship history (flag-hop detector) | `ship history <imo>` | hand-code | GISIS shows current state only; we accumulate immutable snapshots on every fetch and surface per-field diffs (flag, name, owner, operator, class, status). Flag-hopping is THE textbook sanctions-bypass tell | Use this command to see how a vessel's particulars have changed across the snapshots you have fetched. Do NOT use this command for a single current snapshot; use 'ship get' instead. |
| 5 | Stale-cache report | `ship stale [--older-than 30d] [--pinned]` | hand-code | Makes lookup-only API's staleness visible; integrates with hintIfStale; suggests next `ship refresh` | none |
| 6 | Owner-fleet listing from accumulated cache | `owner fleet "ACME" [--exact\|--like]` | hand-code | Companies module is deferred in v1, so GISIS has no public "list ships by owner" surface for SHIPS; this synthesizes the answer from accumulated `ship get` results by registered_owner | Use this command to list cached vessels by owner. Do NOT use this command to fetch a fresh ship by IMO; use 'ship get' instead. Do NOT use this command to filter by flag or type; use 'ship list' instead. |
| 7 | Session liveness ping | `auth ping` | hand-code | Single fast GET to /Public/SHIPS/Default.aspx; checks for WebLogin redirect; exits 0/non-0 so users wire their own cron/launchd. Targets the ASP.NET session-timeout pain that's intrinsic to this API | none |
