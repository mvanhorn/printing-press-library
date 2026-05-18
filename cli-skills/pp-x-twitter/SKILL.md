---
name: pp-x-twitter
description: "Printing Press CLI for X. Combined CLI for multiple API services"
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
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/social-and-messaging/x-twitter/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# X (Twitter) — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `x-twitter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install x-twitter --cli-only
   ```
2. Verify: `x-twitter-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Combined CLI for multiple API services

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

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
- `x-twitter-pp-cli activity stream` — Stream of X Activities
- `x-twitter-pp-cli activity update-subscription` — Updates a subscription for an X activity event

**articles** — X Articles (long-form posts) authoring + media upload

- `x-twitter-pp-cli articles create_draft` — POST /i/api/graphql/g1l5N8BxGewYuCy5USe_bQ/ArticleEntityDraftCreate
- `x-twitter-pp-cli articles delete` — POST /i/api/graphql/e4lWqB6m2TA8Fn_j9L9xEA/ArticleEntityDelete
- `x-twitter-pp-cli articles list` — GET /i/api/graphql/N1zzFzRPspT-sP9Q42n_bg/ArticleEntitiesSlice
- `x-twitter-pp-cli articles publish` — POST /i/api/graphql/m4SHicYMoWO_qkLvjhDk7Q/ArticleEntityPublish
- `x-twitter-pp-cli articles update_content` — POST /i/api/graphql/M7N2FrPrlOmu-YrVIBxFnQ/ArticleEntityUpdateContent
- `x-twitter-pp-cli articles update_cover_media` — POST /i/api/graphql/Es8InPh7mEkK9PxclxFAVQ/ArticleEntityUpdateCoverMedia
- `x-twitter-pp-cli articles update_title` — POST /i/api/graphql/x75E2ABzm8_mGTg1bz8hcA/ArticleEntityUpdateTitle
- `x-twitter-pp-cli articles upload_media` — POST /i/media/upload.json

**chat** — Manage chat

- `x-twitter-pp-cli chat add-group-members` — Adds one or more members to an existing encrypted Chat group conversation, rotating the conversation key.
- `x-twitter-pp-cli chat create-conversation` — Creates a new encrypted Chat group conversation on behalf of the authenticated user.
- `x-twitter-pp-cli chat get-conversation` — Retrieves messages and key change events for a specific Chat conversation with pagination support. For 1:1...
- `x-twitter-pp-cli chat get-conversations` — Retrieves a list of Chat conversations for the authenticated user's inbox.
- `x-twitter-pp-cli chat initialize-conversation-keys` — Initializes encryption keys for a Chat conversation. This is the first step before sending messages in a new 1:1...
- `x-twitter-pp-cli chat initialize-group` — Initializes a new XChat group conversation and returns a unique conversation ID. This endpoint is the first step in...
- `x-twitter-pp-cli chat mark-conversation-read` — Marks a specific Chat conversation as read on behalf of the authenticated user. For 1:1 conversations, provide the...
- `x-twitter-pp-cli chat media-download` — Downloads encrypted media bytes from an XChat conversation. The response body contains raw binary bytes. For 1:1...
- `x-twitter-pp-cli chat media-upload-append` — Appends media data to an XChat upload session.
- `x-twitter-pp-cli chat media-upload-finalize` — Finalizes an XChat media upload session.
- `x-twitter-pp-cli chat media-upload-initialize` — Initializes an XChat media upload session.
- `x-twitter-pp-cli chat send-message` — Sends an encrypted message to a specific Chat conversation. For 1:1 conversations, provide the recipient's user ID;...
- `x-twitter-pp-cli chat send-typing-indicator` — Sends a typing indicator to a specific Chat conversation on behalf of the authenticated user. For 1:1 conversations,...

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
- `x-twitter-pp-cli dm-conversations media-download` — Downloads media attached to a legacy Direct Message. The requesting user must be a participant in the conversation...

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

- `x-twitter-pp-cli likes stream-compliance` — Streams all compliance data related to Likes for Users.
- `x-twitter-pp-cli likes stream-firehose` — Streams all public Likes in real-time.
- `x-twitter-pp-cli likes stream-sample10` — Streams a 10% sample of public Likes in real-time.

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

- `x-twitter-pp-cli openapi-json` — Retrieves the full OpenAPI Specification in JSON format. (See...

**spaces** — Endpoints related to retrieving, managing Spaces

- `x-twitter-pp-cli spaces get-by-creator-ids` — Retrieves details of Spaces created by specified User IDs.
- `x-twitter-pp-cli spaces get-by-id` — Retrieves details of a specific space by its ID.
- `x-twitter-pp-cli spaces get-by-ids` — Retrieves details of multiple Spaces by their IDs.
- `x-twitter-pp-cli spaces search` — Retrieves a list of Spaces matching the specified search query.

**trends** — Manage trends

- `x-twitter-pp-cli trends <woeid>` — Retrieves trending topics for a specific location identified by its WOEID.

**tweets** — Endpoints related to retrieving, searching, and modifying Tweets

- `x-twitter-pp-cli tweets create-posts` — Creates a new Post for the authenticated user, or edits an existing Post when edit_options are provided. Supports...
- `x-twitter-pp-cli tweets create-webhooks-stream-link` — Creates a link to deliver FilteredStream events to the given webhook.
- `x-twitter-pp-cli tweets delete-posts` — Deletes a specific Post by its ID, if owned by the authenticated user.
- `x-twitter-pp-cli tweets delete-webhooks-stream-link` — Deletes a link from FilteredStream events to the given webhook.
- `x-twitter-pp-cli tweets get-posts-analytics` — Retrieves analytics data for specified Posts within a defined time range.
- `x-twitter-pp-cli tweets get-posts-by-id` — Retrieves details of a specific Post by its ID.
- `x-twitter-pp-cli tweets get-posts-by-ids` — Retrieves details of multiple Posts by their IDs.
- `x-twitter-pp-cli tweets get-posts-counts-recent` — Retrieves the count of Posts from the last 7 days matching a search query.
- `x-twitter-pp-cli tweets get-webhooks-stream-links` — Get a list of webhook links associated with a filtered stream ruleset.
- `x-twitter-pp-cli tweets search-posts-recent` — Retrieves Posts from the last 7 days matching a search query.
- `x-twitter-pp-cli tweets stream-labels-compliance` — Streams all labeling events applied to Posts.
- `x-twitter-pp-cli tweets stream-posts-compliance` — Streams all compliance data related to Posts.
- `x-twitter-pp-cli tweets stream-posts-firehose` — Streams all public Posts in real-time.
- `x-twitter-pp-cli tweets stream-posts-firehose-en` — Streams all public English-language Posts in real-time.
- `x-twitter-pp-cli tweets stream-posts-firehose-ja` — Streams all public Japanese-language Posts in real-time.
- `x-twitter-pp-cli tweets stream-posts-firehose-ko` — Streams all public Korean-language Posts in real-time.
- `x-twitter-pp-cli tweets stream-posts-firehose-pt` — Streams all public Portuguese-language Posts in real-time.
- `x-twitter-pp-cli tweets stream-posts-sample` — Streams a 1% sample of public Posts in real-time.
- `x-twitter-pp-cli tweets stream-posts-sample10` — Streams a 10% sample of public Posts in real-time.

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
- `x-twitter-pp-cli users stream-compliance` — Streams all compliance data related to Users.

**webhooks** — Manage webhooks

- `x-twitter-pp-cli webhooks create` — Creates a new webhook configuration.
- `x-twitter-pp-cli webhooks create-replay-job` — Creates a replay job to retrieve events from up to the past 24 hours for all events delivered or attempted to be...
- `x-twitter-pp-cli webhooks delete` — Deletes an existing webhook configuration.
- `x-twitter-pp-cli webhooks get` — Get a list of webhook configs associated with a client app.
- `x-twitter-pp-cli webhooks validate` — Triggers a CRC check for a given webhook.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
x-twitter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
x-twitter-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `X_OAUTH2_USER_TOKEN` as an environment variable.

Run `x-twitter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  x-twitter-pp-cli articles list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
x-twitter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
x-twitter-pp-cli feedback --stdin < notes.txt
x-twitter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.x-twitter-pp-cli/feedback.jsonl`. They are never POSTed unless `X_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `X_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
x-twitter-pp-cli --profile briefing articles list
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

---

## Hand-written notes (survive regen with `printing-press regen-merge`)

### Auth model — two sources

The X surface is split across two auth sources. The CLI selects per-host automatically.

**Source A — OAuth 2.0 PKCE.** Used for `api.x.com` v2 endpoints (tweets, users, lists, spaces, bookmarks, likes, etc.). Standard CLI flow:

```bash
x-twitter-pp-cli auth login        # opens browser, captures token
export X_OAUTH2_USER_TOKEN=<token>   # or set in config
x-twitter-pp-cli doctor            # verifies
```

**Source B — Browser session cookies.** Used for `x.com` Articles GraphQL endpoints and `upload.x.com` media upload. X's Articles editor is browser-only; OAuth 2.0 does NOT authenticate it. One-time capture:

1. In Chrome, open DevTools on `x.com` while logged in.
2. **Application → Cookies → x.com** → copy `auth_token` and `ct0` values.
3. **Network** → reload → click any `/i/api/graphql/...` request → **Request Headers** → copy the `Authorization: Bearer ...` value (this is the web app's hardcoded bearer, NOT a user OAuth token).
4. Write to `~/.config/x-twitter-pp-cli/cookies.json`:

   ```json
   {
     "auth_token": "<paste>",
     "ct0":        "<paste>",
     "web_bearer": "<paste, without the 'Bearer ' prefix>",
     "captured_at": "2026-05-08T22:00:00Z"
   }
   ```
5. `chmod 600 ~/.config/x-twitter-pp-cli/cookies.json`.

When X invalidates the session (logout, security events, ~weeks of inactivity), `articles *` commands return 401 — repeat the capture.

> Note: Chrome's "Save all as HAR with content" SILENTLY redacts cookies and the Authorization header. Do not try to extract them from a HAR — they will be missing. Use the DevTools Application/Network tabs above.

### Articles GraphQL operation hashes — runtime config

X Articles uses GraphQL with rotating operation hashes in the URL path (e.g. `/i/api/graphql/M7N2FrPrlOmu-YrVIBxFnQ/ArticleEntityUpdateContent`). The hash rotates when X redeploys their web app.

Hashes are loaded at runtime from `~/.config/x-twitter-pp-cli/article-ops.json`. A default capture from generation time (2026-05-08) is built into the binary as fallback. When `articles *` commands return 404 with "no such operation," refresh:

1. Re-sniff the Articles editor with the printing-press tool, OR
2. Capture a HAR via DevTools Network (any Articles operation), then:

   ```bash
   python3 -c "
   import json, re
   from sys import argv
   with open(argv[1]) as f: h = json.load(f)
   ops = {}
   for e in h['log']['entries']:
       g = re.search(r'/i/api/graphql/([A-Za-z0-9_-]+)/(Article[A-Za-z0-9_]+)', e['request']['url'])
       if g: ops[g.group(2)] = g.group(1)
   import os, datetime
   path = os.path.expanduser('~/.config/x-twitter-pp-cli/article-ops.json')
   with open(path, 'w') as f: json.dump({'operations': ops, 'captured_at': datetime.datetime.now().isoformat()}, f, indent=2)
   print(f'wrote {len(ops)} ops to {path}')
   " /path/to/your.har
   ```

### Hand-written compound commands

These are NOT auto-generated; they wrap multiple endpoints into single agent-friendly calls.

**`x-twitter-pp-cli thread compose <markdown-file>` [--post]**

Splits a markdown document into a numbered tweet thread (≤280 chars per tweet, atom-aware: paragraphs, list items, code fences are atoms; never splits inside a code fence). The "(N/M)" suffix length is reserved BEFORE packing so final tweets stay within the limit. Default behavior is dry-run (preview); `--post` is gated and not yet wired.

```bash
x-twitter-pp-cli thread compose draft.md         # dry-run preview
x-twitter-pp-cli thread compose draft.md --post  # (not yet wired — preview only)
```

**`x-twitter-pp-cli articles-publish-md <markdown-file>` [--post]**

Parses YAML frontmatter (`title`, `cover`, `tags`, `summary`) and converts the markdown body to Draft.js `content_state` JSON — the format the X Articles editor server-side expects.

Supported in v1: cover image from frontmatter (`cover: ./cover.png`), body image lines (`![alt](./body.png)`), paragraph, header-one (`#`), header-two (`##`), unordered-list-item (`-`/`*`), ordered-list-item (`1.`), blockquote (`>`), fenced code blocks, inline `**bold**` and `*italic*`.

```bash
# Markdown with frontmatter
cat > draft.md <<EOF
---
title: My Article
cover: ./cover.png
tags: [test]
---

# Header

A paragraph with **bold** text.

![Screenshot alt text](./body.png)

- bullet one
- bullet two

```bash
x-twitter-pp-cli articles-publish-md draft.md --post
```
EOF

x-twitter-pp-cli articles-publish-md draft.md   # dry-run; prints constructed content_state JSON
x-twitter-pp-cli articles-publish-md draft.md --post  # create, update, set cover, and publish the article
```

With `--post`, the command creates a draft, updates the title, uploads body images and binds them into the Draft.js `entity_map`, updates the content, uploads and assigns a cover image when `cover` is set, and publishes it as a public X Article using Source B cookie auth and the article-ops.json hash config. The wrapper requires a frontmatter `title`.

**`x-twitter-pp-cli articles set-cover <article-id> <image-file>`**

Uploads a local image through X's browser media-upload flow and assigns the returned media ID as the Article cover.

### Known limitations of v1

- `thread compose --post` is gated as a dry-run only. Real thread posting is the next engineering step.
- OAuth live acceptance is blocked until the registered callback setup is available.
