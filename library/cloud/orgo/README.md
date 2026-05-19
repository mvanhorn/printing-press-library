# Orgo CLI

**Thin Go-binary alias of the Orgo MCP server — every MCP tool, accessible from a shell.**

Mirrors the @orgo-ai/mcp server's tools as Cobra commands across projects (workspaces), computers, screen actions, shell, and files. The CLI uses `projects` as the resource name to match the Orgo API path; the MCP exposes the same resource as `workspaces` semantically. Use this CLI to script Orgo from cron, CI, or the terminal when the MCP transport is the wrong shape.

Learn more at [Orgo](https://orgo.ai).

Printed by [@NickVasilescu](https://github.com/NickVasilescu) (NickVasilescu).

## Install

The recommended path installs both the `orgo-pp-cli` binary and the `pp-orgo` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install orgo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install orgo --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/orgo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-orgo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-orgo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-orgo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-orgo. The skill defines how its required CLI can be installed.
```

## Authentication

Bearer auth via ORGO_API_KEY (sk_live_...). Get a key at https://www.orgo.ai/workspaces. Same env var the MCP reads.

## Quick Start

```bash
# Sanity check: auth works, returns your projects
orgo-pp-cli projects list

# Provision a fresh VM in <500ms
orgo-pp-cli computers create --workspace-id ws_123 --name agent-1 --ram 8 --cpu 2

# Capture the desktop
orgo-pp-cli computers screenshot get cmp_456

# Run a shell command on the VM
orgo-pp-cli computers bash execute cmp_456 --command "ls /home/orgo"

# Pause execution between screen actions
orgo-pp-cli computers wait wait cmp_456 --duration 2
```

## Usage

Run `orgo-pp-cli --help` for the full command reference and flag list. Use `orgo-pp-cli which "<capability>"` to look up a command by natural-language query.

## Commands

### Workspaces (projects)

| Command | Description |
| --- | --- |
| `projects list` | List all workspaces for the authenticated user |
| `projects get <id>` | Return a workspace by ID, including its computers |
| `projects get-by-name <name>` | Look up a workspace by name |
| `projects create` | Create a new workspace |
| `projects delete <id>` | Delete a workspace and all its computers |

### Computers (lifecycle)

| Command | Description |
| --- | --- |
| `computers get <id>` | Return computer details including status |
| `computers create` | Provision a new computer in a workspace |
| `computers delete <id>` | Permanently delete a computer |
| `computers clone computer <id>` | Clone a computer with the same disk state |
| `computers move computer <id>` | Move a computer to a different workspace |
| `computers resize computer <id>` | Live-resize CPU, RAM, disk, or bandwidth |
| `computers restart computer <id>` | Restart a computer (stop + start) |
| `computers ensure-running ensure-computer-running <id>` | Idempotently resume a suspended VM |

### Screen actions

| Command | Description |
| --- | --- |
| `computers screenshot get <id>` | Capture a screenshot (base64 PNG or URL) |
| `computers click mouse <id>` | Click at the given coordinates |
| `computers drag mouse <id>` | Drag from start to end coordinates |
| `computers scroll scroll <id>` | Scroll the mouse wheel |
| `computers type text <id>` | Type literal text at the cursor |
| `computers key press <id>` | Press a key or key combination |
| `computers wait wait <id>` | Pause execution for N seconds |

### Shell & code execution

| Command | Description |
| --- | --- |
| `computers bash execute <id>` | Run a bash command on the computer |
| `computers exec execute-python <id>` | Execute Python code on the computer |

### Files

| Command | Description |
| --- | --- |
| `files list` | List files in a workspace |
| `files upload` | Upload a file (max 10MB) |
| `files download` | Get a signed download URL (expires in 1h) |
| `files export` | Export a file from a computer's filesystem |

### Account & utilities

| Command | Description |
| --- | --- |
| `doctor` | Check config, auth, connectivity, and local cache |
| `auth status` / `auth set-token` / `auth logout` | Manage stored credentials |
| `version` | Print version |
| `which <capability>` | Find the command for a natural-language query |
| `agent-context` | Emit a machine-readable description of this CLI |
| `feedback` | Record a one-liner of friction (local by default) |

### Local data layer

| Command | Description |
| --- | --- |
| `sync` | Sync API data to local SQLite for offline use |
| `search <query>` | Full-text search across synced data |
| `export <resource>` | Export resource data to JSONL or JSON |
| `import` | Import JSONL into the API via create/upsert |
| `load` | Show workload distribution per assignee |
| `stale` | Find items with no updates in N days |
| `orphans` | Find items missing key fields |
| `profile save/list/show/delete/use` | Save flag values for reuse |


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
orgo-pp-cli computers get mock-value

# JSON for scripting and agents
orgo-pp-cli computers get mock-value --json

# Filter to specific fields
orgo-pp-cli computers get mock-value --json --select id,name,status

# Dry run — show the request without sending
orgo-pp-cli computers get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
orgo-pp-cli computers get mock-value --agent
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

## Cookbook

```bash
# Discover the workspace ID for an existing named workspace
orgo-pp-cli projects get-by-name production --json --select id

# Provision a beefier VM in a known workspace
orgo-pp-cli computers create \
  --workspace-id ws_123 \
  --name agent-builder \
  --ram 8 --cpu 2 --disk-size-gb 16

# Ensure a computer is running before driving it (resumes if suspended)
orgo-pp-cli computers ensure-running ensure-computer-running cmp_456

# Open a terminal on the VM, list a directory
orgo-pp-cli computers bash execute cmp_456 --command "ls -la /home/orgo"

# Run Python with stdin so multi-line code does not fight shell quoting
echo 'import platform; print(platform.uname())' | \
  orgo-pp-cli computers exec execute-python cmp_456 --stdin

# Screenshot, save to a file via --deliver
orgo-pp-cli computers screenshot get cmp_456 \
  --deliver file:./screen.json

# Type into a focused window, then press Enter to submit
orgo-pp-cli computers type text cmp_456 --text "hello world"
orgo-pp-cli computers key press cmp_456 --key Enter

# Mouse-drive a UI: double-click at coordinates, then scroll
orgo-pp-cli computers click mouse cmp_456 --x 480 --y 360 --double
orgo-pp-cli computers scroll scroll cmp_456 --direction down --amount 5

# Move a computer between workspaces (no disk copy)
orgo-pp-cli computers move computer cmp_456 --project-id ws_789

# Live-resize a running computer
orgo-pp-cli computers resize computer cmp_456 --vcpus 4 --mem-gb 16

# Upload a file to a workspace, then list it
orgo-pp-cli files upload --project-id ws_123 --file ./local.png
orgo-pp-cli files list --project-id ws_123 --json --select id,name,size

# Export a built file from inside the VM, fetch its download URL
orgo-pp-cli files export --desktop-id cmp_456 --path /home/orgo/build.zip

# Save a profile for a scheduled job, then reuse it
orgo-pp-cli profile save daily-snap --agent --select id,name,status
orgo-pp-cli --profile daily-snap computers get cmp_456

# Delete a workspace and everything in it (cannot be undone)
orgo-pp-cli projects delete ws_123 --yes
```

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-orgo -g
```

Then invoke `/pp-orgo <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add orgo orgo-pp-mcp -e ORGO_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/orgo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ORGO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "orgo": {
      "command": "orgo-pp-mcp",
      "env": {
        "ORGO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
$ orgo-pp-cli doctor
  OK   Config: ok
  OK   Auth: env:ORGO_API_KEY
  OK   Env Vars: ORGO_API_KEY=sk_live_...
  OK   API: reachable
  config_path: ~/.config/orgo-pp-cli/config.toml
  base_url: https://www.orgo.ai/api
  version: 1.0.0
```

Use `--json` for machine-readable output and `--fail-on warn|error` to make doctor exit non-zero when something is wrong (useful in CI).

## Configuration

Config file: `~/.config/orgo-pp-cli/config.toml`

Environment variables:

| Name | Required | Description |
| --- | --- | --- |
| `ORGO_API_KEY` | Yes | Bearer token (sk_live_...). Same env var the @orgo-ai/mcp server reads. |
| `ORGO_BASE_URL` | No | Override the API base URL. Default: `https://www.orgo.ai/api`. |
| `ORGO_CONFIG` | No | Override the config file path. Default: `~/.config/orgo-pp-cli/config.toml`. |
| `ORGO_FEEDBACK_ENDPOINT` | No | URL to POST `feedback` entries to. When unset, feedback is local-only. |
| `ORGO_FEEDBACK_AUTO_SEND` | No | Set to `true` to auto-POST feedback when the endpoint is configured. Otherwise pass `--send`. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `orgo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ORGO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized** — export ORGO_API_KEY=sk_live_... — rotate at https://www.orgo.ai/workspaces
- **404 on computer ID after restart** — fly_instance_id can change; call orgo-pp-cli computers get <id> to refresh
- **Computer suspended** — orgo-pp-cli computers ensure-running ensure-computer-running <id> — idempotent resume

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
