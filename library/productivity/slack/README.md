# Slack CLI

**Every Slack read your token can make, mirrored to local SQLite so your history outlives Slack's own retention wall.**

Slack hides messages past the free-plan retention window and gates export behind admin. This CLI syncs conversations, users, files, and reactions into a local SQLite database with full-text search, so `archive recall` finds decisions Slack itself will no longer serve you. On top of the mirror it computes things no endpoint returns: `catchup` for what is still waiting on you, `threads stale` for unanswered threads, and `health` for which channels are dying.

## Install

The recommended path installs both the `slack-pp-cli` binary and the `pp-slack` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install slack
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install slack --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install slack --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install slack --agent claude-code
npx -y @mvanhorn/printing-press-library install slack --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/slack-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install slack --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-slack --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-slack --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install slack --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/slack-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SLACK_BOT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-mcp@latest
```

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

</details>

## Authentication

Authentication is a bearer token, but Slack splits capability across two token types and no scope grant crosses that line. A bot token (`xoxb-`, set as `SLACK_BOT_TOKEN`) reaches conversations, users, files, reactions, usergroups, emoji, team, dnd, and bots. A user token (`xoxp-`, set as `SLACK_USER_TOKEN`) is required for the `search`, `stars`, and `reminders` families, which return `not_allowed_token_type` for bot tokens no matter which scopes you add. Run `doctor` to see which token types are configured and therefore which command families are reachable. Note also that Slack moved `conversations.history` and `conversations.replies` to a far stricter rate limit for apps redistributed outside the Marketplace; internal apps you create and install in your own workspace keep the original limits.

## Quick Start

```bash
# Confirm the binary runs and see which credentials it will look for, before any network call.
slack-pp-cli doctor --dry-run

# Verify the token is valid and learn which command families your token type can reach.
slack-pp-cli doctor

# Mirror channels and users first; everything else resolves against them.
slack-pp-cli sync --resources conversations,users --full

# Pull message history into the archive. The top-level sync only walks flat list endpoints, so this is the step that makes recall, catchup, threads stale, and health work.
slack-pp-cli archive sync --since 30d

# Search the mirror, including anything already past Slack's retention wall.
slack-pp-cli archive recall "deploy" --agent

# See what is still waiting on you rather than what is merely unread.
slack-pp-cli catchup --since 24h

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local archive that outlives Slack
- **`archive recall`** — Find messages in your local archive, including ones Slack has already hidden behind the 90-day retention wall.

  _Reach for this instead of search when the answer may be older than 90 days, or when you only hold a bot token, since Slack's search endpoint rejects bot tokens outright._

  ```bash
  slack-pp-cli archive recall "deploy" --agent --limit 20
  ```
- **`archive coverage`** — Show what date range your local mirror actually holds per channel, and where the gaps are.

  _Check this before trusting an empty recall result, so you can tell a genuine absence from an unsynced range._

  ```bash
  slack-pp-cli archive coverage --json
  ```

### Obligation over volume
- **`catchup`** — See what happened while you were away: new volume per channel, messages that mention you, and threads still waiting on your reply.

  _Use this as the single call that replaces listing channels, pulling history per channel, and resolving mentions by hand._

  ```bash
  slack-pp-cli catchup --since 24h --agent
  ```
- **`threads stale`** — List threads across the archive where the last reply is not yours and nobody has answered since.

  _Pick this when the question is what is unanswered, rather than what is recent._

  ```bash
  slack-pp-cli threads stale --days 3 --json
  ```

### Workspace stewardship
- **`health`** — Compare your channels by messages per day, distinct posters, median first-reply latency, and days idle.

  _Use this for archive-candidate decisions when you are not a workspace admin and have no export or admin console._

  ```bash
  slack-pp-cli health --dying --json
  ```
- **`users activity`** — Profile where one person posts, which threads they carry, and when they were last seen.

  _Reach for this for handoff and standup context about a named teammate, not about yourself._

  ```bash
  slack-pp-cli users activity chris --days 30 --json
  ```

### Identity resolution
- **`users whois`** — Turn an opaque Slack ID, handle, or email into one card with shared channels, timezone, DND state, and last-seen.

  _Use this the moment a payload hands you a bare U-prefixed or C-prefixed ID, instead of calling users info per identifier._

  ```bash
  slack-pp-cli users whois U04AB9XYZ --agent
  ```

## Recipes

### Find a decision Slack has forgotten

```bash
slack-pp-cli archive recall "deploy" --agent --select results.channel,results.user,results.ts,results.text
```

Searches the local mirror and narrows the payload to just the fields worth reading, so an agent does not burn context on full message objects.

### Morning obligation check

```bash
slack-pp-cli catchup --since 24h --json
```

Returns new volume per channel, messages mentioning you, and threads whose last reply is not yours.

### Find archive candidates without admin access

```bash
slack-pp-cli health --dying --json
```

Ranks your channels by idle days and posting volume so you can propose archives with evidence.

### Resolve an opaque ID from a payload

```bash
slack-pp-cli users whois U04AB9XYZ --agent
```

Turns a bare Slack user ID into shared channels, timezone, DND state, and last-seen in one call.

### Confirm the mirror before trusting an empty result

```bash
slack-pp-cli archive coverage --json
```

Shows the mirrored timestamp range per channel so an empty recall can be read as a sync gap rather than an absence.

## Usage

Run `slack-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SLACK_CONFIG_DIR`, `SLACK_DATA_DIR`, `SLACK_STATE_DIR`, or `SLACK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SLACK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SLACK_HOME=/srv/slack
slack-pp-cli doctor
```

Under `SLACK_HOME=/srv/slack`, the four dirs resolve to `/srv/slack/config`, `/srv/slack/data`, `/srv/slack/state`, and `/srv/slack/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "slack": {
      "command": "slack-pp-mcp",
      "env": {
        "SLACK_HOME": "/srv/slack"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SLACK_DATA_DIR` overrides an explicit `--home` for that kind. Use `SLACK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SLACK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `slack-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### api-test

Manage api test

- **`slack-pp-cli api-test`** - Checks API calling code.

### auth_api

Test and manage authentication

- **`slack-pp-cli auth-api revoke`** - Revoke the current authentication token
- **`slack-pp-cli auth-api test`** - Test the authentication token and get identity info

### bots

Get information about bot users

- **`slack-pp-cli bots`** - Get information about a bot user

### canvases

Create, edit, share, and delete Slack canvases

- **`slack-pp-cli canvases create`** - Create a canvas, standalone or bound to a channel
- **`slack-pp-cli canvases read`** - Read a canvas's content
- **`slack-pp-cli canvases edit`** - Apply a change to an existing canvas
- **`slack-pp-cli canvases delete`** - Delete a canvas
- **`slack-pp-cli canvases access-set`** - Grant users or channels access to a canvas
- **`slack-pp-cli canvases sections`** - Look up section IDs in a canvas, for targeted edits

Requires the `canvases:write` and `canvases:read` scopes, plus `files:read` for
`canvases read`.

Slack publishes no get-canvas-content endpoint, so `read` resolves the canvas's
backing file through `files.info` and downloads `url_private_download`. Content
therefore comes back as **HTML, not the Markdown it was created from** — create and
read are not a lossless round trip. `--format text` is a deliberately crude tag
strip for grepping, not a converter. The HTML does carry section ids inline, which
makes `read` more useful than `sections` for targeting an edit: `sections` returns
ids with no content.

### chat-delete-scheduled-message

Manage chat delete scheduled message

- **`slack-pp-cli chat-delete-scheduled-message`** - Deletes a pending scheduled message from the queue.

### chat-me-message

Manage chat me message

- **`slack-pp-cli chat-me-message`** - Share a me message into a channel.

### chat-post-ephemeral

Manage chat post ephemeral

- **`slack-pp-cli chat-post-ephemeral`** - Sends an ephemeral message to a user in a channel.

### chat-unfurl

Manage chat unfurl

- **`slack-pp-cli chat-unfurl`** - Provide custom unfurl behavior for user-posted URLs

### conversations

Read channel history, list channels, manage channel membership

- **`slack-pp-cli conversations archive`** - Archive a channel
- **`slack-pp-cli conversations create`** - Create a new channel
- **`slack-pp-cli conversations get`** - Get information about a channel
- **`slack-pp-cli conversations history`** - Fetch message history for a channel
- **`slack-pp-cli conversations invite`** - Invite users to a channel
- **`slack-pp-cli conversations list`** - List all channels in the workspace
- **`slack-pp-cli conversations mark`** - Mark a channel as read up to a specific message
- **`slack-pp-cli conversations members`** - List members of a channel
- **`slack-pp-cli conversations replies`** - Fetch replies in a thread
- **`slack-pp-cli conversations set-purpose`** - Set the purpose for a channel
- **`slack-pp-cli conversations set-topic`** - Set the topic for a channel
- **`slack-pp-cli conversations unarchive`** - Unarchive a channel

### conversations-close

Manage conversations close

- **`slack-pp-cli conversations-close`** - Closes a direct message or multi-person direct message.

### conversations-join

Manage conversations join

- **`slack-pp-cli conversations-join`** - Joins an existing conversation.

### conversations-kick

Manage conversations kick

- **`slack-pp-cli conversations-kick`** - Removes a user from a conversation.

### conversations-leave

Manage conversations leave

- **`slack-pp-cli conversations-leave`** - Leaves a conversation.

### conversations-open

Manage conversations open

- **`slack-pp-cli conversations-open`** - Opens or resumes a direct message or multi-person direct message.

### conversations-rename

Manage conversations rename

- **`slack-pp-cli conversations-rename`** - Renames a conversation.

### dnd

Manage Do Not Disturb settings

- **`slack-pp-cli dnd end-dnd`** - End the current Do Not Disturb session immediately
- **`slack-pp-cli dnd end-snooze`** - End the current Do Not Disturb session
- **`slack-pp-cli dnd get`** - Get DND status for the authenticated user
- **`slack-pp-cli dnd set-snooze`** - Turn on Do Not Disturb for a specified number of minutes
- **`slack-pp-cli dnd team-info`** - Get DND status for multiple users

### emoji

List custom emoji in the workspace

- **`slack-pp-cli emoji`** - List all custom emoji for the workspace

### files

Upload, list, and manage files

- **`slack-pp-cli files delete`** - Delete a file
- **`slack-pp-cli files get`** - Get information about a file
- **`slack-pp-cli files list`** - List files in the workspace
- **`slack-pp-cli files upload`** - Upload a file to Slack (`--file` a path, or
  `--content` + `--filename` for inline text; `--channel` to share it)

Uploads run Slack's three-step external flow (`files.getUploadURLExternal` → raw
bytes → `files.completeUploadExternal`); the old `files.upload` endpoint is retired
and answers `method_deprecated`. The bytes stream, so a large file does not have to
fit in memory. `--channels` is accepted as a deprecated alias for a single channel —
the flow shares into one conversation per call, so a multi-channel list is rejected
rather than silently truncated.

### files-comments-delete

Manage files comments delete

- **`slack-pp-cli files-comments-delete`** - Deletes an existing comment on a file.

### files-remote-add

Manage files remote add

- **`slack-pp-cli files-remote-add`** - Adds a file from a remote service

### files-remote-info

Manage files remote info

- **`slack-pp-cli files-remote-info`** - Retrieve information about a remote file added to Slack

### files-remote-list

Manage files remote list

- **`slack-pp-cli files-remote-list`** - Retrieve information about a remote file added to Slack

### files-remote-remove

Manage files remote remove

- **`slack-pp-cli files-remote-remove`** - Remove a remote file.

### files-remote-share

Manage files remote share

- **`slack-pp-cli files-remote-share`** - Share a remote file into a channel.

### files-remote-update

Manage files remote update

- **`slack-pp-cli files-remote-update`** - Updates an existing remote file.

### files-revoke-public-url

Manage files revoke public url

- **`slack-pp-cli files-revoke-public-url`** - Revokes public/external sharing access for a file

### files-shared-public-url

Manage files shared public url

- **`slack-pp-cli files-shared-public-url`** - Enables a file for public/external sharing.

### messages

Send, read, update, and delete messages in channels and DMs

- **`slack-pp-cli messages delete-message`** - Delete a message
- **`slack-pp-cli messages get-permalink`** - Get a permalink URL for a message
- **`slack-pp-cli messages list-scheduled`** - List scheduled messages
- **`slack-pp-cli messages post-message`** - Send a message to a channel, DM, or thread
- **`slack-pp-cli messages schedule-message`** - Schedule a message for later delivery
- **`slack-pp-cli messages update-message`** - Update an existing message

### pins

Pin and unpin messages in channels

- **`slack-pp-cli pins add`** - Pin a message to a channel
- **`slack-pp-cli pins list`** - List pinned items in a channel
- **`slack-pp-cli pins remove`** - Unpin a message from a channel

### reactions

Add and remove emoji reactions on messages

- **`slack-pp-cli reactions add`** - Add an emoji reaction to a message
- **`slack-pp-cli reactions get`** - Get reactions for a message
- **`slack-pp-cli reactions list`** - List reactions made by the authenticated user
- **`slack-pp-cli reactions remove`** - Remove an emoji reaction from a message

### reminders

Create and manage personal reminders

- **`slack-pp-cli reminders add`** - Create a new reminder
- **`slack-pp-cli reminders complete`** - Mark a reminder as complete
- **`slack-pp-cli reminders delete`** - Delete a reminder
- **`slack-pp-cli reminders get`** - Get info about a reminder
- **`slack-pp-cli reminders list`** - List all reminders for the authenticated user

### search_api

Search messages and files across the workspace

- **`slack-pp-cli search-api`** - Search for messages matching a query

### stars

Star and unstar messages and files

- **`slack-pp-cli stars add`** - Star a message, file, or channel
- **`slack-pp-cli stars list`** - List starred items
- **`slack-pp-cli stars remove`** - Remove a star from an item

### team

Get workspace information

- **`slack-pp-cli team access-logs`** - Get workspace access logs (requires admin)
- **`slack-pp-cli team billable-info`** - Get billable information for workspace users
- **`slack-pp-cli team get`** - Get information about the workspace

### team-integration-logs

Manage team integration logs

- **`slack-pp-cli team-integration-logs`** - Gets the integration logs for the current team.

### team-profile-get

Manage team profile get

- **`slack-pp-cli team-profile-get`** - Retrieve a team's profile.

### usergroups

Manage workspace user groups

- **`slack-pp-cli usergroups create`** - Create a new user group
- **`slack-pp-cli usergroups list`** - List all user groups in the workspace
- **`slack-pp-cli usergroups update`** - Update an existing user group
- **`slack-pp-cli usergroups users-list`** - List users in a user group
- **`slack-pp-cli usergroups users-update`** - Update the members of a user group

### usergroups-disable

Manage usergroups disable

- **`slack-pp-cli usergroups-disable`** - Disable an existing User Group

### usergroups-enable

Manage usergroups enable

- **`slack-pp-cli usergroups-enable`** - Enable a User Group

### users

List and look up workspace users

- **`slack-pp-cli users get`** - Get information about a user
- **`slack-pp-cli users get-presence`** - Get a user's online presence status
- **`slack-pp-cli users list`** - List all users in the workspace
- **`slack-pp-cli users lookup-by-email`** - Find a user by their email address
- **`slack-pp-cli users profile-get`** - Get a user's profile information
- **`slack-pp-cli users profile-set`** - Set the user's profile fields
- **`slack-pp-cli users set-presence`** - Set the user's presence status

### users-conversations

Manage users conversations

- **`slack-pp-cli users-conversations`** - List conversations the calling user may access.

### users-delete-photo

Manage users delete photo

- **`slack-pp-cli users-delete-photo`** - Delete the user profile photo

### users-identity

Manage users identity

- **`slack-pp-cli users-identity`** - Get a user's identity.

### users-set-active

Manage users set active

- **`slack-pp-cli users-set-active`** - Marked a user as active. Deprecated and non-functional.

### users-set-photo

Manage users set photo

- **`slack-pp-cli users-set-photo`** - Set the user profile photo


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`slack-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`slack-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`slack-pp-cli learnings list`** - Inspect taught rows
- **`slack-pp-cli learnings forget <query>`** - Undo a teach
- **`slack-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`slack-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`slack-pp-cli teach-pattern`** - Install a query/resource template up front
- **`slack-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SLACK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `slack-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
slack-pp-cli bots --bot example-value

# JSON for scripting and agents
slack-pp-cli bots --bot example-value --json

# Filter to specific fields
slack-pp-cli bots --bot example-value --json --select id,name,status

# Dry run — show the request without sending
slack-pp-cli bots --bot example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
slack-pp-cli bots --bot example-value --agent
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
slack-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `slack-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/slack-pp-cli/config.toml`; `--home`, `SLACK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SLACK_BOT_TOKEN` | per_call | Yes | Set to your API credential. |
| `SLACK_USER_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `slack-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `slack-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SLACK_BOT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **`not_allowed_token_type` on search, stars, or reminders commands** — Those families require a user token. Set SLACK_USER_TOKEN to an xoxp- token with the matching scope; adding bot scopes cannot fix it.
- **`missing_scope` when syncing conversations** — Add the missing read scope in your app's OAuth & Permissions page, then reinstall to the workspace so the token is reissued.
- **Sync returns far fewer records than expected and stops early** — Run `slack-pp-cli archive coverage --json` to see the mirrored range per channel, then re-run sync with a wider `--since` window.
- **`ratelimited` errors while syncing message history** — Slack applies a much stricter tier to conversations.history for apps redistributed outside the Marketplace. Use a token from an app you created and installed in your own workspace, or lower `--max-pages` and sync in smaller windows.
- **`channel_not_found` for a direct-message channel ID** — Raw D-prefixed IDs are not accepted directly; resolve the DM by user with the command's user-targeting flag instead of passing the D id.
- **`archive recall` returns nothing for a message you are certain exists** — Check `archive coverage` first; an empty result means the range was never synced, not that the message is absent.
- **archive sync reports not_in_channel for every channel** — A bot token can only read channels it has joined. Run /invite @your-app in each channel you want mirrored, then re-run archive sync.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**korotovsky/slack-mcp-server**](https://github.com/korotovsky/slack-mcp-server) — TypeScript (1700 stars)
- [**rockymadden/slack-cli**](https://github.com/rockymadden/slack-cli) — Shell
- [**shaharia-lab/slackcli**](https://github.com/shaharia-lab/slackcli) — TypeScript
- [**slackapi/slack-cli**](https://github.com/slackapi/slack-cli) — Go
- [**tumf/slack-rs**](https://github.com/tumf/slack-rs) — Rust
- [**piekstra/slack-cli**](https://github.com/piekstra/slack-cli) — TypeScript
- [**slackapi/python-slack-sdk**](https://github.com/slackapi/python-slack-sdk) — Python
- [**slackapi/slack-api-specs**](https://github.com/slackapi/slack-api-specs) — JSON

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Known Gaps

**Message history needs a separate sync step.** The top-level `sync` command mirrors flat list
endpoints (channels, members, files, reactions, usergroups, emoji). Slack serves message history
from `conversations.history?channel=<id>`, which is keyed by a query parameter rather than a nested
path, so it is not part of that sweep. Run `slack-pp-cli archive sync` to populate the message
archive that `archive recall`, `catchup`, `threads stale`, `health`, and `users activity` read from.
Without it those commands report an unsynced mirror rather than silently returning nothing.

**A bot token cannot reach every command family.** `search`, `stars`, and `reminders` return
`not_allowed_token_type` for a bot token (`xoxb-`) no matter which scopes are granted; they need a
user token (`xoxp-`) in `SLACK_USER_TOKEN`. `sync` reports these as per-resource warnings and
continues. Run `doctor` to see which credentials are configured.

**A bot can only read channels it has joined.** `archive sync` reports `not_in_channel` per channel
rather than storing zero messages silently. Run `/invite @your-app` in each channel you want
mirrored.

**Bot-authored messages show a raw bot ID instead of a name.** `sync` mirrors the `users` table, so
messages posted by a person resolve to a display name. Messages posted by an app carry only a
`bot_id`, which this CLI does not yet resolve, so `archive recall`, `catchup`, and `threads stale`
print the raw identifier (for example `B0EXAMPLE02`) in the `user_name` field. In workspaces whose
traffic is mostly app-generated this affects nearly every row. Use `slack-pp-cli bots --bot <id>` to
look an identifier up by hand.

**Search matches Slack's raw markup, not the rendered text you see.** Output is de-rendered —
`archive recall`, `catchup`, and `threads stale` turn `<@U0EXAMPLE01>` into `@GitHub`, `<#C01234567>`
into `#general`, `<https://example.com|label>` into `label (https://example.com)`, `<!here>` into
`@here`, and `&amp;`/`&lt;`/`&gt;` into `&`/`<`/`>` in the `text`, `parent_text`, and
`last_reply_text` fields. The mirror still stores Slack's original encoding, and the `--channel`,
`--from`, and full-text query filters run against that stored form. So searching for a person's
display name will not match a mention of them: search for the user ID instead, or resolve it first
with `slack-pp-cli users whois <name>`. Raw bodies are also what `sql` and `export` return, by
design — those are the fidelity-preserving escape hatches.

**Mock-mode data-pipeline verification does not pass.** The generator's `verify` harness reports
"domain tables created but 0 rows after sync" when syncing against its mock server, so that leg
fails even though the command surface passes at 100% (70/70, 0 critical). The same pipeline was
verified against the live Slack API — 29 conversations, 32 members, and 991 messages mirrored, with
search relevance and negative controls checked — and the failure reproduces with all local patches
disabled, so it is a limitation of the verification harness rather than of this CLI. Tracked as a
Printing Press issue.
