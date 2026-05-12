# AppsFlyer Absorb Manifest (Trimmed Scope)

User chose trim-scope at the Phase Gate 1.5. They want a CLI focused on ad-hoc
pull workflows; will expand scope after live use. Plan tier caps Pull API at
20 calls/day, so the rate budget is the binding constraint.

## Source Tools Surveyed

| # | Tool | Type | Surface |
|---|------|------|---------|
| 1 | AppsFlyer BETA MCP (loaded) | MCP | 16 tools — strong agent ground truth |
| 2 | `ysntony/appsflyer-mcp` | MCP | Aggregate-only, 2 tools |
| 3 | `Kachit/appsflyer-sdk-go` | Go SDK | Raw data only; stale v0.0.2 (Sept 2020) |
| 4 | `singer-io/tap-appsflyer` | Singer ETL | Raw installs + in-app events |
| 5 | Airbyte / Fivetran connectors | Managed ETL | Nightly EL into warehouses |
| 6 | `fredericojordan/appsflyer-python` | Python | S2S events only |
| 7 | `YuriyOrlov/pyappsflyer` (stale) | Python | daily_report |
| 8 | AppsFlyer Dashboard | Web | Master/Cohort/Raw/SKAN single-app pivot |

## Absorbed (match or beat what's worth absorbing)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Aggregate partners report | AppsFlyer Pull V2 | `agg partners --app-id X --from Y --to Z` | --json/--csv/--select, fanout, rate-budget aware |
| 2 | Aggregate daily report | AppsFlyer Pull V2 | `agg daily` | Same |
| 3 | Aggregate geo report | AppsFlyer Pull V2 | `agg geo` | Same |
| 4 | Master API combined dims | AppsFlyer Master V2 | `master report --groupings X --kpis Y` | Friendly source-name resolution |
| 5 | Cohort D1/D3/D7/D30 | AppsFlyer Cohort V1 | `cohort data --cohort-size d7 --exclude-partial` | `--exclude-partial` defaults TRUE per catalog notes |
| 6 | Raw installs export | Pull V2 | `raw installs --from Y --to Z --out file.csv` | Single-call, CSV stream-to-file |
| 7 | Raw in-app events export | Pull V2 | `raw events --event-name purchase` | Event filter |
| 8 | SKAN aggregated install-date | SKAN V1 | `skan data --date-type install` | Default `--to` = yesterday - 2d (lag-aware) |
| 9 | SKAN postback-arrival | SKAN V1 | `skan data --date-type arrival` | Same lag-aware default |
| 10 | List apps | BETA MCP `get_apps` | `apps list` | Local cache → offline list |
| 11 | Media-source canonical/display catalog | BETA MCP | `sources` | 147+24 sources, FTS by display name → `_int` ID |
| 12 | Doctor / connectivity | (none across competitors) | `doctor` | Dotenv check + token validity + per-family permission probe + rate-budget remaining |
| 13 | Local SQLite cache | (only ETL connectors) | framework `sync` | Inline, agent-callable, rate-budget-aware |
| 14 | SQL passthrough | (none) | `sql "SELECT ..."` | Direct SQLite query |
| 15 | FTS search | (none) | `search "<query>"` | FTS5 over apps + campaigns + media-source names |
| 16 | Config get/set/list | (none) | `config get/set/list` | XDG path, channel-group YAML editable |

## Transcendence (this CLI's wedge)

| # | Feature | Command | Description | Score | Group |
|---|---------|---------|-------------|---|---|
| 1 | Yesterday / WTD / MTD standup | `standup` | Cross-app pivot showing yesterday vs WTD vs MTD ROAS, spend, installs — optionally grouped by channel-group | 9/10 | Morning workflow |
| 2 | General-purpose pull facade | `pull` | One command, rich flags: `--from`, `--to`, `--source`, `--campaign`, `--channel-group`, `--breakdown`, `--metrics`, `--currency`, `--timezone` — routes to the right underlying endpoint | 9/10 | Ad-hoc workflow |

**Status:** every entry is shipping-scope. No stubs.

**Foundation features that ship implicitly:**
- joho/godotenv dotenv loading at `~/.config/appsflyer-pp-cli/.env`
- Bearer-token auth with sensitive redaction
- Rate-limit-aware client (configurable `calls_per_day`, default 20 to match the
  user's plan, with `Retry-After`-aware backoff and budget tracking)
- Channel-group resolver from `~/.config/appsflyer-pp-cli/channels.yaml`
  (default mapping covers social/programmatic/oem/rewarded)
- Region routing (default global; eu/cn left as config knobs pending sniff)

**Deferred for v0.2 (user-stated reason: "Once I understand my usage more I
will add to the scope"):**
- forecast d30, anomalies sweep, replay, freshness, payback rank
- cohort drift, channels drift, coverage restricted, skan reconcile
- audiences, OneLink, integrations commands

These move from approved to deferred at the user's direction. They are NOT
stubs and NOT in `novel_features_built` — they're simply not on the v0.1
roadmap. The user will request them when their workflow is stable.
