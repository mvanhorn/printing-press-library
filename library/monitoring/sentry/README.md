# Sentry CLI

**A broad Sentry API CLI with local search, SQL, export, and MCP surfaces for incident work.**

Use Sentry from scripts and agents without memorizing endpoint shapes. The CLI wraps the catalog OpenAPI, supports structured output, and adds local sync/search/SQL paths for incident triage and release audits.

Learn more at [Sentry](https://sentry.io).

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sentry-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/monitoring/sentry/cmd/sentry-pp-cli@latest
```

## Authentication

Set SENTRY_AUTH_TOKEN to a Sentry user or organization token with read scopes such as org:read, project:read, and event:read. Set SENTRY_REGION=de for EU-region SaaS organizations, or configure the base URL for self-hosted Sentry if supported by the generated config.

## Quick Start

```bash
# Confirm token and API reachability before running endpoint commands.
sentry-pp-cli doctor --json


# Find the organization slug used by most Sentry API calls.
sentry-pp-cli organizations list --json --select slug,name


# List projects once you know the organization slug.
sentry-pp-cli organizations projects list-an-organization-s my-org --json --select slug,name


# Start incident triage with organization issues.
sentry-pp-cli organizations issues list-an-organization-s my-org --json --select shortId,title,count,userCount


# Search synced local data when you need offline or repeated context.
sentry-pp-cli search timeout --json --select title,url,resource

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Read-only Sentry inventory
- **`organizations list`** — List Sentry organizations available to the authenticated token with structured output.

  _Use this first when an agent needs to discover the organization slug for Sentry API work._

  ```bash
  sentry-pp-cli organizations list --json --select slug,name
  ```
- **`seer`** — List the active LLM model names available through Sentry Seer.

  _Use this when an agent needs to know which Seer-backed model identifiers Sentry exposes._

  ```bash
  sentry-pp-cli seer --json
  ```

## Usage

Run `sentry-pp-cli --help` for the full command reference and flag list.

## Commands

### organizations

Endpoints for organizations

- **`sentry-pp-cli organizations list-your`** - Return a list of organizations available to the authenticated session in a region.
This is particularly useful for requests with a user bound context. For API key-based requests this will only return the organization that belongs to the key.
- **`sentry-pp-cli organizations retrieve-an`** - Return details on an individual organization, including various details
such as membership access and teams.
- **`sentry-pp-cli organizations update-an`** - Update various attributes and configurable settings for the given organization.

### projects

Endpoints for projects

- **`sentry-pp-cli projects delete-a`** - Schedules a project for deletion.

Deletion happens asynchronously and therefore is not immediate. However once deletion has
begun the state of a project changes and will be hidden from most public views.
- **`sentry-pp-cli projects retrieve-a`** - Return details on an individual project.
- **`sentry-pp-cli projects update-a`** - Update various attributes and configurable settings for the given project.

Note that solely having the **`project:read`** scope restricts updatable settings to
`isBookmarked`, `autofixAutomationTuning`, `seerScannerAutomation`,
`preprodSizeStatusChecksEnabled`, `preprodSizeStatusChecksRules`,
`preprodSizeEnabledQuery`, `preprodDistributionEnabledQuery`,
`preprodSizeEnabledByCustomer`, `preprodDistributionEnabledByCustomer`,
and `preprodDistributionPrCommentsEnabledByCustomer`.

### seer

Endpoints for Seer features

- **`sentry-pp-cli seer list-ai-models`** - Get list of actively used LLM model names from Seer.

Returns the list of AI models that are currently used in production in Seer.
This endpoint does not require authentication and can be used to discover which models Seer uses.

Requests to this endpoint should use the region-specific domain
eg. `us.sentry.io` or `de.sentry.io`

### sentry-app-installations

Manage sentry app installations


### sentry-apps

Manage sentry apps

- **`sentry-pp-cli sentry-apps delete-a-custom-integration`** - Delete a custom integration.
- **`sentry-pp-cli sentry-apps retrieve-a-custom-integration-by-id-or-slug`** - Retrieve a custom integration.
- **`sentry-pp-cli sentry-apps update-an-existing-custom-integration`** - Update an existing custom integration.

### teams

Endpoints for teams

- **`sentry-pp-cli teams delete-a`** - Schedules a team for deletion.

**Note:** Deletion happens asynchronously and therefore is not
immediate. Teams will have their slug released while waiting for deletion.
- **`sentry-pp-cli teams retrieve-a`** - Return details on an individual team.
- **`sentry-pp-cli teams update-a`** - Update various attributes and configurable settings for the given
team.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sentry-pp-cli organizations list-your

# JSON for scripting and agents
sentry-pp-cli organizations list-your --json

# Filter to specific fields
sentry-pp-cli organizations list-your --json --select id,name,status

# Dry run — show the request without sending
sentry-pp-cli organizations list-your --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sentry-pp-cli organizations list-your --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-sentry -g
```

Then invoke `/pp-sentry <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/sentry/cmd/sentry-pp-mcp@latest
```

Then register it:

```bash
claude mcp add sentry sentry-pp-mcp -e SENTRY_AUTH_TOKEN=<your-token>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sentry --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sentry --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-sentry skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-sentry. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sentry-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SENTRY_AUTH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/sentry/cmd/sentry-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sentry": {
      "command": "sentry-pp-mcp",
      "env": {
        "SENTRY_AUTH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
sentry-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/sentry-pp-cli/config.toml`

Environment variables:
- `SENTRY_AUTH_TOKEN`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sentry-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SENTRY_AUTH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — Export SENTRY_AUTH_TOKEN with a valid Sentry token and rerun `sentry-pp-cli doctor --json`.
- **EU organization returns not found** — Set SENTRY_REGION=de or the generated base-url flag/env var before retrying.
- **Issue or project commands need a slug** — Run `sentry-pp-cli organizations list --json --select slug,name` and then project list commands to discover fixture slugs.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**getsentry/sentry-mcp**](https://github.com/getsentry/sentry-mcp) — TypeScript (673 stars)
- [**getsentry/cli**](https://github.com/getsentry/cli) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
