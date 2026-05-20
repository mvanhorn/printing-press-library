# Excalidraw Mcp CLI

Combined CLI for multiple API services

Printed by [@bk20260126-code](https://github.com/bk20260126-code) (bk20260126-code).

## Install

The recommended path installs both the `excalidraw-mcp-pp-cli` binary and the `pp-excalidraw-mcp` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install excalidraw-mcp
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install excalidraw-mcp --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install excalidraw-mcp --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install excalidraw-mcp --agent claude-code
npx -y @mvanhorn/printing-press install excalidraw-mcp --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/excalidraw-mcp-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-excalidraw-mcp --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-excalidraw-mcp --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-excalidraw-mcp skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-excalidraw-mcp. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/excalidraw-mcp-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EXCALIDRAW_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "excalidraw-mcp": {
      "command": "excalidraw-mcp-pp-mcp",
      "env": {
        "EXCALIDRAW_API_KEY": "<your-key>"
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
excalidraw-mcp-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export EXCALIDRAW_API_KEY="your-token-here"
```

### 3. Verify Setup

```bash
excalidraw-mcp-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
excalidraw-mcp-pp-cli elements list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`diff`** — Compare two canvas snapshots and see exactly which elements were added, removed, or moved.

  _Use when you need to audit diagram changes, review what an AI agent drew, or verify a refactoring didn't break diagram structure._

  ```bash
  excalidraw-mcp-pp-cli diff v1 v2 --json
  ```
- **`stats`** — See element type distribution, color palette in use, and bounding box summary for the current canvas.

  _Use to understand canvas composition before asking an AI to modify or extend a diagram._

  ```bash
  excalidraw-mcp-pp-cli stats --json --agent
  ```

### Agent-native plumbing
- **`convert`** — Convert a Mermaid diagram file to a PNG or SVG in one command — no separate steps needed.

  _Use in CI/CD pipelines to turn Mermaid specs into diagram images without manual canvas interaction._

  ```bash
  excalidraw-mcp-pp-cli convert --input flow.mmd --output diagram.png
  ```
- **`stale`** — Walk a directory for .excalidraw files and flag diagrams that haven't been updated in N days.

  _Use in CI to catch documentation diagrams that may be out of date with the codebase they describe._

  ```bash
  excalidraw-mcp-pp-cli stale --dir ./docs --since 90d --json
  ```
- **`agent-canvas-context`** — Emit a compact canvas summary (element count, type histogram, bounding box) sized for agent context windows.

  _Use at the start of any agent task that involves the canvas so the agent knows current state without reading all elements._

  ```bash
  excalidraw-mcp-pp-cli agent-canvas-context --agent --compact
  ```

## Usage

Run `excalidraw-mcp-pp-cli --help` for the full command reference and flag list.

## Commands

### collections

Manage collections

- **`excalidraw-mcp-pp-cli collections cloud-create`** - Create a scene collection
- **`excalidraw-mcp-pp-cli collections cloud-delete`** - Delete a collection
- **`excalidraw-mcp-pp-cli collections cloud-get`** - Get collection metadata
- **`excalidraw-mcp-pp-cli collections cloud-list`** - List scene collections
- **`excalidraw-mcp-pp-cli collections cloud-update`** - Update collection metadata

### elements

Manage elements

- **`excalidraw-mcp-pp-cli elements batch`** - Add an array of elements to the canvas in a single request. Preserves IDs when provided.
- **`excalidraw-mcp-pp-cli elements clear`** - Remove every element from the canvas. Irreversible unless a snapshot was saved first.
- **`excalidraw-mcp-pp-cli elements create`** - Add a new shape, text, or arrow to the Excalidraw canvas.
- **`excalidraw-mcp-pp-cli elements delete`** - Delete a canvas element
- **`excalidraw-mcp-pp-cli elements from-mermaid`** - Parse Mermaid diagram syntax and place the resulting elements on the canvas.
- **`excalidraw-mcp-pp-cli elements get`** - Get element by ID
- **`excalidraw-mcp-pp-cli elements list`** - Returns every element currently on the Excalidraw canvas.
- **`excalidraw-mcp-pp-cli elements search`** - Filter canvas elements by type, position, or bounding box coordinates.
- **`excalidraw-mcp-pp-cli elements update`** - Modify any property of an existing canvas element.

### excalidraw-canvas-cloud-export

Manage excalidraw canvas cloud export

- **`excalidraw-mcp-pp-cli excalidraw-canvas-cloud-export`** - Render the current canvas to a PNG or SVG file. Requires the browser canvas to be open. Returns base64-encoded image data.

### excalidraw-canvas-cloud-health

Manage excalidraw canvas cloud health

- **`excalidraw-mcp-pp-cli excalidraw-canvas-cloud-health`** - Check if the canvas server is running and return element count and WebSocket status.

### excalidraw-canvas-cloud-sync

Manage excalidraw canvas cloud sync

- **`excalidraw-mcp-pp-cli excalidraw-canvas-cloud-sync`** - Canvas sync status and memory usage

### files

Manage files

- **`excalidraw-mcp-pp-cli files delete`** - Delete an image file
- **`excalidraw-mcp-pp-cli files list`** - List image files on the canvas
- **`excalidraw-mcp-pp-cli files upload`** - Upload image files to the canvas

### invites

Manage invites

- **`excalidraw-mcp-pp-cli invites cloud-create`** - Send a workspace invitation
- **`excalidraw-mcp-pp-cli invites cloud-delete`** - Cancel an invitation
- **`excalidraw-mcp-pp-cli invites cloud-list`** - List pending workspace invitations

### logs

Manage logs

- **`excalidraw-mcp-pp-cli logs`** - Retrieve workspace audit logs

### scenes

Manage scenes

- **`excalidraw-mcp-pp-cli scenes cloud-create`** - Create a cloud scene
- **`excalidraw-mcp-pp-cli scenes cloud-delete`** - Delete a cloud scene
- **`excalidraw-mcp-pp-cli scenes cloud-get`** - Get cloud scene metadata
- **`excalidraw-mcp-pp-cli scenes cloud-list`** - List all scenes in your Excalidraw Plus workspace.
- **`excalidraw-mcp-pp-cli scenes cloud-update`** - Update cloud scene metadata

### snapshots

Manage snapshots

- **`excalidraw-mcp-pp-cli snapshots create`** - Capture the current canvas state. Use before destructive operations or to mark versions.
- **`excalidraw-mcp-pp-cli snapshots get`** - Get snapshot by name
- **`excalidraw-mcp-pp-cli snapshots list`** - Returns all saved canvas checkpoints with name, element count, and creation time.

### viewport

Manage viewport

- **`excalidraw-mcp-pp-cli viewport`** - Adjust the visible area: zoom level, pan position, or auto-fit all elements.

### workspace

Manage workspace

- **`excalidraw-mcp-pp-cli workspace cloud-get`** - Get workspace metadata
- **`excalidraw-mcp-pp-cli workspace cloud-list-users`** - List workspace members
- **`excalidraw-mcp-pp-cli workspace cloud-remove-user`** - Remove a member from the workspace


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
excalidraw-mcp-pp-cli elements list

# JSON for scripting and agents
excalidraw-mcp-pp-cli elements list --json

# Filter to specific fields
excalidraw-mcp-pp-cli elements list --json --select id,name,status

# Dry run — show the request without sending
excalidraw-mcp-pp-cli elements list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
excalidraw-mcp-pp-cli elements list --agent
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
excalidraw-mcp-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/excalidraw-mcp-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EXCALIDRAW_API_KEY` | per_call | No | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `excalidraw-mcp-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EXCALIDRAW_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
