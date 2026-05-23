# Vibecode CLI

**Every Vibecode feature, plus offline sync, cross-project search, and deployment drift detection no other tool has**

A local-first CLI for the vibecode.dev platform that caches your projects, deployments, and commits in SQLite for instant offline access. Adds cross-entity search, since-style delta commands, and deployment drift detection that the official CLI cannot offer.

Printed by [@RAbuseedo](https://github.com/RAbuseedo).

## Install

The recommended path installs both the `vibecode-pp-cli` binary and the `pp-vibecode` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install vibecode
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install vibecode --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install vibecode --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install vibecode --agent claude-code
npx -y @mvanhorn/printing-press-library install vibecode --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/cmd/vibecode-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vibecode-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-vibecode --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-vibecode --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-vibecode skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-vibecode. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vibecode-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `VIBECODE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/cmd/vibecode-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "vibecode": {
      "command": "vibecode-pp-mcp",
      "env": {
        "VIBECODE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set VIBECODE_API_KEY from vibecode.dev/key. The CLI validates on first use and caches your user profile locally.

## Quick Start

```bash
# Verify API key and connectivity
vibecode-pp-cli doctor

# Pull all projects, deployments, and commits locally
vibecode-pp-cli sync --full

# List projects from local cache
vibecode-pp-cli projects list

# Search across all entities
vibecode-pp-cli search 'landing page'

# Build and deploy in one shot
vibecode-pp-cli yolo proj_abc --prompt 'Build a todo app'

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Full-text search across all projects, commits, and deployments in one query

  _Find that project or commit without remembering which entity type contains it_

  ```bash
  vibecode-pp-cli search 'landing page' --json
  ```
- **`changes --since`** — See what changed across all projects since a timestamp or last sync

  _Resume work with full context after stepping away_

  ```bash
  vibecode-pp-cli changes --since '2 hours ago' --json
  ```

### Production safety
- **`drift`** — Compare current live deployment against cached configuration to spot unexpected changes

  _Catch environment variable or build setting changes before they cause production issues_

  ```bash
  vibecode-pp-cli drift proj_abc123 --json
  ```
- **`stale --days`** — Find deployments across all projects that haven't been updated in N days

  _Clean up old deployments to reduce costs and attack surface_

  ```bash
  vibecode-pp-cli stale --days 30 --json
  ```

### Developer experience
- **`metrics builds`** — Track build times over history with averages, p95, and trend indicators

  _Identify builds getting slower before they become a bottleneck_

  ```bash
  vibecode-pp-cli metrics builds --project proj_abc123 --json
  ```
- **`batch deploy`** — Deploy multiple projects matching a glob pattern with parallelism control

  _Deploy coordinated changes across microservices in one command_

  ```bash
  vibecode-pp-cli batch deploy --pattern 'frontend-*' --parallel 3 --json
  ```

## Recipes


### Find stale deployments

```bash
vibecode-pp-cli stale --days 14 --json
```

List deployments not updated in 14 days for cleanup

### Check what changed

```bash
vibecode-pp-cli changes --since '1 day ago' --select name,status
```

See projects modified in the last day with only essential fields

### Batch deploy frontend

```bash
vibecode-pp-cli batch deploy --pattern 'frontend-*' --parallel 2
```

Deploy all frontend projects with controlled parallelism

### Detect deployment drift

```bash
vibecode-pp-cli drift proj_abc123 --json
```

Compare live deployment against cached config

### Build metrics

```bash
vibecode-pp-cli metrics builds --project proj_abc123 --agent
```

Get build duration trends with agent-optimized output

## Usage

Run `vibecode-pp-cli --help` for the full command reference and flag list.

## Commands

### agent

AI coding agent control

- **`vibecode-pp-cli agent send`** - Send prompt to coding agent (streams events)
- **`vibecode-pp-cli agent stop`** - Stop running agent task

### deployment_auth

HTTP Basic Auth for deployments

- **`vibecode-pp-cli deployment_auth disable`** - Disable HTTP Basic Auth
- **`vibecode-pp-cli deployment_auth get`** - Get HTTP Basic Auth configuration
- **`vibecode-pp-cli deployment_auth set`** - Enable HTTP Basic Auth

### deployment_domain

Custom domain configuration for deployments

- **`vibecode-pp-cli deployment_domain get`** - Get custom domain configuration
- **`vibecode-pp-cli deployment_domain remove`** - Remove custom domain
- **`vibecode-pp-cli deployment_domain set`** - Set custom domain
- **`vibecode-pp-cli deployment_domain verify`** - Verify DNS records for custom domain

### deployment_links

Tunnel links for deployments

- **`vibecode-pp-cli deployment_links create`** - Create deployment link
- **`vibecode-pp-cli deployment_links delete`** - Delete deployment link
- **`vibecode-pp-cli deployment_links list`** - List deployment links

### deployment_subdomain

Subdomain configuration for deployments

- **`vibecode-pp-cli deployment_subdomain check`** - Check subdomain availability
- **`vibecode-pp-cli deployment_subdomain set`** - Set deployment subdomain

### deployments

Production deployments

- **`vibecode-pp-cli deployments deploy`** - Deploy project (waits up to 2min)
- **`vibecode-pp-cli deployments destroy`** - Tear down deployment
- **`vibecode-pp-cli deployments get`** - Get deployment details
- **`vibecode-pp-cli deployments list`** - List deployments
- **`vibecode-pp-cli deployments ready`** - Check if deployment is live

### projects

Vibecode projects (web apps, mobile apps, openclaw)

- **`vibecode-pp-cli projects commits`** - List git commits for a project
- **`vibecode-pp-cli projects create`** - Create a new project
- **`vibecode-pp-cli projects delete`** - Delete a project permanently
- **`vibecode-pp-cli projects get`** - Get project details including subdomain and custom domain
- **`vibecode-pp-cli projects list`** - List all projects
- **`vibecode-pp-cli projects rename`** - Rename a project

### sandbox_links

Tunnel links for sandboxes

- **`vibecode-pp-cli sandbox_links create`** - Create tunnel link
- **`vibecode-pp-cli sandbox_links delete`** - Delete tunnel link
- **`vibecode-pp-cli sandbox_links list`** - List tunnel links for a sandbox

### sandboxes

Cloud VM sandboxes for development

- **`vibecode-pp-cli sandboxes acquire`** - Start sandbox and ensure tunnel links
- **`vibecode-pp-cli sandboxes get`** - Get sandbox status for a project
- **`vibecode-pp-cli sandboxes kill`** - Terminate sandbox
- **`vibecode-pp-cli sandboxes list`** - List running sandboxes

### user

Current authenticated user profile

- **`vibecode-pp-cli user`** - Get current user profile

### yolo

Combined build and deploy

- **`vibecode-pp-cli yolo <project_id>`** - Agent send + deploy in one shot


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
vibecode-pp-cli deployment_auth get mock-value

# JSON for scripting and agents
vibecode-pp-cli deployment_auth get mock-value --json

# Filter to specific fields
vibecode-pp-cli deployment_auth get mock-value --json --select id,name,status

# Dry run — show the request without sending
vibecode-pp-cli deployment_auth get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
vibecode-pp-cli deployment_auth get mock-value --agent
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
vibecode-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `VIBECODE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `vibecode-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $VIBECODE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Error: unauthorized** — Check VIBECODE_API_KEY is set. Get your key at vibecode.dev/key
- **Sync takes too long** — Use --project to sync a single project instead of --full
- **Sandbox not found** — Run 'sync' first or use 'sandboxes acquire' to start one
