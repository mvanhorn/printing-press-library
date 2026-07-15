# eRank CLI

**Keyword Tool data from eRank, plus local scoring, drift, and listing-gap analysis for Etsy sellers.**

Use eRank from a terminal to capture keyword stats, top listings, tags, related searches, and keyword lists in agent-ready formats. The CLI adds opportunity scoring, drift detection, and listing-gap analysis on top of the captured Keyword Tool workflow.

Learn more at [eRank](https://members.erank.com).

Created by [@horknfbr](https://github.com/horknfbr) (horknfbr).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `erank-pp-cli` binary and the `pp-erank` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install erank
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install erank --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install erank --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install erank --agent claude-code
npx -y @mvanhorn/printing-press-library install erank --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/erank/cmd/erank-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/erank-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install erank --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-erank --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-erank --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install erank --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
erank-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/erank-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/erank/cmd/erank-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "erank": {
      "command": "erank-pp-mcp"
    }
  }
}
```

</details>

## Authentication

This CLI uses an authenticated eRank member session. Run the generated auth setup flow before live commands; captured endpoints require the same browser-compatible session that powers eRank's member tools.

## Quick Start

```bash
# Confirm eRank session setup and browser-compatible transport.
erank-pp-cli doctor

# Pull a compact top-listing view for one Etsy keyword.
erank-pp-cli keyword-tool list-top-listings --keyword "dad mug" --marketplace etsy --country USA --agent --select title,shop_name,price,tags

# Convert raw eRank keyword signals into a seller decision score.
erank-pp-cli opportunity "dad mug" --source etsy --country USA --agent

# Find recurring tags backed by several eRank surfaces.
erank-pp-cli tags consensus "dad mug" --source etsy --country USA --min-count 3 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Seller decisions
- **`opportunity`** — Score a keyword as a seller opportunity using eRank stats, difficulty, competition, and current top listings.

  _Use this when an agent needs a go/no-go read on a keyword instead of separate raw metric calls._

  ```bash
  erank-pp-cli opportunity "dad mug" --source etsy --country USA --agent
  ```
- **`lists optimize`** — Rank saved keyword lists by weak, saturated, overlapping, and missing keyword opportunities.

  _Use this when an agent needs to clean up a seller's research list before drafting listings._

  ```bash
  erank-pp-cli lists optimize "Father's Day mugs" --country USA --agent
  ```
- **`saturation`** — Flag crowded keywords by combining competition, difficulty, tag reuse, and top-listing density.

  _Use this to avoid chasing keywords that look popular but are too crowded to enter._

  ```bash
  erank-pp-cli saturation "dad mug" --source etsy --country USA --agent
  ```

### Listing optimization
- **`listing gaps`** — Compare a draft listing title and tags against phrases and tags appearing in top eRank results.

  _Use this before publishing or rewriting an Etsy listing from keyword evidence._

  ```bash
  erank-pp-cli listing gaps "dad mug" --title "Funny Dad Coffee Mug" --tags "dad gift,fathers day,mug" --agent
  ```
- **`tags consensus`** — Find tags that repeatedly appear across top listings, Etsy tag data, related searches, and near matches.

  _Use this when an agent needs defensible tag candidates grounded in multiple signals._

  ```bash
  erank-pp-cli tags consensus "dad mug" --source etsy --country USA --min-count 3 --agent
  ```

### Local history
- **`watch drift`** — Detect meaningful changes in keyword competition, difficulty, and top listings across saved snapshots.

  _Use this to monitor seasonal or competitive shifts without rereading full eRank pages._

  ```bash
  erank-pp-cli watch drift "dad mug" --days 30 --threshold 15 --agent
  ```

### Product research
- **`angles`** — Extract product angles from related searches, near matches, tags, and current top listings.

  _Use this when an agent needs product-angle ideas tied to observed demand signals._

  ```bash
  erank-pp-cli angles "dad mug" --source etsy --country USA --limit 10 --agent
  ```

## Recipes

### Compact top listings for an agent

```bash
erank-pp-cli keyword-tool list-top-listings --keyword "dad mug" --marketplace etsy --country USA --agent --select title,shop_name,price,tags
```

Returns only listing fields an agent needs for comparison.

### Score a niche before drafting

```bash
erank-pp-cli opportunity "dad mug" --source etsy --country USA --agent
```

Combines eRank keyword and listing signals into a go/no-go score.

### Find defensible tags

```bash
erank-pp-cli tags consensus "dad mug" --source etsy --country USA --min-count 3 --agent
```

Extracts recurring tag evidence across captured eRank surfaces.

### Check draft listing gaps

```bash
erank-pp-cli listing gaps "dad mug" --title "Funny Dad Coffee Mug" --tags "dad gift,fathers day,mug" --agent
```

Compares draft copy to top-ranking listing evidence.

## Usage

Run `erank-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Operations on sideBarCollapse

- **`erank-pp-cli account get-user-preferences`** - GET /api/account/user-preferences/{user_preference_id}
- **`erank-pp-cli account list-keyword-tool.near-matches.table.config.columns`** - GET /api/account/user-preferences/keyword-tool.near-matches.table.config.columns
- **`erank-pp-cli account list-kt-keyword-ideas`** - GET /api/account/user-preferences/kt_keyword_ideas
- **`erank-pp-cli account list-member-preferences`** - GET /api/account/member-preferences
- **`erank-pp-cli account list-search-country`** - GET /api/account/user-preferences/searchCountry
- **`erank-pp-cli account list-side-bar-collapse`** - GET /api/account/user-preferences/sideBarCollapse

### build

Operations on version.json

- **`erank-pp-cli build`** - GET /build/version.json

### check-paddle-restriction

Operations on check-paddle-restriction

- **`erank-pp-cli check-paddle-restriction`** - GET /api/check-paddle-restriction

### intercom

Operations on intercom

- **`erank-pp-cli intercom`** - GET /api/intercom

### keyword-tool

Operations on stats

- **`erank-pp-cli keyword-tool create-competition`** - POST /api/keyword-tool/competition
- **`erank-pp-cli keyword-tool create-google-data`** - POST /api/keyword-tool/google-data
- **`erank-pp-cli keyword-tool create-keyword-difficulty`** - POST /api/keyword-tool/keyword-difficulty
- **`erank-pp-cli keyword-tool create-save-history`** - POST /api/keyword-tool/save-history
- **`erank-pp-cli keyword-tool list-etsy-tags`** - GET /api/keyword-tool/etsy-tags
- **`erank-pp-cli keyword-tool list-near-matches`** - GET /api/keyword-tool/near-matches
- **`erank-pp-cli keyword-tool list-related-searches`** - GET /api/keyword-tool/related-searches
- **`erank-pp-cli keyword-tool list-stats`** - GET /api/keyword-tool/stats
- **`erank-pp-cli keyword-tool list-top-listings`** - GET /api/keyword-tool/top-listings

### keywordlist

Operations on names

- **`erank-pp-cli keywordlist list-names`** - GET /api/keywordlist/names
- **`erank-pp-cli keywordlist list-terms`** - GET /api/keywordlist/terms

### member-shops

Operations on member-shops

- **`erank-pp-cli member-shops`** - GET /api/member-shops

### motd-v3

Operations on keyword-tool

- **`erank-pp-cli motd-v3`** - GET /api/motd-v3/keyword-tool

### oauth

Operations on check-token-validity

- **`erank-pp-cli oauth`** - GET /api/oauth/check-token-validity

### quota

Operations on daily

- **`erank-pp-cli quota`** - GET /api/quota/daily

### refresh-data

Operations on last-refresh

- **`erank-pp-cli refresh-data`** - GET /api/refresh-data/listings/last-refresh

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
erank-pp-cli account get-user-preferences mock-value

# JSON for scripting and agents
erank-pp-cli account get-user-preferences mock-value --json

# Filter to specific fields
erank-pp-cli account get-user-preferences mock-value --json --select id,name,status

# Dry run — show the request without sending
erank-pp-cli account get-user-preferences mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
erank-pp-cli account get-user-preferences mock-value --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `ERANK_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `erank-pp-cli account`
- `erank-pp-cli account get`
- `erank-pp-cli account list`
- `erank-pp-cli account search`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns get`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns list`
- `erank-pp-cli account-user-preferences-keyword-tool-near-matches-table-config-columns search`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas get`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas list`
- `erank-pp-cli account-user-preferences-kt-keyword-ideas search`
- `erank-pp-cli account-user-preferences-search-country`
- `erank-pp-cli account-user-preferences-search-country get`
- `erank-pp-cli account-user-preferences-search-country list`
- `erank-pp-cli account-user-preferences-search-country search`
- `erank-pp-cli account-user-preferences-side-bar-collapse`
- `erank-pp-cli account-user-preferences-side-bar-collapse get`
- `erank-pp-cli account-user-preferences-side-bar-collapse list`
- `erank-pp-cli account-user-preferences-side-bar-collapse search`
- `erank-pp-cli build`
- `erank-pp-cli build get`
- `erank-pp-cli build list`
- `erank-pp-cli build search`
- `erank-pp-cli check-paddle-restriction`
- `erank-pp-cli check-paddle-restriction get`
- `erank-pp-cli check-paddle-restriction list`
- `erank-pp-cli check-paddle-restriction search`
- `erank-pp-cli intercom`
- `erank-pp-cli intercom get`
- `erank-pp-cli intercom list`
- `erank-pp-cli intercom search`
- `erank-pp-cli keywordlist`
- `erank-pp-cli keywordlist get`
- `erank-pp-cli keywordlist list`
- `erank-pp-cli keywordlist search`
- `erank-pp-cli keywordlist-terms`
- `erank-pp-cli keywordlist-terms get`
- `erank-pp-cli keywordlist-terms list`
- `erank-pp-cli keywordlist-terms search`
- `erank-pp-cli member-shops`
- `erank-pp-cli member-shops get`
- `erank-pp-cli member-shops list`
- `erank-pp-cli member-shops search`
- `erank-pp-cli motd-v3`
- `erank-pp-cli motd-v3 get`
- `erank-pp-cli motd-v3 list`
- `erank-pp-cli motd-v3 search`
- `erank-pp-cli oauth`
- `erank-pp-cli oauth get`
- `erank-pp-cli oauth list`
- `erank-pp-cli oauth search`
- `erank-pp-cli quota`
- `erank-pp-cli quota get`
- `erank-pp-cli quota list`
- `erank-pp-cli quota search`
- `erank-pp-cli refresh-data`
- `erank-pp-cli refresh-data get`
- `erank-pp-cli refresh-data list`
- `erank-pp-cli refresh-data search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
erank-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/erank-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ERANK_XSRF_TOKEN` | harvested | Yes | Populated automatically by auth login. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `erank-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `erank-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ERANK_XSRF_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return login or empty member data.** — Refresh the eRank browser session with the generated auth setup flow, then rerun `erank-pp-cli doctor`.
- **Keyword calls stop returning data.** — Run `erank-pp-cli quota list-daily --agent` and retry after quota resets if daily lookups are exhausted.
- **POST keyword endpoints fail live verification.** — Use `--dry-run` first; this capture has weak POST schema confidence from a single keyword workflow.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://members.erank.com/keyword-tool/top-listings
- Capture coverage: 61 API entries from 233 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: api_key — query: keyword
- Protection signals: cloudflare (90% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_competition — Derived from observed POST /api/keyword-tool/competition traffic.; create_google_data — Derived from observed POST /api/keyword-tool/google-data traffic.; create_keyword_difficulty — Derived from observed POST /api/keyword-tool/keyword-difficulty traffic.; create_save_history — Derived from observed POST /api/keyword-tool/save-history traffic.; get_user_preferences — Derived from observed GET /api/account/user-preferences/{user_preference_id} traffic.; list_check_paddle_restriction — Derived from observed GET /api/check-paddle-restriction traffic.; list_check_token_validity — Derived from observed GET /api/oauth/check-token-validity traffic.; list_customer — Derived from observed GET /dotjs/v1/quests/customer/ traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
