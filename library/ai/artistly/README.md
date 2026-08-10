# Artistly CLI

**The first CLI for Artistly — scriptable AI image generation, batch runs, and a searchable local archive the web app has no way to offer.**

Artistly (app.artistly.ai) is a login-only web app with no public API. This CLI authenticates with your browser session and turns the one-prompt-at-a-time web flow into something an agent or a Makefile can drive: batch-generate from a prompt file, block until images render and download them, search your whole generation history offline, and replay saved style presets — all while respecting the undocumented ~400/day generation cap.

Learn more at [Artistly](https://app.artistly.ai).

Created by [@vcolombo](https://github.com/vcolombo) (Vincent Colombo).

## Install

The recommended path installs both the `artistly-pp-cli` binary and the `pp-artistly` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install artistly
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install artistly --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install artistly --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install artistly --agent claude-code
npx -y @mvanhorn/printing-press-library install artistly --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/artistly/cmd/artistly-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/artistly-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install artistly --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-artistly --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-artistly --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install artistly --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
artistly-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/artistly-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/artistly/cmd/artistly-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "artistly": {
      "command": "artistly-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Artistly has no API keys. Authentication is your logged-in browser session: run `artistly-pp-cli auth login --chrome` to import the `artistly_session` cookie and CSRF token from Chrome (be logged in to app.artistly.ai first). Generation and other write commands send the CSRF token and Inertia headers automatically.

## Quick Start

```bash
# Confirm the CLI is wired up before authenticating.
artistly-pp-cli doctor --dry-run

# After auth login --chrome: verify the session works and shows remaining daily generations.
artistly-pp-cli doctor

# Pull your most recent designs to confirm the session reads real data.
artistly-pp-cli designs list --json --limit 5

# Generate one image, wait for it to render, and save it.
artistly-pp-cli generate "a watercolor fox in a misty forest" --wait --download ./out

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Scriptable generation
- **`batch`** — Generate many images from a file of prompts in one quota-aware run.

  _Reach for this when the user has more than one prompt or a spreadsheet of variations; it is the headline workflow no manual web session can match._

  ```bash
  artistly-pp-cli batch prompts.txt --style watercolor --aspect-ratio 1:1 --wait --download ./out
  ```
- **`generate`** — Submit a generation, block until it renders, and save the images to disk.

  _Use this to make image generation a single blocking step in a script or agent flow instead of a manual click-and-babysit loop._

  ```bash
  artistly-pp-cli generate "a watercolor fox in a misty forest" --wait --download ./out --json
  ```
- **`export`** — Download all images from designs matching a query into a directory tree, with templated filenames — no generation.

  _Use this to assemble a deliverable folder of already-generated assets without re-generating or clicking through downloads._

  ```bash
  artistly-pp-cli export --query "coloring book" --to ./client-assets --name-template '{prompt}-{id}'
  ```
- **`quota`** — Report remaining daily generations and whether a planned batch fits under the cap before you start.

  _Reach for this before a large batch so a quota wall does not silently truncate a run mid-way._

  ```bash
  artistly-pp-cli quota --for prompts.txt --quantity 4 --json
  ```

### Local archive that compounds
- **`search`** — Full-text search your own generation history by prompt, style, folder, or date — entirely offline.

  _Reach for this to find an old design's exact prompt/seed to reuse, instead of scrolling the web gallery._

  ```bash
  artistly-pp-cli search "pirate ship" --status private --json
  ```
- **`redo`** — Resubmit a past design's exact settings (prompt, style, dimensions, seed) with optional overrides.

  _Use this when a user liked a previous result and wants variations or a re-run with one tweak._

  ```bash
  artistly-pp-cli redo 57628105 --seed random --quantity 4 --wait --download ./out
  ```
- **`preset`** — Save a named bundle of generation settings (style, dimensions, aspect ratio, negative prompt, quality) and replay it.

  _Reach for this when a user reuses one 'house style' across many prompts or campaigns._

  ```bash
  artistly-pp-cli preset save house-style --style watercolor --aspect-ratio 1:1 --quality highQuality
  ```

## Recipes

### Batch a spreadsheet of prompts and download finals

```bash
artistly-pp-cli batch prompts.csv --preset house-style --wait --download ./finals
```

Reads each row as a prompt (with optional per-row overrides), applies a saved preset, waits for each to render, and saves the images — the Sunday-afternoon merch workflow in one command.

### Find and re-run a past design

```bash
artistly-pp-cli search "axolotl astronaut" --agent --select designs.uuid,designs.positive_prompt,designs.checkpoint && artistly-pp-cli redo <uuid> --quantity 4
```

Search the local mirror for the old design, then resubmit its exact settings for more variations. The --agent --select pair narrows the verbose design records to just the fields you need.

### Bulk-export a client's assets without re-generating

```bash
artistly-pp-cli export --query "logo concept" --to ./client --name-template '{prompt}-{id}'
```

Downloads every finished image whose prompt matches, into a folder with readable filenames — zero quota cost.

### Enhance a prompt before generating

```bash
artistly-pp-cli prompt enhance "a fox in a forest"
```

Expand a short prompt into a rich, detailed one (no quota cost), then feed it to generate. Use prompt extract <design-id|image-url> for the reverse (image-to-prompt).

### Upscale or remove the background of a design

```bash
artistly-pp-cli edit upscale 57628105 --wait --download ./out
```

Run editor image-to-image tools on an existing design (by id/uuid), an image URL, or a local file. Use 'edit bg-remove' for background removal. Both auto-authenticate via your app session.

## Usage

Run `artistly-pp-cli --help` for the full command reference and flag list.

## Commands

### designs

Browse and sync your Artistly designs (generations)

- **`artistly-pp-cli designs by-folder`** - List designs grouped by folder
- **`artistly-pp-cli designs list`** - List your personal designs (generations)

### checkpoints

Browse Artistly's checkpoint (model) catalog

- **`artistly-pp-cli checkpoints list`** - List available checkpoints/models (filter with `--match`)

Resolve a model name to the integer id that `generate`, `batch`, `redo`, and `preset` expect for `--checkpoint-id`:

```bash
artistly-pp-cli checkpoints list --match comic --json
```


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
artistly-pp-cli designs list

# JSON for scripting and agents
artistly-pp-cli designs list --json

# Filter to specific fields
artistly-pp-cli designs list --json --select id,name,status

# Dry run — show the request without sending
artistly-pp-cli designs list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
artistly-pp-cli designs list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Mutating** - `generate`/`batch`/`redo` create images (consuming quota); `designs delete`/`move` and `folders` change remote state; destructive commands require `--yes`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
artistly-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/artistly-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `artistly-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return 'not authenticated' or redirect to login** — Run `artistly-pp-cli auth login --chrome` while logged in to app.artistly.ai in Chrome; the session cookie expires periodically.
- **Generation never finishes / stays 'processing'** — Artistly may be under load; re-run `artistly-pp-cli designs list` to check status, or raise the wait timeout with `--wait-timeout`.
- **A batch stops part-way through** — You likely hit the ~400/day cap (failed generations count too). Run `artistly-pp-cli quota` to see remaining budget before retrying.
- **search returns nothing** — Run `artistly-pp-cli sync` first to mirror your designs into the local store; search is offline-only.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://app.artistly.ai/ai/ai-design-agents
- Capture coverage: 4 API entries from 91 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: websocket (95% confidence), rest_json (75% confidence)
- Candidate command ideas: create_auth — Derived from observed POST /broadcasting/auth traffic.; list_ai_design_agents — Derived from observed GET /ai/ai-design-agents traffic.; list_fetch_personal_designs — Derived from observed GET /fetch-personal-designs traffic.; list_personal_designs — Derived from observed GET /personal-designs traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
