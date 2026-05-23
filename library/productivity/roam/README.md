# Roam CLI

**Every Roam HQ surface — chat, transcripts, On-Air events, SCIM, webhooks — in one local-first CLI with offline FTS search no other tool has.**

Roam HQ ships a remote MCP but no CLI. roam-pp-cli unifies all five Roam HQ APIs (HQ, On-Air, Chat, SCIM, Webhooks) into a single binary with a local SQLite cache and FTS5 search across messages and transcripts. Cron-friendly chat relay, decision extraction, attendance drift, and SCIM roster diff are built-in.

## Install

The recommended path installs both the `roam-pp-cli` binary and the `pp-roam` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install roam
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install roam --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install roam --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install roam --agent claude-code
npx -y @mvanhorn/printing-press-library install roam --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/roam/cmd/roam-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/roam-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-roam --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-roam --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-roam skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-roam. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/roam-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ROAM_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "roam": {
      "command": "roam-pp-mcp",
      "env": {
        "ROAM_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Bearer auth via ROAM_API_KEY. Roam supports two key tiers: full-access organization keys and personal access tokens. Some HQ v1 endpoints require full-access; Chat/Transcript v0 endpoints work with PATs. Run `roam-pp-cli doctor token` to see which families your key can reach.

## Quick Start

```bash
# Verify which Roam APIs your key tier can reach before anything else.
roam-pp-cli doctor token

# Pull the last week of chat + transcripts into the local store for offline search.
roam-pp-cli sync --since 7d

# Cross-resource FTS5 search over messages and transcripts.
roam-pp-cli grep "pricing" --since 14d --json

# Extract decision-shaped lines from synced transcripts.
roam-pp-cli decisions --since 7d

# Pipe CI output to a Roam group with dedupe + Retry-After backoff.
echo "Deploy v1.2.3 succeeded" | roam-pp-cli relay --to eng-deploys --idempotent-key-prefix deploys

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`grep`** — Search across every chat message and meeting transcript at once with --since, --from-user, --in-meeting, --in-group filters.

  _Reach for this when an agent needs to recall what was said across many meetings without re-paging the rate-limited transcript API._

  ```bash
  roam-pp-cli grep "pricing" --since 14d --in-group eng --json --select transcript_id,line
  ```
- **`decisions`** — Surface decision-shaped lines ("we decided", "action item", "agreed", "let's go with") from synced transcripts.

  _Use when an agent owes the team a Monday recap; deterministic and citation-bearing._

  ```bash
  roam-pp-cli decisions --since 7d --in-group product --agent
  ```
- **`onair-attendance-drift`** — Compare invited guests vs actual attendance for an On-Air event; print invited-no-show and walk-in sets.

  _Use when an agent is asked who didn't show or who attended without an invite._

  ```bash
  roam-pp-cli onair-attendance-drift --event evt_123 --json
  ```
- **`webhook-tail`** — Tail recent webhook deliveries from the local subscription registry, --since filtered.

  _Use when debugging a webhook integration without standing up a listener._

  ```bash
  roam-pp-cli webhook-tail --since 1h --json
  ```
- **`mention-inbox`** — Local FTS over messages.text for @user tokens with --since filter; tail format.

  _Use when an agent must surface unread mentions across all groups without round-tripping the API per group._

  ```bash
  roam-pp-cli mention-inbox --user @me --since 7d --agent
  ```

### Mutation safety
- **`onair-reaper`** — Find recurring On-Air events with zero attendance over N days. --apply cancels them via the absorbed cancel endpoint.

  _Use when cleaning up dead recurring events; safe by default._

  ```bash
  roam-pp-cli onair-reaper --stale-days 60 --dry-run
  ```
- **`scim-diff`** — Diff a CSV/JSON HRIS roster against /Users SCIM list; print add/update/remove sets. --apply runs SCIM CRUD.

  _Use when an agent needs to reconcile an external roster with Roam membership without clicking through admin UI._

  ```bash
  roam-pp-cli scim-diff --roster hris.csv --apply --dry-run
  ```

### Agent-native plumbing
- **`transcript-fanout`** — Run a single question against every transcript in a date range; one row per transcript with answer + citation.

  _Use when an agent must scan many meetings for a single question without re-prompting each one._

  ```bash
  roam-pp-cli transcript-fanout --question "did anyone mention Q3 hiring?" --since 30d --agent
  ```
- **`relay`** — Pipe arbitrary stdin lines into a Roam group via /chat.post with deterministic idempotency keys and 429 backoff.

  _Use when an agent needs to forward a stream (CI, alerts, logs) into Roam without writing a custom webhook._

  ```bash
  tail -F deploys.log | roam-pp-cli relay --to eng-deploys --idempotent-key-prefix deploys
  ```

### Reachability mitigation
- **`doctor token`** — Probe one representative GET per spec family (HQ, On-Air, Chat, SCIM, Webhooks) and print which families this key can reach.

  _Use when an agent needs to know which Roam commands will work with the credential it has before attempting them._

  ```bash
  roam-pp-cli doctor token
  ```

## Usage

Run `roam-pp-cli --help` for the full command reference and flag list.

## Commands

### addr-info

Manage addr info

- **`roam-pp-cli addr-info addr_info`** - Get information about a chat address, which is the name for any entity that
may participate in a chat, such as a user, visitor, or bot.

**Access:** Organization only.

**Required scope:** `chat:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### app-uninstall

Manage app uninstall

- **`roam-pp-cli app-uninstall uninstall-app`** - Revoke an access token and uninstall your app.
On successful response, your access token will no longer be recognized.

This operation is only valid for OAuth access tokens, not for API keys.

**Access:** Organization only.

**No specific scope required.**

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### chat-delete

Manage chat delete

- **`roam-pp-cli chat-delete chat_delete`** - Delete a previously posted bot message. The bot must own the message being deleted (matched by address ID). Personal access tokens are not supported for this endpoint.

Deleting an already-deleted message is idempotent and returns success.

**Access:** Organization only.

**Required scope:** `chat:send_message` or `chat:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### chat-history

Manage chat history

- **`roam-pp-cli chat-history chat_history`** - List messages in a chat, filtered by date range (after/before).

The ordering of results depends on the filter specified:

- When no parameters are provided, the most recent recordings are returned,
  sorted in reverse chronological order. This is equivalent to specifying `before`
  as NOW and leaving `after` unspecified.

- If `after` is specified, the results are sorted in forward chronological order.

Either dates or datetimes may be specified. Dates are interpreted in UTC.

Note that the API operates in UTC with respect to the date range filter.

**Access:** Organization and Personal. In Personal mode, only chats where the authenticated user is a participant are accessible.

**Required scope:** `chat:history`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### chat-list

Manage chat list

- **`roam-pp-cli chat-list chat_list`** - List all accessible chats, which consist of all DMs, MultiDMs, and Channels
that your bot has been added to, in addition to all public groups regardless
of membership.

Chats are returned in reverse chronological order of the chat's start
timestamp, so the first page of results contains the most recently started
chats.

**Access:** Organization only.

**Required scope:** `chat:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### chat-post

Manage chat post

- **`roam-pp-cli chat-post chat_post`** - Post a message to a chat. Messages can be plain markdown text, rich [Block Kit](/docs/guides/block-kit) layouts, or polls.

Messages may be posted to a chat, a group, or one or more addresses (e.g. users, bots).

Mentions are supported with the syntax `<@USER_ID>`, e.g. `<!@U-7861a4c6-765a-495d-898d-fae3d8fbba2d>` or `<@all>` to notify everyone in the chat.
When rendered in the client, the tag will automatically be replaced with the human-readable display name (or "everyone" for `<@all>`).

**Access:** Organization and Personal. In Organization mode, messages are sent with the app's bot persona. In Personal mode, messages are sent as the authenticated user.

**Required scope:** `chat:send_message` or `chat:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### chat-send-message

Manage chat send message

- **`roam-pp-cli chat-send-message send-message`** - Sends the given message to the specified recipients. At this time, we only
support sending to a single group recipient. You can obtain the group ID on the "Group
Settings" page. That means `recipients` will always be a single-element array containing
a single UUIDv4.

The `sender` JSON object contains three (optional) string fields: `id`, `name`, and
`imageUrl`. The `id` is an internal name of the bot you wish to send as, `name` is the
user-visible bot name (that is seen when viewing groups), and `imageUrl` is the image
URL of the bot. Generally, you only need to specify `id` and `name` (e.g.
`datadog` as `id` and `Datadog Alerts` as `name`). If omitted, the ID and
sender are inferred to be the API client name.

## Message Text Format

At this time, we do not fully support standard Markdown, but support is forthcoming. To
indicate a line break, you must use TWO new line characters in the message (e.g.. `\n\n`).

**Access:** Organization only.

**Required scope:** `chat:send_message`

---

**OpenAPI Spec:** [openapi.json](https://developer.ro.am/openapi.json)

### chat-typing

Manage chat typing

- **`roam-pp-cli chat-typing chat_typing`** - Notify other chat participants that you are working on a response.
If they have the chat open, they will see "(Bot name) is typing...".

**Access:** Organization only.

**Required scope:** `chat:send_message` or `chat:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### chat-update

Manage chat update

- **`roam-pp-cli chat-update chat_update`** - Edit a previously posted bot message. The updated message can contain plain markdown text or rich [Block Kit](/docs/guides/block-kit) layouts.

The bot must own the message being updated (matched by address ID). Personal access tokens are not supported for this endpoint.

**Access:** Organization only.

**Required scope:** `chat:send_message` or `chat:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### group-add

Manage group add

- **`roam-pp-cli group-add group_add`** - Add one or more group members and/or admins.

Apps may add members to a group if one of the following conditions is true:
1. It is a public group in their Roam.
2. They are a member of the group.

If attempting to add an admin, the app must be an admin of the group.

**Access:** Organization only.

**Required scope:** `group:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### group-archive

Manage group archive

- **`roam-pp-cli group-archive group_archive`** - Archive a group by ID.

**Access:** Organization only.

**Required scope:** `group:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### group-create

Manage group create

- **`roam-pp-cli group-create group_create`** - Create a group address that can be used for chat.

Groups which specify an admin will operate in an "Admin only" management
mode, where only admins may change settings. Otherwise, all members have
that capability.

Groups require at least one member or admin.
Admins appear only in the admin list, not in both.

**Access:** Organization only.

**Required scope:** `group:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### group-members

Manage group members

- **`roam-pp-cli group-members group_members`** - List members in a group.

Apps may list members if one of the following conditions is true:
1. It is a public group in their Roam.
2. They are a member of the group.

**Access:** Organization only.

**Required scope:** `groups:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### group-remove

Manage group remove

- **`roam-pp-cli group-remove group_remove`** - Remove one or more group members.

Apps may remove members from a group if one of the following conditions is true:
1. It is a public group in their Roam.
2. They are a member of the group.

Removing members with the Admin role is not yet supported.

**Access:** Organization only.

**Required scope:** `group:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### group-rename

Manage group rename

- **`roam-pp-cli group-rename group_rename`** - Rename a group by ID.

Apps may only rename groups for which they are an admin.

**Access:** Organization only.

**Required scope:** `group:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### groups

Manage groups

- **`roam-pp-cli groups 02-create`** - Creates a new group in your Roam organization.

**Required fields:** `displayName` (max 64 characters).

**Optional:** Include `members` array with user IDs to add initial members.

See [RFC 7644 §3.3](https://www.rfc-editor.org/rfc/rfc7644#section-3.3) for SCIM resource creation.
- **`roam-pp-cli groups 02-delete`** - Archives a group in Roam. This is a **soft delete** — the group data is retained but becomes inactive.

**Roam-specific behavior:**
- The group is marked as archived
- Members are removed from the group
- To permanently delete group data, contact Roam support

See [RFC 7644 §3.6](https://www.rfc-editor.org/rfc/rfc7644#section-3.6) for SCIM resource deletion.
- **`roam-pp-cli groups 02-get`** - Retrieves a single group by its Roam Address ID.

The response includes the group's `displayName` and `members` array containing user IDs.

See [RFC 7644 §3.4.1](https://www.rfc-editor.org/rfc/rfc7644#section-3.4.1) for SCIM resource retrieval.
- **`roam-pp-cli groups 02-list`** - Returns a paginated list of groups in your Roam organization.

**Pagination:** Use `startIndex` (1-based) and `count` to paginate through results.

**Note:** The `filter` parameter is accepted but currently ignored for Groups.

See [RFC 7644 §3.4.2](https://www.rfc-editor.org/rfc/rfc7644#section-3.4.2) for SCIM filtering and pagination.
- **`roam-pp-cli groups 02-patch`** - Partially updates a group. Use this to add or remove members without replacing the entire group.

**Supported operations:**
- `add` / `remove` / `replace` for `members`
- `replace` for `displayName`

**IdP compatibility:**
- Okta-style: `path: "members[value eq \"user-id\"]"` with `op: "remove"`
- Entra-style: `path: "members"` with `value: [{ "value": "user-id" }]`

See [RFC 7644 §3.5.2](https://www.rfc-editor.org/rfc/rfc7644#section-3.5.2) for SCIM PATCH operations.
- **`roam-pp-cli groups 02-replace`** - Fully replaces a group's attributes. The entire `members` list is replaced with the provided values.

**Required fields:** `displayName` (max 64 characters).

**Note:** This is a full replacement — any members not included in the request will be removed from the group.

See [RFC 7644 §3.5.1](https://www.rfc-editor.org/rfc/rfc7644#section-3.5.1) for SCIM resource replacement.

### groups-list

Manage groups list

- **`roam-pp-cli groups-list list-groups`** - Lists all public, non-archived groups in your home Roam.

**Access:** Organization only.

**Required scope:** `groups:read`

---

**OpenAPI Spec:** [openapi.json](https://developer.ro.am/openapi.json)

### item-upload

Manage item upload

- **`roam-pp-cli item-upload item_upload`** - Upload a file so that it can be sent as a chat message attachment.
The returned object contains an item ID which can be used with [chat.post](/docs/chat-api/chat-post).

Unlike other endpoints, this uses raw binary upload with metadata in HTTP headers
rather than JSON. This is more efficient for file transfers.

**Limits:**
- Maximum file size: 10 MB

**Supported Content Types:**

| Content-Type | In-Product Behavior |
|--------------|---------------------|
| `image/png`, `image/jpeg`, `image/gif`, `image/webp` | Displayed inline with preview thumbnail |
| `application/octet-stream` | Download link only (no preview) |

**Important:** Use `application/octet-stream` for **any file type not listed above** (e.g., `.txt`, `.docx`, `.xlsx`, `.zip`, `.pdf`, etc.).
These files will be stored and downloadable, but won't have in-product preview functionality.

**Validation:**
- The `Content-Type` header must match the actual file content (server validates this for images)
- For images, if the filename lacks the correct extension, it will be appended automatically

**Access:** Organization only.

**Required scope:** `item:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### lobby-booking-list

Manage lobby booking list

- **`roam-pp-cli lobby-booking-list lobby_booking_list`** - Lists bookings for a specific lobby configuration, filtered by date range (after/before).

The ordering of results depends on the filter specified:

- When no parameters are provided, the most recent bookings are returned,
  sorted in reverse chronological order. This is equivalent to specifying `before`
  as NOW and leaving `after` unspecified.

- If `after` is specified, the results are sorted in forward chronological order.

Either dates or datetimes may be specified. Dates are interpreted in UTC.

**Access:** Organization only.

**Required scope:** `lobby:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### lobby-list

Manage lobby list

- **`roam-pp-cli lobby-list lobby_list`** - Lists active lobbies in your account.

A lobby URL has the form `ro.am/{handle}` or `ro.am/{handle}/{slug}`.
- The "handle" is the first path segment
- The "slug" is the optional second path segment. It may be empty for the default lobby under a handle

Optionally filter by a specific lobby handle. If provided, only lobbies
associated with that handle are returned.

**Access:** Organization only.

**Required scope:** `lobby:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### magicast-info

Manage magicast info

- **`roam-pp-cli magicast-info get-magicast`** - Retrieve a magicast by ID.

**Access:** Organization and Personal. In Personal mode, only magicasts owned by the authenticated user are accessible.

**Required scope:** `magicast:read`

---

**OpenAPI Spec:** [openapi.json](https://developer.ro.am/openapi.json)

### magicast-list

Manage magicast list

- **`roam-pp-cli magicast-list list-magicasts`** - Lists all magicasts in your Roam, sorted in reverse chronological order.

**Access:** Organization and Personal. In Personal mode, only magicasts owned by the authenticated user are returned.

**Required scope:** `magicast:read`

---

**OpenAPI Spec:** [openapi.json](https://developer.ro.am/openapi.json)

### meeting-list

Manage meeting list

- **`roam-pp-cli meeting-list meeting_list`** - Lists all meetings in your home Roam, filtered by date range (after/before).

The ordering of results depends on the filter specified:

- When no parameters are provided, the most recent meetings are returned,
  sorted in reverse chronological order. This is equivalent to specifying `before`
  as NOW and leaving `after` unspecified.

- If `after` is specified, the results are sorted in forward chronological order.

Either dates or datetimes may be specified. Dates are interpreted in UTC.

**Access:** Organization only.

**Required scope:** `meetings:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### meetinglink-create

Manage meetinglink create

- **`roam-pp-cli meetinglink-create meetinglink_create`** - Create a meeting link.

**Access:** Organization and Personal. In Organization mode, specify the host by email. In Personal mode, the host defaults to the authenticated user.

**Required scope:** `meeting:write` or `meetinglink:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### meetinglink-info

Manage meetinglink info

- **`roam-pp-cli meetinglink-info meetinglink_info`** - Get a meeting link.

**Access:** Organization only.

**Required scope:** `meetinglink:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### meetinglink-update

Manage meetinglink update

- **`roam-pp-cli meetinglink-update meetinglink_update`** - Update a meeting link.

**Access:** Organization only.

**Required scope:** `meetinglink:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### messageevent-export

Manage messageevent export

- **`roam-pp-cli messageevent-export messageevent_export`** - Obtain a daily message event export containing DMs and group
chats within your account.

For customers with archival enabled (please reach out to a Roam
ArchiTech to get this process started), at the end of every day,
we export all message events for a particular day as a JSON Lines file.
This file contains all messages sent:
- by a Roam user who is a member of your organization
- into a chat containing (at the time of export) at least one Roam user who is a member of your organization
- by a bot integration that is part of your organization

This file also contains message edit and deletion events that meet the above criteria.
We specifically exclude waves, room invitations, and other non-message content
(that may appear as chats within the Roam application) from the export.

**Access:** Organization only.

**Required scope:** `admin:compliance:read`

### Message Event Structure

Each line within the file is a JSON object containing the following fields:
- eventType: a string that is one of “sent”, “edited”, or “deleted”
- chatId: a UUIDv4 identifier for a particular chat. All messages within the same chat shared the same chatId.
- threadTimestamp (optional): if part of a thread, the Unix epoch timestamp of the thread’s parent message in numerical format. All messages part of a thread share the same threadTimestamp.
- timestamp: the Unix epoch timestamp when the message was originally sent in numerical format.
- messageId: an internal UUIDv4 identifier as a string
- sender: a “Participant” object that identifiers the message sender
- contentType: a string that is one of the contentTypes associated with the “MessageContent” object
- content: a “MessageContent” object that contains the message’s content
### Participant

A Participant is a JSON object that contains three common fields: “participantType”, “id”, and “displayName”
- participantType: one of “email”, “bot”, or “occupant”
- id: a UUID identifier for the participant
- displayName: the name associated with the account or an empty string if not provided

Depending on the participant type, the object also contains additional fields:

Email Participant (a human user with a Roam user account)
- email: the email of the participant

Bot Participant (an automated user maintained by the Roam team or created via the Roam API)
- roamId: the roam ID associated with the integration
- integrationId: a unique integration ID name provided by the bot creator
- botCode: a unique identifier

### Message Content

A “MessageContent” object is a JSON object that contains the field “contentType” and,
depending on the content type, contains additional fields:

*Text Content* (contentType = “text”)
- text: the text in plaintext
- markdownText: the text in Markdown format
- attachments: A list of attachment objects 

*Emoji Content* (contentType = “emoji”)
- text: text representation of the emoji
- colons: emoji in :emoji: format
- fileUrl: an optional field containing the URL to a custom emoji image

*Item Content* (contentType = “item”)
- itemUrl: the URL where the file can be downloaded from
- itemType: the type of item (e.g. "photo", "pdf", "blob", "video", "audio", etc.)

*Text Snippet Content* (contentType = "textSnippet")
- text: the content of the snippet
- language: the language of the snippet

*Members Changed Content* (contentType = “membersChanged”)
- added: a list of Participant objects corresponding to all participants added in this event
- removed: a list of Participant objects corresponding to all participants removed in this event

---

**OpenAPI Spec:** [openapi.json](https://developer.ro.am/openapi.json)

### onair-attendance-list

Manage onair attendance list

- **`roam-pp-cli onair-attendance-list onair_attendance_list`** - Returns the attendance report for an On-Air event, combining RSVP data with
join/duration information. Guests with RSVP records are listed first,
followed by any additional attendees who joined without an RSVP.

**Access:** Organization and Personal.

**Required scope:** `onair:read`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-event-cancel

Manage onair event cancel

- **`roam-pp-cli onair-event-cancel onair_event_cancel`** - Cancels an On-Air event. This action cannot be undone.

**Access:** Organization and Personal.

**Required scope:** `onair:write`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-event-create

Manage onair event create

- **`roam-pp-cli onair-event-create onair_event_create`** - Creates a new On-Air event. The calendar host must be a member of the organization.
When using a personal access token, `calendarHostEmail` must match the authenticated user.

Optionally provide display hosts in the `hosts` array. Display hosts appear on
the event page and are independent of the calendar host.

**Access:** Organization and Personal.

**Required scope:** `onair:write`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-event-info

Manage onair event info

- **`roam-pp-cli onair-event-info onair_event_info`** - Returns details for a single On-Air event.

**Access:** Organization and Personal.

**Required scope:** `onair:read`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-event-list

Manage onair event list

- **`roam-pp-cli onair-event-list onair_event_list`** - Returns a paginated list of On-Air events for the organization.

Results are sorted by start time in descending order (most recent first).

**Access:** Organization and Personal.

**Required scope:** `onair:read`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-event-update

Manage onair event update

- **`roam-pp-cli onair-event-update onair_event_update`** - Updates an existing On-Air event. Only the fields provided in the request body
are updated; omitted fields remain unchanged.

**Access:** Organization and Personal.

**Required scope:** `onair:write`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-guest-add

Manage onair guest add

- **`roam-pp-cli onair-guest-add onair_guest_add`** - Adds one or more guests to an On-Air event.

**Access:** Organization and Personal.

**Required scope:** `onair:write`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-guest-info

Manage onair guest info

- **`roam-pp-cli onair-guest-info onair_guest_info`** - Returns details for a single On-Air event guest.

**Access:** Organization and Personal.

**Required scope:** `onair:read`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-guest-list

Manage onair guest list

- **`roam-pp-cli onair-guest-list onair_guest_list`** - Returns a paginated list of guests for an On-Air event. Optionally filter by RSVP status.

Results are sorted by creation time in descending order (most recently added first).

**Access:** Organization and Personal.

**Required scope:** `onair:read`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-guest-remove

Manage onair guest remove

- **`roam-pp-cli onair-guest-remove onair_guest_remove`** - Removes a guest from an On-Air event.

**Access:** Organization and Personal.

**Required scope:** `onair:write`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### onair-guest-update

Manage onair guest update

- **`roam-pp-cli onair-guest-update onair_guest_update`** - Updates the RSVP status of a guest.

**Access:** Organization and Personal.

**Required scope:** `onair:write`

---

**OpenAPI Spec:** [onair.json](https://developer.ro.am/onair.json)

### reaction-add

Manage reaction add

- **`roam-pp-cli reaction-add reaction_add`** - Add a reaction to a message in a chat.

**Access:** Organization only.

**Required scope:** `chat:send_message` or `chat:write`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### recording-list

Manage recording list

- **`roam-pp-cli recording-list list-recordings`** - Lists all recordings in your home Roam, filtered by date range (after/before).

The ordering of results depends on the filter specified:

- When no parameters are provided, the most recent recordings are returned,
  sorted in reverse chronological order. This is equivalent to specifying `before`
  as NOW and leaving `after` unspecified.

- If `after` is specified, the results are sorted in forward chronological order.

Either dates or datetimes may be specified. Dates are interpreted in UTC.

**Access:** Organization only.

**Required scope:** `recordings:read`

---

**OpenAPI Spec:** [openapi.json](https://developer.ro.am/openapi.json)

### resource-types

Manage resource types

- **`roam-pp-cli resource-types 03-metadata`** - Returns the list of resource types supported by Roam: `User` and `Group`.

**No authentication required** for this discovery endpoint.

See [RFC 7644 §4](https://www.rfc-editor.org/rfc/rfc7644#section-4) for the SCIM resource types specification.

### schemas

Manage schemas

- **`roam-pp-cli schemas 03-metadata-get`** - Returns the definition of a specific SCIM schema by its URN identifier.

**No authentication required** for this discovery endpoint.

Common schema IDs:
- `urn:ietf:params:scim:schemas:core:2.0:User`
- `urn:ietf:params:scim:schemas:core:2.0:Group`
- `urn:ro.am:params:scim:schemas:extension:roam:2.0:User`
- **`roam-pp-cli schemas 03-metadata-list`** - Returns all SCIM schemas supported by Roam, including the core User and Group schemas plus Roam's custom role extension (`urn:ro.am:params:scim:schemas:extension:roam:2.0:User`).

**No authentication required** for this discovery endpoint.

See [RFC 7644 §4](https://www.rfc-editor.org/rfc/rfc7644#section-4) for the SCIM schemas specification.

### service-provider-config

Manage service provider config

- **`roam-pp-cli service-provider-config 03-metadata`** - Returns Roam's SCIM capabilities and supported features. Use this endpoint to discover which SCIM operations are supported.

**No authentication required** for this discovery endpoint.

See [RFC 7644 §4](https://www.rfc-editor.org/rfc/rfc7644#section-4) for the SCIM service provider configuration specification.

### test

Manage test

- **`roam-pp-cli test hq-get`** - Test endpoint

### token-info

Manage token info

- **`roam-pp-cli token-info token_info`** - Get information about the access token, such as the Chat Address.

**Access:** Organization and Personal.

**No specific scope required.**

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### transcript-info

Manage transcript info

- **`roam-pp-cli transcript-info get-transcript`** - Retrieve a transcript by ID from your home Roam. Works for both completed and live (ongoing) meetings.

For live meetings (during Magic Minutes), the `end` field will be empty. Use the `sinceOffset` parameter to poll for incremental transcript updates — only cues with `startOffset` greater than or equal to the given value are returned.

**Access:** Organization and Personal. In Personal mode, only transcripts from meetings where the authenticated user was a participant are accessible.

**Required scope:** `transcript:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### transcript-list

Manage transcript list

- **`roam-pp-cli transcript-list list-transcripts`** - Lists all transcripts in your home Roam, filtered by date range (after/before).

This endpoint returns transcript metadata only. To retrieve the full transcript
content including cues (speaker text), summary, and action items, use
[`/transcript.info`](/docs/chat-api/get-transcript).

The ordering of results depends on the filter specified:

- When no parameters are provided, the most recent recordings are returned,
  sorted in reverse chronological order. This is equivalent to specifying `before`
  as NOW and leaving `after` unspecified.

- If `after` is specified, the results are sorted in forward chronological order.

Either dates or datetimes may be specified. Dates are interpreted in UTC.

Note that the API operates in UTC with respect to the date range filter.

**Access:** Organization and Personal. In Personal mode, only transcripts from meetings where the authenticated user was a participant are returned.

**Required scope:** `transcript:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### transcript-prompt

Manage transcript prompt

- **`roam-pp-cli transcript-prompt prompt-transcript`** - Ask a question about a specific meeting transcript and receive an
AI-generated answer based on its content.

Use [`/transcript.info`](/docs/chat-api/get-transcript) to retrieve the
transcript ID, or [`/transcript.list`](/docs/chat-api/list-transcripts) to
browse available transcripts.

**Required scope:** `transcript:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### user-info

Manage user info

- **`roam-pp-cli user-info user_info`** - Get detailed information about a single user by ID.

**Access:** Organization only.

**Required scope:** `user:read` (add `user:read.email` to include email address, `user:read.status` to expand presence status and availability)

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### user-list

Manage user list

- **`roam-pp-cli user-list user_list`** - List all users in the account.

Users are returned in the order they were added to the account.

**Access:** Organization or Personal.

**Required scope:** `user:read` (add `user:read.email` to include email addresses, `user:read.status` to expand presence status)

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### user-lookup

Manage user lookup

- **`roam-pp-cli user-lookup user_lookup`** - Look up users in the account by email.

**Access:** Organization only.

**Required scopes:** `user:read` and `user:read.email`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### userauditlog-list

Manage userauditlog list

- **`roam-pp-cli userauditlog-list userauditlog_list`** - Get a list of user audit log entries for the account.

**Access:** Organization only.

**Required scope:** `userauditlog:read`

---

**OpenAPI Spec:** [chat.json](https://developer.ro.am/chat.json)

### users

Manage users

- **`roam-pp-cli users 01-create`** - Provisions a new user in your Roam organization.

**Required fields:** `userName`, `name.givenName`, `name.familyName`, and `emails[0].value`.

**Roam-specific behavior:**
- `userName` must exactly match the primary email address
- `displayName` is auto-generated from givenName + familyName (any provided value is ignored)
- Email/userName becomes **immutable** after creation
- Use the `urn:ro.am:params:scim:schemas:extension:roam:2.0:User` extension to set `role` ("User" or "Admin")

See [RFC 7644 §3.3](https://www.rfc-editor.org/rfc/rfc7644#section-3.3) for SCIM resource creation.
- **`roam-pp-cli users 01-delete`** - Archives a user in Roam. This is a **soft delete** — the user's data is retained but they lose access.

**Roam-specific behavior:**
- The user is marked as archived (equivalent to `active: false`)
- Archived users can be reactivated via PUT or PATCH with `active: true`
- To permanently delete user data, contact Roam support

See [RFC 7644 §3.6](https://www.rfc-editor.org/rfc/rfc7644#section-3.6) for SCIM resource deletion.
- **`roam-pp-cli users 01-get`** - Retrieves a single user by their Roam Person ID.

The response includes all user attributes, the `active` status, and the Roam role extension.

See [RFC 7644 §3.4.1](https://www.rfc-editor.org/rfc/rfc7644#section-3.4.1) for SCIM resource retrieval.
- **`roam-pp-cli users 01-list`** - Returns a paginated list of users in your Roam organization.

**Filtering:** Supports SCIM filter expressions, e.g., `filter=userName eq "alice@example.com"`.

**Pagination:** Use `startIndex` (1-based) and `count` to paginate through results.

See [RFC 7644 §3.4.2](https://www.rfc-editor.org/rfc/rfc7644#section-3.4.2) for SCIM filtering and pagination.
- **`roam-pp-cli users 01-patch`** - Partially updates a user. Roam supports SCIM PATCH for Users with **limited semantics**.

**Supported operations:**
- `replace` of `active` only (to archive or reactivate a user)

**Not supported:**
- `add` / `remove` operations
- `replace` for any other attribute

To update other fields like name, use PUT (Replace User) instead.

See [RFC 7644 §3.5.2](https://www.rfc-editor.org/rfc/rfc7644#section-3.5.2) for SCIM PATCH operations.
- **`roam-pp-cli users 01-replace`** - Fully replaces a user's attributes. You must provide all required fields in the request body.

**Roam-specific behavior:**
- `userName` and `emails` are **immutable** — any attempt to change them returns a 400 error
- You can update `name.givenName`, `name.familyName`, `active`, `externalId`, and `role`
- Set `active: false` to archive the user; `active: true` to reactivate

See [RFC 7644 §3.5.1](https://www.rfc-editor.org/rfc/rfc7644#section-3.5.1) for SCIM resource replacement.

### webhook-subscribe

Manage webhook subscribe

- **`roam-pp-cli webhook-subscribe webhook_subscribe`** - Create or update a webhook subscription for a given event. If a subscription
already exists for the same event and URL, its filter is updated instead of
creating a duplicate.

**Access:** Organization only.

**Required scope:** `webhook:write`

---

**OpenAPI Spec:** [webhooks.json](https://developer.ro.am/webhooks.json)

### webhook-unsubscribe

Manage webhook unsubscribe

- **`roam-pp-cli webhook-unsubscribe webhook_unsubscribe`** - Remove a webhook subscription by ID.

**Access:** Organization only.

**Required scope:** `webhook:write`

---

**OpenAPI Spec:** [webhooks.json](https://developer.ro.am/webhooks.json)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
roam-pp-cli addr-info

# JSON for scripting and agents
roam-pp-cli addr-info --json

# Filter to specific fields
roam-pp-cli addr-info --json --select id,name,status

# Dry run — show the request without sending
roam-pp-cli addr-info --dry-run

# Agent mode — JSON + compact + no prompts in one flag
roam-pp-cli addr-info --agent
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
roam-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/roam-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ROAM_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `roam-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ROAM_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **403 "This endpoint requires full access. Personal access tokens are not supported."** — Run `roam-pp-cli doctor token` to see which spec families your token covers; use an organization-level key for HQ v1 endpoints.
- **429 with Retry-After** — The adaptive limiter honors Retry-After automatically. Reduce concurrent batches or use --idempotent-key-prefix for safe retries.
- **Empty grep results after sync** — Run `roam-pp-cli sync --full transcripts messages` — incremental sync may not have populated yet.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
