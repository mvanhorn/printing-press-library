# AppsFlyer CLI

**Ad-hoc pulls from AppsFlyer's V2 API, rate-budget aware: one facade command for any breakdown, a morning standup across all your apps.**

Two power commands on top of the V2 surface. `pull` gives you a single facade over the Aggregate Pull V2 reports with friendly source names and channel-group rollups. `standup` pivots yesterday / WTD / MTD across all apps in your account under the Pull API daily budget. Built for analysts who want fast answers without thinking about which AppsFlyer endpoint to call. (For Master, Cohort, SKAN, or Raw Data pulls, use the typed `master`, `cohort`, `skan`, and `raw` subcommands directly.)

## Install

The recommended path installs both the `appsflyer-pp-cli` binary and the `pp-appsflyer` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install appsflyer
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install appsflyer --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/appsflyer-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-appsflyer --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-appsflyer --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-appsflyer skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-appsflyer. The skill defines how its required CLI can be installed.
```

## Authentication

AppsFlyer V2 uses a single account-level Bearer token (Security Center → AppsFlyer API tokens). The CLI loads APPSFLYER_API_TOKEN from ~/.config/appsflyer-pp-cli/.env via joho/godotenv; process env wins over the file. Token scopes (Master, Cohort, SKAN, Raw Data) depend on your subscription — appsflyer-pp-cli doctor probes each family. The CLI also tracks your Pull API daily-call budget (configurable, default 20) so you never blow the cap mid-day.

## Quick Start

```bash
# Verify token loaded from .env; probe which report families your subscription entitles; show remaining Pull API budget.
appsflyer-pp-cli doctor


# List AppsFlyer apps in the account — cached to local store.
appsflyer-pp-cli apps list


# Yesterday / WTD / MTD ROAS, spend, installs across all apps.
appsflyer-pp-cli standup --app-id id123456 --app-id id654321 --json


# Ad-hoc pull with friendly source names and channel-group support.
appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --source facebook --breakdown campaign --metrics installs,revenue,roas --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Morning workflow
- **`standup`** — Cross-app pivot showing yesterday vs week-to-date vs month-to-date ROAS, spend, and installs — optionally grouped by channel-group (social, programmatic, OEM, rewarded).

  _Run this every morning to see how the portfolio's paid spend is performing without scrolling N dashboards._

  ```bash
  appsflyer-pp-cli standup --app-id id123456 --app-id id654321 --json
  ```

### Ad-hoc workflow
- **`pull`** — One command, rich flags. Specify date range, media source (canonical or friendly), channel group, campaign, breakdown, metrics, currency, and timezone — the CLI routes to the right underlying AppsFlyer endpoint and applies friendly-name resolution.

  _Use this when you have a specific ad-hoc question (e.g. 'how did Facebook campaigns perform last week by campaign?') without thinking about which AppsFlyer endpoint to call._

  ```bash
  appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --source facebook --breakdown campaign --json
  ```

## Usage

Run `appsflyer-pp-cli --help` for the full command reference and flag list.

## Commands

### agg

Aggregate Pull API V2 — campaign-day-source breakdowns

- **`appsflyer-pp-cli agg daily`** - Daily report — aggregate across all media sources by day
- **`appsflyer-pp-cli agg geo`** - Geo report — country breakdown
- **`appsflyer-pp-cli agg geo_by_date`** - Geo-by-date — country breakdown with daily granularity
- **`appsflyer-pp-cli agg partners`** - Partners report — media-source breakdown for date range
- **`appsflyer-pp-cli agg partners_by_date`** - Partners-by-date — daily granularity per media source

### apps

AppsFlyer apps in this account

- **`appsflyer-pp-cli apps list`** - List apps registered to the account

### cohort

Cohort API V1 — D1/D3/D7/D30/D90 retention + LTV by cohort

- **`appsflyer-pp-cli cohort data`** - Cohort retention + LTV. Body controls cohort length, KPIs, groupings, partial_data.

### master

Master API V2 — combined dimensions in one call

- **`appsflyer-pp-cli master report`** - Master API V2 — combined groupings + KPIs per request

### raw

Raw Data Pull API V2 — per-install / per-event CSV exports

- **`appsflyer-pp-cli raw in_app_events`** - Per-event raw report (CSV)
- **`appsflyer-pp-cli raw installs`** - Per-install raw report (CSV)
- **`appsflyer-pp-cli raw organic_in_app_events`** - Organic per-event raw report (CSV)
- **`appsflyer-pp-cli raw organic_installs`** - Per-install organic raw report (CSV)
- **`appsflyer-pp-cli raw uninstalls`** - Per-uninstall raw report (CSV)

### skan

SKAdNetwork API V1 — aggregate SKAN data, install-date and postback-arrival

- **`appsflyer-pp-cli skan data`** - SKAN aggregated install-date report. Note: SKAN data lags ~2 days.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
appsflyer-pp-cli apps

# JSON for scripting and agents
appsflyer-pp-cli apps --json

# Filter to specific fields
appsflyer-pp-cli apps --json --select id,name,status

# Dry run — show the request without sending
appsflyer-pp-cli apps --dry-run

# Agent mode — JSON + compact + no prompts in one flag
appsflyer-pp-cli apps --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-appsflyer -g
```

Then invoke `/pp-appsflyer <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add appsflyer appsflyer-pp-mcp -e APPSFLYER_API_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/appsflyer-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `APPSFLYER_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "appsflyer": {
      "command": "appsflyer-pp-mcp",
      "env": {
        "APPSFLYER_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Cookbook

### Morning standup, agent-friendly subset

```bash
appsflyer-pp-cli standup --app-id id123456 --app-id id654321 --agent --select apps[].app_id,apps[].yesterday.roas,apps[].wtd.roas,apps[].mtd.roas
```

Pull the cross-app standup scoped to just the ROAS fields a triage agent needs across three time windows.

### Last-week campaign performance for one channel

```bash
appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --source facebook --breakdown campaign --metrics installs,revenue,roas --json
```

Ad-hoc pull with friendly source resolution (facebook → facebook_int).

### Channel-group rollup across the portfolio

```bash
appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --channel-group social --breakdown media_source --metrics installs,revenue --json
```

Roll up Meta + TikTok + Snap + Reddit + Pinterest + X into one social group using the channels.yaml mapping.

### Doctor with full diagnostic JSON

```bash
appsflyer-pp-cli doctor --json --probe-families
```

Returns dotenv path, token fingerprint, remaining Pull API budget, and per-family entitlement (200/401/403) when `--probe-families` is set (each probe consumes one daily-budget call).

### Resolve a friendly source name to canonical

```bash
appsflyer-pp-cli sources resolve tiktok
```

Maps TikTok → tiktokglobal_int so you can paste the canonical ID into other tools.

## Health Check

```bash
appsflyer-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/appsflyer-pp-cli/config.yaml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `APPSFLYER_API_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `appsflyer-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $APPSFLYER_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports 403 on cohort or master reports** — Your AppsFlyer subscription doesn't entitle that family. Contact your AppsFlyer CSM or run `appsflyer-pp-cli doctor --probe-families --json` to see the per-family breakdown (each probe consumes one daily-budget call).
- **Daily call budget exhausted before end of day** — Edit `~/.config/appsflyer-pp-cli/config.yaml` and bump `calls_per_day` if your plan allows; otherwise schedule the next sync after midnight UTC.
- **Cohort rows missing or marked partial** — By default `cohort data` excludes partial rows. Pass `--include-partial` to see them.
- **ROAS in CSV shows 0.50 instead of 50%** — AppsFlyer returns percentage metrics as decimals. The CLI's table renderer auto-converts; `--csv` is passthrough — use `--json` if you want post-conversion floats.
- **SKAN data missing for the last 2 days** — SKAN reports lag ~2 days for postback arrivals; the `skan data` command defaults --to to yesterday - 2d. Override with --to YYYY-MM-DD if you really want the partial window.
- **Channel-group flag value not recognized** — Edit `~/.config/appsflyer-pp-cli/channels.yaml` to add the group; the default mapping covers social, programmatic, OEM, and rewarded.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ysntony/appsflyer-mcp**](https://github.com/ysntony/appsflyer-mcp) — TypeScript
- [**Kachit/appsflyer-sdk-go**](https://github.com/Kachit/appsflyer-sdk-go) — Go
- [**singer-io/tap-appsflyer**](https://github.com/singer-io/tap-appsflyer) — Python
- [**fredericojordan/appsflyer-python**](https://github.com/fredericojordan/appsflyer-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
