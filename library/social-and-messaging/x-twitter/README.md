# X (Twitter) CLI

**The only X CLI with an offline, searchable local mirror — full-text search and analytics over your archived posts without re-spending per-read API credits — plus a full-surface MCP server and honest tier/reachability diagnostics.**

Mirrors the official X v2 API and adds what no other X tool has: a local SQLite store you can sync once and query many times with FTS5 search and group-by analytics, agent-native --json/--select output, and an MCP server that exposes the whole surface to AI agents through token-efficient orchestration plus named multi-step intents. Start pasted-link workflows with `post resolve`, use `thread context` when a post needs parent/quote/reply context, save durable source material with `collection save/list/export`, track ongoing searches with `monitor create/run/list`, package saved activity with `brief`, snapshot accounts with `account snapshot`, find launch or repo links with `url mentions`, track post metrics with `performance snapshot/backfill/analyze`, export account/query timelines with `timeline export`, reconstruct synced conversation threads offline with `thread show`, compose self-reply threads from markdown with `thread compose`, author long-form X Articles from markdown with `articles-publish-md`, and rescue your bookmark graveyard with `users bookmarks find` — keyword and author search over your synced bookmarks, which X itself gives you no way to search.

Learn more at [X (Twitter)](https://developer.x.com/).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow), [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `x-twitter-pp-cli` binary and the `pp-x-twitter` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install x-twitter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install x-twitter --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install x-twitter --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install x-twitter --agent claude-code
npx -y @mvanhorn/printing-press-library install x-twitter --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/x-twitter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install x-twitter --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-x-twitter --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-x-twitter --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install x-twitter --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/x-twitter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `X_BEARER_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "x-twitter": {
      "command": "x-twitter-pp-mcp",
      "env": {
        "X_BEARER_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

X auth has three separate lanes. Run `x-twitter-pp-cli doctor --json` and inspect `auth_lanes` before choosing a command; do not assume one credential can stand in for another.

- `auth_lanes.app_only_api`: an app-only X API Bearer token from the X developer console. Use this for public read-only API access such as post/user lookup, recent search, lists, and spaces. Store it with `x-twitter-pp-cli auth set-bearer-token <token>` or export `X_BEARER_TOKEN`.
- `auth_lanes.oauth2_user_context`: `X_OAUTH2_USER_TOKEN` or a stored OAuth2 access token from `x-twitter-pp-cli auth oauth2-login` / `auth import-oauth2`. Required for `/2/users/me`, writes, bookmarks, personal reads, DMs, follows, likes, reposts, and user-context analytics. If this lane is missing or invalid, do not retry with the app-only bearer token; run the OAuth2 authorization-code + PKCE flow or import a real user-context token explicitly.
- `auth_lanes.x_articles_cookie`: logged-in x.com browser cookies captured by `x-twitter-pp-cli auth login --chrome`. This lane is only for X Articles / x.com browser-session endpoints. It does not create `X_OAUTH2_USER_TOKEN` and does not authenticate v2 API user-context commands.

Setup sequence:

1. Attach the app to a Project in the X developer console (`console.x.com`). Any environment, including Development, unlocks v2 API access; standalone-app tokens are rejected.
2. Set app permissions to Read and write when you need posting or other mutations.
3. Copy the app Bearer Token and run `x-twitter-pp-cli auth set-bearer-token <bearer-token>` for app-only public reads. Environment alternative: `export X_BEARER_TOKEN="<bearer-token>"`.
4. Enable OAuth2 with suitable scopes such as `tweet.read`, `tweet.write`, `users.read`, `offline.access`, `bookmark.read`, `like.read`, `like.write`, `follows.read`, and `follows.write`. Add `http://127.0.0.1:8787/callback` as an OAuth2 redirect URI in the X developer app, then run `x-twitter-pp-cli auth oauth2-login --client-id <oauth2-client-id>` to complete the authorization-code + PKCE browser flow. Non-interactive fallback when you already have tokens: `x-twitter-pp-cli auth import-oauth2 --client-id <oauth2-client-id> --access-token <oauth2-access-token> --refresh-token <oauth2-refresh-token> --scopes tweet.read,tweet.write,users.read,offline.access,bookmark.read,like.read,like.write,follows.read,follows.write`. Environment alternative: `export X_OAUTH2_USER_TOKEN="<oauth2-access-token>"`.
5. Separately run `x-twitter-pp-cli auth login --chrome` only when using X Articles commands such as `articles-publish-md` or `articles ...` (needs `pycookiecheat` or `press-auth`; manual DevTools fallback is available).

`auth set-token` is deprecated because “token” is ambiguous for X. It remains as a compatibility alias for `auth set-bearer-token` and must not be used for OAuth2 user-context tokens. Use `auth oauth2-login` for the interactive OAuth2 browser flow or `auth import-oauth2` when you already have tokens.

`doctor --json` is the machine-readable preflight. It reports `selected_profile`, per-lane status, `/2/users/me` probe results, authenticated user identity when returned, stored scopes, missing workflow scopes, token expiry, refresh-token presence, and X Articles cookie status without printing secrets.

A Development project does not limit the account; capability is set by app permissions and the account API tier. As of Feb 2026 X bills reads/writes per-use and restricts programmatic replies/quotes/@mentions; self-reply threads (`thread compose`) still work.

## Quick Start

```bash
# Health check first: reports selected profile, auth lanes, scopes, token expiry, and what workflows are safe.
x-twitter-pp-cli doctor --json

# Debug an endpoint directly when no generated command fits yet. Generated and curated commands are preferred.
x-twitter-pp-cli raw GET /2/users/me --dry-run --agent

# Pull recent posts into the local SQLite store once, so later reads query locally instead of re-spending API credits.
x-twitter-pp-cli sync --resources tweets --since 7d

# Full-text search your archived posts entirely offline.
x-twitter-pp-cli search "launch" --type tweets --limit 20

# Aggregate your synced posts locally — e.g. top authors by post count — entirely offline, no API call.
x-twitter-pp-cli analytics --type tweets --group-by author_id --limit 10

# Resolve a pasted X URL into a canonical, agent-friendly record.
x-twitter-pp-cli post resolve https://x.com/user/status/123 --agent

# Save source material locally and export it later.
x-twitter-pp-cli collection save https://x.com/user/status/123 --collection research --note "Useful example" --agent
x-twitter-pp-cli collection export research --format markdown

# Track ongoing mentions with a local watermark and dedupe.
x-twitter-pp-cli monitor create launch --url https://example.com --agent
x-twitter-pp-cli monitor run launch --since last --agent

# Package saved activity without LLM-dependent claims.
x-twitter-pp-cli brief --monitor launch --since 24h --format markdown

```

## Unique Features

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
- **`performance snapshot/backfill/analyze`** — Store timestamped post metrics locally, backfill recent account posts when auth allows, and analyze saved snapshots by type, hour, media, link presence, or label. Missing metrics stay nullable/absent; the CLI does not treat unavailable fields as zero.
  Snapshot output includes `public_metrics`, available non-public/organic metrics, `metric_source`, and `metric_availability` so scorer pipelines can distinguish unavailable metrics from real zeroes.

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
- **`articles-publish-md`** — Parse a markdown file with YAML frontmatter into the Draft.js content_state JSON X's Articles editor accepts; previews by default; --draft creates a new draft, --update edits an existing draft, --post publishes publicly.

  _The only programmatic way to author a long-form X Article from a document; preview, --draft, and --update keep it private until you explicitly --post._

  ```bash
  x-twitter-pp-cli articles-publish-md ./post.md
  x-twitter-pp-cli articles-publish-md ./post.md --update 1750000000000000000
  ```

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

## Usage

Run `x-twitter-pp-cli --help` for the full command reference and flag list.

## Commands

### account-activity

Endpoints relating to retrieving, managing AAA subscriptions

- **`x-twitter-pp-cli account-activity create-subscription`** - Creates an Account Activity subscription for the user and the given webhook.
- **`x-twitter-pp-cli account-activity delete-subscription`** - Deletes an Account Activity subscription for the given webhook and user ID.
- **`x-twitter-pp-cli account-activity get-subscription-count`** - Retrieves a count of currently active Account Activity subscriptions.
- **`x-twitter-pp-cli account-activity get-subscriptions`** - Retrieves a list of all active subscriptions for a given webhook.
- **`x-twitter-pp-cli account-activity validate-subscription`** - Checks a user’s Account Activity subscription for a given webhook.

### activity

Manage activity

- **`x-twitter-pp-cli activity create-subscription`** - Creates a subscription for an X activity event
- **`x-twitter-pp-cli activity delete-subscription`** - Deletes a subscription for an X activity event
- **`x-twitter-pp-cli activity delete-subscriptions-by-ids`** - Deletes multiple subscriptions for X activity events by their IDs
- **`x-twitter-pp-cli activity get-subscriptions`** - Get a list of active subscriptions for XAA
- **`x-twitter-pp-cli activity update-subscription`** - Updates a subscription for an X activity event

### chat

Manage chat

- **`x-twitter-pp-cli chat add-group-members`** - Adds one or more members to an existing encrypted Chat group conversation, rotating the conversation key.
- **`x-twitter-pp-cli chat create-conversation`** - Creates a new encrypted Chat group conversation on behalf of the authenticated user.
- **`x-twitter-pp-cli chat get-conversation`** - Returns metadata for a Chat conversation including type, muted status, and group details. Use chat_conversation.fields to select which fields are returned. Use expansions to hydrate member, admin, or participant user objects. Use user.fields to control which profile fields are returned for expanded users.
- **`x-twitter-pp-cli chat get-conversation-events`** - Retrieves messages and key change events for a specific Chat conversation with pagination support. For 1:1 conversations, provide the recipient's user ID; the server constructs the canonical conversation ID from the authenticated user and recipient.
- **`x-twitter-pp-cli chat get-conversations`** - Retrieves a list of Chat conversations for the authenticated user's inbox.
- **`x-twitter-pp-cli chat initialize-conversation-keys`** - Initializes encryption keys for a Chat conversation. This is the first step
before sending messages in a new 1:1 conversation.

For 1:1 conversations, provide the recipient's user ID as the conversation_id.
The server constructs the canonical conversation ID from the authenticated user
and recipient.

The request body must contain the conversation key version and participant keys
(the conversation key encrypted for each participant using their public key).

**Workflow (1:1 conversation):**
1. Generate a conversation key using the SDK
2. Encrypt the key for both participants using their public keys
3. Call this endpoint to register the keys
4. Send messages using `POST /chat/conversations/{id}/messages`

**Authentication:**
- Requires OAuth 1.0a User Context or OAuth 2.0 User Context
- Required scopes: `tweet.read`, `users.read`, `dm.write`
- **`x-twitter-pp-cli chat initialize-group`** - Initializes a new XChat group conversation and returns a unique conversation ID.

This endpoint is the first step in creating a group chat. The returned conversation_id 
should be used in subsequent calls to POST /chat/conversations/group to fully create and 
configure the group with members, admins, encryption keys, and other settings.

**Workflow:**
1. Call this endpoint to get a `conversation_id`
2. Use that `conversation_id` when calling `POST /chat/conversations/group` to create the group

**Authentication:**
- Requires OAuth 1.0a User Context or OAuth 2.0 User Context
- Required scope: `dm.write`
- **`x-twitter-pp-cli chat mark-conversation-read`** - Marks a specific Chat conversation as read on behalf of the authenticated user. For 1:1 conversations, provide the recipient's user ID; the server constructs the canonical conversation ID from the authenticated user and recipient.
- **`x-twitter-pp-cli chat media-download`** - Downloads encrypted media bytes from an XChat conversation. The response body contains raw binary bytes. For 1:1 conversations, provide the recipient's user ID; the server constructs the canonical conversation ID from the authenticated user and recipient.
- **`x-twitter-pp-cli chat media-upload-append`** - Appends media data to an XChat upload session.
- **`x-twitter-pp-cli chat media-upload-finalize`** - Finalizes an XChat media upload session.
- **`x-twitter-pp-cli chat media-upload-initialize`** - Initializes an XChat media upload session.
- **`x-twitter-pp-cli chat send-message`** - Sends an encrypted message to a specific Chat conversation. For 1:1 conversations, provide the recipient's user ID; the server constructs the canonical conversation ID from the authenticated user and recipient.
- **`x-twitter-pp-cli chat send-typing-indicator`** - Sends a typing indicator to a specific Chat conversation on behalf of the authenticated user. For 1:1 conversations, provide the recipient's user ID; the server constructs the canonical conversation ID from the authenticated user and recipient.

### communities

Manage communities

- **`x-twitter-pp-cli communities get-by-id`** - Retrieves details of a specific Community by its ID.
- **`x-twitter-pp-cli communities search`** - Retrieves a list of Communities matching the specified search query.

### compliance

Endpoints related to keeping X data in your systems compliant

- **`x-twitter-pp-cli compliance create-jobs`** - Creates a new Compliance Job for the specified job type.
- **`x-twitter-pp-cli compliance get-jobs`** - Retrieves a list of Compliance Jobs filtered by job type and optional status.
- **`x-twitter-pp-cli compliance get-jobs-by-id`** - Retrieves details of a specific Compliance Job by its ID.

### connections

Endpoints related to streaming connections

- **`x-twitter-pp-cli connections delete-all`** - Terminates all active streaming connections for the authenticated application.
- **`x-twitter-pp-cli connections delete-by-endpoint`** - Terminates all streaming connections for a specific endpoint ID for the authenticated application.
- **`x-twitter-pp-cli connections delete-by-uuids`** - Terminates multiple streaming connections by their UUIDs for the authenticated application.
- **`x-twitter-pp-cli connections get-history`** - Returns active and historical streaming connections with disconnect reasons for the authenticated application.

### dm-conversations

Manage dm conversations

- **`x-twitter-pp-cli dm-conversations create-direct-messages-by-participant-id`** - Sends a new direct message to a specific participant by their ID.
- **`x-twitter-pp-cli dm-conversations create-direct-messages-conversation`** - Initiates a new direct message conversation with specified participants.
- **`x-twitter-pp-cli dm-conversations get-direct-messages-events-by-participant-id`** - Retrieves direct message events for a specific conversation.
- **`x-twitter-pp-cli dm-conversations media-download`** - Downloads media attached to a legacy Direct Message. The requesting user must be a participant in the conversation containing the specified DM event. The response body contains raw binary bytes.

### dm-events

Manage dm events

- **`x-twitter-pp-cli dm-events delete-direct-messages-events`** - Deletes a specific direct message event by its ID, if owned by the authenticated user.
- **`x-twitter-pp-cli dm-events get-direct-messages-events`** - Retrieves a list of recent direct message events across all conversations.
- **`x-twitter-pp-cli dm-events get-direct-messages-events-by-id`** - Retrieves details of a specific direct message event by its ID.

### evaluate-note

Manage evaluate note

- **`x-twitter-pp-cli evaluate-note`** - Endpoint to evaluate a community note.

### insights

Manage insights

- **`x-twitter-pp-cli insights get-historical`** - Retrieves historical engagement metrics for specified Posts within a defined time range.
- **`x-twitter-pp-cli insights get-insights28-hr`** - Retrieves engagement metrics for specified Posts over the last 28 hours.

### lists

Endpoints related to retrieving, managing Lists

- **`x-twitter-pp-cli lists create`** - Creates a new List for the authenticated user.
- **`x-twitter-pp-cli lists delete`** - Deletes a specific List owned by the authenticated user by its ID.
- **`x-twitter-pp-cli lists get-by-id`** - Retrieves details of a specific List by its ID.
- **`x-twitter-pp-cli lists update`** - Updates the details of a specific List owned by the authenticated user by its ID.

### media

Endpoints related to Media

- **`x-twitter-pp-cli media append-upload`** - Appends data to a Media upload request.
- **`x-twitter-pp-cli media create-metadata`** - Creates metadata for a Media file.
- **`x-twitter-pp-cli media create-subtitles`** - Creates subtitles for a specific Media file.
- **`x-twitter-pp-cli media delete-subtitles`** - Deletes subtitles for a specific Media file.
- **`x-twitter-pp-cli media finalize-upload`** - Finalizes a Media upload request.
- **`x-twitter-pp-cli media get-analytics`** - Retrieves analytics data for media.
- **`x-twitter-pp-cli media get-by-key`** - Retrieves details of a specific Media file by its media key.
- **`x-twitter-pp-cli media get-by-keys`** - Retrieves details of Media files by their media keys.
- **`x-twitter-pp-cli media get-upload-status`** - Retrieves the status of a Media upload by its ID.
- **`x-twitter-pp-cli media initialize-upload`** - Initializes a media upload.
- **`x-twitter-pp-cli media upload`** - Uploads a media file for use in posts or other content.

### news

Endpoint for retrieving news stories

- **`x-twitter-pp-cli news get`** - Retrieves news story by its ID.
- **`x-twitter-pp-cli news search`** - Retrieves a list of News stories matching the specified search query.

### notes

Manage notes

- **`x-twitter-pp-cli notes create-community`** - Creates a community note endpoint for LLM use case.
- **`x-twitter-pp-cli notes delete-community`** - Deletes a community note.
- **`x-twitter-pp-cli notes search-community-written`** - Returns all the community notes written by the user.
- **`x-twitter-pp-cli notes search-eligible-posts`** - Returns all the posts that are eligible for community notes.

### openapi-json

Manage openapi json

- **`x-twitter-pp-cli openapi-json`** - Retrieves the full OpenAPI Specification in JSON format. (See https://github.com/OAI/OpenAPI-Specification/blob/master/README.md)

### spaces

Endpoints related to retrieving, managing Spaces

- **`x-twitter-pp-cli spaces get-by-creator-ids`** - Retrieves details of Spaces created by specified User IDs.
- **`x-twitter-pp-cli spaces get-by-id`** - Retrieves details of a specific space by its ID.
- **`x-twitter-pp-cli spaces get-by-ids`** - Retrieves details of multiple Spaces by their IDs.
- **`x-twitter-pp-cli spaces search`** - Retrieves a list of Spaces matching the specified search query.

### trends

Manage trends

- **`x-twitter-pp-cli trends <woeid>`** - Retrieves trending topics for a specific location identified by its WOEID.

### tweets

Endpoints related to retrieving, searching, and modifying Tweets

- **`x-twitter-pp-cli tweets create-posts`** - Creates a new Post for the authenticated user, or edits an existing Post when edit_options are provided. Supports paid partnership disclosure via the paid_partnership field.
- **`x-twitter-pp-cli tweets create-webhooks-stream-link`** - Creates a link to deliver FilteredStream events to the given webhook.
- **`x-twitter-pp-cli tweets delete-posts`** - Deletes a specific Post by its ID, if owned by the authenticated user.
- **`x-twitter-pp-cli tweets delete-webhooks-stream-link`** - Deletes a link from FilteredStream events to the given webhook.
- **`x-twitter-pp-cli tweets get-posts-analytics`** - Retrieves analytics data for specified Posts within a defined time range.
- **`x-twitter-pp-cli tweets get-posts-by-id`** - Retrieves details of a specific Post by its ID.
- **`x-twitter-pp-cli tweets get-posts-by-ids`** - Retrieves details of multiple Posts by their IDs.
- **`x-twitter-pp-cli tweets get-posts-counts-all`** - Retrieves the count of Posts matching a search query from the full archive.
- **`x-twitter-pp-cli tweets get-posts-counts-recent`** - Retrieves the count of Posts from the last 7 days matching a search query.
- **`x-twitter-pp-cli tweets get-rule-counts`** - Retrieves the count of rules in the active rule set for the filtered stream.
- **`x-twitter-pp-cli tweets get-rules`** - Retrieves the active rule set or a subset of rules for the filtered stream.
- **`x-twitter-pp-cli tweets get-webhooks-stream-links`** - Get a list of webhook links associated with a filtered stream ruleset.
- **`x-twitter-pp-cli tweets search-posts-all`** - Retrieves Posts from the full archive matching a search query.
- **`x-twitter-pp-cli tweets search-posts-recent`** - Retrieves Posts from the last 7 days matching a search query.
- **`x-twitter-pp-cli tweets update-rules`** - Adds or deletes rules from the active rule set for the filtered stream.

### usage

Manage usage

- **`x-twitter-pp-cli usage`** - Retrieves usage statistics for Posts over a specified number of days.

### users

Endpoints related to retrieving, managing relationships of Users

- **`x-twitter-pp-cli users get-by-id`** - Retrieves details of a specific User by their ID.
- **`x-twitter-pp-cli users get-by-ids`** - Retrieves details of multiple Users by their IDs.
- **`x-twitter-pp-cli users get-by-username`** - Retrieves details of a specific User by their username.
- **`x-twitter-pp-cli users get-by-usernames`** - Retrieves details of multiple Users by their usernames.
- **`x-twitter-pp-cli users get-me`** - Retrieves details of the authenticated user.
- **`x-twitter-pp-cli users get-public-keys`** - Returns the public keys and Juicebox configuration for the specified users.
- **`x-twitter-pp-cli users get-reposts-of-me`** - Retrieves a list of Posts that repost content from the authenticated user.
- **`x-twitter-pp-cli users get-trends-personalized-trends`** - Retrieves personalized trending topics for the authenticated user.
- **`x-twitter-pp-cli users bookmarks find [query]`** - Searches your locally synced bookmarks by keyword and/or author without another API read.
- **`x-twitter-pp-cli users likes post <user_id> --tweet-id <tweet_id>`** - Likes a post on behalf of the authenticated user.
- **`x-twitter-pp-cli users likes unlike-post <user_id> <tweet_id>`** - Unlikes a post on behalf of the authenticated user.
- **`x-twitter-pp-cli users search`** - Retrieves a list of Users matching a search query.

### webhooks

Manage webhooks

- **`x-twitter-pp-cli webhooks create`** - Creates a new webhook configuration.
- **`x-twitter-pp-cli webhooks create-replay-job`** - Creates a replay job to retrieve events from up to the past 24 hours for all events delivered or attempted to be delivered to the webhook.
- **`x-twitter-pp-cli webhooks delete`** - Deletes an existing webhook configuration.
- **`x-twitter-pp-cli webhooks get`** - Get a list of webhook configs associated with a client app.
- **`x-twitter-pp-cli webhooks validate`** - Triggers a CRC check for a given webhook.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
x-twitter-pp-cli communities search --query example-value

# JSON for scripting and agents
x-twitter-pp-cli communities search --query example-value --json

# Filter to specific fields
x-twitter-pp-cli communities search --query example-value --json --select id,name,status

# Dry run — show the request without sending
x-twitter-pp-cli communities search --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
x-twitter-pp-cli communities search --query example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending. Under `--agent`, mutation dry-runs return JSON with `sent:false`, method/path/body, selected profile, auth lane, mutation classification, and public action when known.
- **Debuggable** - `raw <method> <path-or-url>` calls an allowlisted X API path through the same auth, dry-run, retry, masking, and error-classification pipeline as generated commands while preserving the upstream response body. Use it only when no generated command fits yet.
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

Verifies configuration, credentials, and connectivity to the API.

```bash
x-twitter-pp-cli doctor
```

## Raw API Escape Hatch

Use `raw` for debugging new X endpoints or reproducing auth/API failures before a generated command exists. It accepts relative API paths or allowlisted HTTPS X absolute URLs, repeatable query params and headers, and JSON bodies from `--body`, `--body-file`, or `--body @-`.

```bash
x-twitter-pp-cli raw GET /2/users/me --agent
x-twitter-pp-cli raw GET /2/tweets --param ids=123,456 --json
x-twitter-pp-cli raw POST /2/tweets --body '{"text":"hello"}' --dry-run --agent
```

Generated commands remain the default for normal workflows because they carry endpoint-specific validation, pagination, local-cache provenance, and workflow hints. `raw` uses the same auth lanes as the rest of the CLI: app-only public reads still need `X_BEARER_TOKEN`; user-context reads and writes still need `X_OAUTH2_USER_TOKEN` or an imported OAuth2 user-context token.

## Configuration

Config file: `~/.config/x-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `X_BEARER_TOKEN` | per_call | Yes | Set to your API credential. |
| `X_OAUTH2_USER_TOKEN` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `x-twitter-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `x-twitter-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $X_BEARER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 with code 453 / "subset of X API v2 endpoints"** — Your app's access tier is below what the endpoint requires; check your tier in the X Developer Console (https://console.x.com/).
- **403 client-not-enrolled** — Your app is not attached to a Project. Attach it to a Project in the Developer Portal, then retry.
- **403 on a reply, quote-tweet, or @mention post** — X's Feb-2026 restriction blocks programmatic replies/quotes/cold mentions; use self-reply threads (thread compose) instead, which still post.
- **402 Payment Required** — Pay-per-use credit or spend limit exhausted; raise the limit or add credit in the Developer Console.
- **403 on a write or personal read with only a bearer token set** — App-only bearer tokens can't write or read 'me' data; set X_OAUTH2_USER_TOKEN for user-context operations.
- **Need to verify a user-context token non-interactively** — Run `x-twitter-pp-cli auth import-oauth2 --access-token <token> --scopes ... --json` for access-token-only checks, or include `--client-id <oauth2-client-id> --refresh-token <refresh-token>` when storing refresh credentials; then run `x-twitter-pp-cli users get-me --agent` and `x-twitter-pp-cli doctor --json`. Browser cookies from `auth login --chrome` do not satisfy these v2 API calls.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tweepy**](https://github.com/tweepy/tweepy) — Python (10800 stars)
- [**x-cli**](https://github.com/sferik/x-cli) — Rust (5600 stars)
- [**node-twitter-api-v2**](https://github.com/PLhery/node-twitter-api-v2) — TypeScript (2600 stars)
- [**x-article-publisher-skill**](https://github.com/wshuyi/x-article-publisher-skill) — Python (788 stars)
- [**twitter-mcp**](https://github.com/EnesCinr/twitter-mcp) — TypeScript (396 stars)
- [**XActions**](https://github.com/nirholas/XActions) — JavaScript (297 stars)
- [**mcp-twitter-server**](https://github.com/crazyrabbitLTC/mcp-twitter-server) — TypeScript (23 stars)
- [**x-autonomous-mcp**](https://github.com/JohannesHoppe/x-autonomous-mcp) — TypeScript (2 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
