# YouTube CLI

**Every YouTube Data, Analytics, and Reporting API endpoint, plus offline sync, the moderation verbs Studio hides, and a quota meter that keeps n8n from burning your daily limit.**

Wraps the three official Google YouTube APIs (Data v3, Analytics v2, Reporting v1) as one CLI with sub-modules, adds the high-frequency operations creators actually need (held-comment moderation queue, daily analytics digest, bulk metadata, PubSubHubbub upload triggers, A/B thumbnail testing, channel backup via yt-dlp), and persists state in a local SQLite with FTS5 search. Designed for n8n Execute Command nodes: every command emits JSON on stdout with typed exit codes.

## Install

The recommended path installs both the `youtube-creator-pp-cli` binary and the `pp-youtube-creator` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install youtube-creator
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install youtube-creator --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install youtube-creator --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install youtube-creator --agent claude-code
npx -y @mvanhorn/printing-press install youtube-creator --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-youtube-creator --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-youtube-creator --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-youtube-creator skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-youtube-creator. The skill defines how its required CLI can be installed.
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
      "command": "youtube-creator-pp-mcp",
      "env": {
        "YOUTUBE_DATA_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticates via OAuth2 device flow (run `auth login` once, refresh token persisted at ~/.config/youtube-creator-pp-cli/auth.json) or via a read-only YouTube API key for public-data commands. The auth flow handles scope escalation — re-running `auth login --scope yt-analytics-monetary.readonly` adds revenue access without re-doing the full flow. n8n calls the binary as a subprocess; the binary loads the persisted token. members.list requires a Google access-approval form separately; doctor surfaces this status.

## Quick Start

```bash
# One-time OAuth device flow; persists refresh token.
youtube-creator-pp-cli auth login


# Pull videos, playlists, comments into local SQLite for offline queries.
youtube-creator-pp-cli sync


# Show held-for-review comments — the highest-value daily workflow.
youtube-creator-pp-cli mod queue --since 7d --json


# Weekly digest, pipe to mail or Slack via n8n.
youtube-creator-pp-cli digest analytics --since 7d --markdown


# Check remaining quota before running expensive bulk operations.
youtube-creator-pp-cli quota meter --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Moderation as a verb
- **`mod queue`** — Pull every comment YouTube held for review, batch-approve or reject with optional author-ban in one verb.

  _When automating creator workflows, this is the highest-frequency moderation task and the single endpoint Studio's UX hides behind multiple panes._

  ```bash
  youtube-creator-pp-cli mod queue --since 7d --apply approve --ban-author
  ```
- **`mod auto`** — Apply YAML-defined regex/keyword rules (and optional LLM classify hook) to incoming or held comments.

  _Lets an agent enforce site-specific moderation policy without manual queue review._

  ```bash
  youtube-creator-pp-cli mod auto --rules rules.yaml --since 1h --apply
  ```

### Daily creator rituals
- **`digest analytics`** — Pre-baked Analytics API queries packaged as a Markdown report (top videos, CTR, watch time, traffic sources, revenue, Shorts vs long-form).

  _Drop into n8n Schedule Trigger → digest analytics → Send Email. One command replaces a hand-built dashboard._

  ```bash
  youtube-creator-pp-cli digest analytics --since 7d --markdown
  ```
- **`digest video`** — Single-video performance digest: retention curve, traffic sources, demographics, devices, end-screen impressions.

  _When a new upload spikes or flops, this is the first question creators ask._

  ```bash
  youtube-creator-pp-cli digest video dQw4w9WgXcQ --markdown
  ```

### Bulk operations
- **`bulk metadata`** — Filter the full video catalog by jq-style predicate, apply set/append/replace mutations, with dry-run diff.

  _Re-tag or footer-stamp an entire back catalog without manually paging through Studio._

  ```bash
  youtube-creator-pp-cli bulk metadata --category 22 --append-description footer.md --dry-run
  ```
- **`playlist hygiene`** — Auto-add new uploads to playlists by tag/title pattern; reorder existing playlists by view count.

  _Keeps playlists curated without Studio drag-and-drop._

  ```bash
  youtube-creator-pp-cli playlist hygiene --rules rules.yaml --apply
  ```

### Reachability and triggers
- **`pubsub subscribe`** — Subscribe a webhook callback to a channel's upload feed via WebSub; helper command verifies and prints the GET challenge response.

  _Switches uploads from polling (quota-burning, slow) to push-trigger in n8n. Real-time competitor-monitoring and own-upload pipelines._

  ```bash
  youtube-creator-pp-cli pubsub subscribe --channel UCxxx --callback https://n8n.example.com/webhook/yt --hub https://pubsubhubbub.appspot.com/
  ```

### Quota-aware operation
- **`quota meter`** — Print remaining quota estimate based on local-stored per-call cost log, and the cost of the next planned command.

  _Prevents the n8n-blow-the-daily-quota-in-one-execution disaster. Every other command honors --show-quota to print cost before execution._

  ```bash
  youtube-creator-pp-cli quota meter --json
  ```

### Archive and recovery
- **`backup`** — Wraps yt-dlp with cookies-from-browser to archive own videos, captions, thumbnails, info-json to local or S3-compatible target.

  _Weekly cron via n8n → never lose your channel._

  ```bash
  youtube-creator-pp-cli backup --since 30d --captions --thumbnails --info-json --out ./archive/
  ```

### Growth automation
- **`ab thumbnails`** — Schedule thumbnail rotation via thumbnails.set, track CTR via Analytics API, compute statistical significance.

  _Existing third-party tools charge SaaS pricing for this. CLI replaces them._

  ```bash
  youtube-creator-pp-cli ab thumbnails start --video dQw4w9WgXcQ --variants a.png,b.png --rotate 24h
  ```
- **`chapters auto`** — Pull captions via yt-dlp, send them to Claude or OpenAI, write the proposed `0:00 Title` chapter block back to the video description via `videos.update`.

  _Saves 10-15 minutes of manual chapter authoring per video._

  ```bash
  # Configure once:
  export YT_PP_CLI_CLAUDE_API_KEY=sk-ant-...   # or YT_PP_CLI_OPENAI_API_KEY=sk-...

  # Preview chapters (no mutation):
  youtube-creator-pp-cli chapters auto dQw4w9WgXcQ --provider claude

  # Apply to the video's description:
  youtube-creator-pp-cli chapters auto dQw4w9WgXcQ --provider claude --apply
  ```

  Default models: `claude-sonnet-4-6` for Claude, `gpt-4o-mini` for OpenAI. Override via `YT_PP_CLI_CLAUDE_MODEL` / `YT_PP_CLI_OPENAI_MODEL`. `--provider none` returns a heuristic preview without any API call (useful for testing the yt-dlp + write-back pipeline without spending tokens).

### Warehouse-style ingestion
- **`reporting sync`** — Enumerate report types, ensure jobs exist, poll for completed reports, download CSV via redirect.

  _Drop-in for a daily warehouse pull when Analytics API queries are too expensive at scale._

  ```bash
  youtube-creator-pp-cli reporting sync --types content_owner_a1,channel_basic_a2 --since 30d --out ./reports/
  ```

### n8n integration
- **`recipes n8n`** — Prints ready-to-paste n8n workflow JSON snippets for the six highest-value automations.

  _Closes the loop on n8n integration — the user-stated goal._

  ```bash
  youtube-creator-pp-cli recipes n8n print held-comment-mod
  ```

## Usage

Run `youtube-creator-pp-cli --help` for the full command reference and flag list.

## Commands

### group-items

Manage group items

- **`youtube-creator-pp-cli group-items delete`** - Removes an item from a group.
- **`youtube-creator-pp-cli group-items insert`** - Creates a group item.
- **`youtube-creator-pp-cli group-items list`** - Returns a collection of group items that match the API request parameters.

### groups

Manage groups

- **`youtube-creator-pp-cli groups delete`** - Deletes a group.
- **`youtube-creator-pp-cli groups insert`** - Creates a group.
- **`youtube-creator-pp-cli groups list`** - Returns a collection of groups that match the API request parameters. For example, you can retrieve all groups that the authenticated user owns, or you can retrieve one or more groups by their unique IDs.
- **`youtube-creator-pp-cli groups update`** - Modifies a group. For example, you could change a group's title.

### media

Manage media

- **`youtube-creator-pp-cli media <resourceName>`** - Method for media download. Download is supported on the URI `/v1/media/{+name}?alt=media`.

### report-types

Manage report types

- **`youtube-creator-pp-cli report-types`** - Lists report types.

### reports

Manage reports

- **`youtube-creator-pp-cli reports`** - Retrieve your YouTube Analytics reports.

### youtube

Manage youtube

- **`youtube-creator-pp-cli youtube abuse-reports-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube activities-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube captions-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube captions-download`** - Downloads a caption track.
- **`youtube-creator-pp-cli youtube captions-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube captions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube captions-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube channel-banners-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube channel-sections-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube channel-sections-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube channel-sections-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube channel-sections-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube channels-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube channels-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube comment-threads-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube comment-threads-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube comments-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube comments-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube comments-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube comments-mark-as-spam`** - Expresses the caller's opinion that one or more comments should be flagged as spam.
- **`youtube-creator-pp-cli youtube comments-set-moderation-status`** - Sets the moderation status of one or more comments.
- **`youtube-creator-pp-cli youtube comments-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube i18n-languages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube i18n-regions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube live-broadcasts-bind`** - Bind a broadcast to a stream.
- **`youtube-creator-pp-cli youtube live-broadcasts-delete`** - Delete a given broadcast.
- **`youtube-creator-pp-cli youtube live-broadcasts-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-creator-pp-cli youtube live-broadcasts-insert-cuepoint`** - Insert cuepoints in a broadcast
- **`youtube-creator-pp-cli youtube live-broadcasts-list`** - Retrieve the list of broadcasts associated with the given channel.
- **`youtube-creator-pp-cli youtube live-broadcasts-transition`** - Transition a broadcast to a given status.
- **`youtube-creator-pp-cli youtube live-broadcasts-update`** - Updates an existing broadcast for the authenticated user.
- **`youtube-creator-pp-cli youtube live-chat-bans-delete`** - Deletes a chat ban.
- **`youtube-creator-pp-cli youtube live-chat-bans-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube live-chat-messages-delete`** - Deletes a chat message.
- **`youtube-creator-pp-cli youtube live-chat-messages-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube live-chat-messages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube live-chat-moderators-delete`** - Deletes a chat moderator.
- **`youtube-creator-pp-cli youtube live-chat-moderators-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube live-chat-moderators-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube live-streams-delete`** - Deletes an existing stream for the authenticated user.
- **`youtube-creator-pp-cli youtube live-streams-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-creator-pp-cli youtube live-streams-list`** - Retrieve the list of streams associated with the given channel. --
- **`youtube-creator-pp-cli youtube live-streams-update`** - Updates an existing stream for the authenticated user.
- **`youtube-creator-pp-cli youtube members-list`** - Retrieves a list of members that match the request criteria for a channel.
- **`youtube-creator-pp-cli youtube memberships-levels-list`** - Retrieves a list of all pricing levels offered by a creator to the fans.
- **`youtube-creator-pp-cli youtube playlist-items-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube playlist-items-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube playlist-items-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube playlist-items-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube playlists-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube playlists-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube playlists-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube playlists-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube search-list`** - Retrieves a list of search resources
- **`youtube-creator-pp-cli youtube subscriptions-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube subscriptions-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube subscriptions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube super-chat-events-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube tests-insert`** - POST method.
- **`youtube-creator-pp-cli youtube third-party-links-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube third-party-links-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube third-party-links-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube third-party-links-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube thumbnails-set`** - As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which doesn't result in creation of a single resource), I use a custom verb here.
- **`youtube-creator-pp-cli youtube update-comment-threads`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube video-abuse-report-reasons-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube video-categories-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube videos-delete`** - Deletes a resource.
- **`youtube-creator-pp-cli youtube videos-get-rating`** - Retrieves the ratings that the authorized user gave to a list of specified videos.
- **`youtube-creator-pp-cli youtube videos-insert`** - Inserts a new resource into this collection.
- **`youtube-creator-pp-cli youtube videos-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-creator-pp-cli youtube videos-rate`** - Adds a like or dislike rating to a video or removes a rating from a video.
- **`youtube-creator-pp-cli youtube videos-report-abuse`** - Report abuse for a video.
- **`youtube-creator-pp-cli youtube videos-update`** - Updates an existing resource.
- **`youtube-creator-pp-cli youtube watermarks-set`** - Allows upload of watermark image and setting it for a channel.
- **`youtube-creator-pp-cli youtube watermarks-unset`** - Allows removal of channel watermark.

### youtube-reporting-jobs

Manage youtube reporting jobs

- **`youtube-creator-pp-cli youtube-reporting-jobs create`** - Creates a job and returns it.
- **`youtube-creator-pp-cli youtube-reporting-jobs delete`** - Deletes a job.
- **`youtube-creator-pp-cli youtube-reporting-jobs get`** - Gets a job.
- **`youtube-creator-pp-cli youtube-reporting-jobs list`** - Lists jobs.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
youtube-creator-pp-cli group-items list

# JSON for scripting and agents
youtube-creator-pp-cli group-items list --json

# Filter to specific fields
youtube-creator-pp-cli group-items list --json --select id,name,status

# Dry run — show the request without sending
youtube-creator-pp-cli group-items list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
youtube-creator-pp-cli group-items list --agent
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
youtube-creator-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/youtube-creator-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YOUTUBE_DATA_OAUTH2C` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `youtube-creator-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YOUTUBE_DATA_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 unauthorized after `auth login`** — Run `youtube-creator-pp-cli auth status` to see granted scopes; if the operation needs Analytics, re-run `auth login --scope yt-analytics.readonly`.
- **quotaExceeded error mid-run** — Run `youtube-creator-pp-cli quota meter` to see today's spend. Bulk ops should use `bulk metadata --title-contains` (and similar predicates) not `search.list` — the meter prints cost hints.
- **`members.list` returns 403** — members.list requires Google access approval. Run `doctor` to see status, then apply at the link printed.
- **PubSubHubbub callback never fires** — Run `pubsub subscribe --verify-only` to confirm the hub accepted your callback; the helper prints the GET challenge.
- **yt-dlp not found for `backup`** — `backup` shells out to yt-dlp. Install with `pip install -U yt-dlp` or set `--yt-dlp /path/to/binary`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ytstudio-cli**](https://github.com/jdwit/ytstudio-cli) — Java
- [**youtube-cli (danvega)**](https://github.com/danvega/youtube-cli) — Java
- [**youtube-cli (HRSPROJECT)**](https://github.com/HRSPROJECT/youtube-cli) — Python
- [**mcp-youtube (anaisbetts)**](https://github.com/anaisbetts/mcp-youtube) — TypeScript
- [**youtube-mcp-server (dannySubsense)**](https://github.com/dannySubsense/youtube-mcp-server) — Python
- [**youtube-comment-scraper-cli**](https://github.com/philbot9/youtube-comment-scraper-cli) — JavaScript
- [**youtube-comment-suite**](https://github.com/mattwright324/youtube-comment-suite) — Java

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
