# Exa CLI



## Install

The recommended path installs both the `exa-pp-cli` binary and the `pp-exa` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install exa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install exa --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install exa --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install exa --agent claude-code
npx -y @mvanhorn/printing-press-library install exa --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/exa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install exa --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-exa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-exa --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install exa --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/exa-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EXA_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "exa": {
      "command": "exa-pp-mcp",
      "env": {
        "EXA_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
exa-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export EXA_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
exa-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
exa-pp-cli contents
```

## Usage

Run `exa-pp-cli --help` for the full command reference and flag list.

## Commands

### agent

Manage agent

- **`exa-pp-cli agent cancel-run`** - Cancel a queued or running Agent run. If the run has already reached a terminal status, the API returns the existing run.
- **`exa-pp-cli agent create-run`** - Create an asynchronous Agent run. By default, the API returns the run object immediately. Set `Accept: text/event-stream` to stream run lifecycle events until the run reaches a terminal status.
- **`exa-pp-cli agent delete-run`** - Delete a stored Agent run.
- **`exa-pp-cli agent get-run`** - Retrieve a single Agent run by ID.
- **`exa-pp-cli agent list-run-events`** - List stored events for an Agent run. Set `Accept: text/event-stream` to replay stored events as server-sent events. Use `cursor` for JSON pagination or `Last-Event-ID` for SSE replay.
- **`exa-pp-cli agent list-runs`** - List Agent runs for your team, ordered from newest to oldest.

### answer

Manage answer

- **`exa-pp-cli answer`** - Performs a search based on the query and generates either a direct answer or a detailed summary with citations, depending on the query type.

### contents

Manage contents

- **`exa-pp-cli contents`** - Contents

### events

Manage events

- **`exa-pp-cli events get`** - Get a single Event by id.

You can subscribe to Events by creating a Webhook.
- **`exa-pp-cli events list`** - List all events that have occurred in the system.

You can paginate through the results using the `cursor` parameter.

### imports

Manage imports

- **`exa-pp-cli imports create`** - Creates a new import to upload your data into Websets. Imports can be used to:

- **Enrich**: Enhance your data with additional information using our AI-powered enrichment engine
- **Search**: Query your data using Websets' agentic search with natural language filters
- **Exclude**: Prevent duplicate or already known results from appearing in your searches

Once the import is created, you can upload your data to the returned `uploadUrl` until `uploadValidUntil` (by default 1 hour).
- **`exa-pp-cli imports delete`** - Deletes a import.
- **`exa-pp-cli imports get`** - Gets a specific import.
- **`exa-pp-cli imports list`** - Lists all imports for the Webset.
- **`exa-pp-cli imports update`** - Updates a import configuration.

### monitors

Manage monitors

- **`exa-pp-cli monitors batch`** - Perform a batch action on monitors matching the provided filters.

Supported actions:
- **delete**: Permanently remove matching monitors
- **pause**: Pause matching monitors
- **unpause**: Unpause matching monitors

Use `dry_run: true` (the default) to preview which monitors would be affected before performing the action. Results are paginated via the `limit` parameter; loop until `has_more` is `false` to process all matching monitors.
- **`exa-pp-cli monitors create`** - Creates a new Monitor to run recurring Exa searches on a schedule.

Monitors automatically execute your search query on a recurring schedule and deliver results to your webhook endpoint with automatic deduplication:

- **Date-based filtering** only fetches content since the last run

- **Semantic deduplication** tracks previous outputs to surface only new developments

The response includes a `webhookSecret` that is only returned once at creation time. Store it securely for webhook signature verification.
- **`exa-pp-cli monitors create-endpoint`** - Creates a new `Monitor` to continuously keep your Websets updated with fresh data.

Monitors automatically run on your defined schedule to ensure your Websets stay current without manual intervention:

- **Find new content**: Execute `search` operations to discover fresh items matching your criteria
- **Update existing content**: Run `refresh` operations to update items contents and enrichments
- **Automated scheduling**: Configure `cron` expressions and `timezone` for precise scheduling control
- **`exa-pp-cli monitors delete`** - Deletes a monitor. This cannot be undone.
- **`exa-pp-cli monitors delete-id`** - Deletes a monitor.
- **`exa-pp-cli monitors get`** - Retrieves a single monitor by its ID.
- **`exa-pp-cli monitors get-id`** - Gets a specific monitor.
- **`exa-pp-cli monitors list`** - Lists all monitors for the authenticated team. Supports filtering by status and cursor-based pagination.
- **`exa-pp-cli monitors list-endpoint`** - Lists all monitors for the Webset.
- **`exa-pp-cli monitors update`** - Updates an existing monitor. All fields are optional. For `search`, you can send a partial object containing only the fields you want to change. Set `trigger` to `null` to remove the schedule.
- **`exa-pp-cli monitors update-id`** - Updates a monitor configuration.

### neural_search

Manage neural search

- **`exa-pp-cli neural-search`** - Perform a search with an Exa prompt-engineered query and retrieve a list of relevant results. Optionally get contents.

### research

Manage research

- **`exa-pp-cli research create`** - Create a new research request
- **`exa-pp-cli research get`** - Retrieve research by ID. Add ?stream=true for real-time SSE updates.
- **`exa-pp-cli research list`** - Get a paginated list of research requests

### teams

Manage teams

- **`exa-pp-cli teams`** - Returns information about the authenticated team, including current concurrency usage and limits.

### webhooks

Manage webhooks

- **`exa-pp-cli webhooks create`** - Create a Webhook
- **`exa-pp-cli webhooks delete`** - Delete a Webhook
- **`exa-pp-cli webhooks get`** - Get a Webhook
- **`exa-pp-cli webhooks list`** - List webhooks
- **`exa-pp-cli webhooks update`** - Update a Webhook

### websets

Manage websets

- **`exa-pp-cli websets create`** - Creates a new Webset with optional search, import, and enrichment configurations. The Webset will automatically begin processing once created.

You can specify an `externalId` to reference the Webset with your own identifiers for easier integration.
- **`exa-pp-cli websets delete`** - Deletes a Webset.

Once deleted, the Webset and all its Items will no longer be available.
- **`exa-pp-cli websets get`** - Get a Webset
- **`exa-pp-cli websets list`** - Returns a list of Websets.

You can paginate through the results using the `cursor` parameter.

You can filter results using the `search` parameter to find Websets by ID, external ID, or title.
- **`exa-pp-cli websets preview`** - Preview how a search query will be decomposed before creating a webset. This endpoint performs the same query analysis that happens during webset creation, allowing you to see the detected entity type, generated search criteria, and available enrichment columns in advance.

Use this to help users understand how their search will be interpreted before committing to a full webset creation.
- **`exa-pp-cli websets update`** - Update a Webset


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
exa-pp-cli contents

# JSON for scripting and agents
exa-pp-cli contents --json

# Filter to specific fields
exa-pp-cli contents --json --select id,name,status

# Dry run — show the request without sending
exa-pp-cli contents --dry-run

# Agent mode — JSON + compact + no prompts in one flag
exa-pp-cli contents --agent
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
exa-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/exa-public-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EXA_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `exa-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `exa-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EXA_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
