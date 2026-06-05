---
name: pp-x-twitter
description: "The only X CLI with an offline Trigger phrases: `search X for`, `archive tweets about`, `show me the X thread for`, `monitor my X mentions`, `post a thread to X`, `use x-twitter`, `run x-twitter-pp-cli`."
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

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when a task involves reading, searching, or archiving X (Twitter) data and you want the results queryable offline rather than re-fetched each time — building a searchable corpus of posts, reconstructing a conversation thread, snapshotting a user's recent posts with engagement, or monitoring mentions incrementally. It is also the right choice when an AI agent needs an X surface with read-only/destructive safety hints and named multi-step intents rather than a pile of raw endpoint calls. Prefer it over raw API calls whenever the same data will be queried more than once, since the local store avoids re-spending per-read credits.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
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

**likes** — Manage likes


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

X auth is layered; the configured credential decides what works, and the CLI prefers X_OAUTH2_USER_TOKEN when set (reads + writes) over X_BEARER_TOKEN (reads only). Run doctor to see what is configured and what it unlocks. Credentials: X_BEARER_TOKEN (app-only Bearer) for public reads (tweet/user lookup, recent search, lists, spaces); X_OAUTH2_USER_TOKEN (OAuth 2.0 user-context) for v2 writes (post, like, repost, bookmark, follow, DM) and personal reads (me, mentions, home timeline, bookmarks); logged-in x.com browser cookies for X Articles (articles-publish-md, articles ...), captured via 'auth login --chrome' (X Articles has no v2 API). Setup in the X developer console (console.x.com) - a person can do this or an agent with console access can automate it; none of it is manual-only: (1) attach the app to a Project (any environment, incl. Development) - this unlocks v2 API access, standalone-app tokens are rejected; (2) set app permissions to Read and write (needed for posting); (3) copy the app Bearer Token into X_BEARER_TOKEN for reads; (4) enable OAuth 2.0 (Native/public app, callback URL, scopes tweet.read tweet.write users.read offline.access), complete the authorization-code + PKCE flow, set the resulting opaque user token in X_OAUTH2_USER_TOKEN for writes; (5) run 'auth login --chrome' to capture x.com cookies for Articles (needs pycookiecheat or press-auth; manual DevTools fallback). A Development project does not limit the account - capability is set by app permissions and the account API tier. As of Feb 2026 X bills reads/writes per-use and restricts programmatic replies/quotes/@mentions; self-reply threads (thread compose) still work.

Run `x-twitter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  x-twitter-pp-cli communities search --query example-value --agent --select id,name,status
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
