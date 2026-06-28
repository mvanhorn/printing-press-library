# Apple Search Ads CLI

**Every Apple Search Ads feature, plus local cost analytics, bid optimization, and cross-org templating no Python CLI offers.**

apple-search-ads-pp-cli is a native Go CLI covering all campaign, keyword, and reporting operations plus a local SQLite cache for offline analytics. UA teams get bid optimization suggestions, budget pacing forecasts, and cross-org template sync without hitting API rate limits.

## Install

The recommended path installs both the `apple-search-ads-pp-cli` binary and the `pp-apple-search-ads` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install apple-search-ads
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install apple-search-ads --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install apple-search-ads --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install apple-search-ads --agent claude-code
npx -y @mvanhorn/printing-press-library install apple-search-ads --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-search-ads/cmd/apple-search-ads-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apple-search-ads-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-apple-search-ads --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-apple-search-ads --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-apple-search-ads skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-apple-search-ads. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apple-search-ads-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ASA_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/apple-search-ads/cmd/apple-search-ads-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "apple-search-ads": {
      "command": "apple-search-ads-pp-mcp",
      "env": {
        "ASA_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Apple Search Ads uses OAuth 2.0 with a private key JWT flow. You need a client ID, team ID, key ID, org ID, and a .p8 private key file. Run `apple-search-ads-pp-cli auth setup` to configure these, or set ASA_CLIENT_ID, ASA_TEAM_ID, ASA_KEY_ID, ASA_ORG_ID, and ASA_PRIVATE_KEY_PATH in your environment.

## Quick Start

```bash
# Verify auth config and API reachability
apple-search-ads-pp-cli doctor --dry-run

# List all campaigns across your org
apple-search-ads-pp-cli campaigns list-campaigns --json

# See keyword analytics from the local cache
apple-search-ads-pp-cli analytics query --group-by campaign --limit 10 --json

# Get bid adjustment suggestions
apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`analytics query`** — Query locally cached reporting data without API rate limit friction.

  _Use when you need cross-campaign aggregations or historical trending without burning API quota._

  ```bash
  apple-search-ads-pp-cli analytics query --group-by match_type --limit 20 --agent
  ```
- **`keywords auto-promote`** — Batch analyze high-converting search terms and promote them to keywords with smart match-type routing.

  _Use weekly to surface converting search terms that should become targeted keywords._

  ```bash
  apple-search-ads-pp-cli keywords auto-promote --campaign-id 12345 --min-installs 3 --dry-run
  ```
- **`templates apply`** — Version-control campaign structures and apply them across multiple org IDs with a diff preview.

  _Use when managing multiple apps to ensure consistent campaign structure across all orgs._

  ```bash
  apple-search-ads-pp-cli templates apply brand-baseline --org-ids 111,222 --diff --dry-run
  ```

### Agent-native plumbing
- **`optimize suggest`** — Get CPA/ROAS-driven bid adjustment suggestions with revenue impact forecast before applying.

  _Use before bulk bid changes to preview expected spend delta and install rate impact._

  ```bash
  apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50 --agent
  ```
- **`campaigns forecast-spend`** — Predict daily/monthly spend and flag campaigns on track to overshoot budget caps.

  _Use at start of month to catch pacing issues before they become wasted budget._

  ```bash
  apple-search-ads-pp-cli campaigns forecast-spend --days 30 --alert-threshold 95 --agent
  ```

## Recipes

### Find underperforming keywords

```bash
apple-search-ads-pp-cli campaigns get-targetingkeywords 12345 --json
```

Surface keywords with low install rate for bid review or pausing

### Promote converting search terms

```bash
apple-search-ads-pp-cli keywords auto-promote --campaign-id 12345 --min-installs 3 --dry-run
```

Preview search terms that qualify for keyword promotion before committing

### Check budget pacing

```bash
apple-search-ads-pp-cli campaigns forecast-spend --days 7 --alert-threshold 90 --agent
```

Surface campaigns projected to exhaust budget before week end

### Apply CPA-optimized bids

```bash
apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50 --apply --agent
```

Apply CPA-optimized bid suggestions across all active keywords

### Cross-campaign keyword analytics

```bash
apple-search-ads-pp-cli analytics query --group-by keyword_text --limit 20 --agent
```

Query local cache for top-20 keywords by CPA without API calls

## Usage

Run `apple-search-ads-pp-cli --help` for the full command reference and flag list.

## Commands

### adgroups

Operations on find

- **`apple-search-ads-pp-cli adgroups`** - POST /adgroups/find

### ads

Operations on find

- **`apple-search-ads-pp-cli ads`** - POST /ads/find

### apps

Operations on find

- **`apple-search-ads-pp-cli apps create-find`** - POST /apps/{id}/eligibilities/find
- **`apple-search-ads-pp-cli apps create-find-2`** - POST /apps/{id}/assets/find
- **`apple-search-ads-pp-cli apps get-apps`** - GET /apps/{id}
- **`apple-search-ads-pp-cli apps get-locale-details`** - GET /apps/{id}/locale-details
- **`apple-search-ads-pp-cli apps get-locale-details-2`** - GET /apps/{id}/product-pages/{id}/locale-details
- **`apple-search-ads-pp-cli apps get-product-pages`** - GET /apps/{id}/product-pages/{id}
- **`apple-search-ads-pp-cli apps get-product-pages-2`** - GET /apps/{id}/product-pages

### attribution

Operations on identify

- **`apple-search-ads-pp-cli attribution list-identify`** - GET /attribution/device/identify
- **`apple-search-ads-pp-cli attribution list-send-attribute`** - GET /attribution/device/send-attribute
- **`apple-search-ads-pp-cli attribution list-tracking-consent`** - GET /attribution/device/tracking-consent

### budgetorders

Operations on budgetorders

- **`apple-search-ads-pp-cli budgetorders create-budgetorders`** - POST /budgetorders
- **`apple-search-ads-pp-cli budgetorders get-budgetorders`** - GET /budgetorders/{id}
- **`apple-search-ads-pp-cli budgetorders list-budgetorders`** - GET /budgetorders
- **`apple-search-ads-pp-cli budgetorders update-budgetorders`** - PUT /budgetorders/{id}

### campaigns

Operations on find

- **`apple-search-ads-pp-cli campaigns create-adgroups`** - POST /campaigns/{id}/adgroups
- **`apple-search-ads-pp-cli campaigns create-ads`** - POST /campaigns/{id}/adgroups/{id}/ads
- **`apple-search-ads-pp-cli campaigns create-bulk`** - POST /campaigns/{id}/adgroups/{id}/targetingkeywords/delete/bulk
- **`apple-search-ads-pp-cli campaigns create-bulk-2`** - POST /campaigns/{id}/adgroups/{id}/targetingkeywords/bulk
- **`apple-search-ads-pp-cli campaigns create-campaigns`** - POST /campaigns
- **`apple-search-ads-pp-cli campaigns create-find`** - POST /campaigns/{id}/adgroups/find
- **`apple-search-ads-pp-cli campaigns create-find-2`** - POST /campaigns/{id}/ads/find
- **`apple-search-ads-pp-cli campaigns create-find-3`** - POST /campaigns/find
- **`apple-search-ads-pp-cli campaigns create-find-4`** - POST /campaigns/{id}/adgroups/targetingkeywords/find
- **`apple-search-ads-pp-cli campaigns create-find-5`** - POST /campaigns/{id}/adgroups/negativekeywords/find
- **`apple-search-ads-pp-cli campaigns create-find-6`** - POST /campaigns/{id}/negativekeywords/find
- **`apple-search-ads-pp-cli campaigns delete-adgroups`** - DELETE /campaigns/{id}/adgroups/{id}
- **`apple-search-ads-pp-cli campaigns delete-ads`** - DELETE /campaigns/{id}/adgroups/{id}/ads/{id}
- **`apple-search-ads-pp-cli campaigns delete-campaigns`** - DELETE /campaigns/{id}
- **`apple-search-ads-pp-cli campaigns get-adgroups`** - GET /campaigns/{id}/adgroups
- **`apple-search-ads-pp-cli campaigns get-adgroups-2`** - GET /campaigns/{id}/adgroups/{id}
- **`apple-search-ads-pp-cli campaigns get-ads`** - GET /campaigns/{id}/adgroups/{id}/ads
- **`apple-search-ads-pp-cli campaigns get-ads-2`** - GET /campaigns/{id}/adgroups/{id}/ads/{id}
- **`apple-search-ads-pp-cli campaigns get-campaigns`** - GET /campaigns/{id}
- **`apple-search-ads-pp-cli campaigns get-targetingkeywords`** - GET /campaigns/{id}/adgroups/{id}/targetingkeywords
- **`apple-search-ads-pp-cli campaigns get-targetingkeywords-2`** - GET /campaigns/{id}/adgroups/{id}/targetingkeywords/{id}
- **`apple-search-ads-pp-cli campaigns list-campaigns`** - GET /campaigns
- **`apple-search-ads-pp-cli campaigns update-adgroups`** - PUT /campaigns/{id}/adgroups/{id}
- **`apple-search-ads-pp-cli campaigns update-ads`** - PUT /campaigns/{id}/adgroups/{id}/ads/{id}
- **`apple-search-ads-pp-cli campaigns update-bulk`** - PUT /campaigns/{id}/adgroups/{id}/targetingkeywords/bulk
- **`apple-search-ads-pp-cli campaigns update-campaigns`** - PUT /campaigns/{id}

### countries-or-regions

Operations on countries-or-regions

- **`apple-search-ads-pp-cli countries-or-regions`** - GET /countries-or-regions

### creativeappmappings

Operations on devices

- **`apple-search-ads-pp-cli creativeappmappings`** - GET /creativeappmappings/devices

### creatives

Operations on find

- **`apple-search-ads-pp-cli creatives create-creatives`** - POST /creatives
- **`apple-search-ads-pp-cli creatives create-find`** - POST /creatives/find
- **`apple-search-ads-pp-cli creatives get-creatives`** - GET /creatives/{id}
- **`apple-search-ads-pp-cli creatives list-creatives`** - GET /creatives

### custom-reports

Operations on custom-reports

- **`apple-search-ads-pp-cli custom-reports create-custom-reports`** - POST /custom-reports
- **`apple-search-ads-pp-cli custom-reports get-custom-reports`** - GET /custom-reports/{id}
- **`apple-search-ads-pp-cli custom-reports list-custom-reports`** - GET /custom-reports

### deferred-deep-link

Operations on resolve

- **`apple-search-ads-pp-cli deferred-deep-link list-resolve`** - GET /deferred-deep-link/resolve
- **`apple-search-ads-pp-cli deferred-deep-link list-store`** - GET /deferred-deep-link/store

### me

Operations on me

- **`apple-search-ads-pp-cli me`** - GET /me

### product-page-reasons

Operations on product-page-reasons

- **`apple-search-ads-pp-cli product-page-reasons create-find`** - POST /product-page-reasons/find
- **`apple-search-ads-pp-cli product-page-reasons get-product-page-reasons`** - GET /product-page-reasons/{id}

### reports

Operations on adgroups

- **`apple-search-ads-pp-cli reports create-adgroups`** - POST /reports/campaigns/{id}/adgroups
- **`apple-search-ads-pp-cli reports create-ads`** - POST /reports/campaigns/{id}/ads
- **`apple-search-ads-pp-cli reports create-campaigns`** - POST /reports/campaigns
- **`apple-search-ads-pp-cli reports create-keywords`** - POST /reports/campaigns/{id}/keywords
- **`apple-search-ads-pp-cli reports create-searchterms`** - POST /reports/campaigns/{id}/searchterms

### search_resource

Operations on apps

- **`apple-search-ads-pp-cli search-resource create-geo`** - POST /search/geo
- **`apple-search-ads-pp-cli search-resource list-apps`** - GET /search/apps
- **`apple-search-ads-pp-cli search-resource list-geo`** - GET /search/geo


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
apple-search-ads-pp-cli adgroups

# JSON for scripting and agents
apple-search-ads-pp-cli adgroups --json

# Filter to specific fields
apple-search-ads-pp-cli adgroups --json --select id,name,status

# Dry run — show the request without sending
apple-search-ads-pp-cli adgroups --dry-run

# Agent mode — JSON + compact + no prompts in one flag
apple-search-ads-pp-cli adgroups --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
apple-search-ads-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/apple-search-ads-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ASA_CLIENT_ID` | per_call | No | Set to your API credential. |
| `APPLE_SEARCH_ADS_TOKEN` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `apple-search-ads-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `apple-search-ads-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ASA_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on any command** — Run `apple-search-ads-pp-cli auth setup` -- your private key or client ID may be expired or misconfigured
- **Empty campaign list** — Check ASA_ORG_ID matches the org you're targeting; different orgs need different credentials
- **Rate limit errors on reporting** — Use `analytics sync-cache` first, then query locally with `analytics query`

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**phiture/searchads_api**](https://github.com/phiture/searchads_api) — Python (61 stars)
- [**cameronehrlich/apple-search-ads-cli**](https://github.com/cameronehrlich/apple-search-ads-cli) — Python (15 stars)
- [**SamPetherbridge/asa-api-client**](https://github.com/SamPetherbridge/asa-api-client) — Python (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
