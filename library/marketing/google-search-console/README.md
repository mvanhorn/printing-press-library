# Google Search Console CLI

**Every Google Search Console feature you'd reach for, plus an offline SQLite cache that powers period compare, quick wins, cannibalization detection, and history older than the API's 16-month window.**

A single binary covering search analytics, URL inspection, sitemaps, and site management -- with the agent-native JSON and CSV outputs, --dry-run, exit codes, and offline search every other GSC tool half-implements. The transcendence layer (compare, quick-wins, cannibalization, historical, decaying, outliers, cliff, roll-up, coverage-drift, sitemap-watch, new-queries) runs entirely from the local SQLite store, so the workflows the API can't answer in one call are answered in one command.

## Install

The recommended path installs both the `google-search-console-pp-cli` binary and the `pp-google-search-console` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install google-search-console
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install google-search-console --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install google-search-console --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install google-search-console --agent claude-code
npx -y @mvanhorn/printing-press install google-search-console --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/cmd/google-search-console-pp-cli@latest
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

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-search-console-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GSC_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-search-console": {
      "command": "google-search-console-pp-mcp",
      "env": {
        "GSC_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Two paths. Pick one.

### Path A: `auth login` (recommended -- auto-refreshes, never see a token)

One-time setup in Google Cloud Console (about 5 minutes):

1. Open [console.cloud.google.com](https://console.cloud.google.com), create or pick a project.
2. **APIs & Services -> Library** -> enable **Google Search Console API**.
3. **APIs & Services -> OAuth consent screen** -> External -> fill in App name, your email -> Save. Add yourself as a Test user under the **Audience** tab.
4. **APIs & Services -> Credentials** -> **Create Credentials** -> **OAuth client ID** -> Application type: **Desktop app** -> name it -> Create.
5. Copy the **Client ID** and **Client secret** from the resulting dialog.

Then, from your terminal:

```bash
google-search-console-pp-cli auth set-client <client_id> <client_secret>
google-search-console-pp-cli auth login
```

`auth login` opens your default browser, you click Allow, the CLI captures the result on a loopback port, and saves an access + refresh token to `~/.config/google-search-console-pp-cli/config.toml` (mode 0600). Every command from then on silently refreshes the access token when it expires.

First-time consent will show Google's "Google hasn't verified this app" interstitial because your OAuth client is in Testing mode. Click **Advanced -> Go to [your app] (unsafe)** to continue. This is expected for personal-use Desktop clients and does not indicate a problem with the CLI.

WSL2 / headless / SSH:

```bash
google-search-console-pp-cli auth login --no-browser
```

Prints the URL for you to paste into any browser, then waits for the callback. Use this when `xdg-open` is unreliable or there is no display.

Write scope (sitemap submit, site add/delete) is opt-in:

```bash
google-search-console-pp-cli auth login --scope write
```

### Path B: `GSC_ACCESS_TOKEN` (CI / one-shot scripts)

For non-interactive contexts where running `auth login` is impractical:

```bash
export GSC_ACCESS_TOKEN="ya29..."   # fetch from https://developers.google.com/oauthplayground/
```

Tokens last one hour. No auto-refresh. Pre-existing scripts continue to work unchanged.

### Auth command surface

```bash
google-search-console-pp-cli auth status                   # show account, expiry, refresh-token presence
google-search-console-pp-cli auth login                    # browser login (loopback OAuth, PKCE S256)
google-search-console-pp-cli auth logout                   # clear tokens, keep OAuth client
google-search-console-pp-cli auth logout --revoke          # also revoke at Google
google-search-console-pp-cli auth forget                   # nuke everything (tokens + client)
google-search-console-pp-cli auth set-client <id> <secret> # one-time OAuth client setup
google-search-console-pp-cli auth set-token <token>        # save a static access token (no auto-refresh)
```

## Quick Start

```bash
# Verify your token, check API reachability, and confirm scopes.
google-search-console-pp-cli doctor

# List every verified property the token has access to. The fastest sanity check before sync.
google-search-console-pp-cli webmasters list-sites --json

# Pull 90 days of search analytics into the local SQLite cache. Idempotent and incremental on subsequent runs.
google-search-console-pp-cli sync --site sc-domain:example.com --last 90d

# Surface page-2 queries with the highest upside -- the highest-leverage SEO recommendation an agent can make.
google-search-console-pp-cli quick-wins sc-domain:example.com --position 8-20 --min-imps 100 --json

# Period-over-period delta -- what changed week-over-week, month-over-month, or any window you set.
google-search-console-pp-cli compare sc-domain:example.com --period 28d --vs prev-period --dim query --top 50

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### SEO opportunities from local corpus
- **`quick-wins`** — Surface page-2 queries with high impressions and low CTR -- page-2-to-page-1 candidates ranked by upside, computed offline from your synced corpus.

  _Reach for this when an agent needs to recommend the top SEO opportunities for a property without re-fetching from the API._

  ```bash
  google-search-console-pp-cli quick-wins sc-domain:example.com --position 8-20 --min-imps 100 --json
  ```
- **`cannibalization`** — Find queries where multiple pages compete, ranked by combined impressions -- the keyword-cannibalization audit the GSC web UI doesn't offer.

  _Use when an agent investigates why a query ranks worse than expected -- surfaces the competing pages on its own URL._

  ```bash
  google-search-console-pp-cli cannibalization sc-domain:example.com --min-imps 50 --top 25 --json
  ```
- **`outliers`** — Queries or pages with click-through rates that deviate from the observed CTR-by-position curve in your own corpus.

  _Title-tag and snippet-rewrite candidates an agent can act on directly -- high impressions, low CTR for their position._

  ```bash
  google-search-console-pp-cli outliers sc-domain:example.com --metric ctr --sigma 2 --top 50 --json
  ```

### Time-series analysis
- **`compare`** — Period-over-period delta on clicks, impressions, CTR, and position for any dimension -- week-over-week, month-over-month, or arbitrary windows.

  _First-line investigation when a property's traffic shifts -- the agent gets the deltas without spelunking two raw queries._

  ```bash
  google-search-console-pp-cli compare sc-domain:example.com --period 28d --vs prev-period --dim query --top 50 --agent --select rows.keys,rows.delta_clicks,rows.delta_position
  ```
- **`cliff`** — Find the day clicks or impressions cratered, with signature hints matching same-day sitemap regressions or indexing drops.

  _The first command an agent should run when a human says 'traffic dropped' -- points at the day and likely cause._

  ```bash
  google-search-console-pp-cli cliff sc-domain:example.com --metric clicks --threshold -25% --window 7d
  ```
- **`historical`** — Search analytics for date ranges older than the API's 16-month rolling window -- answer 'is this March-2024 normal?' from cached history.

  _Forecasting and seasonality questions; a one-shot agent can answer 'compared to two years ago' without breaking out a backup file._

  ```bash
  google-search-console-pp-cli historical sc-domain:example.com --start 2023-01-01 --end 2023-12-31 --dim query --top 100
  ```

### Cross-property analysis
- **`roll-up`** — Aggregate top queries or pages across every verified property in one command -- the API forces N round-trips.

  _Agency workflows where the agent surfaces top performers across a portfolio without writing per-site loops._

  ```bash
  google-search-console-pp-cli roll-up --metric clicks --group-by query --top 50 --last 28d --json
  ```

### Indexing diagnostics
- **`coverage-drift`** — URLs whose inspection state flipped (indexed → not indexed, robots changed, canonical changed) within a window.

  _Catches indexing regressions an agent would otherwise miss because the API hides them once the state has flipped back._

  ```bash
  google-search-console-pp-cli coverage-drift sc-domain:example.com --field indexingState --days 30 --json
  ```
- **`sitemap-watch`** — Diff sitemap state between snapshots -- surface new errors, new warnings, content-count drops, and stale lastDownloaded times.

  _Friday automation that catches sitemap regressions before the next Monday traffic dip -- runs from local data, no extra API spend._

  ```bash
  google-search-console-pp-cli sitemap-watch sc-domain:example.com --since 7d --json
  ```

### Content workflows
- **`decaying`** — Pages with monotonic click decline over a rolling window, ranked by total impressions × negative slope -- the content-refresh queue.

  _Content marketers' Thursday refresh queue -- the agent picks update candidates with concrete supporting numbers._

  ```bash
  google-search-console-pp-cli decaying sc-domain:example.com --window 90d --min-imps 500 --top 50 --json
  ```
- **`new-queries`** — Queries that started showing up with impressions in the last N days but didn't exist in the corpus before -- emerging demand.

  _Content-ideas surface for marketers; the agent gets emerging-search trends scoped to this site rather than generic Trends data._

  ```bash
  google-search-console-pp-cli new-queries sc-domain:example.com --since 28d --min-imps 50 --top 100 --json
  ```

## Cookbook

End-to-end recipes that combine multiple commands. Every flag below is verified against the running CLI -- no guessed names.

### First-run flow on a new property

```bash
# 1. Confirm token and reachability
google-search-console-pp-cli doctor

# 2. List verified properties the token can see
google-search-console-pp-cli webmasters list-sites --json

# 3. Sync 90 days into the local SQLite store
google-search-console-pp-cli sync --site sc-domain:example.com --last 90d

# 4. Run the high-leverage SEO read on the synced data
google-search-console-pp-cli quick-wins sc-domain:example.com --min-imps 100 --json
```

### Period-over-period investigation when traffic shifts

```bash
# Did clicks drop? Find the day, then drill into which queries moved.
google-search-console-pp-cli cliff sc-domain:example.com --metric clicks --threshold -25% --window 14d --json
google-search-console-pp-cli compare sc-domain:example.com --period 28d --vs prev-period --dim query --top 25 --agent
```

### Content refresh queue (Thursday workflow)

```bash
# Pages with monotonic decline over 90 days, ranked by impact.
google-search-console-pp-cli decaying sc-domain:example.com --window 90d --min-imps 500 --top 30 --json \
  | jq '.[] | select(.slope < -0.5)'
```

### Title-tag rewrite candidates

```bash
# CTR outliers vs. position curve — high impressions, low CTR for their rank.
google-search-console-pp-cli outliers sc-domain:example.com --metric ctr --sigma 2 --min-imps 200 --top 50 --csv > rewrite-queue.csv
```

### Cross-property roll-up across an agency portfolio

```bash
# One command, one table, every site the token sees.
google-search-console-pp-cli roll-up --metric clicks --group-by query --top 100 --last 28d --json
```

### Indexing regression detection

```bash
# Compare the last 30 days of inspection state and surface URLs that flipped.
google-search-console-pp-cli coverage-drift sc-domain:example.com --field indexingState --days 30 --json
```

### Sitemap surveillance

```bash
# Diff sitemap snapshots over the last week — new errors, content drops, stale lastDownloaded.
google-search-console-pp-cli sitemap-watch sc-domain:example.com --since 7d --json
```

### Long-history queries (older than the API's 16-month window)

```bash
# Read directly from local cache for any synced date range.
google-search-console-pp-cli historical sc-domain:example.com --start 2023-01-01 --end 2023-12-31 --dim query --top 200 --json
```

### Submit a sitemap (write workflow)

```bash
google-search-console-pp-cli webmasters submit-sitemap \
  sc-domain:example.com \
  https://example.com/sitemap.xml
```

### Bulk URL inspection with daily-quota throttling

```bash
google-search-console-pp-cli url-inspection inspect-batch \
  --site sc-domain:example.com \
  --file urls.txt \
  --max-per-day 2000 \
  --json > inspections.ndjson
```

### Pipe filtering with `--select` to cut tokens for agents

```bash
google-search-console-pp-cli compare sc-domain:example.com --period 28d --vs prev-period --dim query --top 50 \
  --agent --select rows.keys,rows.delta_clicks,rows.delta_position
```

## Usage

Run `google-search-console-pp-cli --help` for the full command reference and flag list.

## Commands

### url-inspection

Inspect a URL's index status in Google Search

- **`google-search-console-pp-cli url-inspection inspect-url`** - Returns Google's view of a single URL: index status, last crawl time,
canonical URL (Google-selected vs user-declared), mobile usability
verdict, AMP/rich results status, robots.txt and indexing directives,
and referring URLs.

Single-URL only -- no batch endpoint exists. To inspect many URLs,
loop and respect the daily quota (typically 2,000 inspections/day per
property).

### webmasters

Manage webmasters

- **`google-search-console-pp-cli webmasters add-site`** - Adds a site to the set of the user's sites in Search Console. The site
must still be verified separately (via DNS, HTML tag, or analytics) before
Search Console will collect data for it.
- **`google-search-console-pp-cli webmasters delete-site`** - Removes a site from the set of the user's Search Console sites. Does not delete data Google has collected.
- **`google-search-console-pp-cli webmasters delete-sitemap`** - Deletes a sitemap from the Sitemaps report. Does not stop Google from
crawling the sitemap if it's still discoverable; just removes it from
the explicit submission list.
- **`google-search-console-pp-cli webmasters get-site`** - Retrieves information about a specific verified site, including the user's permission level.
- **`google-search-console-pp-cli webmasters get-sitemap`** - Retrieves information about a specific sitemap, including warnings, errors,
last submitted/downloaded times, and content type counts (web, image, video, news).
- **`google-search-console-pp-cli webmasters list-sitemaps`** - Lists the sitemaps submitted for this site, or included in the sitemap
index file (if `sitemapIndex` is provided in the request).
- **`google-search-console-pp-cli webmasters list-sites`** - Returns every site (domain property or URL-prefix property) the authenticated
user has access to in Search Console, along with the user's permission level
for each (siteOwner, siteFullUser, siteRestrictedUser, siteUnverifiedUser).
- **`google-search-console-pp-cli webmasters query-search-analytics`** - Returns clicks, impressions, CTR, and average position grouped by the
dimensions you specify (query, page, country, device, searchAppearance,
date). The workhorse endpoint of the Search Console API.

Common usage: pass `dimensions: [query, page]`, a date range, and
optionally `dimensionFilterGroups` to drill into a specific country
or device. Results are capped at 25,000 rows per request; use
`startRow` and `rowLimit` for pagination.

Date range maximum is 16 months back from today. Data lag is typically
2-3 days for the most recent date.
- **`google-search-console-pp-cli webmasters submit-sitemap`** - Submits a sitemap for a site. The sitemap URL must be an absolute URL on
the same site as `siteUrl`. Submission is idempotent.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-search-console-pp-cli url-inspection --inspection-url https://example.com/resource

# JSON for scripting and agents
google-search-console-pp-cli url-inspection --inspection-url https://example.com/resource --json

# Filter to specific fields
google-search-console-pp-cli url-inspection --inspection-url https://example.com/resource --json --select id,name,status

# Dry run — show the request without sending
google-search-console-pp-cli url-inspection --inspection-url https://example.com/resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-search-console-pp-cli url-inspection --inspection-url https://example.com/resource --agent
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

## Health Check

```bash
google-search-console-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-search-console-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GSC_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-search-console-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GSC_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized from any command** — Token is expired or missing. Generate a new one at https://developers.google.com/oauthplayground/ with scope webmasters.readonly, then `export GSC_ACCESS_TOKEN=<token>` and retry. Tokens last 1 hour.
- **403 PERMISSION_DENIED on sites/sitemaps write** — Read-only scope was used. Re-authorize with scope https://www.googleapis.com/auth/webmasters (full) instead of webmasters.readonly to allow sitemap submit/delete and site add/delete.
- **Empty rows from searchanalytics for the last 1-2 days** — GSC has a 2-3 day data lag. Use `--data-state all` to include preliminary data, or set --end-date to 3 days ago for stable numbers.
- **Search analytics request returns at most 25,000 rows** — API caps a single request at 25k. Paginate manually with `--start-row 25000` (then 50000, etc.) until an empty response, or run `sync` once and query the local store with `historical` / `quick-wins` / etc., which page automatically.
- **URL Inspection RESOURCE_EXHAUSTED** — Per-property daily quota (~2,000/day) is exhausted. Use `inspect-batch --max-per-day 2000` to throttle, or wait for the next UTC day.
- **Date earlier than 16 months returns nothing in `searchanalytics query`** — API window is rolling 16 months. Use `historical` instead -- it reads from the local cache and works for any date you have synced data for.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**AminForou/mcp-gsc**](https://github.com/AminForou/mcp-gsc) — Python
- [**Bin-Huang/google-search-console-cli**](https://github.com/Bin-Huang/google-search-console-cli) — Node
- [**ahonn/mcp-server-gsc**](https://github.com/ahonn/mcp-server-gsc) — TypeScript
- [**joshcarty/google-searchconsole**](https://github.com/joshcarty/google-searchconsole) — Python
- [**kasdimg/analytics-cli**](https://github.com/kasdimg/analytics-cli) — Node

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
