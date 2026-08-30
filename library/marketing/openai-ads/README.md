# OpenAI Ads CLI

**The first OpenAI Ads client that can write, and the only one that keeps local history.**

Covers the whole Advertiser API surface rather than a read-only slice of it, and mirrors your account into SQLite so questions the REST API structurally cannot answer become one command. Pacing, drift, creative fatigue, and structural audits all come from local snapshots. Every monetary value is rendered in your account currency instead of raw micros.

## Install

The recommended path installs both the `openai-ads-pp-cli` binary and the `pp-openai-ads` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install openai-ads
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install openai-ads --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install openai-ads --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install openai-ads --agent claude-code
npx -y @mvanhorn/printing-press-library install openai-ads --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/cmd/openai-ads-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openai-ads-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install openai-ads --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openai-ads --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openai-ads --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install openai-ads --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openai-ads-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPENAI_ADS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/cmd/openai-ads-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openai-ads": {
      "command": "openai-ads-pp-mcp",
      "env": {
        "OPENAI_ADS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Ads Manager issues two different keys and they are easy to confuse. The Ads API key comes from the Settings tab and is scoped to one ad account; set it as OPENAI_ADS_API_KEY. The Conversions API key comes from the Conversions tab, is scoped to a single conversion event, and returns 403 Unauthorized to read ads data if used for ads calls. Set that one as OPENAI_ADS_CONVERSIONS_API_KEY only if you send server-side conversion events. Run doctor to see which key is configured.

## Quick Start

```bash
# confirm which credential is configured before anything hits the network
openai-ads-pp-cli doctor --dry-run

# mirror the account hierarchy and insights into local SQLite
openai-ads-pp-cli sync

# see every campaign, ad group, and ad with status and budget in one view
openai-ads-pp-cli tree

# check spend trajectory against budget caps
openai-ads-pp-cli pace --agent

# catch bids that are irrational against the parent campaign budget
openai-ads-pp-cli bid-check --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history the API does not keep
- **`pace`** — See whether a campaign will underspend or blow through its cap before the period ends.

  _Reach for this instead of raw insights when the question is about trajectory rather than a current number._

  ```bash
  openai-ads-pp-cli pace --agent
  ```
- **`drift`** — Show what changed across campaigns, ad groups, and ads between two syncs.

  _Use this to answer 'what changed and when', which no single API call can report._

  ```bash
  openai-ads-pp-cli drift --since 7d --agent
  ```
- **`fatigue`** — Rank ads by click-through decay so tired creative is obvious before spend is wasted.

  _Pick this over ad insights when the question is whether performance is declining, not what it is today._

  ```bash
  openai-ads-pp-cli fatigue --limit 10 --agent
  ```
- **`review-watch`** — Track approval and review status transitions across the account and every ad.

  _Use this to catch a flip to rejected or in_review that a status read would not reveal as a change._

  ```bash
  openai-ads-pp-cli review-watch --agent
  ```

### Cross-entity joins the API cannot do
- **`bid-check`** — Flag ad groups whose maximum bid is irrational against the parent campaign budget.

  _Catches configurations that permit only a handful of clicks per day before any spend happens._

  ```bash
  openai-ads-pp-cli bid-check --agent
  ```
- **`orphans`** — Find ad groups with no ads, campaigns with no delivery, and audiences nothing references.

  _Use this for structural dead weight rather than performance questions._

  ```bash
  openai-ads-pp-cli orphans --agent
  ```
- **`tree`** — Render the whole campaign, ad group, and ad hierarchy with status, budget, and review state.

  _Start here to orient in an unfamiliar account before drilling into any single resource._

  ```bash
  openai-ads-pp-cli tree --agent
  ```

### Readability
- **`geo resolve`** — Turn the bare location IDs in campaign targeting into readable place names.

  _Use this whenever targeting output shows numeric IDs you cannot interpret._

  ```bash
  openai-ads-pp-cli geo resolve --agent
  ```

## Recipes

### Orient in an unfamiliar account

```bash
openai-ads-pp-cli tree --agent
```

Renders the whole hierarchy with status and budget so you can see the shape before touching anything.

### Narrow a verbose campaign payload

```bash
openai-ads-pp-cli campaigns list --agent --select data.id,data.name,data.status,data.budget.daily_spend_limit_micros
```

Campaign objects carry targeting and landing page blocks; selecting fields keeps agent context small.

### Find tired creative

```bash
openai-ads-pp-cli fatigue --limit 5 --agent
```

Ranks ads by click-through decay across stored snapshots rather than a single reading.

### See what changed this week

```bash
openai-ads-pp-cli drift --since 7d --agent
```

Diffs local snapshots to surface status, budget, bid, and creative changes the API keeps no record of.

### Preview a mutation safely

```bash
openai-ads-pp-cli campaigns pause campaign-method cmpn_example --dry-run
```

Shows the request that would be sent without changing anything in the live account.

## Usage

Run `openai-ads-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `OPENAI_ADS_CONFIG_DIR`, `OPENAI_ADS_DATA_DIR`, `OPENAI_ADS_STATE_DIR`, or `OPENAI_ADS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `OPENAI_ADS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export OPENAI_ADS_HOME=/srv/openai-ads
openai-ads-pp-cli doctor
```

Under `OPENAI_ADS_HOME=/srv/openai-ads`, the four dirs resolve to `/srv/openai-ads/config`, `/srv/openai-ads/data`, `/srv/openai-ads/state`, and `/srv/openai-ads/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "openai-ads": {
      "command": "openai-ads-pp-mcp",
      "env": {
        "OPENAI_ADS_HOME": "/srv/openai-ads"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `OPENAI_ADS_DATA_DIR` overrides an explicit `--home` for that kind. Use `OPENAI_ADS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `OPENAI_ADS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `openai-ads-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### ad-account

Manage ad account

- **`openai-ads-pp-cli ad-account activate-method`** - Activate the ad account.
- **`openai-ads-pp-cli ad-account get-insights-method`** - Get ad account insights aggregated by time granularity.
- **`openai-ads-pp-cli ad-account get-method`** - Get metadata for the ad account
- **`openai-ads-pp-cli ad-account pause-method`** - Pause the ad account.
- **`openai-ads-pp-cli ad-account update-method`** - Update ad account brand metadata.

### ad-groups

Manage ad groups

- **`openai-ads-pp-cli ad-groups create-method`** - Create an ad group for a campaign
- **`openai-ads-pp-cli ad-groups get-method`** - Get an ad group
- **`openai-ads-pp-cli ad-groups list-method`** - Get all ad groups for a campaign
- **`openai-ads-pp-cli ad-groups update-method`** - Update an ad group

### ads

Manage ads

- **`openai-ads-pp-cli ads create-method`** - Create an ad for an ad group
- **`openai-ads-pp-cli ads get-method`** - Get an ad
- **`openai-ads-pp-cli ads list-method`** - Get all ads for an ad group
- **`openai-ads-pp-cli ads update-method`** - Update an ad

### campaigns

Manage campaigns

- **`openai-ads-pp-cli campaigns create-method`** - Create a campaign for an ad account
- **`openai-ads-pp-cli campaigns get-method`** - Get a campaign
- **`openai-ads-pp-cli campaigns list-method`** - Get all campaigns for an ad account
- **`openai-ads-pp-cli campaigns update-method`** - Update a campaign

### conversions

Manage conversions

- **`openai-ads-pp-cli conversions create-api-key-method`** - Create a Conversions API key for the currently authenticated ad account.
- **`openai-ads-pp-cli conversions create-event-setting-method`** - Create a conversion event setting for the currently authenticated ad account.
- **`openai-ads-pp-cli conversions create-source-method`** - Create a conversion pixel.
- **`openai-ads-pp-cli conversions list-event-settings-method`** - List conversion event settings for the currently authenticated ad account.
- **`openai-ads-pp-cli conversions post-insights-method`** - Get attributed conversion totals for the authenticated ad account.

### custom-audiences

Manage custom audiences

- **`openai-ads-pp-cli custom-audiences create-method`** - Create a custom audience for the authenticated ad account.
- **`openai-ads-pp-cli custom-audiences create-upload-method`** - Create a custom audience from an uploaded file and start processing.
- **`openai-ads-pp-cli custom-audiences get-method`** - Get a custom audience for the authenticated ad account.
- **`openai-ads-pp-cli custom-audiences list-method`** - List custom audiences for the authenticated ad account.

### geo-lookup

Manage geo lookup

- **`openai-ads-pp-cli geo-lookup`** - Search DMA and standard region codes for advertiser geo targeting.

### upload

Manage upload

- **`openai-ads-pp-cli upload`** - Upload an image URL or image file and return a file id


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`openai-ads-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`openai-ads-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`openai-ads-pp-cli learnings list`** - Inspect taught rows
- **`openai-ads-pp-cli learnings forget <query>`** - Undo a teach
- **`openai-ads-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`openai-ads-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`openai-ads-pp-cli teach-pattern`** - Install a query/resource template up front
- **`openai-ads-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `OPENAI_ADS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `openai-ads-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openai-ads-pp-cli ad-account activate-method

# JSON for scripting and agents
openai-ads-pp-cli ad-account activate-method --json

# Filter to specific fields
openai-ads-pp-cli ad-account activate-method --json --select id,name,status

# Dry run — show the request without sending
openai-ads-pp-cli ad-account activate-method --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openai-ads-pp-cli ad-account activate-method --agent
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

## Health Check

```bash
openai-ads-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `openai-ads-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/openai-ads-pp-cli/config.toml`; `--home`, `OPENAI_ADS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OPENAI_ADS_API_KEY` | per_call | Yes | Set to your API credential. |
| `OPENAI_ADS_CONVERSIONS_API_KEY` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `openai-ads-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openai-ads-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPENAI_ADS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 Unauthorized to read ads data** — You are using a Conversions API key. Issue an Ads API key from Ads Manager > Settings and set OPENAI_ADS_API_KEY.
- **401 Missing or invalid SDK key in Authorization header** — No credential reached the API. Export OPENAI_ADS_API_KEY, then run doctor.
- **403 with Conversion bidding is not enabled** — Conversion-optimized campaigns are not enabled for the account. Contact your OpenAI partner representative.
- **404 Not found on conversions pixels or api_keys** — Conversion management is not enabled for this ad account. Contact your OpenAI partner representative.
- **Account will not serve ads and review.reason is missing_favicon** — Upload an image with purpose account_favicon at least 128x128, then assign it to the brand.
- **Budgets and bids look like huge integers** — Those are micros. One million micros is one unit of account currency; the human output renders them for you.
