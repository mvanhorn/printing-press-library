# Seranking CLI

Agent-native OpenAPI subset for SE Ranking Data API and Project API based on official docs at https://seranking.com/api.html. Includes high-value read endpoints plus guarded write/recheck audit operations.

Printed by [@Pimmetjeoss](https://github.com/Pimmetjeoss) (Pimmetjeoss).

## Install

The recommended path installs both the `seranking-pp-cli` binary and the `pp-seranking` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install seranking
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install seranking --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seranking-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-seranking --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-seranking --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-seranking skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-seranking. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export SE_RANKING_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/se-ranking-pp-cli/config.toml`.

### 3. Verify Setup

```bash
seranking-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
seranking-pp-cli backlinks list
```

## Usage

Run `seranking-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`seranking-pp-cli account get-subscription-details`** - Get subscription details

### ai-search

Manage ai search

- **`seranking-pp-cli ai-search discover-brand-by-url`** - Discover brand by URL
- **`seranking-pp-cli ai-search get-leaderboard`** - Get AI search leaderboard
- **`seranking-pp-cli ai-search get-overview-time-series`** - Get AI search overview time series
- **`seranking-pp-cli ai-search get-prompts-by-brand`** - Get prompts by brand
- **`seranking-pp-cli ai-search get-prompts-by-target`** - Get prompts by target

### backlinks

Manage backlinks

- **`seranking-pp-cli backlinks fetch-raw-batch`** - Fetch raw backlinks batch
- **`seranking-pp-cli backlinks get-anchor-texts`** - Get backlink anchor texts
- **`seranking-pp-cli backlinks get-count`** - Get backlink count
- **`seranking-pp-cli backlinks get-cumulative-history`** - Get cumulative backlinks history
- **`seranking-pp-cli backlinks get-domain-authority`** - Get domain authority
- **`seranking-pp-cli backlinks get-domain-authority-distribution`** - Get domain authority distribution
- **`seranking-pp-cli backlinks get-domain-authority-history`** - Get domain authority history
- **`seranking-pp-cli backlinks get-history-count`** - Get backlink history count
- **`seranking-pp-cli backlinks get-metrics`** - Get backlink metrics
- **`seranking-pp-cli backlinks get-page-authority`** - Get page authority
- **`seranking-pp-cli backlinks get-page-authority-history`** - Get page authority history
- **`seranking-pp-cli backlinks get-page-or-domain-authority`** - Get page or domain authority
- **`seranking-pp-cli backlinks get-referring-domain-count`** - Get referring domain count
- **`seranking-pp-cli backlinks get-referring-domain-history-count`** - Get referring domain history count
- **`seranking-pp-cli backlinks get-referring-ip-count`** - Get referring IP count
- **`seranking-pp-cli backlinks get-referring-subnet-count`** - Get referring subnet count
- **`seranking-pp-cli backlinks get-summary`** - Get backlink summary
- **`seranking-pp-cli backlinks list`** - List backlinks
- **`seranking-pp-cli backlinks list-new-or-lost`** - List new or lost backlinks
- **`seranking-pp-cli backlinks list-new-or-lost-referring-domains`** - List new or lost referring domains
- **`seranking-pp-cli backlinks list-pages-with`** - List pages with backlinks
- **`seranking-pp-cli backlinks list-referring-domains`** - List referring domains
- **`seranking-pp-cli backlinks list-referring-ips`** - List referring IPs

### domain

Manage domain

- **`seranking-pp-cli domain analysis-analyze-keyword-overlap-and-gaps`** - Analyze keyword overlap and gaps
- **`seranking-pp-cli domain analysis-get-competitors`** - Get domain competitors
- **`seranking-pp-cli domain analysis-get-keyword-rankings`** - Get domain keyword rankings
- **`seranking-pp-cli domain analysis-get-overview-by-region`** - Get domain overview by region
- **`seranking-pp-cli domain analysis-get-pages`** - Get domain pages
- **`seranking-pp-cli domain analysis-get-paid-ads-by-keyword-or`** - Get paid ads by keyword or domain
- **`seranking-pp-cli domain analysis-get-subdomains`** - Get domain subdomains
- **`seranking-pp-cli domain analysis-get-traffic-and-keyword-history`** - Get traffic and keyword history
- **`seranking-pp-cli domain analysis-get-worldwide-overview`** - Get worldwide domain overview
- **`seranking-pp-cli domain analysis-get-worldwide-url-overview`** - Get worldwide URL overview

### keywords

Manage keywords

- **`seranking-pp-cli keywords research-export-metrics`** - Export keyword metrics
- **`seranking-pp-cli keywords research-get-longtail`** - Get longtail keywords
- **`seranking-pp-cli keywords research-get-question`** - Get question keywords
- **`seranking-pp-cli keywords research-get-related`** - Get related keywords
- **`seranking-pp-cli keywords research-get-similar`** - Get similar keywords

### project-management

Manage project management

- **`seranking-pp-cli project-management ai-result-tracker-get-llm-statistics`** - Get LLM statistics
- **`seranking-pp-cli project-management ai-result-tracker-get-llm-status`** - Get LLM status
- **`seranking-pp-cli project-management ai-result-tracker-get-prompt-answer`** - Get prompt answer
- **`seranking-pp-cli project-management ai-result-tracker-get-prompt-rankings`** - Get prompt rankings
- **`seranking-pp-cli project-management ai-result-tracker-get-site-brand`** - Get site brand
- **`seranking-pp-cli project-management ai-result-tracker-list-or-get-llm-engines`** - List or get LLM engines
- **`seranking-pp-cli project-management ai-result-tracker-list-prompts`** - List prompts
- **`seranking-pp-cli project-management competitors-get-all-competitors-metrics`** - Get all competitors metrics
- **`seranking-pp-cli project-management competitors-get-competitor-keyword-positions`** - Get competitor keyword positions
- **`seranking-pp-cli project-management competitors-get-top-10-results`** - Get top 10 results
- **`seranking-pp-cli project-management competitors-get-top-100-results`** - Get top 100 results
- **`seranking-pp-cli project-management competitors-list-competitors`** - List competitors
- **`seranking-pp-cli project-management project-account-get-user-profile`** - Get user profile
- **`seranking-pp-cli project-management project-account-list-or-manage-sub-accounts`** - List or manage sub accounts
- **`seranking-pp-cli project-management project-account-list-owned-projects`** - List owned projects
- **`seranking-pp-cli project-management project-account-list-shared-projects`** - List shared projects
- **`seranking-pp-cli project-management projects-get-ads-chart-statistics`** - Get ads chart statistics
- **`seranking-pp-cli project-management projects-get-check-dates`** - Get check dates
- **`seranking-pp-cli project-management projects-get-historical-dates`** - Get historical dates
- **`seranking-pp-cli project-management projects-get-keyword-positions`** - Get keyword positions
- **`seranking-pp-cli project-management projects-get-project-position-history`** - Get project position history
- **`seranking-pp-cli project-management projects-get-project-summary-statistics`** - Get project summary statistics
- **`seranking-pp-cli project-management projects-list-or-manage-projects`** - List or manage projects
- **`seranking-pp-cli project-management projects-list-project-search-engines`** - List project search engines
- **`seranking-pp-cli project-management projects-list-website-keywords`** - List website keywords
- **`seranking-pp-cli project-management projects-run-position-check`** - Run position check
- **`seranking-pp-cli project-management reference-list-google-languages`** - List Google languages
- **`seranking-pp-cli project-management reference-list-google-regions`** - List Google regions
- **`seranking-pp-cli project-management reference-list-search-engines`** - List search engines

### site-audit

Manage site audit

- **`seranking-pp-cli site-audit create-advanced-audit`** - Create advanced audit
- **`seranking-pp-cli site-audit create-standard-audit`** - Create standard audit
- **`seranking-pp-cli site-audit get-audit-history`** - Get audit history
- **`seranking-pp-cli site-audit get-audit-links`** - Get audit links
- **`seranking-pp-cli site-audit get-audit-pages-by-issue`** - Get audit pages by issue
- **`seranking-pp-cli site-audit get-audit-report`** - Get audit report
- **`seranking-pp-cli site-audit get-audit-status`** - Get audit status
- **`seranking-pp-cli site-audit get-crawled-pages`** - Get crawled pages
- **`seranking-pp-cli site-audit get-issues-for-url`** - Get issues for URL
- **`seranking-pp-cli site-audit list-audits`** - List audits
- **`seranking-pp-cli site-audit recheck-advanced-audit`** - Recheck advanced audit
- **`seranking-pp-cli site-audit recheck-standard-audit`** - Recheck standard audit


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
seranking-pp-cli backlinks list

# JSON for scripting and agents
seranking-pp-cli backlinks list --json

# Filter to specific fields
seranking-pp-cli backlinks list --json --select id,name,status

# Dry run — show the request without sending
seranking-pp-cli backlinks list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
seranking-pp-cli backlinks list --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-seranking -g
```

Then invoke `/pp-seranking <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add seranking seranking-pp-mcp -e SE_RANKING_API_KEY_QUERY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/seranking-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SE_RANKING_API_KEY_QUERY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "seranking": {
      "command": "seranking-pp-mcp",
      "env": {
        "SE_RANKING_API_KEY_QUERY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
seranking-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/se-ranking-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SE_RANKING_API_KEY_QUERY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `seranking-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SE_RANKING_API_KEY_QUERY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
