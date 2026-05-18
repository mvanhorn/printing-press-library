# Youtube CLI

Combined CLI for multiple API services

Printed by [@CarazzoSantiago](https://github.com/CarazzoSantiago).

## Install

The recommended path installs both the `youtube-creator-analytics-pp-cli` binary and the `pp-youtube-creator-analytics` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install youtube
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install youtube --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install youtube --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install youtube --agent claude-code
npx -y @mvanhorn/printing-press install youtube --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-youtube-creator-analytics --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-youtube-creator-analytics --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-youtube-creator-analytics skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-youtube-creator-analytics. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YOUTUBE_DATA_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "youtube": {
      "command": "youtube-creator-analytics-pp-mcp",
      "env": {
        "YOUTUBE_DATA_OAUTH2C": "<your-key>"
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

Register an OAuth app in the Google Cloud Console (see `youtube-creator-analytics-pp-cli auth setup`), then run the OAuth flow:

```bash
youtube-creator-analytics-pp-cli auth login --client-id <id> --client-secret <secret>
```

Or set an access token via environment variable:

```bash
export YOUTUBE_DATA_OAUTH2C="your-token-here"
```

### 3. Verify Setup

```bash
youtube-creator-analytics-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
youtube-creator-analytics-pp-cli group-items list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local analytics that compound
- **`decay`** — Flag underperforming videos by views-per-day vs the channel baseline (--metric velocity, default), with a --days window and a noise warning at <30 videos.

  _Tells the creator which videos to re-promote or re-edit, before the algorithm forgets them._

  ```bash
  youtube-creator-analytics-pp-cli decay --metric velocity --days 90 --json
  ```
- **`retention-leaderboard`** — Top videos by average view percentage from Analytics API. Statistically noisy with <30 videos in the lookback window — treat as directional.

  _Ranks content by attention, not just views — the actual retention signal._

  ```bash
  youtube-creator-analytics-pp-cli retention-leaderboard --days 90 --json
  ```
- **`sub-velocity`** — Daily subscriber gain/loss rate from Analytics, day-bucketed.

  _Pinpoints which days a creator added or lost subs so launch-day pulls become visible._

  ```bash
  youtube-creator-analytics-pp-cli sub-velocity --days 28 --json
  ```
- **`posting-cadence`** — Correlate publish day-of-week or hour-of-day with average views.

  _Surfaces the channel's actual best posting window from observed data, not folklore._

  ```bash
  youtube-creator-analytics-pp-cli posting-cadence --mode dow --json
  ```
- **`ctr-cohort`** — Impressions and click-through-rate per video (Analytics API).

  _Tells the creator which thumbnails earn the click before they reroll the entire batch._

  ```bash
  youtube-creator-analytics-pp-cli ctr-cohort --days 90 --json
  ```

### Cross-channel intelligence
- **`competitor-diff`** — Compare uploads/cadence/avg views across cached channels (your channel + competitors).

  _One call answers 'how does my niche move?' without leaving the terminal._

  ```bash
  youtube-creator-analytics-pp-cli competitor-diff --json
  ```
- **`topic-cluster`** — Group videos into topic clusters by tag overlap and rank by avg views.

  _Reveals which themes pull views, not just which individual videos won._

  ```bash
  youtube-creator-analytics-pp-cli topic-cluster --limit 12 --json
  ```

### Transcript & comment mining
- **`transcript search`** — Full-text search across scraped transcripts (offline, FTS5).

  _Lets an agent grep an entire channel's spoken content offline, with no quota cost._

  ```bash
  youtube-creator-analytics-pp-cli transcript search 'sueño infantil' --json
  ```
- **`theme-mine`** — Extract most-repeated bigrams/trigrams from cached comments (FAQ surface area).

  _Shows what audience actually talks about so the creator picks topics with proven demand._

  ```bash
  youtube-creator-analytics-pp-cli theme-mine --n 2 --limit 30 --json
  ```
- **`comment-faq`** — Surface most-liked question-shaped comments across cached videos.

  _Generates a ready-to-answer Q&A pipeline straight from the audience._

  ```bash
  youtube-creator-analytics-pp-cli comment-faq --limit 20 --json
  ```

### Quota-aware ops
- **`sync-plan`** — Estimate Data v3 quota units needed for a full refresh + comment sync, with daily-quota headroom.

  _Prevents the surprise 'quotaExceeded' that wipes a creator's tooling for the day._

  ```bash
  youtube-creator-analytics-pp-cli sync-plan --comments-per-video 2 --json
  ```

## Usage

Run `youtube-creator-analytics-pp-cli --help` for the full command reference and flag list.

## Commands

### group-items

Manage group items

- **`youtube-creator-analytics-pp-cli group-items delete`** - Removes an item from a group.
- **`youtube-creator-analytics-pp-cli group-items insert`** - Creates a group item.
- **`youtube-creator-analytics-pp-cli group-items list`** - Returns a collection of group items that match the API request parameters.

### groups

Manage groups

- **`youtube-creator-analytics-pp-cli groups delete`** - Deletes a group.
- **`youtube-creator-analytics-pp-cli groups insert`** - Creates a group.
- **`youtube-creator-analytics-pp-cli groups list`** - Returns a collection of groups that match the API request parameters. For example, you can retrieve all groups that the authenticated user owns, or you can retrieve one or more groups by their unique IDs.
- **`youtube-creator-analytics-pp-cli groups update`** - Modifies a group. For example, you could change a group's title.

### reports

Manage reports

- **`youtube-creator-analytics-pp-cli reports`** - Retrieve your YouTube Analytics reports.

### youtube

Manage youtube

- **`youtube-creator-analytics-pp-cli youtube abuse-reports-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube activities-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube captions-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube captions-download`** - Downloads a caption track.
- **`youtube-creator-analytics-pp-cli youtube captions-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube captions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube captions-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube channel-banners-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube channel-sections-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube channel-sections-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube channel-sections-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube channel-sections-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube channels-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube channels-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube comment-threads-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube comment-threads-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube comments-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube comments-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube comments-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube comments-mark-as-spam`** - Expresses the caller's opinion that one or more comments should be flagged as spam.
- **`youtube-creator-analytics-pp-cli youtube comments-set-moderation-status`** - Sets the moderation status of one or more comments.
- **`youtube-creator-analytics-pp-cli youtube comments-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube i18n-languages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube i18n-regions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube live-broadcasts-bind`** - Bind a broadcast to a stream.
- **`youtube-creator-analytics-pp-cli youtube live-broadcasts-delete`** - Delete a given broadcast.
- **`youtube-creator-analytics-pp-cli youtube live-broadcasts-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-creator-analytics-pp-cli youtube live-broadcasts-insert-cuepoint`** - Insert cuepoints in a broadcast
- **`youtube-creator-analytics-pp-cli youtube live-broadcasts-list`** - Retrieve the list of broadcasts associated with the given channel.
- **`youtube-creator-analytics-pp-cli youtube live-broadcasts-transition`** - Transition a broadcast to a given status.
- **`youtube-creator-analytics-pp-cli youtube live-broadcasts-update`** - Updates an existing broadcast for the authenticated user.
- **`youtube-creator-analytics-pp-cli youtube live-chat-bans-delete`** - Deletes a chat ban.
- **`youtube-creator-analytics-pp-cli youtube live-chat-bans-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube live-chat-messages-delete`** - Deletes a chat message.
- **`youtube-creator-analytics-pp-cli youtube live-chat-messages-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube live-chat-messages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube live-chat-moderators-delete`** - Deletes a chat moderator.
- **`youtube-creator-analytics-pp-cli youtube live-chat-moderators-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube live-chat-moderators-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube live-streams-delete`** - Deletes an existing stream for the authenticated user.
- **`youtube-creator-analytics-pp-cli youtube live-streams-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-creator-analytics-pp-cli youtube live-streams-list`** - Retrieve the list of streams associated with the given channel. --
- **`youtube-creator-analytics-pp-cli youtube live-streams-update`** - Updates an existing stream for the authenticated user.
- **`youtube-creator-analytics-pp-cli youtube members-list`** - Retrieves a list of members that match the request criteria for a channel.
- **`youtube-creator-analytics-pp-cli youtube memberships-levels-list`** - Retrieves a list of all pricing levels offered by a creator to the fans.
- **`youtube-creator-analytics-pp-cli youtube playlist-items-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube playlist-items-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube playlist-items-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube playlist-items-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube playlists-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube playlists-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube playlists-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube playlists-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube search-list`** - Retrieves a list of search resources
- **`youtube-creator-analytics-pp-cli youtube subscriptions-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube subscriptions-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube subscriptions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube super-chat-events-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube tests-insert`** - POST method.
- **`youtube-creator-analytics-pp-cli youtube third-party-links-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube third-party-links-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube third-party-links-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube third-party-links-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube thumbnails-set`** - As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which doesn't result in creation of a single resource), I use a custom verb here.
- **`youtube-creator-analytics-pp-cli youtube update-comment-threads`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube video-abuse-report-reasons-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube video-categories-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube videos-delete`** - Deletes a resource.
- **`youtube-creator-analytics-pp-cli youtube videos-get-rating`** - Retrieves the ratings that the authorized user gave to a list of specified videos.
- **`youtube-creator-analytics-pp-cli youtube videos-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-analytics-pp-cli youtube videos-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-analytics-pp-cli youtube videos-rate`** - Adds a like or dislike rating to a video or removes a rating from a video.
- **`youtube-creator-analytics-pp-cli youtube videos-report-abuse`** - Report abuse for a video.
- **`youtube-creator-analytics-pp-cli youtube videos-update`** - Updates an existing resource.
- **`youtube-creator-analytics-pp-cli youtube watermarks-set`** - Allows upload of watermark image and setting it for a channel.
- **`youtube-creator-analytics-pp-cli youtube watermarks-unset`** - Allows removal of channel watermark.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
youtube-creator-analytics-pp-cli group-items list

# JSON for scripting and agents
youtube-creator-analytics-pp-cli group-items list --json

# Filter to specific fields
youtube-creator-analytics-pp-cli group-items list --json --select id,name,status

# Dry run — show the request without sending
youtube-creator-analytics-pp-cli group-items list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
youtube-creator-analytics-pp-cli group-items list --agent
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
youtube-creator-analytics-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/youtube-creator-analytics-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YOUTUBE_DATA_OAUTH2C` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `youtube-creator-analytics-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YOUTUBE_DATA_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
