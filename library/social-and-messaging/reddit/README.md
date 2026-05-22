# Reddit CLI

**Every Reddit feature absorbed across PRAW, snoowrap, and four MCP servers, plus offline FTS5 search, mod-queue triage primitives, and cross-sub aggregation no other tool has.**

A Go CLI for Reddit that absorbs every endpoint covered by PRAW, snoowrap, RedditWarp, and the major Reddit MCP servers. It adds an offline SQLite layer with FTS5 search (because Reddit native search misses ~50% of hits), mod-queue triage helpers no existing tool offers, plan-driven crosspost batching, and cross-subreddit user dossiers. All commands are agent-friendly with json, dry-run, select, and typed exit codes.

Printed by [@mcsyauqi](https://github.com/mcsyauqi) (Ahmad Thariq Syauqi).

## Install

The recommended path installs both the `reddit-pp-cli` binary and the `pp-reddit` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install reddit
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install reddit --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install reddit --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install reddit --agent claude-code
npx -y @mvanhorn/printing-press install reddit --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/reddit-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-reddit --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-reddit --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-reddit skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-reddit. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/reddit-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `REDDIT_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "reddit": {
      "command": "reddit-pp-mcp",
      "env": {
        "REDDIT_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Reddit OAuth2 has two flavors. Script apps (most common for personal use) need REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, REDDIT_USERNAME, and REDDIT_PASSWORD. Web/installed apps use REDDIT_REFRESH_TOKEN instead. Either way you also need REDDIT_USER_AGENT in the form `<product>/<version> by /u/<username>` — Reddit returns 429 for default Go/curl UAs. The CLI auto-targets `oauth.reddit.com` for authenticated calls and falls back to `old.reddit.com/*.json` for unauthenticated reads.

## Quick Start

```bash
# Verify OAuth, User-Agent, and reachability to oauth.reddit.com.
reddit-pp-cli doctor


# Browse a subreddit's hot listing with compact agent-friendly JSON.
reddit-pp-cli listings sub-hot programming --limit 25 --agent


# See how local FTS5 search works (run sync first to populate the corpus).
reddit-pp-cli me search --help


# Per-sub activity, karma, and recent posts for any user across N subs in one shot.
reddit-pp-cli dossier spez --in programming,golang --agent


# Surface the modqueue backlog the web UI can't sort by age.
reddit-pp-cli mod queue programming --sort age --older-than 24h --agent


# Multi-sub brand-mention watch with thread context and OP karma.
reddit-pp-cli watch "creativism" --in entrepreneur,smallbusiness --since 24h --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local FTS5 over your Reddit corpus
- **`me search`** — Search your own saved, submitted, upvoted, downvoted, and commented content with full-body FTS5 instead of Reddit's broken native search.

  _Agents researching what the user previously said or saved cannot rely on Reddit's search API — this is the only way to recover the full corpus._

  ```bash
  reddit-pp-cli me search "webhooks" --scope saved,comments --agent
  ```
- **`search-local`** — After syncing a subreddit, search its submissions and comments via FTS5 instead of Reddit's broken native search.

  _Researchers and brand-monitoring workflows need reliable text search; native search misses too much._

  ```bash
  reddit-pp-cli search-local "webhook auth" --sub programming --type comments --agent
  ```

### Mod-team primitives
- **`mod queue`** — Sort modqueue items by age and filter to ones sitting longer than a threshold. The web UI has no concept of item age.

  _Moderators triaging large subs need to find the >24h backlog without clicking through every item._

  ```bash
  reddit-pp-cli mod queue programming --sort age --older-than 24h --agent
  ```
- **`mod reporters`** — Per-reporter statistics over a rolling window: reports filed, removal-led %, approval %, no-action %. Identifies trusted reporters and karma-farmer-snitches.

  _Mod teams need to know which reporters are signal vs noise; this is the metric they actually want._

  ```bash
  reddit-pp-cli mod reporters programming --window 30d --min-reports 3 --agent
  ```
- **`mod ghost-actions`** — Detect actions where one mod approved an item and another later removed it (or vice versa). Surfaces silent disagreements in the mod team.

  _Mod teams need to find policy splits before they become public disputes._

  ```bash
  reddit-pp-cli mod ghost-actions programming --since 7d --agent
  ```
- **`mod remove-batch`** — CSV-driven mass removal with per-row removal-reason template, optional ban duration, and modmail note. Idempotent via plan-row hash.

  _Mod-queue clear-outs need batch operations; no existing tool batches with removal-reason templates._

  ```bash
  reddit-pp-cli mod remove-batch programming --plan ./removals.csv --dry-run
  ```

### Cross-entity aggregation
- **`dossier`** — Aggregate a user's submissions, comments, karma, and mod-actions-against across N subreddits. Answers 'is this a karma farmer or a real contributor?'

  _Vetting users (mod context) and academic research both need a per-sub activity breakdown that no single endpoint returns._

  ```bash
  reddit-pp-cli dossier spez --in programming,golang --agent
  ```
- **`watch`** — Fan-out search across multiple subreddits, dedupe by submission ID, enrich each hit with parent-comment context and OP karma-in-sub.

  _Brand-monitoring agents need thread context to decide whether a mention is worth replying to._

  ```bash
  reddit-pp-cli watch "creativism" --in entrepreneur,smallbusiness --since 24h --enrich-karma --agent
  ```

### Agent-native workflows
- **`crosspost-batch`** — YAML plan lists target subreddits with per-sub title overrides, flair-id, send-replies flag, NSFW flag, and OC flag. One command publishes the post to all targets.

  _Content creators publishing to N subs need per-sub customization in one orchestrated, idempotent call._

  ```bash
  reddit-pp-cli crosspost-batch t3_abc123 --plan ./targets.yaml --dry-run
  ```
- **`me posting-stats`** — Per-sub median and 75th-percentile score by day-of-week and hour-of-day, computed from your own submission history.

  _Content creators planning publish times need per-sub timing data based on their own track record._

  ```bash
  reddit-pp-cli me posting-stats --sub programming --by hour --agent
  ```
- **`post velocity`** — Comments-per-minute over the first 60 minutes since post creation, compared to the sub's median for the same window.

  _Content creators and brand monitors need to know when a thread is about to go viral._

  ```bash
  reddit-pp-cli post velocity t3_abc123 --baseline-sample 50 --agent
  ```
- **`inbox-digest`** — Group inbox items (replies, mentions, modmail) by source-sub and enrich each with the current parent-thread score. Replaces the flat inbox UI.

  _Moderators and active users with high inbox volume need grouped triage, not a flat list._

  ```bash
  reddit-pp-cli inbox-digest --window 24h --agent
  ```

## Usage

Run `reddit-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Authenticated user account info, karma, prefs, friends, blocked, trophies

- **`reddit-pp-cli account blocked`** - List users blocked by the authenticated user
- **`reddit-pp-cli account friends`** - List users friended by the authenticated user
- **`reddit-pp-cli account karma`** - Get per-subreddit karma breakdown for the authenticated user
- **`reddit-pp-cli account me`** - Get the authenticated user's profile
- **`reddit-pp-cli account prefs`** - Get the authenticated user's preferences
- **`reddit-pp-cli account trophies`** - Get trophies for the authenticated user

### flair

Flair templates and assignment

- **`reddit-pp-cli flair link-templates`** - List link flair templates for a subreddit
- **`reddit-pp-cli flair select-link`** - Set a link's flair using a template
- **`reddit-pp-cli flair set-user`** - Set a user's flair in a subreddit (mod-only)
- **`reddit-pp-cli flair user-templates`** - List user flair templates for a subreddit

### friends

Friend / unfriend / block / unblock users

- **`reddit-pp-cli friends add`** - Add a Redditor as a friend
- **`reddit-pp-cli friends remove`** - Remove a Redditor from friends

### inbox

Inbox, unread, mentions, sent messages, modmail

- **`reddit-pp-cli inbox all`** - Get inbox items (comments, replies, mentions, messages)
- **`reddit-pp-cli inbox compose`** - Send a private message to a user
- **`reddit-pp-cli inbox mark-all-read`** - Mark all inbox items as read
- **`reddit-pp-cli inbox mark-read`** - Mark inbox items as read
- **`reddit-pp-cli inbox mark-unread`** - Mark inbox items as unread
- **`reddit-pp-cli inbox mentions`** - Get username mentions in comments
- **`reddit-pp-cli inbox messages`** - Get private messages (excluding comment replies)
- **`reddit-pp-cli inbox sent`** - Get messages you have sent
- **`reddit-pp-cli inbox unread`** - Get unread inbox items

### listings

Reddit listings: frontpage, subreddit feeds, sort variants

- **`reddit-pp-cli listings frontpage-hot`** - Get hot posts from the front page
- **`reddit-pp-cli listings frontpage-new`** - Get newest posts from the front page
- **`reddit-pp-cli listings frontpage-rising`** - Get rising posts from the front page
- **`reddit-pp-cli listings frontpage-top`** - Get top posts from the front page
- **`reddit-pp-cli listings sub-controversial`** - Get controversial posts from a specific subreddit
- **`reddit-pp-cli listings sub-hot`** - Get hot posts from a specific subreddit
- **`reddit-pp-cli listings sub-new`** - Get newest posts from a specific subreddit
- **`reddit-pp-cli listings sub-rising`** - Get rising posts from a specific subreddit
- **`reddit-pp-cli listings sub-top`** - Get top posts from a specific subreddit

### me_listings

Personal history: saved, upvoted, downvoted, hidden

- **`reddit-pp-cli me_listings downvoted`** - Get items downvoted by you (own account only)
- **`reddit-pp-cli me_listings hidden`** - Get items hidden by you
- **`reddit-pp-cli me_listings saved`** - Get items saved by a user (yourself unless mod)
- **`reddit-pp-cli me_listings upvoted`** - Get items upvoted by you (own account only)

### moderation

Moderation: modqueue, reports, spam, edited, approve, remove, distinguish, ban, mute, modlog

- **`reddit-pp-cli moderation approve`** - Approve a submission or comment
- **`reddit-pp-cli moderation ban`** - Ban a user from a subreddit
- **`reddit-pp-cli moderation banned`** - List banned users in a subreddit
- **`reddit-pp-cli moderation distinguish`** - Distinguish a comment or post as mod/admin (with optional sticky for comments)
- **`reddit-pp-cli moderation edited`** - Get edited items in a subreddit
- **`reddit-pp-cli moderation lock`** - Lock a submission or comment (no new replies)
- **`reddit-pp-cli moderation modlog`** - Get the moderation log for a subreddit
- **`reddit-pp-cli moderation modqueue`** - Get items in the modqueue for a subreddit
- **`reddit-pp-cli moderation remove`** - Remove a submission or comment (mark spam=true to send to spam)
- **`reddit-pp-cli moderation reports`** - Get reported items in a subreddit
- **`reddit-pp-cli moderation spam`** - Get spam-filtered items in a subreddit
- **`reddit-pp-cli moderation sticky`** - Sticky a submission to the top of a subreddit
- **`reddit-pp-cli moderation unban`** - Unban a user from a subreddit
- **`reddit-pp-cli moderation unlock`** - Unlock a previously locked submission or comment

### multi

Multireddits: list, get, create, update, delete

- **`reddit-pp-cli multi create`** - Create a new multireddit
- **`reddit-pp-cli multi delete`** - Delete a multireddit
- **`reddit-pp-cli multi get`** - Get a specific multireddit
- **`reddit-pp-cli multi mine`** - List your multireddits

### query

Search submissions, comments, users, subreddits (cross-Reddit and per-subreddit)

- **`reddit-pp-cli query reddit`** - Search across all of Reddit
- **`reddit-pp-cli query subreddit`** - Search within a specific subreddit

### submissions

Get, submit, edit, delete posts; vote, save, hide

- **`reddit-pp-cli submissions delete`** - Delete your own post or comment
- **`reddit-pp-cli submissions edit`** - Edit your own post or comment body (self/comment text only)
- **`reddit-pp-cli submissions get`** - Get a submission by ID, with its comments tree
- **`reddit-pp-cli submissions hide`** - Hide a submission from your listings
- **`reddit-pp-cli submissions morechildren`** - Expand a MoreComments placeholder into actual comments
- **`reddit-pp-cli submissions reply`** - Reply to a submission or comment
- **`reddit-pp-cli submissions save`** - Save a submission or comment
- **`reddit-pp-cli submissions submit`** - Submit a new post (link or self-text)
- **`reddit-pp-cli submissions unhide`** - Unhide a previously hidden submission
- **`reddit-pp-cli submissions unsave`** - Unsave a previously saved submission or comment
- **`reddit-pp-cli submissions vote`** - Cast or clear a vote on a submission or comment

### subreddit

Subreddit info, rules, traffic, sticky posts

- **`reddit-pp-cli subreddit about`** - Get a subreddit's metadata
- **`reddit-pp-cli subreddit moderators`** - List a subreddit's moderators
- **`reddit-pp-cli subreddit rules`** - Get a subreddit's rules
- **`reddit-pp-cli subreddit subscribe`** - Subscribe to a subreddit
- **`reddit-pp-cli subreddit traffic`** - Get traffic stats for a subreddit (mod only)

### subreddits

Discover, search, and list subreddits

- **`reddit-pp-cli subreddits default`** - List Reddit's default subreddits
- **`reddit-pp-cli subreddits mine`** - List subreddits the authenticated user is subscribed to
- **`reddit-pp-cli subreddits new`** - List recently created subreddits
- **`reddit-pp-cli subreddits popular`** - List popular subreddits
- **`reddit-pp-cli subreddits search`** - Search subreddits by name and description

### user

User profile, submissions, comments, history

- **`reddit-pp-cli user about`** - Get a Redditor's public profile
- **`reddit-pp-cli user comments`** - Get a user's comment history
- **`reddit-pp-cli user gilded`** - Get a user's gilded posts and comments
- **`reddit-pp-cli user submitted`** - Get a user's submitted posts
- **`reddit-pp-cli user trophies`** - Get a user's trophies

### wiki

Wiki pages: get, list, edit, revisions

- **`reddit-pp-cli wiki edit`** - Edit a wiki page
- **`reddit-pp-cli wiki page`** - Get a wiki page
- **`reddit-pp-cli wiki pages`** - List all wiki pages in a subreddit
- **`reddit-pp-cli wiki revisions`** - Get the revision history of a wiki page


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
reddit-pp-cli multi get mock-value mock-value

# JSON for scripting and agents
reddit-pp-cli multi get mock-value mock-value --json

# Filter to specific fields
reddit-pp-cli multi get mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
reddit-pp-cli multi get mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
reddit-pp-cli multi get mock-value mock-value --agent
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
reddit-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/reddit-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `REDDIT_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |
| `REDDIT_CLIENT_ID` | per_call | Yes | Set to your API credential. |
| `REDDIT_CLIENT_SECRET` | per_call | Yes | Set to your API credential. |
| `REDDIT_USERNAME` | per_call | Yes | Set to your API credential. |
| `REDDIT_PASSWORD` | per_call | Yes | Set to your API credential. |
| `REDDIT_REFRESH_TOKEN` | per_call | Yes | Set to your API credential. |
| `REDDIT_USER_AGENT` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `reddit-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $REDDIT_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 429 on every call** — Set a Reddit-compliant User-Agent: `export REDDIT_USER_AGENT="reddit-pp-cli/0.1 by /u/<your-username>"`. Default Go/curl UAs are rate-limited aggressively.
- **HTTP 401 'invalid_grant' on auth login** — Script app username/password is wrong, or 2FA is enabled. Use the refresh-token flow instead: set REDDIT_REFRESH_TOKEN and `auth refresh`.
- **www.reddit.com unreachable but oauth.reddit.com works** — Some ISPs (e.g., Biznet 'SafeSurf') DNS-block www.reddit.com but leave oauth.reddit.com and old.reddit.com alone. The CLI defaults to oauth.reddit.com — no action needed.
- **Rate limit exhausted mid-sync** — Reddit free tier is 100 QPM per OAuth client_id. Resume with `sync sub <name> --resume`; the cursor is persisted in SQLite.
- **Comments tree missing nested replies** — Reddit returns `MoreComments` placeholders for deep threads. Use `post comments <id> --expand-all` to recursively resolve them (32 IDs per `morechildren` call).

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**praw-dev/praw**](https://github.com/praw-dev/praw) — Python (3500 stars)
- [**not-an-aardvark/snoowrap**](https://github.com/not-an-aardvark/snoowrap) — JavaScript (1100 stars)
- [**adhikasp/mcp-reddit**](https://github.com/adhikasp/mcp-reddit) — Python (401 stars)
- [**vartanbeno/go-reddit**](https://github.com/vartanbeno/go-reddit) — Go (331 stars)
- [**turnage/graw**](https://github.com/turnage/graw) — Go (300 stars)
- [**Arindam200/reddit-mcp**](https://github.com/Arindam200/reddit-mcp) — Python (286 stars)
- [**Hawstein/mcp-server-reddit**](https://github.com/Hawstein/mcp-server-reddit) — Python (177 stars)
- [**jordanburke/reddit-mcp-server**](https://github.com/jordanburke/reddit-mcp-server) — TypeScript (100 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
