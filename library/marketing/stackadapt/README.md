# StackAdapt CLI

**The first agent-native StackAdapt CLI: pacing, delivery-drift, and budget-waste analysis the dashboard and warehouse connectors can't answer in one call.**

StackAdapt is a programmatic advertising platform. This read-only CLI queries your advertisers, campaigns, ads, audiences, and delivery reporting straight from the StackAdapt GraphQL API and answers questions the dashboard shows one screen at a time and the warehouse connectors only dump raw rows for: which campaigns are under-pacing, whose CTR has drifted, where budget is being wasted. It does not change anything in StackAdapt (read-only); it tells you what your campaigns are doing.

Created by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `stackadapt-pp-cli` binary and the `pp-stackadapt` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install stackadapt
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install stackadapt --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install stackadapt --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install stackadapt --agent claude-code
npx -y @mvanhorn/printing-press-library install stackadapt --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/cmd/stackadapt-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stackadapt-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install stackadapt --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-stackadapt --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-stackadapt --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install stackadapt --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/stackadapt-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `STACKADAPT_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/cmd/stackadapt-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "stackadapt": {
      "command": "stackadapt-pp-mcp",
      "env": {
        "STACKADAPT_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

This CLI uses the StackAdapt GraphQL API, which needs a GraphQL API token (the legacy REST key will not work). To get set up:

1. Ask your StackAdapt account manager for a GraphQL API token if you do not have one.
2. Export it so the CLI can read it:
   `export STACKADAPT_API_TOKEN=your-token`
3. Run `stackadapt-pp-cli doctor` to confirm it connects.

The token is sent as `Authorization: Bearer <token>`. The CLI is read-only and never modifies your campaigns.

## Quick Start

```bash
# Confirms the CLI is installed and shows what a health check verifies (no token needed).
stackadapt-pp-cli doctor --dry-run

# Lists your advertisers — the top of the hierarchy and your starting point.
stackadapt-pp-cli advertisers list

# List campaigns with status and budget metadata.
stackadapt-pp-cli campaigns list --agent

# The headline answer: which campaigns are under- or over-pacing against budget.
stackadapt-pp-cli pacing --agent

# Where your ad budget is being wasted — highest spend, worst performance.
stackadapt-pp-cli bottleneck --agent

```

## Offline store (sync, search, SQL)

StackAdapt is a live GraphQL API, but you can mirror your advertisers, campaigns,
campaign groups, ads, and segments into a local SQLite store and then work
offline — search across everything and run ad-hoc SQL without another API round
trip.

```bash
# Pull every object into the local store (advertisers, campaigns, groups, ads, segments).
stackadapt-pp-cli sync

# Sync just what you need.
stackadapt-pp-cli sync --resources advertisers,campaigns

# Substring-search synced objects by name or any field — fully offline.
stackadapt-pp-cli search "spring" --agent
stackadapt-pp-cli search acme --type advertisers

# Read-only SQL over the store (SELECT/WITH only). The 'resources' table holds
# resource_type, id, name, data (JSON), synced_at — reach into JSON with json_extract.
stackadapt-pp-cli sql "SELECT resource_type, count(*) FROM resources GROUP BY resource_type"
stackadapt-pp-cli sql "SELECT json_extract(data,'\$.channelType') AS channel, count(*) FROM resources WHERE resource_type='ads' GROUP BY channel" --json

# Serve list commands from the store instead of the API.
stackadapt-pp-cli campaigns list --data-source local
```

The store lives at `~/.local/share/stackadapt-pp-cli/data.db` (override with the
`STACKADAPT_DB` environment variable). Re-running `sync` refreshes every object in
place. With `--data-source auto` (the default), list commands query the API and
fall back to the store if the API is unreachable; `--data-source local` reads the
store only; `--data-source live` skips it entirely.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Pacing & delivery observatory
- **`pacing`** — See which campaigns are under- or over-pacing against their budget, not just current spend.

  _Reach for this to catch campaigns that will overspend or underdeliver before the budget cycle ends._

  ```bash
  stackadapt-pp-cli pacing --advertiser adv_123 --agent
  ```
- **`delivery-drift`** — Track CTR, CPM, and spend drift week-over-week for a campaign and flag when performance degrades.

  _Reach for this to catch a slowly degrading campaign before it wastes budget._

  ```bash
  stackadapt-pp-cli delivery-drift --advertiser adv_123 --days 7 --agent
  ```

### Audience & efficiency
- **`bottleneck`** — Rank the highest-spend campaigns by worst ROAS or CPA, with a reason column.

  _Reach for this to find where ad budget is being wasted._

  ```bash
  stackadapt-pp-cli bottleneck --advertiser adv_123 --agent
  ```
- **`stale-campaigns`** — Find active campaigns with zero delivery in the last N days.

  _Reach for this to find live-but-not-delivering campaigns._

  ```bash
  stackadapt-pp-cli stale-campaigns --days 14 --agent
  ```

## Recipes

### Find under-pacing campaigns

```bash
stackadapt-pp-cli pacing --advertiser adv_123 --agent
```

Returns each campaign's pace (actual vs expected spend) so you can fix budget delivery before the cycle ends.

### Catch a degrading campaign

```bash
stackadapt-pp-cli delivery-drift --advertiser adv_123 --days 7 --agent
```

Shows CTR/CPM/spend drift week-over-week and flags when performance slips.

### Trim a verbose report for an agent

```bash
stackadapt-pp-cli report campaign-delivery --days 30 --agent --select records.campaign_name,records.metrics.cost,records.metrics.ctr
```

Delivery reports are large; --select with dotted paths returns only the fields an agent needs, saving context.

### Find where budget is wasted

```bash
stackadapt-pp-cli bottleneck --advertiser adv_123 --agent
```

Ranks high-spend campaigns by worst ROAS/CPA with a reason column.

## Usage

Run `stackadapt-pp-cli --help` for the full command reference and flag list.

## Commands

### graphql

Raw GraphQL query passthrough (advanced; prefer the typed commands)

- **`stackadapt-pp-cli graphql`** - Execute a raw GraphQL query against the StackAdapt API

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
stackadapt-pp-cli graphql --query example-value

# JSON for scripting and agents
stackadapt-pp-cli graphql --query example-value --json

# Filter to specific fields
stackadapt-pp-cli graphql --query example-value --json --select id,name,status

# Dry run — show the request without sending
stackadapt-pp-cli graphql --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
stackadapt-pp-cli graphql --query example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
stackadapt-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/stackadapt/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `STACKADAPT_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `stackadapt-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `stackadapt-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $STACKADAPT_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor fails with 'unauthorized' or 'invalid token'** — Make sure STACKADAPT_API_TOKEN is a GraphQL API token (not a legacy REST key); request one from your StackAdapt account manager.
- **advertisers list returns empty** — Your token may be scoped to a different account; run `stackadapt-pp-cli account` to see what it can access.
- **HTTP 429 Too Many Requests** — StackAdapt rate-limits the API; the CLI backs off and retries — re-run after a moment or narrow the date window.
