# Instagram CLI

Instagram Web API (reverse-engineered from authenticated browser session)

Learn more at [Instagram](https://www.instagram.com).

## Install

The recommended path installs both the `instagram-pp-cli` binary and the `pp-instagram` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install instagram
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install instagram --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/instagram-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-instagram --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-instagram --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-instagram skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-instagram. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to .instagram.com in Chrome, then:

```bash
instagram-pp-cli auth login --chrome
```

Requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
instagram-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
instagram-pp-cli graphql
```

## Usage

Run `instagram-pp-cli --help` for the full command reference and flag list.

## Commands

### explore

Explore/discover content

- **`instagram-pp-cli explore grid`** - Get explore grid content (trending posts)

### feed

Home feed and user posts

- **`instagram-pp-cli feed liked`** - Get media liked by the authenticated user
- **`instagram-pp-cli feed saved`** - Get the authenticated user's saved posts
- **`instagram-pp-cli feed timeline`** - Get the home feed (posts from accounts you follow)
- **`instagram-pp-cli feed user_posts`** - Get posts from a specific user

### friendships

Follow relationships and pending requests

- **`instagram-pp-cli friendships followers`** - Get followers of a user
- **`instagram-pp-cli friendships following`** - Get accounts a user is following
- **`instagram-pp-cli friendships pending`** - Get pending follow requests (for private accounts)

### graphql

Instagram GraphQL API (for complex queries)

- **`instagram-pp-cli graphql query`** - Execute a GraphQL query against the Instagram API

### hashtags

Hashtag search and info

- **`instagram-pp-cli hashtags info`** - Get info about a hashtag including post count

### locations

Location search

- **`instagram-pp-cli locations search`** - Search for locations and places

### media

Posts, reels, and media

- **`instagram-pp-cli media comments`** - Get comments on a post
- **`instagram-pp-cli media info`** - Get detailed info for a specific post/reel
- **`instagram-pp-cli media likers`** - Get users who liked a post

### notifications

Activity notifications

- **`instagram-pp-cli notifications inbox`** - Get the notifications inbox (likes, comments, follows, mentions)
- **`instagram-pp-cli notifications mark_seen`** - Mark notifications as seen

### reels

Reels/Clips

- **`instagram-pp-cli reels user_clips`** - Get reels posted by a specific user

### stories

Stories and highlights

- **`instagram-pp-cli stories highlights_tray`** - Get highlights for a user
- **`instagram-pp-cli stories reels_media`** - Get story reels for specific users
- **`instagram-pp-cli stories tray`** - Get the stories tray (all unviewed stories from followed accounts)

### users

User profiles and account info

- **`instagram-pp-cli users current_user`** - Get the authenticated user's own account info
- **`instagram-pp-cli users profile`** - Get full user profile by username
- **`instagram-pp-cli users search`** - Search users, hashtags, and places


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
instagram-pp-cli graphql

# JSON for scripting and agents
instagram-pp-cli graphql --json

# Filter to specific fields
instagram-pp-cli graphql --json --select id,name,status

# Dry run — show the request without sending
instagram-pp-cli graphql --dry-run

# Agent mode — JSON + compact + no prompts in one flag
instagram-pp-cli graphql --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-instagram -g
```

Then invoke `/pp-instagram <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
instagram-pp-cli auth login --chrome

claude mcp add instagram instagram-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
instagram-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/instagram-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "instagram": {
      "command": "instagram-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
instagram-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INSTAGRAM_SESSION_ID` | per_call | Yes | Set to your API credential. |
| `INSTAGRAM_CSRF_TOKEN` | per_call | Yes | Set to your API credential. |
| `INSTAGRAM_USER_ID` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `instagram-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INSTAGRAM_SESSION_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: browser_http (85% confidence)
- Protocols: rest_json (95% confidence), graphql (70% confidence)
- Generation hints: requires_browser_auth, requires_browser_http, chrome_ua_required

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
