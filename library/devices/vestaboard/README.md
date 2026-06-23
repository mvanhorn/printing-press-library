# Vestaboard CLI

**Read, render, and write your Vestaboard from the terminal — with a board preview and character-code table no other Vestaboard tool ships.**

Wraps the Vestaboard Cloud Read/Write API (read the current message, send text or character-code messages, control transitions) plus the VBML text formatter on its own host. The novel 'message preview' renders the live board as readable text, and 'characters' prints the code table so you can build character-array payloads by hand.

Learn more at [Vestaboard](https://www.vestaboard.com).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `vestaboard-pp-cli` binary and the `pp-vestaboard` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install vestaboard
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install vestaboard --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install vestaboard --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install vestaboard --agent claude-code
npx -y @mvanhorn/printing-press-library install vestaboard --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/vestaboard/cmd/vestaboard-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vestaboard-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install vestaboard --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-vestaboard --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-vestaboard --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install vestaboard --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/vestaboard-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `VESTABOARD_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/vestaboard/cmd/vestaboard-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "vestaboard": {
      "command": "vestaboard-pp-mcp",
      "env": {
        "VESTABOARD_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Create a Cloud API token in the API tab on web.vestaboard.com (or the mobile app under Settings → Advanced Settings), then run 'vestaboard-pp-cli auth set-token <token>'. The token is sent only to cloud.vestaboard.com; the VBML formatter on vbml.vestaboard.com takes no credentials.

## Quick Start

```bash
# check config and connectivity before sending anything
vestaboard-pp-cli doctor --dry-run

# see the board's current contents as readable text
vestaboard-pp-cli message preview

# format text into a character-code grid (no auth, second host)
vestaboard-pp-cli vbml --message "HELLO" --json

# put a message on the board (1 msg / 15s rate limit)
vestaboard-pp-cli message send --body-json '{"text":"hello"}'

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Make the board legible
- **`message preview`** — See what's on your Vestaboard right now as readable text instead of a raw integer grid.

  _Reach for this before composing or sending — it is the only way to know the board's current contents without decoding codes by hand._

  ```bash
  vestaboard-pp-cli message preview --json
  ```
- **`characters`** — Print the full Vestaboard character-code table (code to glyph) for hand-building a character-array message.

  _Use this when constructing a 'message send' character payload so you map glyphs to the right integer codes._

  ```bash
  vestaboard-pp-cli characters --json
  ```

## Recipes

### Read the board as text

```bash
vestaboard-pp-cli message preview
```

Decodes the live layout into a bordered block of glyphs you can read.

### Inspect the raw layout with field selection

```bash
vestaboard-pp-cli message get --agent --select currentMessage.id,currentMessage.layout
```

Pulls just the id and raw character grid for an agent without the surrounding envelope.

### Format then send a message

```bash
vestaboard-pp-cli vbml --message "GM" --json
```

Convert text to a character-code grid on the VBML host, then pass it to 'message send --body-json'.

### Set a transition

```bash
vestaboard-pp-cli transition set --transition wave --transition-speed gentle
```

Change the flip animation style and speed.

## Usage

Run `vestaboard-pp-cli --help` for the full command reference and flag list.

## Commands

### message

The message currently shown on the Vestaboard.

- **`vestaboard-pp-cli message get`** - Read the message currently displayed on the board. The layout is a JSON-encoded 2D array of character codes whose dimensions depend on the board type.
- **`vestaboard-pp-cli message send`** - Send a new message to the board. Provide --text (plain text / VBML) or --characters (a JSON 2D array of character codes). Rate limited to 1 message per 15 seconds.

### transition

Transition animation settings for the board (Flagship and Note devices).

- **`vestaboard-pp-cli transition get`** - Get the current transition style and speed.
- **`vestaboard-pp-cli transition set`** - Set the transition style and speed. Both fields are required.

### vbml

Vestaboard Markup Language (VBML) text formatting service.

- **`vestaboard-pp-cli vbml`** - Convert a text string into a 2D array of Vestaboard character codes.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
vestaboard-pp-cli message get

# JSON for scripting and agents
vestaboard-pp-cli message get --json

# Filter to specific fields
vestaboard-pp-cli message get --json --select id,name,status

# Dry run — show the request without sending
vestaboard-pp-cli message get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
vestaboard-pp-cli message get --agent
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
vestaboard-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/vestaboard-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `VESTABOARD_API_KEY` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `vestaboard-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `vestaboard-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $VESTABOARD_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Messages are dropped or 429-rate-limited** — The board accepts at most 1 message per 15 seconds; space out 'message send' calls.
- **'message preview' renders a blank or wrong grid** — Board size varies by device (Flagship 6x22, Note 3x15); preview renders whatever the API returns. Re-run 'message get --json' to inspect the raw layout.
- **'message send' returns success but quiet hours suppress it** — Add "forced": true to the body-json to override configured quiet hours.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**vestaboard/installables**](https://github.com/vestaboard/installables) — JavaScript
- [**vestaboard-python (ShaneBenetz)**](https://github.com/ShaneBenetz/vestaboard) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
