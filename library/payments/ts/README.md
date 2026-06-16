# TreasurySpring CLI

**Every TreasurySpring read and write endpoint, plus a local SQLite mirror that answers portfolio-level questions no single API call can: maturity ladders, obligor concentration, and a consolidated group book.**

ts mirrors your entities, indications, holdings, subscriptions and obligor exposures into local SQLite, then joins across them offline. Run `ts ladder` for a settlement-adjusted cash-flow forecast, `ts concentration --by obligor` for group-wide credit risk, or `ts book` for the consolidated portfolio across every entity. Agent-native: --json, --select, typed exit codes.

Learn more at [TreasurySpring](https://treasuryspring.com/).

## Install

The recommended path installs both the `ts-pp-cli` binary and the `pp-ts` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ts
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ts --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ts --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ts --agent claude-code
npx -y @mvanhorn/printing-press-library install ts --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/ts/cmd/ts-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ts-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ts --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ts --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ts --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ts --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ts-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TS_BEARER_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/ts/cmd/ts-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ts": {
      "command": "ts-pp-mcp",
      "env": {
        "TS_BEARER_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

TreasurySpring uses OAuth2 client-credentials. Set TS_CLIENT_ID and TS_CLIENT_SECRET, then `ts auth login` exchanges them at /oauth/token for a bearer token cached locally. Alternatively set TS_BEARER_TOKEN with a pre-minted token. Use the sandbox with --base-url or TS_ENV=sandbox.

## Quick Start

```bash
# Health check: confirms config and reachability without needing a token.
ts doctor --dry-run

# Exchange client_id/secret for a bearer token.
ts auth login

# Mirror entities, holdings, indications, subscriptions and events into local SQLite.
ts sync

# Settlement-adjusted cash-flow forecast across all entities.
ts ladder --by week --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local mirror that compounds
- **`ladder`** — Week-by-week projection of cash landing back, with maturities shifted to real settlement dates across all entities.

  _Reach for this to answer 'when does liquidity arrive' instead of reading raw per-holding maturity dates._

  ```bash
  ts ladder --by week --currency USD --json
  ```
- **`concentration`** — Total credit exposure to each obligor as a share of the consolidated book, flagging self-set limit breaches.

  _Reach for this for group-wide counterparty risk; per-entity obligor-exposure calls cannot see the consolidated position._

  ```bash
  ts concentration --by obligor --limit 10% --json
  ```
- **`book`** — One portfolio across every legal entity: total invested, weighted-average yield/maturity, currency split.

  _Reach for this for the group-treasurer view no single login or call returns._

  ```bash
  ts book --group-by currency,maturity-bucket --json
  ```

## Recipes


### Catch up on lifecycle events

```bash
ts changed --since last-sync --json
```

Replays the event stream since the last sync and summarises issuances, redemptions and extensions.

### Group-wide credit concentration

```bash
ts concentration --by obligor --limit 10% --json --select obligor,exposure,share
```

Consolidated obligor exposure with limit-breach flags; --select narrows the deeply nested rollup.

### Find the best place for EUR cash

```bash
ts screen --currency EUR --min-tenor 3m --top 10 --json
```

Ranked best-yield indications across all entities.

## Usage

Run `ts-pp-cli --help` for the full command reference and flag list.

## Commands

### cell

Get information about Cells

- **`ts-pp-cli cell <code>`** - Retrieves data for a single Cell.

### entity

Manage entity

- **`ts-pp-cli entity get`** - Retrieves data for a single Entity if the user has permission to view it.
- **`ts-pp-cli entity get-entities`** - Retrieves a list of all entities that the user has permission to view.

### event

Stream of normalised events for integration and reconciliation

- **`ts-pp-cli event checkpoint`** - Delete a named event checkpoint.

If `expected_checkpoint_version` is supplied, the request uses it as an optimistic
lock and returns 409 Conflict if the checkpoint has been modified since you last read it.
- **`ts-pp-cli event checkpoint-checkpoint`** - Return a single event checkpoint by name.
- **`ts-pp-cli event checkpoint-checkpoint-2`** - Advance the checkpoint to a new cursor position.

Supply the `new_cursor` to move to. If you also provide the
`expected_checkpoint_version` from the last read, the request uses it as an optimistic
lock and returns 409 Conflict if the checkpoint has changed since you last read it.
- **`ts-pp-cli event checkpoint-checkpoint-3`** - Create a new named event checkpoint, or return the existing one if it already exists.

On creation, the cursor is initialised to initial_cursor if provided, or to the current
end of the event stream otherwise. initial_cursor is ignored if the checkpoint already exists.
- **`ts-pp-cli event checkpoints`** - Return all event checkpoints for the authenticated user, ordered by name.
- **`ts-pp-cli event get`** - Return a page of events from the stream.

Supports cursor-based pagination with an optional timestamp lower bound.

### health

Manage health

- **`ts-pp-cli health`** - Perform a health check by returning a JSON response with a status code of 200 (OK).

### holding

Get information about holdings. For how subscriptions become holdings and how holdings move through their lifecycle, see the FTF Lifecycle section.

- **`ts-pp-cli holding get`** - Retrieves a list of all holdings that the user has permission to view.
- **`ts-pp-cli holding get-entitycode`** - Retrieves data for a single holding if the user has permission to view it.

### holidays

Manage holidays

- **`ts-pp-cli holidays`** - Retrieves a list of all holidays for a given year.

### indication

Get information about Indications

- **`ts-pp-cli indication <code>`** - Retrieves a list of all Indications that the user has permission to view.

### oauth

OAuth 2.0 endpoint to exchange your Client Credentials for a token. This token can then be used to access the API.

- **`ts-pp-cli oauth`** - Obtain an access token using the client credentials grant type.

### obligor-exposure

Get information about Obligors

- **`ts-pp-cli obligor-exposure <code>`** - Get data for obligor exposure by code

### subscribe

Manage subscribe

- **`ts-pp-cli subscribe`** - Subscribe to an FTF

### subscription

FTF Subscriptions

- **`ts-pp-cli subscription`** - Retrieves a list of all subscriptions that the user has permission to view.

### task

Get information about Pending Tasks

- **`ts-pp-cli task get`** - Retrieves a list of all pending tasks that the user has.
- **`ts-pp-cli task get-uid`** - Retrieves a pending task by uid.
- **`ts-pp-cli task post`** - Used to approve or deny a pending task.

### webhook

Integrate with webhooks to receive notifications

- **`ts-pp-cli webhook delete`** - Deregister an existing webhook for a user
- **`ts-pp-cli webhook post`** - Register a url to a user for webhook notifications


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ts-pp-cli cell mock-value

# JSON for scripting and agents
ts-pp-cli cell mock-value --json

# Filter to specific fields
ts-pp-cli cell mock-value --json --select id,name,status

# Dry run — show the request without sending
ts-pp-cli cell mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ts-pp-cli cell mock-value --agent
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
ts-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/treasuryspring-public-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TS_BEARER_TOKEN` | per_call | No | Set to your API credential. |
| `TS_CLIENT_ID` | auth_flow_input | No | OAuth2 client id; used by `ts auth login` to mint a token at /oauth/token. |
| `TS_CLIENT_SECRET` | auth_flow_input | No | Set during initial auth setup. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ts-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ts-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TS_BEARER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / Authorization field missing** — Run `ts auth login` (or set TS_BEARER_TOKEN). Every endpoint, including health, needs a bearer token.
- **Empty ladder/concentration output** — Run `ts sync` first; transcendence commands read the local mirror.
- **Testing against sandbox** — Set TS_ENV=sandbox or pass --base-url https://api.sandbox.treasuryspring.com/api/v1.
