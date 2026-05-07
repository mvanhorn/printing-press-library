# Google Photos CLI

Google Photos Library and Picker APIs for app-created media, albums, uploads, and user-selected media.

Learn more at [Google Photos](https://developers.google.com/photos).

## Install

The recommended path installs both the `google-photos-pp-cli` binary and the `pp-google-photos` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install google-photos
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install google-photos --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/google-photos/cmd/google-photos-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-photos-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

Authorize via your browser:

```bash
google-photos-pp-cli auth login --client-id "$GOOGLE_PHOTOS_CLIENT_ID"
```

This opens a browser window to complete the OAuth2 flow. Your tokens are stored locally and refreshed automatically.
For multiple Google accounts, pass an account email and select it later:

```bash
google-photos-pp-cli auth login you@example.com --client-id "$GOOGLE_PHOTOS_CLIENT_ID"
google-photos-pp-cli auth list
google-photos-pp-cli auth use you@example.com
google-photos-pp-cli --account you@example.com albums list
```

### 3. Verify Setup

```bash
google-photos-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
google-photos-pp-cli albums list
```

## Usage

Run `google-photos-pp-cli --help` for the full command reference and flag list.

## Commands

### albums

Manage app-created Google Photos albums.

- **`google-photos-pp-cli albums add-enrichment`** - Add text, location, or map enrichment to an app-created album.
- **`google-photos-pp-cli albums batch-add-media-items`** - Add app-created media items to an app-created album.
- **`google-photos-pp-cli albums batch-remove-media-items`** - Remove app-created media items from an app-created album.
- **`google-photos-pp-cli albums create`** - Create an album in the user's Google Photos library.
- **`google-photos-pp-cli albums get`** - Get an app-created album by ID.
- **`google-photos-pp-cli albums list`** - List albums created by this app.
- **`google-photos-pp-cli albums patch`** - Update title or cover photo on an app-created album.

### media-items

Manage app-created Google Photos media items.

- **`google-photos-pp-cli media-items batch-create`** - Create media items from upload tokens.
- **`google-photos-pp-cli media-items batch-get`** - Get multiple app-created media items by ID.
- **`google-photos-pp-cli media-items get`** - Get an app-created media item by ID.
- **`google-photos-pp-cli media-items list`** - List media items created by this app.
- **`google-photos-pp-cli media-items patch`** - Update the description on an app-created media item.
- **`google-photos-pp-cli media-items search`** - Search app-created media items by album or filters.

### picker

Create, poll, clean up, and read Google Photos Picker sessions.

- **`google-photos-pp-cli picker create-session`** - Create a Picker session and return the picker URI.
- **`google-photos-pp-cli picker delete-session`** - Delete a Picker session after selected media bytes have been retrieved.
- **`google-photos-pp-cli picker get-session`** - Get Picker session status.
- **`google-photos-pp-cli picker list-media-items`** - List media items picked by the user during a Picker session.
- **`google-photos-pp-cli picker wait <session-id>`** - Poll a Picker session until selected media items are ready.

### upload

Upload media bytes and create upload tokens.

- **`google-photos-pp-cli upload file <path>`** - Upload raw photo or video bytes and print the upload token for media-items batch-create.

### auth

Manage OAuth accounts and token storage.

- **`google-photos-pp-cli auth login [account-email]`** - Authenticate via OAuth2 and store tokens, optionally under an account email.
- **`google-photos-pp-cli auth list`** - List stored OAuth accounts.
- **`google-photos-pp-cli auth status`** - Show authentication status for the selected account.
- **`google-photos-pp-cli auth use <account-email>`** - Set the default OAuth account.
- **`google-photos-pp-cli auth remove <account-email>`** - Remove a stored OAuth account.
- **`google-photos-pp-cli auth logout`** - Remove the selected account token, or legacy token when no account is selected.

### schema

Emit machine-readable command, flag, auth, and safety-policy metadata.

- **`google-photos-pp-cli schema --pretty`** - Print the same structured contract agents use for discovery.

## Google Photos Scope Limits

Google Photos Library API read and edit scopes are limited to app-created albums and media. Use Library API commands here for app-created content, uploads, and album/media management. Use Picker commands when a user needs to select media from their broader Google Photos library.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-photos-pp-cli albums list

# JSON for scripting and agents
google-photos-pp-cli albums list --json

# Filter to specific fields
google-photos-pp-cli albums list --json --select id,name,status

# Dry run — show the request without sending
google-photos-pp-cli albums list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-photos-pp-cli albums list --agent

# Command guard — permit only selected command families
google-photos-pp-cli --enable-commands albums.list albums list --agent

# Command guard — block specific risky commands
google-photos-pp-cli --disable-commands picker.delete-session picker delete-session SESSION_ID --agent
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
- **Guarded** - `--enable-commands` and `--disable-commands` restrict runtime command execution with dotted command paths
- **Introspectable** - `schema` and `agent-context` expose commands, flags, auth account selection, and safety policy as JSON

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Safety Profiles

The stock binary allows every command unless runtime guards are provided:

```bash
google-photos-pp-cli --enable-commands albums.list,media-items.get albums list --agent
google-photos-pp-cli --disable-commands picker.delete-session picker delete-session SESSION_ID --agent
```

For agent or MCP deployments that need stronger local guardrails, build a baked profile. Baked profiles are compiled into the binary and cannot be loosened by command-line flags:

```bash
make build-readonly
make build-agent-safe
make build-mcp-readonly
make build-mcp-agent-safe
```

`safety_readonly` permits read/list/search/export/schema-style commands and blocks Google Photos mutations. `safety_agent_safe` permits reads and local archive/search workflows while blocking auth writes, uploads, creates, patches, deletes, and album/media mutations.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-google-photos -g
```

Then invoke `/pp-google-photos <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/google-photos/cmd/google-photos-pp-mcp@latest
```

Then register it:

```bash
# Set up auth first:
google-photos-pp-cli auth login --client-id "$GOOGLE_PHOTOS_CLIENT_ID"

claude mcp add google-photos google-photos-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-photos --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-photos --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-google-photos skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-google-photos. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
google-photos-pp-cli auth login --client-id "$GOOGLE_PHOTOS_CLIENT_ID"
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-photos-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_PHOTOS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/google-photos/cmd/google-photos-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-photos": {
      "command": "google-photos-pp-mcp",
      "env": {
        "GOOGLE_PHOTOS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
google-photos-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-photos-pp-cli/config.toml`

Environment variables:
- `GOOGLE_PHOTOS_TOKEN`
- `GOOGLE_PHOTOS_ACCOUNT`
- `GOOGLE_PHOTOS_ENABLE_COMMANDS`
- `GOOGLE_PHOTOS_DISABLE_COMMANDS`

OAuth account selection order:
1. `--account <email>`
2. `GOOGLE_PHOTOS_ACCOUNT`
3. `default_account` from `auth use`
4. legacy single-token fields in the config file

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-photos-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLE_PHOTOS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
