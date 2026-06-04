# Etherpad CLI

Etherpad is a real-time collaborative editor scalable to thousands of simultaneous real time users. It provides full data export capabilities, and runs on your server, under your control.

Learn more at [Etherpad](https://etherpad.org/).

Created by [@JohnMcLear](https://github.com/JohnMcLear) (John McLear).

## Install

The recommended path installs both the `etherpad-pp-cli` binary and the `pp-etherpad` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install etherpad
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install etherpad --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install etherpad --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install etherpad --agent claude-code
npx -y @mvanhorn/printing-press-library install etherpad --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/etherpad/cmd/etherpad-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/etherpad-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-etherpad --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-etherpad --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install etherpad --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/etherpad-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ETHERPAD_OPENID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "etherpad": {
      "command": "etherpad-pp-mcp",
      "env": {
        "ETHERPAD_OPENID": "<your-key>"
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
etherpad-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export ETHERPAD_OPENID="your-token-here"
```

### 3. Verify Setup

```bash
etherpad-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
etherpad-pp-cli anonymize-author
```

## Usage

Run `etherpad-pp-cli --help` for the full command reference and flag list.

## Commands

### anonymize-author

Manage anonymize author

- **`etherpad-pp-cli anonymize-author using-post`** - anonymizes an author across all their edits

### append-chat-message

Manage append chat message

- **`etherpad-pp-cli append-chat-message using-post`** - appends a chat message

### append-text

Manage append text

- **`etherpad-pp-cli append-text using-post`** - appends text to a pad

### check-token

Manage check token

- **`etherpad-pp-cli check-token using-post`** - returns ok when the current API token is valid

### compact-pad

Manage compact pad

- **`etherpad-pp-cli compact-pad using-post`** - compacts a pad's revision history, keeping recent revisions only

### copy-pad

Manage copy pad

- **`etherpad-pp-cli copy-pad using-post`** - copies a pad with full history and chat

### copy-pad-without-history

Manage copy pad without history

- **`etherpad-pp-cli copy-pad-without-history using-post`** - copies a pad without history or chat

### create-author

Manage create author

- **`etherpad-pp-cli create-author using-post`** - creates a new author

### create-author-if-not-exists-for

Manage create author if not exists for

- **`etherpad-pp-cli create-author-if-not-exists-for using-post`** - this functions helps you to map your application author ids to Etherpad author ids

### create-diff-html

Manage create diff html

- **`etherpad-pp-cli create-diff-html create-diff-htmlusing-post`** - returns an HTML diff between two revisions of a pad

### create-group

Manage create group

- **`etherpad-pp-cli create-group using-post`** - creates a new group

### create-group-if-not-exists-for

Manage create group if not exists for

- **`etherpad-pp-cli create-group-if-not-exists-for using-post`** - this functions helps you to map your application group ids to Etherpad group ids

### create-group-pad

Manage create group pad

- **`etherpad-pp-cli create-group-pad using-post`** - creates a new pad in this group

### create-pad

Manage create pad

- **`etherpad-pp-cli create-pad using-post`** - creates a new (non-group) pad. Note that if you need to create a group Pad, you should call createGroupPad

### create-session

Manage create session

- **`etherpad-pp-cli create-session using-post`** - creates a new session. validUntil is an unix timestamp in seconds

### delete-group

Manage delete group

- **`etherpad-pp-cli delete-group using-post`** - deletes a group

### delete-pad

Manage delete pad

- **`etherpad-pp-cli delete-pad using-post`** - deletes a pad

### delete-session

Manage delete session

- **`etherpad-pp-cli delete-session using-post`** - deletes a session

### get-attribute-pool

Manage get attribute pool

- **`etherpad-pp-cli get-attribute-pool using-post`** - returns the attribute pool of a pad

### get-author-name

Manage get author name

- **`etherpad-pp-cli get-author-name using-post`** - Returns the Author Name of the author

### get-chat-head

Manage get chat head

- **`etherpad-pp-cli get-chat-head using-post`** - returns the chatHead (chat-message) of the pad

### get-chat-history

Manage get chat history

- **`etherpad-pp-cli get-chat-history using-post`** - returns the chat history

### get-html

Manage get html

- **`etherpad-pp-cli get-html get-htmlusing-post`** - returns the text of a pad formatted as HTML

### get-last-edited

Manage get last edited

- **`etherpad-pp-cli get-last-edited using-post`** - returns the timestamp of the last revision of the pad

### get-pad-id

Manage get pad id

- **`etherpad-pp-cli get-pad-id get-pad-idusing-post`** - returns the read-write pad ID for a given read-only pad ID

### get-public-status

Manage get public status

- **`etherpad-pp-cli get-public-status using-post`** - return true of false

### get-read-only-id

Manage get read only id

- **`etherpad-pp-cli get-read-only-id get-read-only-idusing-post`** - returns the read only link of a pad

### get-revision-changeset

Manage get revision changeset

- **`etherpad-pp-cli get-revision-changeset using-post`** - returns the changeset at a given revision of a pad

### get-revisions-count

Manage get revisions count

- **`etherpad-pp-cli get-revisions-count using-post`** - returns the number of revisions of this pad

### get-saved-revisions-count

Manage get saved revisions count

- **`etherpad-pp-cli get-saved-revisions-count using-post`** - returns the number of saved revisions of a pad

### get-session-info

Manage get session info

- **`etherpad-pp-cli get-session-info using-post`** - returns information about a session

### get-stats

Manage get stats

- **`etherpad-pp-cli get-stats using-post`** - returns server-wide statistics

### get-text

Manage get text

- **`etherpad-pp-cli get-text using-post`** - returns the text of a pad

### list-all-groups

Manage list all groups

- **`etherpad-pp-cli list-all-groups using-post`** - returns the IDs of all groups on this server

### list-all-pads

Manage list all pads

- **`etherpad-pp-cli list-all-pads using-post`** - list all the pads

### list-authors-of-pad

Manage list authors of pad

- **`etherpad-pp-cli list-authors-of-pad using-post`** - returns an array of authors who contributed to this pad

### list-pads

Manage list pads

- **`etherpad-pp-cli list-pads using-post`** - returns all pads of this group

### list-pads-of-author

Manage list pads of author

- **`etherpad-pp-cli list-pads-of-author using-post`** - returns an array of all pads this author contributed to

### list-saved-revisions

Manage list saved revisions

- **`etherpad-pp-cli list-saved-revisions using-post`** - returns the list of saved revisions of a pad

### list-sessions-of-author

Manage list sessions of author

- **`etherpad-pp-cli list-sessions-of-author using-post`** - returns all sessions of an author

### list-sessions-of-group

Manage list sessions of group

- **`etherpad-pp-cli list-sessions-of-group using-post`** - returns all sessions of a group

### move-pad

Manage move pad

- **`etherpad-pp-cli move-pad using-post`** - moves a pad — copy then delete the original

### pad-users

Manage pad users

- **`etherpad-pp-cli pad-users using-post`** - returns the list of users that are currently editing this pad

### pad-users-count

Manage pad users count

- **`etherpad-pp-cli pad-users-count using-post`** - returns the number of user that are currently editing this pad

### restore-revision

Manage restore revision

- **`etherpad-pp-cli restore-revision using-post`** - restores a pad to a specific revision

### save-revision

Manage save revision

- **`etherpad-pp-cli save-revision using-post`** - saves a revision of a pad

### send-clients-message

Manage send clients message

- **`etherpad-pp-cli send-clients-message using-post`** - sends a custom message of type msg to the pad

### set-html

Manage set html

- **`etherpad-pp-cli set-html set-htmlusing-post`** - sets the text of a pad with HTML

### set-public-status

Manage set public status

- **`etherpad-pp-cli set-public-status using-post`** - sets a boolean for the public status of a pad

### set-text

Manage set text

- **`etherpad-pp-cli set-text using-post`** - sets the text of a pad

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
etherpad-pp-cli anonymize-author

# JSON for scripting and agents
etherpad-pp-cli anonymize-author --json

# Filter to specific fields
etherpad-pp-cli anonymize-author --json --select id,name,status

# Dry run — show the request without sending
etherpad-pp-cli anonymize-author --dry-run

# Agent mode — JSON + compact + no prompts in one flag
etherpad-pp-cli anonymize-author --agent
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
etherpad-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/etherpad-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ETHERPAD_OPENID` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `etherpad-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ETHERPAD_OPENID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
