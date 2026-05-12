# WHOOP CLI

Official WHOOP Developer Platform v2 API. Provides programmatic access to a user's
cycles, sleep, recovery, workouts, and profile data. Authentication is OAuth 2.0
with PKCE; this spec also accepts a static bearer token via the WHOOP_ACCESS_TOKEN
environment variable for non-interactive contexts (CI, agents).

Pagination: every list endpoint enforces `limit <= 25` (default `10`). Use the
`nextToken` query parameter with the `next_token` value from the previous response.
Sort order is `start` descending. Time filters: `start`, `end` (ISO-8601 UTC).

WHOOP gives you a number every morning. whoop-pp-cli gives you the *why*. It syncs your full WHOOP history into a local SQLite database, runs cross-resource joins (sleep ⋈ recovery ⋈ workouts ⋈ cycles), and surfaces trends, correlations, and overtraining alerts no live API call can compute. Everything is agent-native: JSON output, --select field filtering, --agent mode for Claude Code, plus an MCP server for Claude Desktop.

Learn more at [WHOOP](https://developer.whoop.com).

## Install

The recommended path installs both the `whoop-pp-cli` binary and the `pp-whoop` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install whoop
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install whoop --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/whoop-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-whoop --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-whoop --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-whoop skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-whoop. The skill defines how its required CLI can be installed.
```

## Authentication

WHOOP uses OAuth 2.0 with PKCE — there is no static API key. You register an app at developer.whoop.com, get a Client ID and Client Secret, then run `whoop-pp-cli auth login`. The CLI opens your browser, you approve the requested scopes (sleep, recovery, workout, cycles, profile, body measurement, plus offline so we can refresh), and WHOOP redirects back to http://localhost:8085/callback where the CLI is listening. Tokens are stored under ~/.config/whoop-pp-cli/tokens.json and auto-refreshed 60 seconds before expiry. For non-interactive contexts (CI, serverless), you can skip the flow and just set WHOOP_ACCESS_TOKEN (or WHOOP_OAUTH for back-compat with the prior 1.0 release).

## Quick Start

```bash














```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`analyze efficiency`** — Buckets cycles by strain (0-5, 5-10, 10-15, 15-21) and shows mean recovery per bucket vs. the prior equivalent window.
- **`analyze sleep-debt`** — Cumulative sum of need_from_sleep_debt_milli weekly, with trend slope and human-friendly interpretation.
- **`analyze overtraining`** — Flags days where strain exceeds N sigma above the 90-day mean and shows the recovery delta vs. window mean.
- **`analyze correlate`** — Pearson correlation between any two whitelisted WHOOP metrics over a chosen window.
- **`analyze why-today`** — Ranks today's recovery, HRV, RHR, sleep consistency, and prior-day strain by abs(z-score) vs. personal 14-day baseline.
- **`sql`** — Execute read-only SELECT/WITH queries (or read-only PRAGMAs like table_info, table_list, foreign_key_list for schema introspection) against the local SQLite store. Accept the query as a positional arg or via --query.
- **`search`** — FTS5 full-text search across all synced resources (cycle, sleep, recovery, workouts).

## Usage

Run `whoop-pp-cli --help` for the full command reference and flag list.

## Commands

### activity

Manage activity

- **`whoop-pp-cli activity get-sleep`** - Get a single sleep by UUID
- **`whoop-pp-cli activity get-workout`** - Get a single workout by UUID
- **`whoop-pp-cli activity list-sleeps`** - List sleep activities
- **`whoop-pp-cli activity list-workouts`** - List workouts

### activity-mapping

V1 -> V2 identifier mapping helper.

- **`whoop-pp-cli activity-mapping get`** - Translate a v1 sleep/workout id to a v2 UUID

### cycle

Physiological cycles (a WHOOP "day").

- **`whoop-pp-cli cycle get`** - Get a single cycle by id
- **`whoop-pp-cli cycle list`** - Returns the user's cycles (a WHOOP day) ordered by start time descending.

### recovery

Recovery scores for each cycle.

- **`whoop-pp-cli recovery list-recoveries`** - List recovery records

### user

User profile and body measurements.

- **`whoop-pp-cli user get-body-measurement`** - Get user body measurement
- **`whoop-pp-cli user get-profile`** - Get user profile (basic)
- **`whoop-pp-cli user revoke-oauth-access`** - Revoke the user's OAuth access (delete token grants)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
whoop-pp-cli activity-mapping mock-value --type example-value

# JSON for scripting and agents
whoop-pp-cli activity-mapping mock-value --type example-value --json

# Filter to specific fields
whoop-pp-cli activity-mapping mock-value --type example-value --json --select id,name,status

# Dry run — show the request without sending
whoop-pp-cli activity-mapping mock-value --type example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
whoop-pp-cli activity-mapping mock-value --type example-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-whoop -g
```

Then invoke `/pp-whoop <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add whoop whoop-pp-mcp -e WHOOP_OAUTH2=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/whoop-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WHOOP_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "whoop": {
      "command": "whoop-pp-mcp",
      "env": {
        "WHOOP_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
whoop-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/whoop-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WHOOP_OAUTH2` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `whoop-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WHOOP_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 400 on every sync request** — Fixed in this version: client auto-clamps limit to 25 and paginates via next_token. If you see this on 1.x, upgrade to 2.x.
- **`redirect_uri does not match` during auth login** — Add http://localhost:8085/callback to your app's Redirect URIs at developer.whoop.com. Or use `auth login --port <n>` and register http://localhost:<n>/callback instead.
- **401 Unauthorized after some hours** — Re-run `whoop-pp-cli auth login` — by default the CLI requests `offline` and the refresh token is saved. The CLI auto-refreshes 60s before expiry. If you must use a static bearer, set `WHOOP_ACCESS_TOKEN` and refresh it manually.
- **Pagination feels slow on long backfills** — `sync --concurrency 4 --rate-limit 5/s` parallelizes per-resource fetches. Or `sync --since 30d` for incremental updates after the first backfill.
- **Recovery missing for some cycles** — Filter with `--scored-only` on list commands, or check `score_state` in your queries. WHOOP backfills these later; re-sync.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
