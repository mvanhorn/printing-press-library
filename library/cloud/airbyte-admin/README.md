# Airbyte Admin CLI

Airbyte is a data integration platform used by data teams to move data from sources into warehouses, lakes, and operational destinations. This CLI gives engineers and agents a read-only terminal surface for inspecting Airbyte workspaces, sources, destinations, connections, sync jobs, connector definitions, users, organizations, permissions, and tags.

It is generated from Airbyte's official Public API specification and works with Airbyte Cloud or a self-managed Airbyte deployment. The default base URL is `https://cloud.airbyte.com/api`; local self-managed testing can point `AIRBYTE_ADMIN_BASE_URL` at `http://localhost:8000/api`.

Most operational commands require an Airbyte API credential. Configure either `AIRBYTE_ADMIN_TOKEN` for bearer-token auth or `AIRBYTE_ADMIN_AUTH_HEADER` when your deployment needs a full Authorization header such as `Basic ...`.

Learn more at [Airbyte](https://airbyte.io).

Printed by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `airbyte-admin-pp-cli` binary and the `pp-airbyte-admin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install airbyte-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install airbyte-admin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install airbyte-admin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install airbyte-admin --agent claude-code
npx -y @mvanhorn/printing-press-library install airbyte-admin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/airbyte-admin/cmd/airbyte-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airbyte-admin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airbyte-admin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airbyte-admin --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-airbyte-admin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-airbyte-admin. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airbyte-admin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/airbyte-admin/cmd/airbyte-admin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "airbyte-admin": {
      "command": "airbyte-admin-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
airbyte-admin-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
airbyte-admin-pp-cli public get-health-check --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`public list-sources`** — Lists Airbyte sources, with sibling destination and connection-detail commands for inspecting configured data movement.
- **`public list-jobs`** — Lists sync jobs with connection filters and pagination for troubleshooting recent ELT activity.
- **`public list-connector-definitions`** — Shows available source or destination connector definitions for a workspace or organization.
- **`public list-workspaces`** — Surfaces workspaces, users, organizations, permissions, and tags for Airbyte operational review.
- **`sync`** — Caches Airbyte read endpoints locally for offline search, export, and repeatable agent analysis.

## Usage

Run `airbyte-admin-pp-cli --help` for the full command reference and flag list.

## Commands

### public

Manage public

- **`airbyte-admin-pp-cli public get-application`** - Get an Application detail
- **`airbyte-admin-pp-cli public get-connection`** - Get Connection details
- **`airbyte-admin-pp-cli public get-destination`** - Get Destination details
- **`airbyte-admin-pp-cli public get-documentation`** - Root path, currently returns a redirect to the documentation
- **`airbyte-admin-pp-cli public get-health-check`** - Health Check
- **`airbyte-admin-pp-cli public get-job`** - Get Job status and details
- **`airbyte-admin-pp-cli public get-source`** - Get Source details
- **`airbyte-admin-pp-cli public get-stream-properties`** - Get stream properties
- **`airbyte-admin-pp-cli public list-applications`** - List Applications
- **`airbyte-admin-pp-cli public list-connector-definitions`** - List connector definitions
- **`airbyte-admin-pp-cli public list-destinations`** - List destinations
- **`airbyte-admin-pp-cli public list-jobs`** - List Jobs
- **`airbyte-admin-pp-cli public list-organizations-for-user`** - List all organizations for a user
- **`airbyte-admin-pp-cli public list-permissions`** - List permissions
- **`airbyte-admin-pp-cli public list-sources`** - List sources
- **`airbyte-admin-pp-cli public list-tags`** - Lists all tags
- **`airbyte-admin-pp-cli public list-users`** - Organization Admin user can list all users within the same organization. Also provide filtering on a list of user IDs or/and a list of user emails.
- **`airbyte-admin-pp-cli public list-workspaces`** - List workspaces
- **`airbyte-admin-pp-cli public oauth-callback`** - Redirected to by identity providers after authentication.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airbyte-admin-pp-cli public list-sources

# JSON for scripting and agents
airbyte-admin-pp-cli public list-sources --json

# Filter to specific fields
airbyte-admin-pp-cli public list-sources --json --select id,name,status

# Dry run — show the request without sending
airbyte-admin-pp-cli public list-sources --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airbyte-admin-pp-cli public list-sources --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
airbyte-admin-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/airbyte-admin-pp-cli/config.toml`

Environment variables:

| Variable | Purpose |
|----------|---------|
| `AIRBYTE_ADMIN_BASE_URL` | Override the API root. Use `http://localhost:8000/api` for a local self-managed Airbyte instance. |
| `AIRBYTE_ADMIN_TOKEN` | Bearer token for Airbyte Cloud or a protected self-managed Public API. The CLI adds the `Bearer ` prefix if omitted. |
| `AIRBYTE_ADMIN_AUTH_HEADER` | Full Authorization header, useful for local deployments that require `Basic ...` or another scheme. |
| `AIRBYTE_ADMIN_CONFIG` | Optional config file path override. |

Config file example:

```toml
base_url = "http://localhost:8000/api"
auth_header = "Bearer <token>"
```

Static request headers can also be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
