# Plaud Web CLI

Unofficial Plaud Web endpoints observed from a user's signed-in Plaud Web session for organizing user-owned recordings.

Learn more at [Plaud Web](https://api.plaud.ai).

Created by [@eranium21](https://github.com/eranium21) (Stefan Erschwendner).

## Install

The recommended path installs both the `plaud-web-pp-cli` binary and the `pp-plaud-web` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install plaud-web
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install plaud-web --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install plaud-web --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install plaud-web --agent claude-code
npx -y @mvanhorn/printing-press-library install plaud-web --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/plaud-web/cmd/plaud-web-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/plaud-web-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install plaud-web --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-plaud-web --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-plaud-web --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install plaud-web --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/plaud-web-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PLAUD_WEB_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/plaud-web/cmd/plaud-web-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "plaud-web": {
      "command": "plaud-web-pp-mcp",
      "env": {
        "PLAUD_WEB_BEARER_AUTH": "<your-key>"
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

Use a Plaud Web session bearer token from your own signed-in browser session, then store it:

```bash
plaud-web-pp-cli auth set-token YOUR_PLAUD_WEB_TOKEN
```

Or set it via environment variable:

```bash
export PLAUD_WEB_BEARER_AUTH="your-plaud-web-token"
```

The value may be pasted with or without the leading `bearer ` prefix. Keep it local; do not paste Plaud tokens, cookies, signed audio URLs, or downloaded recordings into issues, logs, or manuscripts.

### 3. Verify Setup

```bash
plaud-web-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
plaud-web-pp-cli speaker
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Knowledge-work organization

- **`batch-rename`** — Rename multiple Plaud recordings from a JSON or CSV mapping after transcription or note synthesis.

  _Lets agents keep Plaud Web aligned with Obsidian, CRM, and meeting-note systems without manual timestamp cleanup._

  ```bash
  plaud-web-pp-cli batch-rename titles.json --dry-run
  ```
- **`move`** — Move one or more recordings into a Plaud folder/tag or back to unfiled.

  _Useful for client calls, event recordings, and presentation-prep batches._

  ```bash
  plaud-web-pp-cli move <recording-id> --folder-id <folder-id>
  ```

### Data portability

- **`export-audio`** — Resolve a short-lived Plaud audio URL and download the recording to a local file with a safe title-based filename.

  _Supports data portability and downstream transcript, archival, or review workflows._

  ```bash
  plaud-web-pp-cli export-audio <recording-id> --output-dir exports
  ```

## Usage

Run `plaud-web-pp-cli --help` for the full command reference and flag list.

## Commands

### file

Manage file

- **`plaud-web-pp-cli file get-detail`** - Fetch detail metadata for one recording.
- **`plaud-web-pp-cli file get-temporary-audio-url`** - Fetch a short-lived audio download URL.
- **`plaud-web-pp-cli file rename`** - Rename one recording.
- **`plaud-web-pp-cli file update-tags`** - Move recordings into a folder/tag or back to unfiled.

### filetag

Manage filetag

- **`plaud-web-pp-cli filetag create-file-tag`** - Create a Plaud folder/tag.
- **`plaud-web-pp-cli filetag list-file-tags`** - List Plaud folders/tags.
- **`plaud-web-pp-cli filetag update-file-tag`** - Rename or update a Plaud folder/tag.

### gsearch

Manage gsearch

- **`plaud-web-pp-cli gsearch`** - Search recordings and generated note content.

### speaker

Manage speaker

- **`plaud-web-pp-cli speaker`** - List cloud speaker labels when speaker cloud is enabled.

### user

Manage user

- **`plaud-web-pp-cli user`** - Fetch recording statistics.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
plaud-web-pp-cli speaker

# JSON for scripting and agents
plaud-web-pp-cli speaker --json

# Filter to specific fields
plaud-web-pp-cli speaker --json --select id,name,status

# Dry run — show the request without sending
plaud-web-pp-cli speaker --dry-run

# Agent mode — JSON + compact + no prompts in one flag
plaud-web-pp-cli speaker --agent
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

## Health Check

```bash
plaud-web-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/plaud-web-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PLAUD_WEB_BEARER_AUTH` | per_call | Yes | Plaud Web session bearer token for your own account. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `plaud-web-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `plaud-web-pp-cli doctor` to check credentials
- Verify the environment variable is set without printing the token value: `test -n "$PLAUD_WEB_BEARER_AUTH" && echo set`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
