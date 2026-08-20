## Absorb Manifest (Step 1.5b - pre-novel-features)

Greenfield API — no existing GISIS-specific CLI/MCP found on GitHub (only tangential vessel scrapers). Adjacent maritime DD reference: `rhinonix/equasis-cli` (17 stars, v2.0.0). Vessel data MCP reference: `vessel-api/vesselapi-mcp` (commercial AIS, not authoritative registry). Both inform the table-stakes UX, neither is a direct competitor.

### Absorbed (match or beat every feature competing tools have)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Ship lookup by IMO number | equasis-cli `vessel /imo` + VesselAPI MCP `vessel by IMO` | gisis-pp-cli ship get <imo> | Authoritative IMO source (vs. equasis paid; vs. VesselAPI commercial); JSON/CSV/table; MCP-callable |
| 2 | JSON / CSV / table output | equasis-cli | (behavior in gisis-pp-cli ship get) | Standard press --json/--csv/--select/--compact across all commands |
| 3 | Persistent credential store | equasis-cli `~/.config/equasis-cli/credentials.json` | (behavior in gisis-pp-cli auth) | press-auth companion (keychain-backed cookie store), one-command setup; survives Cloudflare Turnstile |
| 4 | Polite throttle baseline | equasis-cli 1s + exponential backoff | (behavior in gisis-pp-cli client) | 1 req/2-3s default (per execution log), AdaptiveLimiter from press cliutil |
| 5 | Local SQLite cache | equasis-cli | (generated endpoint) ship get | press default; ships accumulate as queried; FTS on name/owner |
| 6 | MCP tool exposure | VesselAPI MCP `vessel` tools | (behavior in cobratree mirror) | Auto-generated; `gisis_ship_get` available to orchestrator + Claude Desktop |
| 7 | Health check (auth + reachability) | equasis-cli implicit | gisis-pp-cli doctor | Verifies cookies present, session live (probe-ping returns non-WebLogin) |

(No deferred-stub items in v1: ship search [VIEWSTATE postback], casualty *, company * are explicitly out of scope for v1 and added later via /printing-press-amend.)
