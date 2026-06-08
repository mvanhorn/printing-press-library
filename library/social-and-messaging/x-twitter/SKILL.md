---
name: pp-x-twitter
description: "Offline-searchable X/Twitter CLI and MCP surface for archiving posts, resolving links, monitoring mentions, composing threads, publishing Articles, and searching synced bookmarks. Trigger phrases: `search X for`, `archive tweets about`, `show me the X thread for`, `monitor my X mentions`, `post a thread to X`, `use x-twitter`, `run x-twitter-pp-cli`."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - x-twitter-pp-cli
    install:
      - kind: go
        bins: [x-twitter-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-cli
---

# X (Twitter) — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `x-twitter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install x-twitter --cli-only
   ```
2. Verify: `x-twitter-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Mirrors the official X v2 API and adds what no other X tool has: a local SQLite store you can sync once and query many times with FTS5 search and group-by analytics, agent-native --json/--select output, and an MCP server that exposes the whole surface to AI agents through token-efficient orchestration plus named multi-step intents. Start pasted-link workflows with `post resolve`, use `thread context` when a post needs parent/quote/reply context, save durable source material with `collection save/list/export`, track ongoing searches with `monitor create/run/list`, package saved activity with `brief`, snapshot accounts with `account snapshot`, find launch or repo links with `url mentions`, track post metrics with `performance snapshot/backfill/analyze`, export account/query timelines with `timeline export`, reconstruct synced conversation threads offline with `thread show`, compose self-reply threads from markdown with `thread compose`, author long-form X Articles from markdown with `articles-publish-md`, and rescue your bookmark graveyard with `users bookmarks find` — keyword and author search over your synced bookmarks, which X itself gives you no way to search.

## When to Use This CLI

Reach for this CLI when a task involves reading, searching, or archiving X (Twitter) data and you want the results queryable offline rather than re-fetched each time — building a searchable corpus of posts, reconstructing a conversation thread, snapshotting a user's recent posts with engagement, or monitoring mentions incrementally. It is also the right choice when an AI agent needs an X surface with read-only/destructive safety hints and named multi-step intents rather than a pile of raw endpoint calls. Prefer it over raw API calls whenever the same data will be queried more than once, since the local store avoids re-spending per-read credits.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`post resolve`** — Normalize any X post URL or raw post ID into a canonical structured record. It prefers local data in auto mode, falls back to public v2 reads when needed, includes provenance (`live`, `local`, or `mixed`), and emits suggested next workflow commands.

  _Use this as the first command when an agent starts from a pasted X link. It avoids brittle URL parsing and returns stable fields for downstream workflows._

  ```bash
  x-twitter-pp-cli post resolve https://x.com/user/status/123 --agent
  x-twitter-pp-cli post resolve 123 --include author,media,links,refs,metrics --agent
  ```
- **`thread context`** — Resolve a post URL or ID, include parent and quoted posts when available, and optionally include bounded replies from the local store and/or recent search.

  _Use this before summarizing or drafting around a post. `thread show` is still the pure offline conversation reconstruction command; `thread context` is the URL-first workflow that can mix local and live data._

  ```bash
  x-twitter-pp-cli thread context https://x.com/user/status/123 --agent
  x-twitter-pp-cli thread context 123 --replies --depth 3 --limit 100 --agent
  ```
- **`collection save/list/export`** — Save resolved X posts into durable named local collections, then list or export them as markdown, JSON, JSONL, or CSV. This never writes to X; it only writes the local SQLite store.

  _Use collections for research libraries, launch mentions, examples, recruiting/source material, and anything an agent should reuse offline after the first API read._

  ```bash
  x-twitter-pp-cli collection save https://x.com/user/status/123 --collection ai-agents --note "Good framing" --agent
  x-twitter-pp-cli collection list ai-agents --agent
  x-twitter-pp-cli collection export ai-agents --format markdown
  ```
- **`monitor create/run/list`** — Create named query, URL, or account monitors, then run them repeatedly with local watermarks and dedupe. `monitor create` and `monitor run` write only local SQLite state; they never post, like, reply, follow, or mutate X.

  _Use monitors for launch mentions, product/customer feedback, account tracking, and recurring agent jobs. `--since last` uses the saved watermark; `--preview` fetches without updating local state._

  ```bash
  x-twitter-pp-cli monitor create ai-labs --query 'from:openai OR from:anthropic' --agent
  x-twitter-pp-cli monitor create product-mentions --url https://example.com --agent
  x-twitter-pp-cli monitor run ai-labs --since last --agent
  x-twitter-pp-cli monitor list --agent
  ```
- **`brief`** — Build a deterministic source-backed JSON or markdown brief from monitor results, collections, or explicit post IDs. It packages links, authors, text, and available metrics; it does not invent conclusions or run LLM summarization.

  _Use this when an agent needs a daily/weekly X activity packet or a collection summary that can be pasted into notes, docs, or a CRM._

  ```bash
  x-twitter-pp-cli brief --monitor ai-labs --since 24h --agent
  x-twitter-pp-cli brief --collection launch-feedback --format markdown
  ```
- **`account snapshot`** — Capture profile basics, public metrics, pinned post, and recent posts for a username or user ID. It is read-only toward X and uses local data first unless `--live` or `--data-source live` is set.

  ```bash
  x-twitter-pp-cli account snapshot @username --recent 20 --agent
  x-twitter-pp-cli account snapshot 12345 --include recent,profile,metrics,pinned --format markdown
  ```
- **`url mentions`** — Search for recent posts mentioning a URL, domain, repo, article, or product page. It can optionally save results into a local collection or create/update a local monitor for future runs.

  ```bash
  x-twitter-pp-cli url mentions https://example.com --since 7d --agent
  x-twitter-pp-cli url mentions github.com/org/repo --collection launch-feedback --monitor repo-links --agent
  ```
- **`performance snapshot/backfill/analyze`** — Store timestamped post metrics locally, backfill recent account posts when auth allows, and analyze saved snapshots by type, hour, media, link presence, or label. Missing metrics stay nullable/absent; the CLI does not treat unavailable fields as zero. Snapshot output includes `public_metrics`, available non-public/organic metrics, `metric_source`, and `metric_availability`.

  ```bash
  x-twitter-pp-cli performance snapshot --ids 123,456 --label 24h --agent
  x-twitter-pp-cli performance backfill --mine --days 90 --agent
  x-twitter-pp-cli performance analyze --since 90d --group-by type,hour,has_media,has_link --agent
  ```
- **`timeline export`** — Export an account or query timeline as markdown, JSON, or JSONL. Account exports use local synced tweets when available and fetch live only when needed or requested.

  ```bash
  x-twitter-pp-cli timeline export @username --since 30d --format markdown
  x-twitter-pp-cli timeline export --query 'ai agents' --since 7d --format jsonl
  ```
- **`thread show`** — Rebuild a full conversation thread from your locally synced posts — ordered and depth-tagged — without re-spending API read credits.

  _When an agent needs the shape of a discussion (who replied to whom, in order), reach for this instead of paginating the search API and re-assembling the tree by hand._

  ```bash
  x-twitter-pp-cli thread show 1750000000000000000 --agent
  ```
- **`users bookmarks find`** — Search your synced bookmarks by keyword and/or author, offline. X has no bookmark search and its API exposes no bookmark timestamp, so bookmarks pile up unread; this rebuilds the missing retrieval layer from the local store. Read-only and free to re-run once synced — the X read credit is spent once at sync time, not per query.

  _The "I bookmarked something about this and can't find it" rescue. Sync bookmarks once, then let an agent retrieve and act on them (summarize, draft a thread, cluster) — the offline store is the agent's working set, not a per-query API spend._

  ```bash
  # one-time populate (personal read — needs X_OAUTH2_USER_TOKEN); add author field + users for --from
  x-twitter-pp-cli sync --resources bookmarks --param tweet.fields=author_id,created_at
  x-twitter-pp-cli sync --resources users

  x-twitter-pp-cli users bookmarks find "rust async" --agent
  x-twitter-pp-cli users bookmarks find "llm" --from @karpathy --limit 20 --agent
  ```

### Authoring workflows
- **`thread compose`** — Split a markdown file into a numbered, 280-char-packed self-reply thread; prints by default and only posts with --post.

  _Compose a thread from a document deterministically; the dry-run default lets an agent preview the exact tweets before any write._

  ```bash
  x-twitter-pp-cli thread compose ./update.md
  ```
- **`articles-publish-md`** — Parse a markdown file with YAML frontmatter into the Draft.js content_state JSON X's Articles editor accepts; previews by default; --draft saves a draft, --post publishes publicly.

  _The only programmatic way to author a long-form X Article from a document; preview and --draft keep it private until you explicitly --post._

  ```bash
  x-twitter-pp-cli articles-publish-md ./post.md
  ```

## Command Reference

**account-activity** — Endpoints relating to retrieving, managing AAA subscriptions

- `x-twitter-pp-cli account-activity create-subscription` — Creates an Account Activity subscription for the user and the given webhook.
- `x-twitter-pp-cli account-activity delete-subscription` — Deletes an Account Activity subscription for the given webhook and user ID.
- `x-twitter-pp-cli account-activity get-subscription-count` — Retrieves a count of currently active Account Activity subscriptions.
- `x-twitter-pp-cli account-activity get-subscriptions` — Retrieves a list of all active subscriptions for a given webhook.
- `x-twitter-pp-cli account-activity validate-subscription` — Checks a user’s Account Activity subscription for a given webhook.

**activity** — Manage activity

- `x-twitter-pp-cli activity create-subscription` — Creates a subscription for an X activity event
- `x-twitter-pp-cli activity delete-subscription` — Deletes a subscription for an X activity event
- `x-twitter-pp-cli activity delete-subscriptions-by-ids` — Deletes multiple subscriptions for X activity events by their IDs
- `x-twitter-pp-cli activity get-subscriptions` — Get a list of active subscriptions for XAA
- `x-twitter-pp-cli activity update-subscription` — Updates a subscription for an X activity event

**chat** — Manage chat

- `x-twitter-pp-cli chat add-group-members` — Adds one or more members to an existing encrypted Chat group conversation, rotating the conversation key.
- `x-twitter-pp-cli chat create-conversation` — Creates a new encrypted Chat group conversation on behalf of the authenticated user.
- `x-twitter-pp-cli chat get-conversation` — Returns metadata for a Chat conversation including type, muted status, and group details. Use chat_conversation.
- `x-twitter-pp-cli chat get-conversation-events` — Retrieves messages and key change events for a specific Chat conversation with pagination support.
- `x-twitter-pp-cli chat get-conversations` — Retrieves a list of Chat conversations for the authenticated user's inbox.
- `x-twitter-pp-cli chat initialize-conversation-keys` — Initializes encryption keys for a Chat conversation.
- `x-twitter-pp-cli chat initialize-group` — Initializes a new XChat group conversation and returns a unique conversation ID.
- `x-twitter-pp-cli chat mark-conversation-read` — Marks a specific Chat conversation as read on behalf of the authenticated user.
- `x-twitter-pp-cli chat media-download` — Downloads encrypted media bytes from an XChat conversation. The response body contains raw binary bytes.
- `x-twitter-pp-cli chat media-upload-append` — Appends media data to an XChat upload session.
- `x-twitter-pp-cli chat media-upload-finalize` — Finalizes an XChat media upload session.
- `x-twitter-pp-cli chat media-upload-initialize` — Initializes an XChat media upload session.
- `x-twitter-pp-cli chat send-message` — Sends an encrypted message to a specific Chat conversation.
- `x-twitter-pp-cli chat send-typing-indicator` — Sends a typing indicator to a specific Chat conversation on behalf of the authenticated user.

**communities** — Manage communities

- `x-twitter-pp-cli communities get-by-id` — Retrieves details of a specific Community by its ID.
- `x-twitter-pp-cli communities search` — Retrieves a list of Communities matching the specified search query.

**compliance** — Endpoints related to keeping X data in your systems compliant

- `x-twitter-pp-cli compliance create-jobs` — Creates a new Compliance Job for the specified job type.
- `x-twitter-pp-cli compliance get-jobs` — Retrieves a list of Compliance Jobs filtered by job type and optional status.
- `x-twitter-pp-cli compliance get-jobs-by-id` — Retrieves details of a specific Compliance Job by its ID.

**connections** — Endpoints related to streaming connections

- `x-twitter-pp-cli connections delete-all` — Terminates all active streaming connections for the authenticated application.
- `x-twitter-pp-cli connections delete-by-endpoint` — Terminates all streaming connections for a specific endpoint ID for the authenticated application.
- `x-twitter-pp-cli connections delete-by-uuids` — Terminates multiple streaming connections by their UUIDs for the authenticated application.
- `x-twitter-pp-cli connections get-history` — Returns active and historical streaming connections with disconnect reasons for the authenticated application.

**dm-conversations** — Manage dm conversations

- `x-twitter-pp-cli dm-conversations create-direct-messages-by-participant-id` — Sends a new direct message to a specific participant by their ID.
- `x-twitter-pp-cli dm-conversations create-direct-messages-conversation` — Initiates a new direct message conversation with specified participants.
- `x-twitter-pp-cli dm-conversations get-direct-messages-events-by-participant-id` — Retrieves direct message events for a specific conversation.
- `x-twitter-pp-cli dm-conversations media-download` — Downloads media attached to a legacy Direct Message.

**dm-events** — Manage dm events

- `x-twitter-pp-cli dm-events delete-direct-messages-events` — Deletes a specific direct message event by its ID, if owned by the authenticated user.
- `x-twitter-pp-cli dm-events get-direct-messages-events` — Retrieves a list of recent direct message events across all conversations.
- `x-twitter-pp-cli dm-events get-direct-messages-events-by-id` — Retrieves details of a specific direct message event by its ID.

**evaluate-note** — Manage evaluate note

- `x-twitter-pp-cli evaluate-note` — Endpoint to evaluate a community note.

**insights** — Manage insights

- `x-twitter-pp-cli insights get-historical` — Retrieves historical engagement metrics for specified Posts within a defined time range.
- `x-twitter-pp-cli insights get-insights28-hr` — Retrieves engagement metrics for specified Posts over the last 28 hours.

**lists** — Endpoints related to retrieving, managing Lists

- `x-twitter-pp-cli lists create` — Creates a new List for the authenticated user.
- `x-twitter-pp-cli lists delete` — Deletes a specific List owned by the authenticated user by its ID.
- `x-twitter-pp-cli lists get-by-id` — Retrieves details of a specific List by its ID.
- `x-twitter-pp-cli lists update` — Updates the details of a specific List owned by the authenticated user by its ID.

**media** — Endpoints related to Media

- `x-twitter-pp-cli media append-upload` — Appends data to a Media upload request.
- `x-twitter-pp-cli media create-metadata` — Creates metadata for a Media file.
- `x-twitter-pp-cli media create-subtitles` — Creates subtitles for a specific Media file.
- `x-twitter-pp-cli media delete-subtitles` — Deletes subtitles for a specific Media file.
- `x-twitter-pp-cli media finalize-upload` — Finalizes a Media upload request.
- `x-twitter-pp-cli media get-analytics` — Retrieves analytics data for media.
- `x-twitter-pp-cli media get-by-key` — Retrieves details of a specific Media file by its media key.
- `x-twitter-pp-cli media get-by-keys` — Retrieves details of Media files by their media keys.
- `x-twitter-pp-cli media get-upload-status` — Retrieves the status of a Media upload by its ID.
- `x-twitter-pp-cli media initialize-upload` — Initializes a media upload.
- `x-twitter-pp-cli media upload` — Uploads a media file for use in posts or other content.

**news** — Endpoint for retrieving news stories

- `x-twitter-pp-cli news get` — Retrieves news story by its ID.
- `x-twitter-pp-cli news search` — Retrieves a list of News stories matching the specified search query.

**notes** — Manage notes

- `x-twitter-pp-cli notes create-community` — Creates a community note endpoint for LLM use case.
- `x-twitter-pp-cli notes delete-community` — Deletes a community note.
- `x-twitter-pp-cli notes search-community-written` — Returns all the community notes written by the user.
- `x-twitter-pp-cli notes search-eligible-posts` — Returns all the posts that are eligible for community notes.

**openapi-json** — Manage openapi json

- `x-twitter-pp-cli openapi-json` — Retrieves the full OpenAPI Specification in JSON format. (See https://github.

**spaces** — Endpoints related to retrieving, managing Spaces

- `x-twitter-pp-cli spaces get-by-creator-ids` — Retrieves details of Spaces created by specified User IDs.
- `x-twitter-pp-cli spaces get-by-id` — Retrieves details of a specific space by its ID.
- `x-twitter-pp-cli spaces get-by-ids` — Retrieves details of multiple Spaces by their IDs.
- `x-twitter-pp-cli spaces search` — Retrieves a list of Spaces matching the specified search query.

**trends** — Manage trends

- `x-twitter-pp-cli trends <woeid>` — Retrieves trending topics for a specific location identified by its WOEID.

**tweets** — Endpoints related to retrieving, searching, and modifying Tweets

- `x-twitter-pp-cli tweets create-posts` — Creates a new Post for the authenticated user, or edits an existing Post when edit_options are provided.
- `x-twitter-pp-cli tweets create-webhooks-stream-link` — Creates a link to deliver FilteredStream events to the given webhook.
- `x-twitter-pp-cli tweets delete-posts` — Deletes a specific Post by its ID, if owned by the authenticated user.
- `x-twitter-pp-cli tweets delete-webhooks-stream-link` — Deletes a link from FilteredStream events to the given webhook.
- `x-twitter-pp-cli tweets get-posts-analytics` — Retrieves analytics data for specified Posts within a defined time range.
- `x-twitter-pp-cli tweets get-posts-by-id` — Retrieves details of a specific Post by its ID.
- `x-twitter-pp-cli tweets get-posts-by-ids` — Retrieves details of multiple Posts by their IDs.
- `x-twitter-pp-cli tweets get-posts-counts-all` — Retrieves the count of Posts matching a search query from the full archive.
- `x-twitter-pp-cli tweets get-posts-counts-recent` — Retrieves the count of Posts from the last 7 days matching a search query.
- `x-twitter-pp-cli tweets get-rule-counts` — Retrieves the count of rules in the active rule set for the filtered stream.
- `x-twitter-pp-cli tweets get-rules` — Retrieves the active rule set or a subset of rules for the filtered stream.
- `x-twitter-pp-cli tweets get-webhooks-stream-links` — Get a list of webhook links associated with a filtered stream ruleset.
- `x-twitter-pp-cli tweets search-posts-all` — Retrieves Posts from the full archive matching a search query.
- `x-twitter-pp-cli tweets search-posts-recent` — Retrieves Posts from the last 7 days matching a search query.
- `x-twitter-pp-cli tweets update-rules` — Adds or deletes rules from the active rule set for the filtered stream.

**usage** — Manage usage

- `x-twitter-pp-cli usage` — Retrieves usage statistics for Posts over a specified number of days.

**users** — Endpoints related to retrieving, managing relationships of Users

- `x-twitter-pp-cli users get-by-id` — Retrieves details of a specific User by their ID.
- `x-twitter-pp-cli users get-by-ids` — Retrieves details of multiple Users by their IDs.
- `x-twitter-pp-cli users get-by-username` — Retrieves details of a specific User by their username.
- `x-twitter-pp-cli users get-by-usernames` — Retrieves details of multiple Users by their usernames.
- `x-twitter-pp-cli users get-me` — Retrieves details of the authenticated user.
- `x-twitter-pp-cli users get-public-keys` — Returns the public keys and Juicebox configuration for the specified users.
- `x-twitter-pp-cli users get-reposts-of-me` — Retrieves a list of Posts that repost content from the authenticated user.
- `x-twitter-pp-cli users get-trends-personalized-trends` — Retrieves personalized trending topics for the authenticated user.
- `x-twitter-pp-cli users bookmarks find [query]` — Searches locally synced bookmarks by keyword and/or author without another API read.
- `x-twitter-pp-cli users likes post <user_id> --tweet-id <tweet_id>` — Likes a post on behalf of the authenticated user.
- `x-twitter-pp-cli users likes unlike-post <user_id> <tweet_id>` — Unlikes a post on behalf of the authenticated user.
- `x-twitter-pp-cli users search` — Retrieves a list of Users matching a search query.

**webhooks** — Manage webhooks

- `x-twitter-pp-cli webhooks create` — Creates a new webhook configuration.
- `x-twitter-pp-cli webhooks create-replay-job` — Creates a replay job to retrieve events from up to the past 24 hours for all events delivered or attempted to be
- `x-twitter-pp-cli webhooks delete` — Deletes an existing webhook configuration.
- `x-twitter-pp-cli webhooks get` — Get a list of webhook configs associated with a client app.
- `x-twitter-pp-cli webhooks validate` — Triggers a CRC check for a given webhook.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
x-twitter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Archive a topic, then query it offline

```bash
x-twitter-pp-cli sync --resources tweets --since 24h && x-twitter-pp-cli search "ai agents" --type tweets --limit 50
```

Sync once into SQLite, then run as many offline FTS queries as you want without further API reads.

### Agent-friendly field projection on a large response

```bash
x-twitter-pp-cli search "openai" --type tweets --agent --select id,text,author_id,public_metrics.like_count
```

Posts carry large nested payloads; --agent + --select trims the response to just the fields an agent needs, keeping context cheap.

### Reconstruct a conversation thread offline

```bash
x-twitter-pp-cli thread show 1750000000000000000 --agent
```

Walks the synced posts joined on conversation_id and referenced_tweets into an ordered, depth-tagged tree — no API call.

### Compose a self-reply thread from a document (dry-run by default)

```bash
x-twitter-pp-cli thread compose ./release-notes.md
```

Splits the markdown into a numbered 280-char-packed self-reply thread and prints it; add --post to actually publish.

## Auth Setup

X auth has three separate lanes. Do not infer one lane from another; run `x-twitter-pp-cli doctor --json` and inspect `auth_lanes` before choosing a command.

- `auth_lanes.app_only_api`: `X_BEARER_TOKEN`, the app-only bearer token from the X developer console. Use this for public reads such as tweet/user lookup, recent search, lists, and spaces.
- `auth_lanes.oauth2_user_context`: `X_OAUTH2_USER_TOKEN` or a stored OAuth2 access token. Required for `/2/users/me`, writes, bookmarks, personal reads, DMs, follows, likes, reposts, and user-context analytics. If this lane is `missing` or `invalid`, do not retry with `X_BEARER_TOKEN`; get a real OAuth2 authorization-code + PKCE user token and set/import it explicitly.
- `auth_lanes.x_articles_cookie`: browser cookies captured by `x-twitter-pp-cli auth login --chrome`. This lane is only for X Articles / x.com browser-session endpoints. It does not create `X_OAUTH2_USER_TOKEN` and does not authenticate v2 API user-context commands.

Setup sequence:

1. Attach the app to a Project in the X developer console.
2. Set app permissions to Read and write when you need posting or other mutations.
3. Copy the app Bearer Token into `X_BEARER_TOKEN` for app-only public reads.
4. Enable OAuth2 with suitable scopes such as `tweet.read`, `tweet.write`, `users.read`, `offline.access`, and `bookmark.read` (required if you intend to sync or search bookmarks), complete the authorization-code + PKCE flow, and set the resulting user-context token in `X_OAUTH2_USER_TOKEN` or import it with `x-twitter-pp-cli auth import-oauth2 --access-token <token> --refresh-token <token> --scopes tweet.read,tweet.write,users.read,offline.access,bookmark.read`.
5. Separately run `x-twitter-pp-cli auth login --chrome` only when using `articles ...` commands.

`doctor --json` is the machine-readable preflight. It reports `selected_profile`, per-lane status, `/2/users/me` probe results, authenticated user identity when returned, stored scopes, missing workflow scopes, token expiry, refresh-token presence, and X Articles cookie status without printing secrets.

When X returns `Unsupported Authentication` with `Application-Only is forbidden`, the command requires OAuth2 user-context auth. Fix `auth_lanes.oauth2_user_context`; cookie auth and app-only bearer auth will not satisfy that endpoint.

## Raw API Escape Hatch

Use `raw` only for debugging new X endpoints, auth-lane behavior, or API errors when no generated command fits yet. It accepts relative API paths or allowlisted HTTPS X absolute URLs, repeatable query params and headers, and JSON bodies from `--body`, `--body-file`, or `--body @-`.

```bash
x-twitter-pp-cli raw GET /2/users/me --agent
x-twitter-pp-cli raw GET /2/tweets --param ids=123,456 --json
x-twitter-pp-cli raw POST /2/tweets --body '{"text":"hello"}' --dry-run --agent
```

Generated commands remain preferred for normal workflows because they include endpoint-specific validation, pagination, provenance, response envelopes, and workflow hints. `raw` preserves the upstream response body while using the same auth lanes as every other command: app-only public reads still need `X_BEARER_TOKEN`; user-context reads and writes still need `X_OAUTH2_USER_TOKEN` or an imported OAuth2 user-context token.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  x-twitter-pp-cli communities search --query example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending. Under `--agent`, mutation dry-runs return JSON with `sent:false`, method/path/body, selected profile, auth lane, mutation classification, and public action when known.
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
x-twitter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
x-twitter-pp-cli feedback --stdin < notes.txt
x-twitter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/x-twitter-pp-cli/feedback.jsonl`. They are never POSTed unless `X_TWITTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `X_TWITTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
x-twitter-pp-cli profile save briefing --json
x-twitter-pp-cli --profile briefing communities search --query example-value
x-twitter-pp-cli profile list --json
x-twitter-pp-cli profile show briefing
x-twitter-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `x-twitter-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add x-twitter-pp-mcp -- x-twitter-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which x-twitter-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   x-twitter-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `x-twitter-pp-cli <command> --help`.
