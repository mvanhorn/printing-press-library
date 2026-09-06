# Bing Webmaster Tools CLI

**Every Bing Webmaster API endpoint, plus the SEO intelligence no other Bing tool ships: period-over-period deltas, ranking drift, quota-aware bulk submission, and Bing-vs-Google reconciliation — all backed by a local SQLite store.**

The most complete Bing Webmaster CLI: all 60 documented API methods as composable commands, with offline FTS search, read-only SQL, and agent-native output. On top of raw endpoints it layers what SEOs actually need — `review` for weekly query deltas, `drift` for ranking movement, `publish` for quota-paced bulk indexing, `triage` for crawl-error prioritization, and `gap` to reconcile Bing against Google Search Console.

Learn more at [Bing Webmaster Tools](https://www.bing.com/webmasters).

Printed by [@Pimmetjeoss](https://github.com/Pimmetjeoss) (Pimmetjeoss).

## Install

The recommended path installs both the `bing-webmaster-pp-cli` binary and the `pp-bing-webmaster` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install bing-webmaster
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install bing-webmaster --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/cmd/bing-webmaster-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bing-webmaster-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bing-webmaster --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bing-webmaster --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-bing-webmaster skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-bing-webmaster. The skill defines how its required CLI can be installed.
```

## Authentication

Authentication uses a single Bing Webmaster API key passed as the `apikey` query parameter. The key is per-user (it covers every site you've verified), not per-site. Get it at bing.com/webmasters → Settings → API Access → Generate API Key, then set it as the BING_WEBMASTER_API_KEY environment variable. Run `doctor` to confirm the key is valid and the API is reachable.

## Quick Start

```bash
# Confirm BING_WEBMASTER_API_KEY is set and the API is reachable before anything else.
bing-webmaster-pp-cli doctor


# List the sites your key can manage and pick a default.
bing-webmaster-pp-cli sites list


# Pull current top query stats for a site (may be empty below Bing's data threshold).
bing-webmaster-pp-cli traffic queries --site https://example.com


# Capture a baseline and see what changed in query performance — run again later to get deltas.
bing-webmaster-pp-cli review --site https://example.com --days 7


# Check remaining submission quota before a bulk push.
bing-webmaster-pp-cli quota --site https://example.com

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`review`** — See which Bing queries you gained or lost, and how CTR and average position shifted, vs the previous period — not just a raw snapshot.

  _Reach for this for a weekly SEO standup instead of eyeballing raw GetQueryStats; it tells the agent what actually changed._

  ```bash
  bing-webmaster-pp-cli review --site https://example.com --days 7 --agent
  ```
- **`drift`** — Track average-position movement per query and page over time and surface the biggest climbers and droppers.

  _Use when an agent must explain why traffic moved — drift points at the exact queries/pages that re-ranked._

  ```bash
  bing-webmaster-pp-cli drift --site https://example.com --top 20 --agent
  ```
- **`feed-health`** — Track submitted, discovered, and indexed counts for each feed over time and flag drops.

  _Use to catch a sitemap that silently stopped being indexed before it tanks traffic._

  ```bash
  bing-webmaster-pp-cli feed-health --site https://example.com --agent
  ```
- **`watch`** — Diff the latest sync against the previous one and surface indexation, crawl, and impression regressions per site.

  _Run after each scheduled sync so the agent gets a single 'what regressed' digest instead of re-reading everything._

  ```bash
  bing-webmaster-pp-cli watch --site https://example.com --agent
  ```

### Submission at scale
- **`publish`** — Submit many URLs (or a whole sitemap) for indexing, automatically chunked to the 500-per-request cap, paced against your live remaining quota, and deduped against URLs already submitted.

  _Pick this over raw SubmitUrlBatch whenever submitting more than a handful of URLs; it won't blow the daily quota or resubmit._

  ```bash
  bing-webmaster-pp-cli publish --site https://example.com --from-sitemap https://example.com/sitemap.xml --dry-run
  ```
- **`quota`** — One view of URL and content submission quota — daily and monthly remaining — plus a pacing recommendation.

  _Check before any bulk submission so the agent knows how many URLs it can push today without hitting the wall._

  ```bash
  bing-webmaster-pp-cli quota --site https://example.com --agent
  ```

### Operational triage
- **`triage`** — Categorize crawl issues by severity, diff them against your last sync, and map each issue to the affected child URLs in one view.

  _Use to turn a raw GetCrawlIssues dump into a prioritized, what-changed-since-last-time worklist._

  ```bash
  bing-webmaster-pp-cli triage --site https://example.com --agent
  ```

### Cross-engine intelligence
- **`gap`** — Reconcile your Bing query/page performance against a Google Search Console export to find queries and pages you rank for on one engine but not the other.

  _Use to find low-effort wins: pages already strong on Google that are missing from Bing (and vice-versa)._

  ```bash
  bing-webmaster-pp-cli gap --site https://example.com --gsc ./gsc-queries.csv --agent
  ```

## Usage

Run `bing-webmaster-pp-cli --help` for the full command reference and flag list.

## Commands

### blocked

Block URLs from the index and manage page-preview blocks

- **`bing-webmaster-pp-cli blocked add`** - Block a page or directory from the index
- **`bing-webmaster-pp-cli blocked add-preview-block`** - Block the page preview for a URL
- **`bing-webmaster-pp-cli blocked list`** - List blocked pages and directories
- **`bing-webmaster-pp-cli blocked preview-blocks`** - List active page-preview blocks
- **`bing-webmaster-pp-cli blocked remove`** - Unblock a previously blocked page or directory
- **`bing-webmaster-pp-cli blocked remove-preview-block`** - Remove a page-preview block

### crawl

Read crawl stats and issues, and manage crawl settings

- **`bing-webmaster-pp-cli crawl children-info`** - Index details for the pages under a directory
- **`bing-webmaster-pp-cli crawl issues`** - Crawl issues found for a site
- **`bing-webmaster-pp-cli crawl save-settings`** - Save crawl settings for a site
- **`bing-webmaster-pp-cli crawl settings`** - Get crawl settings for a site
- **`bing-webmaster-pp-cli crawl stats`** - Daily crawl statistics for a site
- **`bing-webmaster-pp-cli crawl url-info`** - Index details for a single page

### deeplinks

Deep link blocks (several get/update methods are obsolete in the Bing API)

- **`bing-webmaster-pp-cli deeplinks add-block`** - Block a deep link
- **`bing-webmaster-pp-cli deeplinks algo-urls`** - [OBSOLETE in Bing API] Get algorithmic deep link URLs
- **`bing-webmaster-pp-cli deeplinks blocks`** - List deep link blocks for a site
- **`bing-webmaster-pp-cli deeplinks get`** - [OBSOLETE in Bing API] Get deep links for a URL
- **`bing-webmaster-pp-cli deeplinks remove-block`** - Remove a deep link block
- **`bing-webmaster-pp-cli deeplinks update`** - [OBSOLETE in Bing API] Update a deep link weight

### feeds

Manage submitted sitemaps and feeds

- **`bing-webmaster-pp-cli feeds details`** - Get details for a feed or sitemap index
- **`bing-webmaster-pp-cli feeds list`** - List all top-level feeds (sitemaps) for a site
- **`bing-webmaster-pp-cli feeds remove`** - Remove a submitted feed
- **`bing-webmaster-pp-cli feeds submit`** - Submit a sitemap or feed URL

### geo

Country/region (geo) targeting settings (a feature Google Search Console removed)

- **`bing-webmaster-pp-cli geo add`** - Add a country/region targeting setting
- **`bing-webmaster-pp-cli geo remove`** - Remove a country/region targeting setting
- **`bing-webmaster-pp-cli geo settings`** - List country/region targeting settings

### keywords

Keyword research (Bing-unique; takes query/country/language, not a site)

- **`bing-webmaster-pp-cli keywords get`** - Keyword impressions for a period
- **`bing-webmaster-pp-cli keywords related`** - Keywords related to a query for a period
- **`bing-webmaster-pp-cli keywords stats`** - Historical stats for a keyword

### links

Inbound link analytics and connected pages

- **`bing-webmaster-pp-cli links add-connected`** - Add a connected page to the site
- **`bing-webmaster-pp-cli links connected`** - List pages connected to the site
- **`bing-webmaster-pp-cli links counts`** - Pages with inbound link counts (paged)
- **`bing-webmaster-pp-cli links url-links`** - Inbound links for a specific URL (paged)

### params

URL normalization query-parameter rules (a feature Google Search Console removed)

- **`bing-webmaster-pp-cli params add`** - Add a query-parameter normalization rule
- **`bing-webmaster-pp-cli params list`** - List query-parameter normalization rules
- **`bing-webmaster-pp-cli params remove`** - Remove a query-parameter rule
- **`bing-webmaster-pp-cli params toggle`** - Enable or disable a query-parameter rule

### sites

Manage and inspect the sites your API key controls

- **`bing-webmaster-pp-cli sites add`** - Add a new site to your account
- **`bing-webmaster-pp-cli sites add-role`** - Delegate site access to another user
- **`bing-webmaster-pp-cli sites list`** - List all verified sites for the current user
- **`bing-webmaster-pp-cli sites moves`** - List submitted site moves (migrations) for a site
- **`bing-webmaster-pp-cli sites remove`** - Remove a site from your account
- **`bing-webmaster-pp-cli sites remove-role`** - Revoke a user's delegated access to a site
- **`bing-webmaster-pp-cli sites roles`** - Get delegated user roles for a site
- **`bing-webmaster-pp-cli sites submit-move`** - Submit a site migration
- **`bing-webmaster-pp-cli sites verify`** - Attempt verification of a site you have added

### submission

Submit URLs and content to Bing for indexing, and check quotas

- **`bing-webmaster-pp-cli submission content-quota`** - Get content submission quota
- **`bing-webmaster-pp-cli submission fetch`** - Request that Bing fetch a single URL
- **`bing-webmaster-pp-cli submission fetched`** - List URLs Bing has fetched on request
- **`bing-webmaster-pp-cli submission fetched-details`** - Get details of a fetched URL
- **`bing-webmaster-pp-cli submission submit-batch`** - Submit up to 500 URLs for indexing in one request
- **`bing-webmaster-pp-cli submission submit-content`** - Submit page content directly to Bing
- **`bing-webmaster-pp-cli submission submit-url`** - Submit a single URL for indexing
- **`bing-webmaster-pp-cli submission url-quota`** - Get daily and monthly URL submission quota

### traffic

Read query, page, and ranking/traffic statistics

- **`bing-webmaster-pp-cli traffic children-traffic`** - Index and traffic info for the pages under a directory
- **`bing-webmaster-pp-cli traffic page-queries`** - Queries that lead to a given page
- **`bing-webmaster-pp-cli traffic pages`** - Top pages for a site
- **`bing-webmaster-pp-cli traffic queries`** - Top query stats for a site (may be empty below Bing's data threshold)
- **`bing-webmaster-pp-cli traffic query-page-detail`** - Detailed stats for a specific query and page pair
- **`bing-webmaster-pp-cli traffic query-pages`** - Pages that rank for a given query
- **`bing-webmaster-pp-cli traffic query-traffic`** - Traffic stats for a single query
- **`bing-webmaster-pp-cli traffic rank-traffic`** - Daily impressions and clicks for a site
- **`bing-webmaster-pp-cli traffic url-traffic`** - Index and traffic info for a single page


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bing-webmaster-pp-cli blocked list --site https://example.com/resource

# JSON for scripting and agents
bing-webmaster-pp-cli blocked list --site https://example.com/resource --json

# Filter to specific fields
bing-webmaster-pp-cli blocked list --site https://example.com/resource --json --select id,name,status

# Dry run — show the request without sending
bing-webmaster-pp-cli blocked list --site https://example.com/resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bing-webmaster-pp-cli blocked list --site https://example.com/resource --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-bing-webmaster -g
```

Then invoke `/pp-bing-webmaster <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/cmd/bing-webmaster-pp-mcp@latest
```

Then register it:

```bash
claude mcp add bing-webmaster bing-webmaster-pp-mcp -e BING_WEBMASTER_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bing-webmaster-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BING_WEBMASTER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/cmd/bing-webmaster-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bing-webmaster": {
      "command": "bing-webmaster-pp-mcp",
      "env": {
        "BING_WEBMASTER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
bing-webmaster-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/bing-webmaster-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BING_WEBMASTER_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bing-webmaster-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BING_WEBMASTER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports InvalidApiKey** — Regenerate the key at bing.com/webmasters → Settings → API Access and re-export BING_WEBMASTER_API_KEY; the key is per-user and only one is active at a time.
- **Query stats come back empty** — This is expected when a site is below Bing's data threshold — it is valid, not an error. Verify with a higher-traffic site or wait for more data.
- **URL submission returns HTTP 403** — Usually upstream WAF/Bingbot blocking rather than the API. Confirm Bingbot is allowed in robots.txt and not blocked by your CDN/WAF.
- **Submission rejected for quota** — Run `quota` to see remaining daily/monthly allowance; new sites get far less than the 10,000/day headline. Quota resets at midnight GMT.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**merj/bing-webmaster-tools**](https://github.com/merj/bing-webmaster-tools) — Python (20 stars)
- [**isiahw1/mcp-server-bing-webmaster**](https://github.com/isiahw1/mcp-server-bing-webmaster) — Python (13 stars)
- [**zizzfizzix/mcp-server-bwt**](https://github.com/zizzfizzix/mcp-server-bwt) — Python (6 stars)
- [**webjeyros/bing-webmaster-api**](https://github.com/webjeyros/bing-webmaster-api) — PHP (4 stars)
- [**NmadeleiDev/bing_webmaster_cli**](https://github.com/NmadeleiDev/bing_webmaster_cli) — Python (2 stars)
- [**seo-meow/bing-webmaster-api**](https://github.com/seo-meow/bing-webmaster-api) — Rust

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
