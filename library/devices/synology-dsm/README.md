# Synology DSM CLI

A command-line interface for **Synology DiskStation Manager (DSM 7.x)** — monitor storage health, run backups, manage Docker containers, schedule tasks, and browse files on your NAS from any terminal or AI agent.

## What It Does

**One command to check if your NAS is healthy:**

```bash
synology-dsm-pp-cli health
# ✓ Overall: ok
# ✓ Backup: ok (3 tasks)
# ✓ Disks: ok
# ✓ Volumes: ok (2 volumes)
```

**Storage at a glance — volumes, pools, disks, and SMART status:**

```bash
synology-dsm-pp-cli stats --json        # aggregate capacity across all volumes
synology-dsm-pp-cli utilization          # per-resource usage with ok/warn/critical severity
synology-dsm-pp-cli trends               # growth rates and estimated time-to-capacity
```

**Backup operations without logging into the web UI:**

```bash
synology-dsm-pp-cli webapi list-backup-tasks --json
synology-dsm-pp-cli webapi run-backup-task --task-id 1 --yes    # trigger now
synology-dsm-pp-cli webapi get-backup-task-status --task-id 1   # check progress
```

**Docker container management:**

```bash
synology-dsm-pp-cli webapi list-containers --json
synology-dsm-pp-cli webapi restart-container --name homeassistant --yes
synology-dsm-pp-cli webapi get-container-logs --name portainer --json
```

**Task Scheduler automation:**

```bash
synology-dsm-pp-cli webapi list-scheduled-tasks --json
synology-dsm-pp-cli webapi run-scheduled-task --id 3 --real-owner root --yes
synology-dsm-pp-cli webapi set-scheduled-task-enabled --id 3 --real-owner root --enable --yes
```

## Key Features

| Feature | Description |
|---|---|
| **46+ API commands** | Full coverage of Synology Web API: storage, backup, containers, scheduler, files, system info |
| **Offline sync & search** | `sync` snapshots NAS state to a local SQLite store; `search` provides instant full-text queries |
| **Agent-native** | Non-interactive, pipeable JSON output, `--dry-run` previews, `--select` field filtering, structured exit codes |
| **Health dashboard** | `health` aggregates backup/disk/volume status into a single ok/warn/error summary |
| **Utilization tracking** | `utilization` shows per-resource usage with configurable severity thresholds (`--warn 70 --critical 90`) |
| **Capacity planning** | `trends` computes storage growth rates and estimated days-to-full from sync history |
| **Export & scripting** | `export` dumps synced resources to JSON/CSV; `stats` gives aggregate capacity numbers |
| **MCP server** | Ships `synology-dsm-pp-mcp` for one-click integration with Claude Desktop, VS Code, and other MCP clients |
| **Offline-first store** | Local SQLite with FTS5 search, schema versioning, and WAL mode for concurrent reads |

## How It Works

All API calls go through Synology's `/webapi/entry.cgi` endpoint using session-based SID tokens. No API key — just authenticate with your DSM username and password:

```bash
# Get a session ID
synology-dsm-pp-cli webapi login --account admin --passwd <password> --json

# Save it
synology-dsm-pp-cli auth set-token <SID>

# Or set the environment variable
export SYNOLOGY_DSM_SIDCOOKIE="<SID>"
```

Point the CLI at your NAS with two environment variables:

```bash
export SYNOLOGY_DSM_HOST="192.168.1.100"    # or your Synology hostname
export SYNOLOGY_DSM_PORT="5000"              # 5000 for HTTP, 5001 for HTTPS
```

Then verify everything works:

```bash
synology-dsm-pp-cli doctor
```

## Learn More

- [Synology Web API](https://github.com/synology-community/go-synology)
- [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) — how this CLI was generated

Printed by [@e-jung](https://github.com/e-jung) (Eric Jung).

## Install

The recommended path installs both the `synology-dsm-pp-cli` binary and the `pp-synology-dsm` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install synology-dsm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install synology-dsm --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install synology-dsm --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install synology-dsm --agent claude-code
npx -y @mvanhorn/printing-press install synology-dsm --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/synology-dsm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-synology-dsm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-synology-dsm --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-synology-dsm skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-synology-dsm. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/synology-dsm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SYNOLOGY_DSM_SIDCOOKIE` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "synology-dsm": {
      "command": "synology-dsm-pp-mcp",
      "env": {
        "SYNOLOGY_DSM_HOST": "<host>",
        "SYNOLOGY_DSM_PORT": "<port>",
        "SYNOLOGY_DSM_SIDCOOKIE": "<your-key>"
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

Set the endpoint variables for the tenant, workspace, or API version you want this CLI to use:

```bash
export SYNOLOGY_DSM_HOST="<host>"
export SYNOLOGY_DSM_PORT="<port>"
```

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export SYNOLOGY_DSM_SIDCOOKIE="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/synology-dsm-pp-cli/config.toml`.

### 3. Verify Setup

```bash
synology-dsm-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
synology-dsm-pp-cli webapi cancel-backup-task
```

## Usage

Run `synology-dsm-pp-cli --help` for the full command reference and flag list.

## Commands

### webapi

Manage webapi

- **`synology-dsm-pp-cli webapi cancel-backup-task`** - Cancel an in-progress Hyper Backup task
- **`synology-dsm-pp-cli webapi delete-scheduled-task`** - Delete a scheduled task permanently
- **`synology-dsm-pp-cli webapi get-backup-task`** - Get details for a specific Hyper Backup task
- **`synology-dsm-pp-cli webapi get-backup-task-status`** - Get current status and progress of a Hyper Backup task
- **`synology-dsm-pp-cli webapi get-container`** - Get detailed configuration and status for a specific container
- **`synology-dsm-pp-cli webapi get-container-logs`** - Get recent log output from a container
- **`synology-dsm-pp-cli webapi get-dsminfo`** - Get DSM system information — model, version, uptime, CPU
- **`synology-dsm-pp-cli webapi get-file-info`** - Get metadata for specific files or directories
- **`synology-dsm-pp-cli webapi get-file-station-info`** - Get File Station capabilities and hostname
- **`synology-dsm-pp-cli webapi get-scheduled-task`** - Get configuration for a specific scheduled task
- **`synology-dsm-pp-cli webapi get-storage-disk`** - Get detailed information for a specific disk including SMART data
- **`synology-dsm-pp-cli webapi get-storage-volume`** - Get details for a specific volume
- **`synology-dsm-pp-cli webapi get-system-utilization`** - Get real-time CPU, memory, network, and disk utilization
- **`synology-dsm-pp-cli webapi list-backup-repositories`** - List all Hyper Backup repositories (destinations)
- **`synology-dsm-pp-cli webapi list-backup-tasks`** - List all Hyper Backup tasks with schedule and destination info
- **`synology-dsm-pp-cli webapi list-container-images`** - List all downloaded Docker images
- **`synology-dsm-pp-cli webapi list-containers`** - List all Docker containers with running status and image
- **`synology-dsm-pp-cli webapi list-files`** - List files and directories in a folder path
- **`synology-dsm-pp-cli webapi list-scheduled-tasks`** - List all Task Scheduler tasks with schedule and enable status
- **`synology-dsm-pp-cli webapi list-shared-folders`** - List all shared folders visible to the authenticated user
- **`synology-dsm-pp-cli webapi list-storage-disks`** - List all disks with health status, temperature, and SMART indicators
- **`synology-dsm-pp-cli webapi list-storage-pools`** - List all storage pools (RAID groups) with health and usage
- **`synology-dsm-pp-cli webapi list-storage-volumes`** - List all storage volumes with usage and mount point
- **`synology-dsm-pp-cli webapi login`** - Authenticate with DSM and obtain a session ID (SID). After login, save the returned sid with: auth set-token id=<SID_VALUE>
- **`synology-dsm-pp-cli webapi logout`** - Log out and invalidate the current session
- **`synology-dsm-pp-cli webapi restart-container`** - Restart a container (stop then start)
- **`synology-dsm-pp-cli webapi run-backup-task`** - Trigger an immediate Hyper Backup for a task
- **`synology-dsm-pp-cli webapi run-scheduled-task`** - Run a scheduled task immediately (outside its normal schedule)
- **`synology-dsm-pp-cli webapi set-scheduled-task-enabled`** - Enable or disable a scheduled task
- **`synology-dsm-pp-cli webapi start-container`** - Start a stopped container
- **`synology-dsm-pp-cli webapi stop-container`** - Stop a running container gracefully


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
synology-dsm-pp-cli webapi cancel-backup-task

# JSON for scripting and agents
synology-dsm-pp-cli webapi cancel-backup-task --json

# Filter to specific fields
synology-dsm-pp-cli webapi cancel-backup-task --json --select id,name,status

# Dry run — show the request without sending
synology-dsm-pp-cli webapi cancel-backup-task --dry-run

# Agent mode — JSON + compact + no prompts in one flag
synology-dsm-pp-cli webapi cancel-backup-task --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SYNOLOGY_DSM_HOST` resolves `{host}`
- `SYNOLOGY_DSM_PORT` resolves `{port}`

Base URL: `http://{host}:{port}`

## Health Check

```bash
synology-dsm-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/synology-dsm-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SYNOLOGY_DSM_HOST` | endpoint | Yes |  |
| `SYNOLOGY_DSM_PORT` | endpoint | Yes |  |
| `SYNOLOGY_DSM_SIDCOOKIE` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `synology-dsm-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SYNOLOGY_DSM_SIDCOOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Cookbook

Common workflows and agent recipes.

### Check NAS health before a backup run

```bash
# One-shot health check — backup status, disk SMART, and volume usage
synology-dsm-pp-cli health --json

# Trigger backup only if NAS is healthy
STATUS=$(synology-dsm-pp-cli health --json --select overall | jq -r '.overall')
if [ "$STATUS" = "ok" ]; then
  synology-dsm-pp-cli webapi run-backup-task --task-id 1 --yes
fi
```

### Manage containers

```bash
# List all running containers
synology-dsm-pp-cli webapi list-containers --type running --json

# Restart a container by name
synology-dsm-pp-cli webapi restart-container --name homeassistant --yes

# Get recent logs for a container
synology-dsm-pp-cli webapi get-container-logs --name portainer --json
```

### Storage capacity planning

```bash
# Aggregate storage stats (total/used/free/usage%)
synology-dsm-pp-cli stats --json

# Check all disk health
synology-dsm-pp-cli webapi list-storage-disks --json \
  | jq '[.data[] | select(.exceed_bad_sector_thr or .below_remain_life_thr)]'
```

### Automate scheduled tasks

```bash
# List all scheduled tasks
synology-dsm-pp-cli webapi list-scheduled-tasks --json

# Run a specific task immediately
synology-dsm-pp-cli webapi run-scheduled-task --id 3 --real-owner root --yes

# Enable a task
synology-dsm-pp-cli webapi set-scheduled-task-enabled --id 3 --real-owner root --enable --yes
```

### Agent workflow (MCP)

```bash
# Start the MCP server (stdio — for Claude Desktop / VS Code)
synology-dsm-pp-mcp

# Start the MCP server (HTTP — for cloud agents)
SYNOLOGY_DSM_MCP_TRANSPORT=http SYNOLOGY_DSM_MCP_PORT=8080 synology-dsm-pp-mcp
```

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
