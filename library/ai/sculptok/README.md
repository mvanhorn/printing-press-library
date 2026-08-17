# SculptOK CLI

**The first CLI and MCP for SculptOK: turn a local image into a depth map, printable STL, or 3D model in one command, with credit-cost preflight and a local job store no other tool has.**

SculptOK's web tool is async and credit-metered: you upload, submit a draw, wait, refresh, then download. This CLI collapses that into one command (generate depthmap/stl/threed/restore) that uploads a local image, previews the credit cost, polls to completion, and saves the results. Every draw, credit event, and upload is mirrored into a local SQLite database so you can search past jobs offline, track spend by kind with analytics, and reconcile credits against produced jobs.

## Install

The recommended path installs both the `sculptok-pp-cli` binary and the `pp-sculptok` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sculptok
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sculptok --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sculptok --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sculptok --agent claude-code
npx -y @mvanhorn/printing-press-library install sculptok --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/sculptok/cmd/sculptok-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sculptok-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sculptok --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sculptok --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sculptok --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sculptok --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sculptok-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SCULPTOK_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/sculptok/cmd/sculptok-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sculptok": {
      "command": "sculptok-pp-mcp",
      "env": {
        "SCULPTOK_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

SculptOK uses a single API key sent in the apikey header. Create a key in the logged-in SculptOK dashboard (it is shown only once), then set SCULPTOK_API_KEY or run the auth command. Credits are shared with the web app; reads (credits, history, status) are free, while draws cost credits (depth-map 10, pro 2k 15, pro 4k 30; background/HD 2; 3D 10; STL 3).

## Quick Start

```bash
# Health check that works without credentials before anything else.
sculptok-pp-cli doctor --dry-run

# Free read: confirm your API key works and see your credit balance.
sculptok-pp-cli credits balance

# See what a draw would cost before spending credits.
sculptok-pp-cli cost depthmap --style pro --draw-hd 4k

# Dry-run the headline workflow to see the planned upload and draw without spending credits.
sculptok-pp-cli generate depthmap photo.jpg --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### One-command generation
- **`generate depthmap`** — Turn a local image into SculptOK depth-map candidates in one command: upload, submit, auto-poll to completion, and download the results.

  _Reach for this whenever a user wants a depth map or relief from an image without babysitting an async job._

  ```bash
  sculptok-pp-cli generate depthmap photo.jpg --style pro --dry-run
  ```
- **`generate stl`** — Convert a local image straight to a printable STL with full thickness, width, invert, and scale control, polled to completion and saved.

  _Use this to go from a photo or logo to a 3D-printable mesh for lithophanes and relief plaques._

  ```bash
  sculptok-pp-cli generate stl logo.png --width-mm 120 --max-thickness 5 --dry-run
  ```
- **`generate threed`** — Generate a 3D model from a local image at basic, standard, or high precision, polled and downloaded automatically.

  _Pick this when the goal is a full 3D model rather than a 2.5D relief._

  ```bash
  sculptok-pp-cli generate threed bust.jpg --hd-fix high --dry-run
  ```

### Credit awareness
- **`cost`** — Estimate exactly how many credits a draw or a whole batch would cost and compare it against your live balance before spending anything.

  _Run this before any paid generation to avoid surprise credit burn, especially for batches and 4k/pro draws._

  ```bash
  sculptok-pp-cli cost depthmap --style pro --draw-hd 4k --batch ./photos
  ```
- **`analytics`** — Aggregate your locally synced credit events by action type to see where credits actually went.

  _Use this to understand historical credit spend; pair it with cost for forward estimates._

  ```bash
  sculptok-pp-cli analytics --type credits --group-by actionType
  ```
- **`reconcile`** — Cross-check synced credit charges against produced draw jobs to surface credits spent with no matching result.

  _Reach for this to audit whether every credit charge produced a usable job._

  ```bash
  sculptok-pp-cli reconcile --db ./sculptok.db
  ```

### Local state that compounds
- **`search`** — Full-text search your local mirror of draws, credit events, and drawings by kind, status, parameters, remarks, or image URL without any API call.

  _Use this to find a past job and the settings that produced a good result._

  ```bash
  sculptok-pp-cli search --type jobs --limit 20 "stl"
  ```

## Recipes


### Depth map from a photo, picking lean fields

```bash
sculptok-pp-cli generate depthmap portrait.jpg --style pro --agent --select promptId,status,imgRecords
```

Generates depth-map candidates and returns only the job id, status, and result URLs so an agent does not parse the full job envelope.

### Printable STL with explicit thickness

```bash
sculptok-pp-cli generate stl logo.png --width-mm 120 --min-thickness 1.6 --max-thickness 5 --invert
```

Produces a printable STL with documented thickness and width and inverted grayscale for a recessed relief.

### Clean then carve in one step

```bash
sculptok-pp-cli generate depthmap jewelry.jpg --restore-first --style pro
```

Runs background removal plus HD restoration first, then depth-maps the cleaned image, persisting both jobs.

### Estimate a batch before spending

```bash
sculptok-pp-cli cost stl --batch ./photos
```

Sums the credit cost of converting every image in a folder to STL and compares it to your live balance.

### Find the settings that worked

```bash
sculptok-pp-cli search --type jobs --limit 10 --agent "depthmap"
```

Searches the local job mirror for past depth-map runs so you can reuse the parameters that produced a good carve.

## Usage

Run `sculptok-pp-cli --help` for the full command reference and flag list.

## Commands

### credits

Credit balance and change history (free reads)

- **`sculptok-pp-cli credits balance`** - Get the current credit balance (free)
- **`sculptok-pp-cli credits history`** - List credit-change history, newest first (free)

### draw

Submit and track async draw jobs (each draw costs credits)

- **`sculptok-pp-cli draw depthmap`** - Submit a depth-map draw. Cost: 10 credits (style=pro 2k 15; style=pro + draw_hd=4k 30). Async: returns promptId; poll with 'draw status'.
- **`sculptok-pp-cli draw restore`** - Submit background-removal and/or HD restoration. Cost: 2 credits. Async: returns promptId.
- **`sculptok-pp-cli draw status`** - Get the status of a submitted draw by promptId (free). status/currentStep/position and imgRecords on completion.
- **`sculptok-pp-cli draw stl`** - Submit an image-to-STL job. Cost: 3 credits. Async: returns promptId.
- **`sculptok-pp-cli draw threed`** - Submit a 3D draw. Cost: 10 credits. Async: returns promptId.

### drawings

History of generated images/draws (free read)

- **`sculptok-pp-cli drawings`** - List your past generated images, newest first (free)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sculptok-pp-cli drawings

# JSON for scripting and agents
sculptok-pp-cli drawings --json

# Filter to specific fields
sculptok-pp-cli drawings --json --select id,name,status

# Dry run — show the request without sending
sculptok-pp-cli drawings --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sculptok-pp-cli drawings --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
sculptok-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/sculptok-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SCULPTOK_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sculptok-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sculptok-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SCULPTOK_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return an empty data object or an apikey error** — Set SCULPTOK_API_KEY to a valid key from the SculptOK dashboard; code 10020 means the key is missing and 10021 means it is invalid.
- **A generate command seems to hang** — Draws are async and queued; raise --timeout or check queue position with draw status --uuid <promptId>.
- **A draw fails with insufficient credits** — Run cost <kind> first and account credits to confirm balance; draws cost 2-30 credits depending on kind and resolution.
- **Offline search or analytics returns nothing** — Run sync --resources credits,drawings first to populate the local SQLite mirror.
