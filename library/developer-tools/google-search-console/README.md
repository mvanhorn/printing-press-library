# Google Search Console CLI

**Every Search Console feature, plus a local SQLite store that turns one-shot API calls into time-series insights no other GSC tool ships.**

Sync your search analytics, sites, sitemaps, and URL inspection state into a local store, then run cross-time queries the live API cannot express: query decay, keyword cannibalization, coverage drift, opportunity-with-baseline, and a cross-property mover board. Every command supports --json, --select, --csv, --dry-run, and ships an MCP server out of the box.

Learn more at [Google Search Console](https://google.com).

## Install

The recommended path installs both the `google-search-console-pp-cli` binary and the `pp-google-search-console` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install google-search-console
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install google-search-console --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/cmd/google-search-console-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-search-console-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-search-console --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-search-console --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-google-search-console skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-google-search-console. The skill defines how its required CLI can be installed.
```

## Authentication

OAuth2 with the `webmasters` (or `webmasters.readonly`) scope. Two paths to a token:

1. **Interactive (recommended):** run `google-search-console-pp-cli auth login` and the CLI walks you through the OAuth2 flow, then stores and refreshes tokens for you. `auth status` confirms it works; `auth logout` removes stored tokens.
2. **Headless / CI:** mint an access token elsewhere and set `GOOGLE_SEARCH_CONSOLE_OAUTH2C` in env. Easiest source for local dev:

   ```bash
   gcloud auth login
   export GOOGLE_SEARCH_CONSOLE_OAUTH2C=$(gcloud auth print-access-token)
   ```

   The token must come from a Google account that owns or has been granted access to at least one Search Console property in a project where the Search Console API is enabled. Tokens minted by `gcloud` expire in roughly an hour; re-export when you see a 401.

## Quick Start

```bash
# Confirm the CLI can mint an access token via gcloud or service account before anything else.
google-search-console-pp-cli auth status


# List your verified properties so you have a site URL to point sync at.
google-search-console-pp-cli webmasters sites-list --json


# Backfill 90 days of search-analytics rows and current url-inspection state into the local SQLite store.
google-search-console-pp-cli sync --site sc-domain:example.com --backfill 90d


# Pages on positions 4-20 with high impressions and low CTR that entered the opportunity zone within the last 14 days.
google-search-console-pp-cli opportunity --site sc-domain:example.com --new-since 14d --agent


# Cross-property mover board: largest click deltas this week across every verified site, with a per-site rollup.
google-search-console-pp-cli book --window 7d --top 25 --agent

```

## Known Gaps

- **No automatic cache-freshness signal.** The local store doesn't expose a `--max-age` / staleness flag on read commands; you decide when to re-run `sync`. If you query immediately after a long gap, results reflect whatever was last synced, not "now."
- **Search-appearance dimension requires a separate sync pass.** The default `sync` doesn't backfill `searchAppearance` rows; the `appearance` command will emit an empty result with a hint until you sync that dimension explicitly.
- **No multi-step orchestration commands.** Each command does one thing; chaining (e.g., "sync, then run opportunity, then post the top 5 to Slack") is left to the agent or shell. The MCP surface mirrors the CLI tree without higher-level intent tools.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-property and time-series leverage
- **`book`** — One report covering every verified property: top pages and queries by absolute click delta in the last window, with per-site rollup rows.

  _Reach for this whenever you need to compare performance across multiple Search Console properties without N round trips._

  ```bash
  google-search-console-pp-cli book --window 7d --top 25 --agent
  ```
- **`momentum`** — Pages whose 7-day clicks exceed (or fall below) their trailing 28-day daily average by N×, sorted by absolute lift.

  _Spot rising stars and collapsing pages in one pass instead of eyeballing the GSC UI's compare mode._

  ```bash
  google-search-console-pp-cli momentum --site sc-domain:example.com --window 7d --vs 28d --agent
  ```
- **`territory`** — For each query, change in country and device mix week-over-week; flags queries whose geo or device split moved more than N points.

  _Diagnose 'why did our US traffic drop' or 'mobile shift' questions in one command._

  ```bash
  google-search-console-pp-cli territory --site sc-domain:example.com --by country,device --agent
  ```

### Local-store joins
- **`cannibalize`** — Find queries where 2+ pages on the same site rank in the top 20 competing for the same intent; reports impression split and CTR drag.

  _Use when planning content consolidation or canonical fixes — surfaces internal SERP competition the GSC UI hides._

  ```bash
  google-search-console-pp-cli cannibalize --site sc-domain:example.com --top 20 --min-impressions 100 --agent
  ```
- **`decay`** — Queries whose impressions or clicks have steadily declined over a rolling N-week window, ranked by absolute click loss.

  _Catch gradual ranking erosion before it becomes a missing-traffic emergency._

  ```bash
  google-search-console-pp-cli decay --site sc-domain:example.com --window 12w --min-loss 100 --agent
  ```
- **`new-queries`** — Queries that appeared this week with material impressions and were absent in the prior trailing window; --lost inverts.

  _Aki's 'what's new since my last visit' loop without burning quota on full re-pulls._

  ```bash
  google-search-console-pp-cli new-queries --site sc-domain:example.com --window 7d --min-impressions 50 --agent
  ```
- **`triage`** — Non-INDEXED pages from url_inspections joined against the last 30 days of impressions, ranked by traffic lost.

  _Fix the broken pages that actually drive traffic, not whatever was inspected most recently._

  ```bash
  google-search-console-pp-cli triage --site sc-domain:example.com --by impact --agent
  ```

### Snapshot diffs
- **`coverage-drift`** — Diff successive URL-inspection snapshots: pages whose coverageState, lastCrawlTime, or googleCanonical changed since last sync.

  _Spot silent deindexings within hours of the next sync instead of weeks later from a stakeholder._

  ```bash
  google-search-console-pp-cli coverage-drift --site sc-domain:example.com --since last-sync --agent
  ```
- **`opportunity`** — Today's quick-wins (positions 4-20, high-impression / low-CTR) joined to prior daily snapshots so each row carries the date the page entered the opportunity zone.

  _Lets agents distinguish fresh opportunities (worth a refresh) from chronic ones (already triaged)._

  ```bash
  google-search-console-pp-cli opportunity --site sc-domain:example.com --new-since 14d --agent
  ```
- **`appearance`** — Per-page or per-query breakdown of clicks/impressions by searchAppearance (featured snippet, sitelinks, video, FAQ rich result), with gain/loss flags across windows.

  _Catch lost rich-result eligibility (featured snippets, sitelinks) before traffic drops._

  ```bash
  google-search-console-pp-cli appearance --site sc-domain:example.com --window 28d --vs prior --agent
  ```
- **`sitemap-health`** — Cross-snapshot diff on per-property sitemap state; flags new errors, new warnings, and indexed-vs-submitted ratio drops.

  _Surface sitemap regressions in your weekly status email instead of next quarter's audit._

  ```bash
  google-search-console-pp-cli sitemap-health --regressed --agent
  ```

## Usage

Run `google-search-console-pp-cli --help` for the full command reference and flag list.

## Commands

### url-inspection

Manage url inspection

- **`google-search-console-pp-cli webmasters sites-list inspect`** - Index inspection.

### url-testing-tools

Manage url testing tools

- **`google-search-console-pp-cli url-testing-tools run`** - Runs Mobile-Friendly Test for a given URL.

### webmasters

Manage webmasters

- **`google-search-console-pp-cli webmasters searchanalytics-query`** - Query your data with filters and parameters that you define. Returns zero or more rows grouped by the row keys that you define. You must define a date range of one or more days. When date is one of the group by values, any days without data are omitted from the result list. If you need to know which days have data, issue a broad date range query grouped by date for any metric, and see which day rows are returned.
- **`google-search-console-pp-cli webmasters sitemaps-delete`** - Deletes a sitemap from the Sitemaps report. Does not stop Google from crawling this sitemap or the URLs that were previously crawled in the deleted sitemap.
- **`google-search-console-pp-cli webmasters sitemaps-get`** - Retrieves information about a specific sitemap.
- **`google-search-console-pp-cli webmasters sitemaps-list`** - Lists the [sitemaps-entries](/webmaster-tools/v3/sitemaps) submitted for this site, or included in the sitemap index file (if `sitemapIndex` is specified in the request).
- **`google-search-console-pp-cli webmasters sitemaps-submit`** - Submits a sitemap for a site.
- **`google-search-console-pp-cli webmasters sites-add`** - Adds a site to the set of the user's sites in Search Console.
- **`google-search-console-pp-cli webmasters sites-delete`** - Removes a site from the set of the user's Search Console sites.
- **`google-search-console-pp-cli webmasters sites-get`** - Retrieves information about specific site.
- **`google-search-console-pp-cli webmasters sites-list`** - Lists the user's Search Console sites.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-search-console-pp-cli webmasters sites-list

# JSON for scripting and agents
google-search-console-pp-cli webmasters sites-list --json

# Filter to specific fields
google-search-console-pp-cli webmasters sites-list --json --select id,name,status

# Dry run — show the request without sending
google-search-console-pp-cli webmasters sites-list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-search-console-pp-cli webmasters sites-list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-google-search-console -g
```

Then invoke `/pp-google-search-console <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/cmd/google-search-console-pp-mcp@latest
```

Then register it:

```bash
claude mcp add google-search-console google-search-console-pp-mcp -e GOOGLE_SEARCH_CONSOLE_OAUTH2C=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-search-console-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_SEARCH_CONSOLE_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/google-search-console/cmd/google-search-console-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-search-console": {
      "command": "google-search-console-pp-mcp",
      "env": {
        "GOOGLE_SEARCH_CONSOLE_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
google-search-console-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-search-console-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOGLE_SEARCH_CONSOLE_OAUTH2C` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-search-console-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLE_SEARCH_CONSOLE_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 'Login Required' on every call** — Run `gcloud auth login` then `gcloud auth print-access-token` to confirm a token mints; the CLI shells out to the same command.
- **403 'User does not have sufficient permissions'** — Make sure the authed account is verified on the property in Search Console, or pass `--credentials` to a service account that is.
- **429 quota errors during sync** — Retry with `--throttle slow`; default quotas are 1,200 QPM per project, 30,000 QPD.
- **`searchanalytics query --dimensions searchAppearance,query` returns 400** — `searchAppearance` is mutually exclusive with all other dimensions in a single call; request it alone or use the `appearance` command which handles the split.
- **Sync fills in zero rows for the last 2-3 days** — GSC search analytics finalize ~3 days late; sync only writes through `today-3` by default. Use `--data-state all` to include preliminary rows.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Bin-Huang/google-search-console-cli**](https://github.com/Bin-Huang/google-search-console-cli) — JavaScript
- [**ahonn/mcp-server-gsc**](https://github.com/ahonn/mcp-server-gsc) — TypeScript
- [**AminForou/mcp-gsc**](https://github.com/AminForou/mcp-gsc) — Python
- [**joshcarty/google-searchconsole**](https://github.com/joshcarty/google-searchconsole) — Python
- [**ncosentino/google-search-console-mcp**](https://github.com/ncosentino/google-search-console-mcp) — C#
- [**saurabhsharma2u/search-console-mcp**](https://github.com/saurabhsharma2u/search-console-mcp) — TypeScript
- [**ivankristianto/google-search-console-cli**](https://github.com/ivankristianto/google-search-console-cli) — PHP
- [**kasdimg/analytics-cli**](https://github.com/kasdimg/analytics-cli) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
