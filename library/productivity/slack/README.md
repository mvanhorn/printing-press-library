# Slack CLI

Send messages, search conversations, monitor channels, and manage your Slack workspace from the terminal

## Install

The recommended path installs both the `slack-pp-cli` binary and the `pp-slack` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install slack
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install slack --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/slack-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-slack --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-slack --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-slack skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-slack. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# 1. Authenticate
export SLACK_BOT_TOKEN="xoxb-your-token-here"

# 2. Verify setup
slack-pp-cli doctor

# 3. Sync workspace data locally for search and analytics
slack-pp-cli sync

# 4. Get a daily digest of activity across channels
slack-pp-cli digest

# 5. Search messages across the workspace
slack-pp-cli search "project launch"
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`health`** - See which channels are thriving and which are dying with messages/day, response times, and active poster counts
- **`response-times`** - Measure how fast your team responds to messages and threads in any channel
- **`digest`** - Get a daily or weekly summary of activity, mentions, and action items across all channels
- **`threads-stale`** - Find unanswered threads that need attention before they go cold
- **`trends`** - Track channel activity over time to spot patterns and shifts
- **`quiet`** - Find channels with no recent activity that may be candidates for archiving
- **`activity`** - See where a team member is active across channels, threads, and reactions

## Usage

```
slack-pp-cli [command]

Available Commands:
  activity       User activity summary across channels from local sync data
  analytics      Run analytics queries on locally synced data
  api            Browse all API endpoints by interface name
  auth           Manage authentication tokens
  bots           Get information about a bot user
  conversations  List all channels in the workspace
  digest         Daily or weekly digest from locally synced activity
  dnd            Get DND status for the authenticated user
  doctor         Check CLI health
  emoji          List all custom emoji for the workspace
  export         Export data to JSONL or JSON for backup, migration, or analysis
  files          Get information about a file
  funny          Find the funniest locally synced messages from public channels
  health         Channel health report from locally synced activity
  messages       Get a permalink URL for a message
  pins           List pinned items in a channel
  quiet          Find quiet or dead channels from locally synced data
  reactions      Get reactions for a message
  reminders      List all reminders for the authenticated user
  response-times Average first-response time in threads from local sync data
  search         Full-text search across synced data or live API
  stars          List starred items
  sync           Sync API data to local SQLite for offline search and analysis
  tail           Stream live changes by polling the API at regular intervals
  team           Get workspace access logs (requires admin)
  threads-stale  Find unanswered or stale threads from local sync data
  trends         Channel activity trends by week from local sync data
  usergroups     List all user groups in the workspace
  users          List all users in the workspace
  workflow       Compound workflows that combine multiple API operations
```

## Commands

### Messaging

| Command | Description |
|---------|-------------|
| `messages post_message` | Send a message to a channel, DM, or thread |
| `messages update_message` | Update an existing message |
| `messages delete_message` | Delete a message |
| `messages schedule_message` | Schedule a message for later delivery |
| `messages list_scheduled` | List scheduled messages |
| `messages get_permalink` | Get a permalink URL for a message |
| `search` | Full-text search across synced data or live API |

### Channels & Conversations

| Command | Description |
|---------|-------------|
| `conversations` | List all channels in the workspace |
| `conversations history` | Fetch message history for a channel |
| `conversations create` | Create a new channel |
| `conversations archive` | Archive a channel |
| `conversations unarchive` | Unarchive a channel |
| `conversations info` | Get information about a channel |
| `conversations invite` | Invite users to a channel |
| `conversations mark` | Mark a channel as read |
| `conversations members` | List members of a channel |
| `conversations replies` | Fetch replies in a thread |
| `conversations set_purpose` | Set the purpose for a channel |
| `conversations set_topic` | Set the topic for a channel |

### Analytics & Insights

| Command | Description |
|---------|-------------|
| `health` | Channel health report from locally synced activity |
| `digest` | Daily or weekly digest from locally synced activity |
| `trends` | Channel activity trends by week from local sync data |
| `response-times` | Average first-response time in threads |
| `threads-stale` | Find unanswered or stale threads |
| `quiet` | Find quiet or dead channels |
| `activity` | User activity summary across channels |
| `funny` | Find the funniest locally synced messages |
| `analytics` | Run analytics queries on locally synced data |

### Users & Groups

| Command | Description |
|---------|-------------|
| `users` | List all users in the workspace |
| `users info` | Get information about a user |
| `users get_presence` | Get a user's online presence status |
| `users lookup_by_email` | Find a user by email |
| `users profile_get` | Get a user's profile information |
| `users profile_set` | Set the user's profile fields |
| `users set_presence` | Set the user's presence status |
| `usergroups` | List all user groups |
| `usergroups create` | Create a new user group |
| `usergroups update` | Update an existing user group |
| `usergroups users_list` | List users in a user group |
| `usergroups users_update` | Update the members of a user group |

### Reactions, Pins & Stars

| Command | Description |
|---------|-------------|
| `reactions add` | Add an emoji reaction to a message |
| `reactions list` | List reactions by the authenticated user |
| `reactions remove` | Remove an emoji reaction |
| `pins add` | Pin a message to a channel |
| `pins list` | List pinned items in a channel |
| `pins remove` | Unpin a message from a channel |
| `stars add` | Star a message, file, or channel |
| `stars list` | List starred items |
| `stars remove` | Remove a star from an item |

### Files & Reminders

| Command | Description |
|---------|-------------|
| `files` | Get information about a file |
| `files list` | List files in the workspace |
| `files upload` | Upload a file to Slack |
| `files delete` | Delete a file |
| `reminders add` | Create a new reminder |
| `reminders complete` | Mark a reminder as complete |
| `reminders delete` | Delete a reminder |
| `reminders info` | Get info about a reminder |
| `reminders list` | List all reminders |

### Workspace & Admin

| Command | Description |
|---------|-------------|
| `team info` | Get information about the workspace |
| `team billable_info` | Get billable information for users |
| `bots` | Get information about a bot user |
| `emoji` | List all custom emoji |
| `dnd set_snooze` | Turn on Do Not Disturb |
| `dnd end_dnd` | End the current DND session |
| `dnd end_snooze` | End the current snooze |
| `dnd team_info` | Get DND status for multiple users |
| `auth status` | Show authentication status |
| `auth revoke` | Revoke the current authentication token |

### Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Check CLI health |
| `sync` | Sync API data to local SQLite |
| `search` | Full-text search across synced data or live API |
| `export` | Export data to JSONL or JSON |
| `import` | Import data from JSONL file |
| `tail` | Stream live changes by polling the API |
| `workflow archive` | Sync all resources to local store |
| `workflow status` | Show local archive status |
| `api` | Browse all API endpoints |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
slack-pp-cli users

# JSON for scripting and agents
slack-pp-cli users --json

# Filter to specific fields
slack-pp-cli users --json --select id,name,real_name

# CSV for spreadsheets
slack-pp-cli users --csv

# Dry run - show the request without sending
slack-pp-cli conversations history --channel C0123456789 --dry-run

# Agent mode - JSON + compact + no prompts in one flag
slack-pp-cli users --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | slack-pp-cli messages post_message --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add slack slack-pp-mcp -e SLACK_BOT_TOKEN=<your-token>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "slack": {
      "command": "slack-pp-mcp",
      "env": {
        "SLACK_BOT_TOKEN": "<your-key>"
      }
    }
  }
}
```

## Cookbook

```bash
# Send a message to a channel
slack-pp-cli messages post_message --channel C0123456789 --text "Hello from the CLI"

# Reply in a thread
slack-pp-cli messages post_message --channel C0123456789 --text "Thread reply" --thread-ts 1234567890.123456

# Search for messages mentioning a keyword
slack-pp-cli search "deploy failed"

# Get channel health report after syncing
slack-pp-cli sync && slack-pp-cli health

# Find stale threads that need responses
slack-pp-cli threads-stale --days 3

# Check which channels have gone quiet
slack-pp-cli quiet --days 14 --json

# See who is most active across channels
slack-pp-cli activity --user U0123456789 --days 7

# Measure team response times
slack-pp-cli response-times --channel general --days 30

# Create a reminder
slack-pp-cli reminders add --text "Review PRs" --time "in 2 hours"

# Add a reaction to a message
slack-pp-cli reactions add --channel C0123456789 --name thumbsup --timestamp 1234567890.123456

# Export workspace messages for backup
slack-pp-cli export messages --format jsonl --output messages.jsonl

# Stream live channel activity to a monitoring pipeline
slack-pp-cli tail messages --interval 10s | jq 'select(.text | contains("error"))'

# Create a private channel
slack-pp-cli conversations create --name "secret-project" --is-private true

# Look up a user by email
slack-pp-cli users lookup_by_email --email alice@example.com --json
```

## Health Check

```bash
slack-pp-cli doctor
```

```
  OK Config: ok
  OK Auth: configured
  OK API: reachable
  config_path: ~/.config/slack-pp-cli/config.json
  base_url: https://slack.com/api
```

## Configuration

Config file: `~/.config/slack-pp-cli/config.json`

Environment variables:
- `SLACK_BOT_TOKEN` - Slack Bot User OAuth Token (starts with `xoxb-`). Get one at https://api.slack.com/apps
- `SLACK_USER_TOKEN` - Slack User OAuth Token (starts with `xoxp-`). Get one at https://api.slack.com/apps
- `SLACK_BASE_URL` - Override the API base URL (default: `https://slack.com/api`). Useful for testing or proxies
- `SLACK_CONFIG` - Override config file path

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `slack-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SLACK_BOT_TOKEN`
- Ensure the token has the required scopes for the operation

**Not found errors (exit code 3)**
- Check the resource ID is correct (channel IDs start with `C`, user IDs with `U`)
- Run `slack-pp-cli conversations` or `slack-pp-cli users` to list available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Use `--rate-limit 1` to limit requests per second
- If persistent, wait a few minutes and try again

**Empty analytics results**
- Run `slack-pp-cli sync` first to populate local data
- Use `--days` to adjust the time window

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**slacrawl**](https://github.com/vincentkoc/slacrawl) - Go
- [**korotovsky/slack-mcp-server**](https://github.com/korotovsky/slack-mcp-server) - TypeScript
- [**piekstra/slack-mcp-server**](https://github.com/piekstra/slack-mcp-server) - TypeScript
- [**rockymadden/slack-cli**](https://github.com/rockymadden/slack-cli) - Bash
- [**shaharia-lab/slackcli**](https://github.com/shaharia-lab/slackcli) - TypeScript
- [**lox/slack-cli**](https://github.com/lox/slack-cli) - Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `slack-pp-cli profile save <name> --<flag> <value>`, then `slack-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.slack-pp-cli/feedback.jsonl`. When `SLACK_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `SLACK_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
