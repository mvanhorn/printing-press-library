# X (Twitter) CLI

Combined CLI for multiple API services

Learn more at [X](https://docs.x.com/x-api).

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/cmd/x-twitter-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/x-twitter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-x-twitter --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-x-twitter --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-x-twitter skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-x-twitter. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/x-twitter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `X_OAUTH2_USER_TOKEN` when Claude Desktop prompts you.

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
    "x": {
      "command": "x-twitter-pp-mcp",
      "env": {
        "X_OAUTH2_USER_TOKEN": "<your-key>"
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

Get your access token from your API provider's developer portal, then store it:

```bash
x-twitter-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export X_OAUTH2_USER_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
x-twitter-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
x-twitter-pp-cli articles list
```

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
- **`x-twitter-pp-cli activity stream`** - Stream of X Activities
- **`x-twitter-pp-cli activity update-subscription`** - Updates a subscription for an X activity event

### articles

X Articles (long-form posts) authoring + media upload

- **`x-twitter-pp-cli articles create_draft`** - POST /i/api/graphql/g1l5N8BxGewYuCy5USe_bQ/ArticleEntityDraftCreate
- **`x-twitter-pp-cli articles delete`** - POST /i/api/graphql/e4lWqB6m2TA8Fn_j9L9xEA/ArticleEntityDelete
- **`x-twitter-pp-cli articles list`** - GET /i/api/graphql/N1zzFzRPspT-sP9Q42n_bg/ArticleEntitiesSlice
- **`x-twitter-pp-cli articles publish`** - POST /i/api/graphql/m4SHicYMoWO_qkLvjhDk7Q/ArticleEntityPublish
- **`x-twitter-pp-cli articles set-cover`** - Upload an image and set it as an X Article cover
- **`x-twitter-pp-cli articles unpublish`** - POST /i/api/graphql/WbeMAOZdMHilHrqhgpjObw/ArticleEntityUnpublish
- **`x-twitter-pp-cli articles update_content`** - POST /i/api/graphql/M7N2FrPrlOmu-YrVIBxFnQ/ArticleEntityUpdateContent
- **`x-twitter-pp-cli articles update_cover_media`** - POST /i/api/graphql/Es8InPh7mEkK9PxclxFAVQ/ArticleEntityUpdateCoverMedia
- **`x-twitter-pp-cli articles update_title`** - POST /i/api/graphql/x75E2ABzm8_mGTg1bz8hcA/ArticleEntityUpdateTitle
- **`x-twitter-pp-cli articles upload_media`** - POST /i/media/upload.json

### chat

Manage chat

- **`x-twitter-pp-cli chat add-group-members`** - Adds one or more members to an existing encrypted Chat group conversation, rotating the conversation key.
- **`x-twitter-pp-cli chat create-conversation`** - Creates a new encrypted Chat group conversation on behalf of the authenticated user.
- **`x-twitter-pp-cli chat get-conversation`** - Retrieves messages and key change events for a specific Chat conversation with pagination support. For 1:1 conversations, provide the recipient's user ID; the server constructs the canonical conversation ID from the authenticated user and recipient.
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

- **`x-twitter-pp-cli evaluate-note evaluate-community-notes`** - Endpoint to evaluate a community note.

### insights

Manage insights

- **`x-twitter-pp-cli insights get-historical`** - Retrieves historical engagement metrics for specified Posts within a defined time range.
- **`x-twitter-pp-cli insights get-insights28-hr`** - Retrieves engagement metrics for specified Posts over the last 28 hours.

### likes

Manage likes

- **`x-twitter-pp-cli likes stream-compliance`** - Streams all compliance data related to Likes for Users.
- **`x-twitter-pp-cli likes stream-firehose`** - Streams all public Likes in real-time.
- **`x-twitter-pp-cli likes stream-sample10`** - Streams a 10% sample of public Likes in real-time.

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

- **`x-twitter-pp-cli openapi-json get-open-api-spec`** - Retrieves the full OpenAPI Specification in JSON format. (See https://github.com/OAI/OpenAPI-Specification/blob/master/README.md)

### spaces

Endpoints related to retrieving, managing Spaces

- **`x-twitter-pp-cli spaces get-by-creator-ids`** - Retrieves details of Spaces created by specified User IDs.
- **`x-twitter-pp-cli spaces get-by-id`** - Retrieves details of a specific space by its ID.
- **`x-twitter-pp-cli spaces get-by-ids`** - Retrieves details of multiple Spaces by their IDs.
- **`x-twitter-pp-cli spaces search`** - Retrieves a list of Spaces matching the specified search query.

### trends

Manage trends

- **`x-twitter-pp-cli trends get-by-woeid`** - Retrieves trending topics for a specific location identified by its WOEID.

### tweets

Endpoints related to retrieving, searching, and modifying Tweets

- **`x-twitter-pp-cli tweets create-posts`** - Creates a new Post for the authenticated user, or edits an existing Post when edit_options are provided. Supports paid partnership disclosure via the paid_partnership field.
- **`x-twitter-pp-cli tweets create-webhooks-stream-link`** - Creates a link to deliver FilteredStream events to the given webhook.
- **`x-twitter-pp-cli tweets delete-posts`** - Deletes a specific Post by its ID, if owned by the authenticated user.
- **`x-twitter-pp-cli tweets delete-webhooks-stream-link`** - Deletes a link from FilteredStream events to the given webhook.
- **`x-twitter-pp-cli tweets get-posts-analytics`** - Retrieves analytics data for specified Posts within a defined time range.
- **`x-twitter-pp-cli tweets get-posts-by-id`** - Retrieves details of a specific Post by its ID.
- **`x-twitter-pp-cli tweets get-posts-by-ids`** - Retrieves details of multiple Posts by their IDs.
- **`x-twitter-pp-cli tweets get-posts-counts-recent`** - Retrieves the count of Posts from the last 7 days matching a search query.
- **`x-twitter-pp-cli tweets get-webhooks-stream-links`** - Get a list of webhook links associated with a filtered stream ruleset.
- **`x-twitter-pp-cli tweets search-posts-recent`** - Retrieves Posts from the last 7 days matching a search query.
- **`x-twitter-pp-cli tweets stream-labels-compliance`** - Streams all labeling events applied to Posts.
- **`x-twitter-pp-cli tweets stream-posts-compliance`** - Streams all compliance data related to Posts.
- **`x-twitter-pp-cli tweets stream-posts-firehose`** - Streams all public Posts in real-time.
- **`x-twitter-pp-cli tweets stream-posts-firehose-en`** - Streams all public English-language Posts in real-time.
- **`x-twitter-pp-cli tweets stream-posts-firehose-ja`** - Streams all public Japanese-language Posts in real-time.
- **`x-twitter-pp-cli tweets stream-posts-firehose-ko`** - Streams all public Korean-language Posts in real-time.
- **`x-twitter-pp-cli tweets stream-posts-firehose-pt`** - Streams all public Portuguese-language Posts in real-time.
- **`x-twitter-pp-cli tweets stream-posts-sample`** - Streams a 1% sample of public Posts in real-time.
- **`x-twitter-pp-cli tweets stream-posts-sample10`** - Streams a 10% sample of public Posts in real-time.

### usage

Manage usage

- **`x-twitter-pp-cli usage get`** - Retrieves usage statistics for Posts over a specified number of days.

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
- **`x-twitter-pp-cli users search`** - Retrieves a list of Users matching a search query.
- **`x-twitter-pp-cli users stream-compliance`** - Streams all compliance data related to Users.

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
x-twitter-pp-cli articles list

# JSON for scripting and agents
x-twitter-pp-cli articles list --json

# Filter to specific fields
x-twitter-pp-cli articles list --json --select id,name,status

# Dry run — show the request without sending
x-twitter-pp-cli articles list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
x-twitter-pp-cli articles list --agent
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
x-twitter-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/x-twitter-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `X_OAUTH2_USER_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `x-twitter-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $X_OAUTH2_USER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
