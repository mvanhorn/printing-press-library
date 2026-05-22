---
name: pp-reddit
description: "Every Reddit feature absorbed across PRAW, snoowrap, and four MCP servers, plus offline FTS5 search Trigger phrases: `search reddit`, `post to reddit`, `reddit modqueue`, `vote on reddit`, `subreddit activity`, `reddit user history`, `crosspost to reddit`, `use reddit-pp-cli`, `run reddit-pp-cli`."
author: "Ahmad Thariq Syauqi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - reddit-pp-cli
---

# Reddit — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `reddit-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install reddit --cli-only
   ```
2. Verify: `reddit-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/cmd/reddit-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A Go CLI for Reddit that absorbs every endpoint covered by PRAW, snoowrap, RedditWarp, and the major Reddit MCP servers. It adds an offline SQLite layer with FTS5 search (because Reddit native search misses ~50% of hits), mod-queue triage helpers no existing tool offers, plan-driven crosspost batching, and cross-subreddit user dossiers. All commands are agent-friendly with json, dry-run, select, and typed exit codes.

## When to Use This CLI

Reach for reddit-pp-cli when you need to query Reddit from a script, agent, or terminal session and want the answers to be honest. The CLI's local FTS5 layer is the killer feature for own-history search and tracked-sub research. Mod teams should reach for it whenever the web UI's lack of age-sortable modqueue or reporter-reputation views becomes the friction. Content creators should use it for crosspost batches and posting-time analytics.

## Unique Capabilities

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

## Command Reference

**account** — Authenticated user account info, karma, prefs, friends, blocked, trophies

- `reddit-pp-cli account blocked` — List users blocked by the authenticated user
- `reddit-pp-cli account friends` — List users friended by the authenticated user
- `reddit-pp-cli account karma` — Get per-subreddit karma breakdown for the authenticated user
- `reddit-pp-cli account me` — Get the authenticated user's profile
- `reddit-pp-cli account prefs` — Get the authenticated user's preferences
- `reddit-pp-cli account trophies` — Get trophies for the authenticated user

**flair** — Flair templates and assignment

- `reddit-pp-cli flair link-templates` — List link flair templates for a subreddit
- `reddit-pp-cli flair select-link` — Set a link's flair using a template
- `reddit-pp-cli flair set-user` — Set a user's flair in a subreddit (mod-only)
- `reddit-pp-cli flair user-templates` — List user flair templates for a subreddit

**friends** — Friend / unfriend / block / unblock users

- `reddit-pp-cli friends add` — Add a Redditor as a friend
- `reddit-pp-cli friends remove` — Remove a Redditor from friends

**inbox** — Inbox, unread, mentions, sent messages, modmail

- `reddit-pp-cli inbox all` — Get inbox items (comments, replies, mentions, messages)
- `reddit-pp-cli inbox compose` — Send a private message to a user
- `reddit-pp-cli inbox mark-all-read` — Mark all inbox items as read
- `reddit-pp-cli inbox mark-read` — Mark inbox items as read
- `reddit-pp-cli inbox mark-unread` — Mark inbox items as unread
- `reddit-pp-cli inbox mentions` — Get username mentions in comments
- `reddit-pp-cli inbox messages` — Get private messages (excluding comment replies)
- `reddit-pp-cli inbox sent` — Get messages you have sent
- `reddit-pp-cli inbox unread` — Get unread inbox items

**listings** — Reddit listings: frontpage, subreddit feeds, sort variants

- `reddit-pp-cli listings frontpage-hot` — Get hot posts from the front page
- `reddit-pp-cli listings frontpage-new` — Get newest posts from the front page
- `reddit-pp-cli listings frontpage-rising` — Get rising posts from the front page
- `reddit-pp-cli listings frontpage-top` — Get top posts from the front page
- `reddit-pp-cli listings sub-controversial` — Get controversial posts from a specific subreddit
- `reddit-pp-cli listings sub-hot` — Get hot posts from a specific subreddit
- `reddit-pp-cli listings sub-new` — Get newest posts from a specific subreddit
- `reddit-pp-cli listings sub-rising` — Get rising posts from a specific subreddit
- `reddit-pp-cli listings sub-top` — Get top posts from a specific subreddit

**me_listings** — Personal history: saved, upvoted, downvoted, hidden

- `reddit-pp-cli me_listings downvoted` — Get items downvoted by you (own account only)
- `reddit-pp-cli me_listings hidden` — Get items hidden by you
- `reddit-pp-cli me_listings saved` — Get items saved by a user (yourself unless mod)
- `reddit-pp-cli me_listings upvoted` — Get items upvoted by you (own account only)

**moderation** — Moderation: modqueue, reports, spam, edited, approve, remove, distinguish, ban, mute, modlog

- `reddit-pp-cli moderation approve` — Approve a submission or comment
- `reddit-pp-cli moderation ban` — Ban a user from a subreddit
- `reddit-pp-cli moderation banned` — List banned users in a subreddit
- `reddit-pp-cli moderation distinguish` — Distinguish a comment or post as mod/admin (with optional sticky for comments)
- `reddit-pp-cli moderation edited` — Get edited items in a subreddit
- `reddit-pp-cli moderation lock` — Lock a submission or comment (no new replies)
- `reddit-pp-cli moderation modlog` — Get the moderation log for a subreddit
- `reddit-pp-cli moderation modqueue` — Get items in the modqueue for a subreddit
- `reddit-pp-cli moderation remove` — Remove a submission or comment (mark spam=true to send to spam)
- `reddit-pp-cli moderation reports` — Get reported items in a subreddit
- `reddit-pp-cli moderation spam` — Get spam-filtered items in a subreddit
- `reddit-pp-cli moderation sticky` — Sticky a submission to the top of a subreddit
- `reddit-pp-cli moderation unban` — Unban a user from a subreddit
- `reddit-pp-cli moderation unlock` — Unlock a previously locked submission or comment

**multi** — Multireddits: list, get, create, update, delete

- `reddit-pp-cli multi create` — Create a new multireddit
- `reddit-pp-cli multi delete` — Delete a multireddit
- `reddit-pp-cli multi get` — Get a specific multireddit
- `reddit-pp-cli multi mine` — List your multireddits

**query** — Search submissions, comments, users, subreddits (cross-Reddit and per-subreddit)

- `reddit-pp-cli query reddit` — Search across all of Reddit
- `reddit-pp-cli query subreddit` — Search within a specific subreddit

**submissions** — Get, submit, edit, delete posts; vote, save, hide

- `reddit-pp-cli submissions delete` — Delete your own post or comment
- `reddit-pp-cli submissions edit` — Edit your own post or comment body (self/comment text only)
- `reddit-pp-cli submissions get` — Get a submission by ID, with its comments tree
- `reddit-pp-cli submissions hide` — Hide a submission from your listings
- `reddit-pp-cli submissions morechildren` — Expand a MoreComments placeholder into actual comments
- `reddit-pp-cli submissions reply` — Reply to a submission or comment
- `reddit-pp-cli submissions save` — Save a submission or comment
- `reddit-pp-cli submissions submit` — Submit a new post (link or self-text)
- `reddit-pp-cli submissions unhide` — Unhide a previously hidden submission
- `reddit-pp-cli submissions unsave` — Unsave a previously saved submission or comment
- `reddit-pp-cli submissions vote` — Cast or clear a vote on a submission or comment

**subreddit** — Subreddit info, rules, traffic, sticky posts

- `reddit-pp-cli subreddit about` — Get a subreddit's metadata
- `reddit-pp-cli subreddit moderators` — List a subreddit's moderators
- `reddit-pp-cli subreddit rules` — Get a subreddit's rules
- `reddit-pp-cli subreddit subscribe` — Subscribe to a subreddit
- `reddit-pp-cli subreddit traffic` — Get traffic stats for a subreddit (mod only)

**subreddits** — Discover, search, and list subreddits

- `reddit-pp-cli subreddits default` — List Reddit's default subreddits
- `reddit-pp-cli subreddits mine` — List subreddits the authenticated user is subscribed to
- `reddit-pp-cli subreddits new` — List recently created subreddits
- `reddit-pp-cli subreddits popular` — List popular subreddits
- `reddit-pp-cli subreddits search` — Search subreddits by name and description

**user** — User profile, submissions, comments, history

- `reddit-pp-cli user about` — Get a Redditor's public profile
- `reddit-pp-cli user comments` — Get a user's comment history
- `reddit-pp-cli user gilded` — Get a user's gilded posts and comments
- `reddit-pp-cli user submitted` — Get a user's submitted posts
- `reddit-pp-cli user trophies` — Get a user's trophies

**wiki** — Wiki pages: get, list, edit, revisions

- `reddit-pp-cli wiki edit` — Edit a wiki page
- `reddit-pp-cli wiki page` — Get a wiki page
- `reddit-pp-cli wiki pages` — List all wiki pages in a subreddit
- `reddit-pp-cli wiki revisions` — Get the revision history of a wiki page


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
reddit-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Triage modqueue with reporter context

```bash
reddit-pp-cli mod reporters programming --window 30d --min-reports 3 --agent --select reporter,removed_pct,filed_count
```

Top reporters by removal-rate over the last 30 days; agents can identify trusted vs spam-reporting accounts.

### Crosspost a single article to multiple subs with overrides

```bash
reddit-pp-cli crosspost-batch --help
```

See the YAML plan structure: per-sub title/flair/send-replies/NSFW overrides. Default --dry-run prints the plan; add --confirm to execute.

### Local search of own saved posts

```bash
reddit-pp-cli me search "webhook auth" --scope saved,comments --agent
```

FTS5 over your synced own-history. Reddit native search misses ~50% of own-history hits.

### Vet a user before granting mod permissions

```bash
reddit-pp-cli dossier candidate-username --in mysub,relatedsub --agent --select per_sub,recent_top_posts
```

Aggregates submissions, comments, and karma across the relevant subs so mod teams can see a candidate whole footprint.

### Find every brand mention in target subs with thread context

```bash
reddit-pp-cli watch "creativism" --in entrepreneur,smallbusiness,marketing --since 24h --enrich-karma --agent --select sub,title,context,op_karma_in_sub
```

One command fans out across 3 subs, dedupes, and enriches with the parent thread excerpt plus OP karma-in-sub.

## Auth Setup

Reddit OAuth2 has two flavors. Script apps (most common for personal use) need REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, REDDIT_USERNAME, and REDDIT_PASSWORD. Web/installed apps use REDDIT_REFRESH_TOKEN instead. Either way you also need REDDIT_USER_AGENT in the form `<product>/<version> by /u/<username>` — Reddit returns 429 for default Go/curl UAs. The CLI auto-targets `oauth.reddit.com` for authenticated calls and falls back to `old.reddit.com/*.json` for unauthenticated reads.

Run `reddit-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  reddit-pp-cli multi get mock-value mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
reddit-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
reddit-pp-cli feedback --stdin < notes.txt
reddit-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.reddit-pp-cli/feedback.jsonl`. They are never POSTed unless `REDDIT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `REDDIT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
reddit-pp-cli profile save briefing --json
reddit-pp-cli --profile briefing multi get mock-value mock-value
reddit-pp-cli profile list --json
reddit-pp-cli profile show briefing
reddit-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `reddit-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add reddit-pp-mcp -- reddit-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which reddit-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   reddit-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `reddit-pp-cli <command> --help`.
