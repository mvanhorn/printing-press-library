# Zulip CLI

Powerful open source group chat

Learn more at [Zulip](https://zulip.com).

Printed by [@haqiramadhani](https://github.com/haqiramadhani) (Haqi Ramadhani).

## Install

The recommended path installs both the `zulip-pp-cli` binary and the `pp-zulip` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install zulip
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install zulip --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install zulip --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install zulip --agent claude-code
npx -y @mvanhorn/printing-press install zulip --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zulip-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zulip --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zulip --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-zulip skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-zulip. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zulip-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zulip": {
      "command": "zulip-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
zulip-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
zulip-pp-cli attachments get
```

## Usage

Run `zulip-pp-cli --help` for the full command reference and flag list.

## Commands

### attachments

Manage attachments

- **`zulip-pp-cli attachments get`** - Fetch metadata on files uploaded by the requesting user.
- **`zulip-pp-cli attachments remove`** - Delete an uploaded file given its attachment ID.

Note that uploaded files that have been referenced in at least
one message are automatically deleted once the last message
containing a link to them is deleted (whether directly or via
a [message retention policy](/help/message-retention-policy)).

Uploaded files that are never used in a message are
automatically deleted a few weeks after being uploaded.

Attachment IDs can be contained from [GET /attachments](/api/get-attachments).

### bot-storage

Manage bot storage

- **`zulip-pp-cli bot-storage get`** - !!! warn ""

    **Note:** This endpoint is only available to [bot user](/help/bots-overview)
    accounts.

Retrieve [data stored](/help/interactive-bots-api#bot_handlerstorage)
for a bot user.

**Changes**: Prior to Zulip 12.0 (feature level 494), users who
were not bots could access this endpoint.
- **`zulip-pp-cli bot-storage remove`** - !!! warn ""

    **Note:** This endpoint is only available to [bot user](/help/bots-overview)
    accounts.

Delete [data stored](/help/interactive-bots-api#bot_handlerstorage)
for a bot user.

**Changes**: Prior to Zulip 12.0 (feature level 494), users who
were not bots could access this endpoint.
- **`zulip-pp-cli bot-storage update`** - !!! warn ""

    **Note:** This endpoint is only available to [bot user](/help/bots-overview)
    accounts.

Add or update [data stored](/help/interactive-bots-api#bot_handlerstorage)
for a bot user.

Each bot has a limited storage set by the server, which normally is a
default of 10,000,000 characters.

**Changes**: Prior to Zulip 12.0 (feature level 494), users who
were not bots could access this endpoint.

### bots

Manage bots


### calls

Manage calls

- **`zulip-pp-cli calls create-big-blue-button-video`** - Create a video call URL for a BigBlueButton video call.
Requires [BigBlueButton 2.4+](/integrations/big-blue-button)
to be configured on the Zulip server.

The acting user will be given the moderator role on the call.

**Changes**: Prior to Zulip 10.0 (feature level 337), every
user was given the moderator role on BigBlueButton calls, via
encoding a moderator password in the generated URLs.
- **`zulip-pp-cli calls create-constructor-groups-video`** - Create a video call URL for a Constructor Groups video call.
Requires [Constructor Groups](/integrations/constructor-groups)
to be configured on the Zulip server.

**Changes**: New in Zulip 12.0 (feature level 460).
- **`zulip-pp-cli calls create-nextcloud-talk-video`** - Create a video call URL for a Nextcloud Talk video call. Requires
[Nextcloud Talk](/integrations/nextcloud-talk) to be configured on the
Zulip server.

**Changes**: New in Zulip 12.0 (feature level 465).
- **`zulip-pp-cli calls create-webex-video`** - Create a video call URL for a Webex video call.

Requires [Webex integration](/integrations/webex) to be configured
on the Zulip server.

Clients should confirm that the user has completed the OAuth process
with Webex and has a Webex token before attempting to create a video
call URL. See the `has_webex_token` field in the [`POST /register`
response](/api/register-queue), as well as the [`has_webex_token`
event type](/api/get-events#has_webex_token).

**Changes**: New in Zulip 12.0 (feature level 493).

### channel-folders

Manage channel folders

- **`zulip-pp-cli channel-folders create`** - Create a new [channel folder](/help/channel-folders).

**Changes**: New in Zulip 11.0 (feature level 389).
- **`zulip-pp-cli channel-folders get`** - Fetches all of the [channel folders](/help/channel-folders) in the
organization, sorted by the `order` field.

**Changes**: Before Zulip 11.0 (feature level 414), the list of channel
folders was sorted by ID as the `order` field didn't exist.

New in Zulip 11.0 (feature level 389).
- **`zulip-pp-cli channel-folders patch`** - Reorder the [channel folders](/help/channel-folders) in the user's
organization.

Channel folders are displayed in Zulip UI in order; this endpoint allows
administrative settings UI to change the ordering of channel folders.

This endpoint is used to implement the dragging feature described in the
[manage channel folders documentation](/help/manage-channel-folders).

**Changes**: New in Zulip 11.0 (feature level 414).
- **`zulip-pp-cli channel-folders update`** - Update the name or description of a [channel folder](/help/channel-folders)
with the specified ID.

This endpoint is also used to archive or unarchive the specified channel
folder.

**Changes**: New in Zulip 11.0 (feature level 389).

### channels

Manage channels

- **`zulip-pp-cli channels`** - Create a new [channel](/help/create-channels), and optionally subscribe
users to the newly created channel.

The initial [channel settings](/api/update-stream) will be determined
by the optional parameters, like `invite_only`, detailed below.

**Changes**: New in Zulip 11.0 (feature level 417). Previously, this was
only possible via the [`POST /api/subscribe`](/api/subscribe) endpoint,
which handles both channel subscription and creation.

### default-streams

Manage default streams

- **`zulip-pp-cli default-streams add`** - Add a channel to the set of [default channels][default-channels]
for new users joining the organization.

[default-channels]: /help/set-default-channels-for-new-users
- **`zulip-pp-cli default-streams remove`** - Remove a channel from the set of [default channels][default-channels]
for new users joining the organization.

[default-channels]: /help/set-default-channels-for-new-users

### dev-fetch-api-key

Manage dev fetch api key

- **`zulip-pp-cli dev-fetch-api-key`** - For easy testing of mobile apps and other clients and against Zulip
development servers, we support fetching a Zulip API key for any user
on the development server without authentication (so that they can
implement analogues of the one-click login process available for Zulip
development servers on the web).

!!! warn ""

    **Note:** This endpoint is only available on Zulip development
    servers; for obvious security reasons it will always return an error
    in a Zulip production server.

### dev-list-users

Manage dev list users

- **`zulip-pp-cli dev-list-users`** - Get a list of all, non-bot users in a [Zulip development
server](https://zulip.readthedocs.io/en/latest/development/overview.html).
This endpoint is used by mobile developers to fetch users for the
development login flow.

!!! warn ""

    **Note:** This endpoint is only available on Zulip development
    servers; for obvious security reasons it will always return an error
    in a Zulip production server.

### drafts

Manage drafts

- **`zulip-pp-cli drafts create`** - Create one or more drafts on the server. These drafts will be automatically
synchronized to other clients via `drafts` events.
- **`zulip-pp-cli drafts delete`** - Delete a single draft from the server. The deletion will be automatically
synchronized to other clients via a `drafts` event.
- **`zulip-pp-cli drafts edit`** - Edit a draft on the server. The edit will be automatically
synchronized to other clients via `drafts` events.
- **`zulip-pp-cli drafts get`** - Fetch all drafts for the current user.

### events

Manage events

- **`zulip-pp-cli events delete-queue`** - Delete a previously registered queue.
- **`zulip-pp-cli events get`** - This endpoint allows you to receive new events from
[a registered event queue](/api/register-queue).

Long-lived clients should use the
`event_queue_longpoll_timeout_seconds` property returned by
`POST /register` as the client-side HTTP request timeout for
calls to this endpoint. It is guaranteed to be higher than
heartbeat timeout and should be respected by clients to
avoid breaking when heartbeat timeout increases.

### fetch-api-key

Manage fetch api key

- **`zulip-pp-cli fetch-api-key`** - This API endpoint is used by clients such as the Zulip mobile and
terminal apps to implement password-based authentication. Given the
user's Zulip login credentials, it returns a Zulip API key that the client
can use to make requests as the user.

This endpoint is only useful for Zulip servers/organizations with
EmailAuthBackend or LDAPAuthBackend enabled.

The Zulip mobile apps also support SSO/social authentication (GitHub
auth, Google auth, SAML, etc.) that does not use this endpoint. Instead,
the mobile apps reuse the web login flow passing the `mobile_flow_otp` in
a webview, and the credentials are returned to the app (encrypted) via a redirect
to a `zulip://` URL.

!!! warn ""

    **Note:** If you signed up using passwordless authentication and
    never had a password, you can [reset your password](/help/change-your-password).

See the [API keys](/api/api-keys) documentation for more details
on how to download an API key manually.

In a [Zulip development environment](https://zulip.readthedocs.io/en/latest/development/overview.html),
see also [the unauthenticated variant](/api/dev-fetch-api-key).

### get-stream-id

Manage get stream id

- **`zulip-pp-cli get-stream-id`** - Get the unique ID of a given channel.

### invites

Manage invites

- **`zulip-pp-cli invites create-link`** - Create a [reusable invitation link](/help/invite-new-users#create-a-reusable-invitation-link)
which can be used to invite new users to the organization.

**Changes**: In Zulip 8.0 (feature level 209), added support for non-admin
users [with permission](/help/restrict-account-creation#change-who-can-send-invitations)
to use this endpoint. Previously, it was restricted to administrators only.

In Zulip 6.0 (feature level 126), the `invite_expires_in_days`
parameter was removed and replaced by `invite_expires_in_minutes`.

In Zulip 5.0 (feature level 117), added support for passing `null` as
the `invite_expires_in_days` parameter to request an invitation that never
expires.

In Zulip 5.0 (feature level 96), the `invite_expires_in_days` parameter was
added which specified the number of days before the invitation would expire.
- **`zulip-pp-cli invites get`** - Fetch all unexpired [invitations](/help/invite-new-users) (i.e. email
invitations and reusable invitation links) that can be managed by the user.

Note that administrators can manage invitations that were created by other users.

**Changes**: Prior to Zulip 8.0 (feature level 209), non-admin users could
only create email invitations, and therefore the response would never include
reusable invitation links for these users.
- **`zulip-pp-cli invites revoke-email`** - Revoke an [email invitation](/help/invite-new-users#send-email-invitations).

A user can only revoke [invitations that they can
manage](/help/invite-new-users#manage-pending-invitations).
- **`zulip-pp-cli invites revoke-link`** - Revoke a [reusable invitation link](/help/invite-new-users#create-a-reusable-invitation-link).

A user can only revoke [invitations that they can
manage](/help/invite-new-users#manage-pending-invitations).

**Changes**: Prior to Zulip 8.0 (feature level 209), only organization
administrators were able to create and revoke reusable invitation links.
- **`zulip-pp-cli invites send`** - Send [invitations](/help/invite-new-users) to specified email addresses.

**Changes**: In Zulip 6.0 (feature level 126), the `invite_expires_in_days`
parameter was removed and replaced by `invite_expires_in_minutes`.

In Zulip 5.0 (feature level 117), added support for passing `null` as
the `invite_expires_in_days` parameter to request an invitation that never
expires.

In Zulip 5.0 (feature level 96), the `invite_expires_in_days` parameter was
added which specified the number of days before the invitation would expire.

### jwt

Manage jwt

- **`zulip-pp-cli jwt`** - This API endpoint is used by clients to implement JSON Web Token
(JWT) authentication. Given a JWT identifying a Zulip user, it
returns a Zulip API key that the client can use to make requests
as the user.

!!! warn ""

    **Note:** This endpoint is only useful for Zulip servers/organizations
    with [JSON web token authentication][prod-jwt-auth] enabled.

See the [API keys](/api/api-keys) documentation for more details
on how to manage API keys manually.

**Changes**: New in Zulip 7.0 (feature level 160).

[prod-jwt-auth]: https://zulip.readthedocs.io/en/latest/production/authentication-methods.html#json-web-tokens-jwt

### mark-all-as-read

Manage mark all as read

- **`zulip-pp-cli mark-all-as-read`** - Marks all of the current user's unread messages as read.

Because this endpoint marks messages as read in batches, it is possible
for the request to time out after only marking some messages as read.
When this happens, the `complete` boolean field in the success response
will be `false`. Clients should repeat the request when handling such a
response. If all messages were marked as read, then the success response
will return `"complete": true`.

**Changes**: Deprecated; clients should use the [update personal message
flags for narrow](/api/update-message-flags-for-narrow) endpoint instead
as this endpoint will be removed in a future release.

Before Zulip 8.0 (feature level 211), if the server's
processing was interrupted by a timeout, but some messages were marked
as read, then it would return `"result": "partially_completed"`, along
with a `code` field for an error string, in the success response to
indicate that there was a timeout and that the client should repeat the
request.

Before Zulip 6.0 (feature level 153), this request did a single atomic
operation, which could time out with 10,000s of unread messages to mark
as read. As of this feature level, messages are marked as read in
batches, starting with the newest messages, so that progress is made
even if the request times out. And, instead of returning an error when
the request times out and some messages have been marked as read, a
success response with `"result": "partially_completed"` is returned.

### mark-stream-as-read

Manage mark stream as read

- **`zulip-pp-cli mark-stream-as-read`** - Mark all the unread messages in a channel as read.

**Changes**: Deprecated; clients should use the [update personal message
flags for narrow](/api/update-message-flags-for-narrow) endpoint instead
as this endpoint will be removed in a future release.

### mark-topic-as-read

Manage mark topic as read

- **`zulip-pp-cli mark-topic-as-read`** - Mark all the unread messages in a topic as read.

**Changes**: Deprecated; clients should use the [update personal message
flags for narrow](/api/update-message-flags-for-narrow) endpoint instead
as this endpoint will be removed in a future release.

### messages

Manage messages

- **`zulip-pp-cli messages check-match-narrow`** - Check whether a set of messages match a [narrow](/api/construct-narrow).

For many common narrows (e.g. a topic), clients can write an efficient
client-side check to determine whether a newly arrived message belongs
in the view.

This endpoint is designed to allow clients to handle more complex narrows
for which the client does not (or in the case of full-text search, cannot)
implement this check.

The format of the `match_subject` and `match_content` objects is designed
to match those returned by the [`GET /messages`](/api/get-messages#response)
endpoint, so that a client can splice these fields into a `message` object
received from [`GET /events`](/api/get-events#message) and end up with an
extended message object identical to how a [`GET /messages`](/api/get-messages)
request for the current narrow would have returned the message.
- **`zulip-pp-cli messages delete`** - Permanently delete a message.

This API corresponds to the [delete a message completely][delete-completely]
feature documented in the Zulip help center.

A user must be able to access the content of a message in order to delete it.
See [channel permissions](/help/channel-permissions) for more information
about content access for channel messages. For direct messages, the user
must have received or sent the direct message to have content access.

See [restricting message deletion](/help/restrict-message-editing-and-deletion)
for documentation on when users are allowed to delete messages.

The relevant realm settings in the API that are related to the above linked
documentation on when users are allowed to delete messages are:

- `realm_can_delete_any_message_group`
- `realm_can_delete_own_message_group`
- `realm_can_set_delete_message_policy_group`
- `realm_message_content_delete_limit_seconds`

The relevant per-channel permission settings in the API that are related to the
above linked documentation on when users are allowed to delete messages in a
specific channel are:

- `can_delete_any_message_group`
- `can_delete_own_message_group`

More details about these realm and channel settings can be found in the
[`POST /register`](/api/register-queue) response.

**Changes**: Prior to Zulip 10.0 (feature level 281), only organization
administrators had permission to permanently delete a message.

[delete-completely]: /help/delete-a-message#delete-a-message-completely
- **`zulip-pp-cli messages get`** - This endpoint is the primary way to fetch a messages. It is used by all official
Zulip clients (e.g. the web, desktop, mobile, and terminal clients) as well as
many bots, API clients, backup scripts, etc.

Most queries will specify a [narrow filter](/api/get-messages#parameter-narrow),
to fetch the messages matching any supported [search
query](/help/search-for-messages). If not specified, it will return messages
corresponding to the user's [combined feed](/help/combined-feed). There are two
ways to specify which messages matching the narrow filter to fetch:

- A range of messages, described by an `anchor` message ID (or a string-format
  specification of how the server should computer an anchor to use) and a maximum
  number of messages in each direction from that anchor.

- A rarely used variant (`message_ids`) where the client specifies the message IDs
  to fetch.

The server returns the matching messages, sorted by message ID, as well as some
metadata that makes it easy for a client to determine whether there are more
messages matching the query that were not returned due to the `num_before` and
`num_after` limits.

Note that a user's message history does not contain messages sent to
channels before they [subscribe](/api/subscribe), and newly created
bot users are not usually subscribed to any channels.

We recommend requesting at most 1000 messages in a batch, to avoid generating very
large HTTP responses. A maximum of 5000 messages can be obtained per request;
attempting to exceed this will result in an error.

**Changes**: The `message_ids` option is new in Zulip 10.0 (feature level 300).
- **`zulip-pp-cli messages get-messageid`** - Given a message ID, return the message object.

Additionally, a `raw_content` field is included. This field is
useful for clients that primarily work with HTML-rendered
messages but might need to occasionally fetch the message's
raw [Zulip-flavored Markdown](/help/format-your-message-using-markdown) (e.g. for [view
source](/help/view-the-markdown-source-of-a-message) or
prefilling a message edit textarea).

**Changes**: Before Zulip 5.0 (feature level 120), this
endpoint only returned the `raw_content` field.
- **`zulip-pp-cli messages render`** - Render a message to HTML.
- **`zulip-pp-cli messages send`** - Send a [channel message](/help/introduction-to-topics) or a
[direct message](/help/direct-messages).
- **`zulip-pp-cli messages update`** - Update the content, topic, or channel of the message with the specified
ID.

You can [resolve topics](/help/resolve-a-topic) by editing the topic to
`✔ {original_topic}` with the `propagate_mode` parameter set to
`"change_all"`.

See [configuring message editing][config-message-editing] for detailed
documentation on when users are allowed to edit message content, and
[restricting moving messages][restrict-move-messages] for detailed
documentation on when users are allowed to change a message's topic
and/or channel.

The relevant realm settings in the API that are related to the above
linked documentation on when users are allowed to update messages are:

- `allow_message_editing`
- `can_resolve_topics_group`
- `can_move_messages_between_channels_group`
- `can_move_messages_between_topics_group`
- `message_content_edit_limit_seconds`
- `move_messages_within_stream_limit_seconds`
- `move_messages_between_streams_limit_seconds`

More details about these realm settings can be found in the
[`POST /register`](/api/register-queue) response or in the documentation
of the [`realm op: update_dict`](/api/get-events#realm-update_dict)
event in [`GET /events`](/api/get-events).

**Changes**: Prior to Zulip 10.0 (feature level 367), the permission for
resolving a topic was managed by `can_move_messages_between_topics_group`.
As of this feature level, users belonging to the `can_resolve_topics_group`
will have the permission to [resolve topics](/help/resolve-a-topic) in the organization.

In Zulip 10.0 (feature level 316), `edit_topic_policy`
was removed and replaced by `can_move_messages_between_topics_group`
realm setting.

**Changes**: In Zulip 10.0 (feature level 310), `move_messages_between_streams_policy`
was removed and replaced by `can_move_messages_between_channels_group`
realm setting.

Prior to Zulip 7.0 (feature level 172), anyone could add a
topic to channel messages without a topic, regardless of the organization's
[topic editing permissions](/help/restrict-moving-messages). As of this
feature level, messages without topics have the same restrictions for
topic edits as messages with topics.

Before Zulip 7.0 (feature level 172), by using the `change_all` value for
the `propagate_mode` parameter, users could move messages after the
organization's configured time limits for changing a message's topic or
channel had passed. As of this feature level, the server will [return an
error](/api/update-message#response) with `"code":
"MOVE_MESSAGES_TIME_LIMIT_EXCEEDED"` if users, other than organization
administrators or moderators, try to move messages after these time
limits have passed.

Before Zulip 7.0 (feature level 162), users who were not administrators or
moderators could only edit topics if the target message was sent within the
last 3 days. As of this feature level, that time limit is now controlled by
the realm setting `move_messages_within_stream_limit_seconds`. Also at this
feature level, a similar time limit for moving messages between channels was
added, controlled by the realm setting
`move_messages_between_streams_limit_seconds`. Previously, all users who
had permission to move messages between channels did not have any time limit
restrictions when doing so.

Before Zulip 7.0 (feature level 159), editing channels and topics of messages
was forbidden if the realm setting for `allow_message_editing` was `false`,
regardless of an organization's configuration for the realm settings
`edit_topic_policy` or `move_messages_between_streams_policy`.

Before Zulip 7.0 (feature level 159), message senders were allowed to edit
the topic of their messages indefinitely.

In Zulip 5.0 (feature level 75), the `edit_topic_policy` realm setting
was added, replacing the `allow_community_topic_editing` boolean.

In Zulip 4.0 (feature level 56), the `move_messages_between_streams_policy`
realm setting was added.

[config-message-editing]: /help/restrict-message-editing-and-deletion
[restrict-move-messages]: /help/restrict-moving-messages
- **`zulip-pp-cli messages update-flags`** - Add or remove personal message flags like `read` and `starred`
on a collection of message IDs.

See also the endpoint for [updating flags on a range of
messages within a narrow](/api/update-message-flags-for-narrow).
- **`zulip-pp-cli messages update-flags-for-narrow`** - Add or remove personal message flags like `read` and `starred`
on a range of messages within a narrow.

See also [the endpoint for updating flags on specific message
IDs](/api/update-message-flags).

**Changes**: New in Zulip 6.0 (feature level 155).

### mobile-push

Manage mobile push

- **`zulip-pp-cli mobile-push e2ee-test-notify`** - Trigger sending an end-to-end encrypted (E2EE) test push notification
to the user's selected mobile device or all of their mobile devices.

**Changes**: New in Zulip 11.0 (feature level 420).
- **`zulip-pp-cli mobile-push register-push-device`** - Register a device to receive end-to-end encrypted mobile push notifications,
or update such a registration.

To perform an initial registration, clients must provide both the
push key fields (`push_key` and `push_key_id`) and the token fields
(`token_kind`, `token_id`, `bouncer_public_key`, and `encrypted_push_registration`).

Once registered, clients should use this endpoint to rotate `push_key` or
FCM/APNs provided token:

- **Rotate push key**: Provide only the push key fields.
- **Rotate token**: Provide only the token fields.

On a successful registration, the server automatically removes
any legacy push device registration with a matching token for
the user. For self-hosted servers, if removing the legacy
registration from the push notification bouncer fails (e.g.,
due to a network error), legacy notifications to that token
will continue until all of the user's legacy registrations
have been removed from the local server, at which point the
server will stop sending legacy notification requests to the
bouncer entirely.

**Changes**: In Zulip 12.0 (feature level 483),
the server began automatically removing legacy registrations
with a matching token on successful E2EE registration.

In Zulip 12.0 (feature level 468), the endpoint was significantly
redesigned to support rotation of `push_key` and token provided by FCM/APNs.

New in Zulip 11.0 (feature level 406).
- **`zulip-pp-cli mobile-push test-notify`** - Trigger sending a test push notification to the user's
selected mobile device or all of their mobile devices.

**Changes**: Deprecated in Zulip 11.0 (feature level 420).
Clients connecting to newer servers and with E2EE push
notifications support should use the
[Send an E2EE test notification to mobile device(s)](/api/e2ee-test-notify)
endpoint, as this endpoint will be removed in a future release.

Starting with Zulip 8.0 (feature level 234), test
notifications sent via this endpoint use `test` rather than
`test-by-device-token` in the `event` field. Also, as of this
feature level, all mobile push notifications now include a
`realm_name` field.

New in Zulip 8.0 (feature level 217).

### navigation-views

Manage navigation views

- **`zulip-pp-cli navigation-views add`** - Adds a new custom left sidebar navigation view configuration
for the current user.

This can be used both to configure built-in navigation views,
or to add new navigation views.

**Changes**: New in Zulip 11.0 (feature level 390).
- **`zulip-pp-cli navigation-views edit`** - Update the details of an existing configured navigation view,
such as its name or whether it's pinned.

**Changes**: New in Zulip 11.0 (feature level 390).
- **`zulip-pp-cli navigation-views get`** - Fetch all configured custom navigation views for the current user.

**Changes**: New in Zulip 11.0 (feature level 390).
- **`zulip-pp-cli navigation-views remove`** - Remove a navigation view.

**Changes**: New in Zulip 11.0 (feature level 390).

### real-time

Manage real time

- **`zulip-pp-cli real-time`** - (Ignored)

### realm

Manage realm

- **`zulip-pp-cli realm add-code-playground`** - Configure [code playgrounds](/help/code-blocks#code-playgrounds) for the organization.

**Changes**: New in Zulip 4.0 (feature level 49). A parameter encoding bug was
fixed in Zulip 4.0 (feature level 57).
- **`zulip-pp-cli realm add-domain`** - Add a domain to the set of allowed domains configured in the organization
for [user account email addresses](/help/restrict-account-creation).

**Changes**: Prior to Zulip 6.0 (feature level 143), organization
administrators who were not owners could access this endpoint.
- **`zulip-pp-cli realm add-linkifier`** - Configure [linkifiers](/help/add-a-custom-linkifier),
regular expression patterns that are automatically linkified when they
appear in messages and topics.
- **`zulip-pp-cli realm create-custom-profile-field`** - [Create a custom profile field](/help/custom-profile-fields#add-a-custom-profile-field) in the user's organization.
- **`zulip-pp-cli realm deactivate-custom-emoji`** - [Deactivate a custom emoji](/help/custom-emoji#deactivate-custom-emoji) from
the user's organization.

Users can only deactivate custom emoji that they added themselves except for
organization administrators, who can deactivate any custom emoji.

Note that deactivated emoji will still be visible in old messages, reactions,
user statuses and channel descriptions.

**Changes**: Before Zulip 8.0 (feature level 190), this endpoint returned an
HTTP status code of 400 when the emoji did not exist, instead of 404.
- **`zulip-pp-cli realm delete-domain`** - Remove the specified domain from the set of allowed domains configured in
the organization for [user account email addresses](/help/restrict-account-creation).

**Changes**: Prior to Zulip 6.0 (feature level 143), organization
administrators who were not owners could access this endpoint.
- **`zulip-pp-cli realm get-custom-emoji`** - Get all the custom emoji in the user's organization.
- **`zulip-pp-cli realm get-custom-profile-fields`** - Get all the [custom profile fields](/help/custom-profile-fields)
configured for the user's organization.
- **`zulip-pp-cli realm get-domains`** - Get the set of allowed domains configured in the organization for user
account email addresses.

As each Zulip user account is associated with an email address, organization
owners can [restrict new account creation (and email
changes)](/help/restrict-account-creation) to email addresses with these
domains.
- **`zulip-pp-cli realm get-linkifiers`** - List all of an organization's configured
[linkifiers](/help/add-a-custom-linkifier), regular
expression patterns that are automatically linkified when they appear
in messages and topics.

**Changes**: New in Zulip 4.0 (feature level 54). On older versions,
a similar `GET /realm/filters` endpoint was available with each entry in
a `[pattern, url_format, id]` tuple format.
- **`zulip-pp-cli realm get-presence`** - Get the presence information of all the users in an organization.

If the `CAN_ACCESS_ALL_USERS_GROUP_LIMITS_PRESENCE` server-level
setting is set to `true`, presence information of only accessible
users are returned.

Complete Zulip apps are recommended to fetch presence
information when they post their own state using the [`POST
/presence`](/api/update-presence) API endpoint.
- **`zulip-pp-cli realm patch-domain`** - Update whether subdomains are allowed in [user account email
addresses](/help/restrict-account-creation) for the specified domain.

**Changes**: Prior to Zulip 6.0 (feature level 143), organization
administrators who were not owners could access this endpoint.
- **`zulip-pp-cli realm remove-code-playground`** - Remove a [code playground](/help/code-blocks#code-playgrounds) previously
configured for an organization.

**Changes**: New in Zulip 4.0 (feature level 49).
- **`zulip-pp-cli realm remove-linkifier`** - Remove [linkifiers](/help/add-a-custom-linkifier), regular
expression patterns that are automatically linkified when they appear
in messages and topics.
- **`zulip-pp-cli realm reorder-custom-profile-fields`** - Reorder the custom profile fields in the user's organization.

Custom profile fields are displayed in Zulip UI widgets in order; this
endpoint allows administrative settings UI to change the field ordering.

This endpoint is used to implement the dragging feature described in the
[custom profile fields documentation](/help/custom-profile-fields).
- **`zulip-pp-cli realm reorder-linkifiers`** - Change the order that the regular expression patterns in the organization's
[linkifiers](/help/add-a-custom-linkifier) are matched in messages and topics.
Useful when defining linkifiers with overlapping patterns.

**Changes**: New in Zulip 8.0 (feature level 202). Before this feature level,
linkifiers were always processed in order by ID, which meant users would
need to delete and recreate them to reorder the list of linkifiers.
- **`zulip-pp-cli realm test-welcome-bot-custom-message`** - Sends a test Welcome Bot custom message to the acting administrator.
This allows administrators to preview how the custom welcome message will
appear when received by new users upon joining the organization.

**Changes**: New in Zulip 11.0 (feature level 416).
- **`zulip-pp-cli realm update-linkifier`** - Update a [linkifier](/help/add-a-custom-linkifier), regular
expression patterns that are automatically linkified when they appear
in messages and topics.

**Changes**: New in Zulip 4.0 (feature level 57).
- **`zulip-pp-cli realm update-user-settings-defaults`** - Change the [default values of settings][new-user-defaults] for new users
joining the organization. Essentially all
[personal preference settings](/api/update-settings) are supported.

This feature can be invaluable for customizing Zulip's default
settings for notifications or UI to be appropriate for how the
organization is using Zulip. (Note that this only supports
personal preference settings, like when to send push
notifications or what emoji set to use, not profile or
identity settings that naturally should be different for each user).

Note that this endpoint cannot, at present, be used to modify
settings for existing users in any way.

**Changes**: Removed `dense_mode` setting in Zulip 10.0 (feature level 364)
as we now have `web_font_size_px` and `web_line_height_percent`
settings for more control.

New in Zulip 5.0 (feature level 96). If any parameters sent in the
request are not supported by this endpoint, an
[`ignored_parameters_unsupported`][ignored-parameters] array will
be returned in the JSON success response.

[new-user-defaults]: /help/configure-default-new-user-settings
[ignored-parameters]: /api/rest-error-handling#ignored-parameters
- **`zulip-pp-cli realm upload-custom-emoji`** - This endpoint is used to upload a custom emoji for use in the user's
organization. Access to this endpoint depends on the
[organization's configuration](https://zulip.com/help/custom-emoji#change-who-can-add-custom-emoji).

### register

Manage register

- **`zulip-pp-cli register`** - This powerful endpoint can be used to register a Zulip "event queue"
(subscribed to certain types of "events", or updates to the messages
and other Zulip data the current user has access to), as well as to
fetch the current state of that data.

(`register` also powers the `call_on_each_event` Python API, and is
intended primarily for complex applications for which the more convenient
`call_on_each_event` API is insufficient).

This endpoint returns a `queue_id` and a `last_event_id`; these can be
used in subsequent calls to the
["events" endpoint](/api/get-events) to request events from
the Zulip server using long-polling.

The server will queue events for up to `idle_queue_timeout_secs`
seconds of inactivity. After that timeout, your event queue will be
garbage-collected. The server will send `heartbeat` events
every minute, which makes it easy to implement a robust client that
does not miss events unless the client loses network connectivity
with the Zulip server for longer than the configured timeout.

Once the server garbage-collects your event queue, the server will
[return an error](/api/get-events#bad_event_queue_id-errors)
with a code of `BAD_EVENT_QUEUE_ID` if you try to fetch events from
the event queue. Your software will need to handle that error
condition by re-initializing itself (e.g. this is what triggers your
browser reloading the Zulip web app when your laptop comes back online
after being offline for more than `idle_queue_timeout_secs` seconds).

When prototyping with this API, we recommend first calling `register`
with no `event_types` parameter to see all the available data from all
supported event types. Before using your client in production, you
should set appropriate `event_types` and `fetch_event_types` filters
so that your client only requests the data it needs. A few minutes
doing this often saves 90% of the total bandwidth and other resources
consumed by a client using this API.

See the [events system developer documentation][events-system-docs]
if you need deeper details about how the Zulip event queue system
works, avoids clients needing to worry about large classes of
potentially messy races, etc.

**Changes**: New in Zulip 12.0 (feature level 481),
the `idle_queue_timeout` request parameter and `idle_queue_timeout_secs`
response field were added to allow clients to configure
how long the server keeps an event queue alive during inactivity.

Removed `dense_mode` setting in Zulip 10.0 (feature level 364)
as we now have `web_font_size_px` and `web_line_height_percent`
settings for more control.

Before Zulip 7.0 (feature level 183), the
`realm_community_topic_editing_limit_seconds` property
was returned by the response. It was removed because it
had not been in use since the realm setting
`move_messages_within_stream_limit_seconds` was introduced
in feature level 162.

In Zulip 7.0 (feature level 163), the realm setting
`email_address_visibility` was removed. It was replaced by a [user
setting](/api/update-settings#parameter-email_address_visibility) with
a [realm user default][user-defaults], with the encoding of different
values preserved. Clients can support all versions by supporting the
current API and treating every user as having the realm's
`email_address_visibility` value.

[user-defaults]: /api/update-realm-user-settings-defaults#parameter-email_address_visibility
[events-system-docs]: https://zulip.readthedocs.io/en/latest/subsystems/events-system.html

### register-client-device

Manage register client device

- **`zulip-pp-cli register-client-device`** - Logged-in mobile devices use this endpoint as an initial step to
register themselves, before registering for E2EE push notifications.

This endpoint is currently not useful for clients other than mobile.

**Changes**: New in Zulip 12.0 (feature level 468).

### reminders

Manage reminders

- **`zulip-pp-cli reminders create-message`** - Schedule a reminder to be sent to the current user at the specified time. The reminder will link the relevant message.

**Changes**: New in Zulip 11.0 (feature level 381).
- **`zulip-pp-cli reminders delete`** - Delete, and therefore cancel sending, a previously [scheduled
reminder](/help/schedule-a-reminder).

**Changes**: New in Zulip 11.0 (feature level 399).
- **`zulip-pp-cli reminders get`** - Fetch all [reminders](/help/schedule-a-reminder) for the
current user.

Reminders are messages the user has scheduled to be sent in the
future to themself.

**Changes**: New in Zulip 11.0 (feature level 399).

### remotes

Manage remotes

- **`zulip-pp-cli remotes`** - Register a push device to bouncer to receive end-to-end encrypted
mobile push notifications.

Self-hosted servers use this endpoint to asynchronously register
a push device to the bouncer server after receiving a request from
the mobile client to [register E2EE push device](/api/register-push-device).

It is not meant to be used by mobile clients directly.

**Changes**: New in Zulip 11.0 (feature level 406).

### remove-client-device

Manage remove client device

- **`zulip-pp-cli remove-client-device`** - Mobile devices use this endpoint to remove their device record
registered using [`POST /register_client_device`](/api/register-client-device)
when the user logs out.

This endpoint is currently not useful for clients other than mobile.

**Changes**: New in Zulip 12.0 (feature level 470).

### rest-error-handling

Manage rest error handling

- **`zulip-pp-cli rest-error-handling`** - Common error to many endpoints

### saved-snippets

Manage saved snippets

- **`zulip-pp-cli saved-snippets create`** - Create a new saved snippet for the current user.

**Changes**: New in Zulip 10.0 (feature level 297).
- **`zulip-pp-cli saved-snippets delete`** - Delete a saved snippet.

**Changes**: New in Zulip 10.0 (feature level 297).
- **`zulip-pp-cli saved-snippets edit`** - Edit a saved snippet for the current user.

**Changes**: New in Zulip 10.0 (feature level 368).
- **`zulip-pp-cli saved-snippets get`** - Fetch all the saved snippets for the current user.

**Changes**: New in Zulip 10.0 (feature level 297).

### scheduled-messages

Manage scheduled messages

- **`zulip-pp-cli scheduled-messages create`** - Create a new [scheduled message](/help/schedule-a-message).

**Changes**: In Zulip 7.0 (feature level 184), moved support for
[editing a scheduled message](/api/update-scheduled-message) to a
separate API endpoint, which removed the `scheduled_message_id`
parameter from this endpoint.

New in Zulip 7.0 (feature level 179).
- **`zulip-pp-cli scheduled-messages delete`** - Delete, and therefore cancel sending, a previously [scheduled
message](/help/schedule-a-message).

**Changes**: New in Zulip 7.0 (feature level 173).
- **`zulip-pp-cli scheduled-messages get`** - Fetch all [scheduled messages](/help/schedule-a-message) for
the current user.

Scheduled messages are messages the user has scheduled to be
sent in the future via the send later feature.

**Changes**: New in Zulip 7.0 (feature level 173).
- **`zulip-pp-cli scheduled-messages update`** - Edit an existing [scheduled message](/help/schedule-a-message).

**Changes**: New in Zulip 7.0 (feature level 184).

### server-settings

Manage server settings

- **`zulip-pp-cli server-settings`** - Fetch global settings for a Zulip server.

**Note:** this endpoint does not require any authentication at all, and you can use it to check:

- If this is a Zulip server, and if so, what version of Zulip it's running.
- What a Zulip client (e.g. a mobile app or
  [zulip-terminal](https://github.com/zulip/zulip-terminal/)) needs to
  know in order to display a login prompt for the server (e.g. what
  authentication methods are available).

### settings

Manage settings

- **`zulip-pp-cli settings`** - This endpoint is used to edit the current user's settings.

When invoked by a realm admin, it supports bulk updates to
settings for specified users or members of user groups using
the `target_users` parameter.

**Changes**: Removed `dense_mode` setting in Zulip 10.0 (feature level 364)
as we now have `web_font_size_px` and `web_line_height_percent`
settings for more control.

Prior to Zulip 5.0 (feature level 80), this endpoint only
supported the `full_name`, `email`, `old_password`, and
`new_password` parameters. Notification settings were
managed by `PATCH /settings/notifications`, and all other
settings by `PATCH /settings/display`.

The feature level 80 migration to merge these endpoints did not
change how request parameters are encoded. However, it did change
the handling of any invalid parameters present in a request
(see feature level 78 change below).

As of feature level 80, the `PATCH /settings/display` and
`PATCH /settings/notifications` endpoints are deprecated aliases
for this endpoint for backwards-compatibility, and will be removed
once clients have migrated to use this endpoint.

Prior to Zulip 5.0 (feature level 78), this endpoint indicated
which parameters it had processed by including in the response
object `"key": value` entries for values successfully changed by
the request. That was replaced by the more ergonomic
[`ignored_parameters_unsupported`][ignored-parameters] array.

The `PATCH /settings/notifications` and `PATCH /settings/display`
endpoints also had this behavior of indicating processed parameters
before they became aliases of this endpoint in Zulip 5.0 (see
feature level 80 change above).

Before feature level 78, request parameters that were not supported
(or were unchanged) were silently ignored.

[ignored-parameters]: /api/rest-error-handling#ignored-parameters

### streams

Manage streams

- **`zulip-pp-cli streams archive`** - [Archive the channel](/help/archive-a-channel) with the ID `stream_id`.
- **`zulip-pp-cli streams get`** - Get all channels that the user [has access to](/help/channel-permissions).
- **`zulip-pp-cli streams get-by-id`** - Fetch details for the channel with the ID `stream_id`.

**Changes**: Before Zulip 12.0 (feature level 480), this
endpoint was not supported for archived channels.

New in Zulip 6.0 (feature level 132).
- **`zulip-pp-cli streams update`** - Configure the channel with the ID `stream_id`. This endpoint supports
an organization administrator editing any property of a channel,
including:

- Channel [name](/help/rename-a-channel) and [description](/help/change-the-channel-description)
- Channel [permissions](/help/channel-permissions), including
  [privacy](/help/change-the-privacy-of-a-channel) and [who can
  send](/help/channel-posting-policy).

Note that an organization administrator's ability to change a
[private channel's permissions](/help/channel-permissions#private-channels)
depends on them being subscribed to the channel.

**Changes**: Before Zulip 10.0 (feature level 362), channel privacy could not be
edited for archived channels.

Removed `stream_post_policy` and `is_announcement_only`
parameters in Zulip 10.0 (feature level 333), as permission to post
in the channel is now controlled by `can_send_message_group`.

### thumbnail

Manage thumbnail

- **`zulip-pp-cli thumbnail <realm_id_str> <filename>`** - Check whether a thumbnail exists for a specific file uploaded by a user.
This endpoint is intended to be polled by clients to determine when
thumbnail generation is complete.

**Changes**: New in Zulip 12.0 (feature level 479).

### typing

Manage typing

- **`zulip-pp-cli typing`** - Notify other users whether the current user is
[typing a message][help-typing].

Clients implementing Zulip's typing notifications
protocol should work as follows:

- Send a request to this endpoint with `"op": "start"` when a user
  starts composing a message.
- While the user continues to actively type or otherwise interact with
  the compose UI (e.g. interacting with the compose box emoji picker),
  send regular `"op": "start"` requests to this endpoint, using
  `server_typing_started_wait_period_milliseconds` in the
  [`POST /register`][api-register] response as the time interval
  between each request.
- Send a request to this endpoint with `"op": "stop"` when a user
  has stopped using the compose UI for the time period indicated by
  `server_typing_stopped_wait_period_milliseconds` in the
  [`POST /register`][api-register] response or when a user
  cancels the compose action (if it had previously sent a "start"
  notification for that compose action).
- Start displaying a visual typing indicator for a given conversation
  when a [`typing op:start`][start-typing] event is received
  from the server.
- Continue displaying a visual typing indicator for the conversation
  until a [`typing op:stop`][stop-typing] event is received
  from the server or the time period indicated by
  `server_typing_started_expiry_period_milliseconds` in the
  [`POST /register`][api-register] response has passed without
  a new `typing "op": "start"` event for the conversation.

This protocol is designed to allow the server-side typing notifications
implementation to be stateless while being resilient as network failures
will not result in a user being incorrectly displayed as perpetually
typing.

See the subsystems documentation on [typing indicators][typing-protocol-docs]
for additional design details on Zulip's typing notifications protocol.

**Changes**: Clients shouldn't care about the APIs prior to Zulip 8.0 (feature level 215)
for channel typing notifications, as no client actually implemented
the previous API for those.

Support for displaying channel typing notifications was new
in Zulip 4.0 (feature level 58). Clients should indicate they support
processing channel typing notifications via the `stream_typing_notifications`
value in the `client_capabilities` parameter of the
[`POST /register`][client-capabilities] endpoint.

[help-typing]: /help/typing-notifications
[api-register]: /api/register-queue
[start-typing]: /api/get-events#typing-start
[stop-typing]: /api/get-events#typing-stop
[client-capabilities]: /api/register-queue#parameter-client_capabilities
[typing-protocol-docs]: https://zulip.readthedocs.io/en/latest/subsystems/typing-indicators.html

### user-groups

Manage user groups

- **`zulip-pp-cli user-groups create`** - Create a new [user group](/help/user-groups).

**Changes**: Prior to Zulip 12.0 (feature level 496), bot
users were not permitted to call this endpoint.
- **`zulip-pp-cli user-groups get`** - Fetches all of the user groups in the organization.

!!! warn ""

    **Note**: This endpoint is not available to
    [guest users](/help/user-roles).

**Changes**: Prior to Zulip 12.0 (feature level 496), bot
users were not permitted to call this endpoint.
- **`zulip-pp-cli user-groups update`** - Update the name, description or any of the permission settings
of a [user group](/help/user-groups).

This endpoint is also used to reactivate a user group.

Note that while permissions settings of deactivated groups can
be edited by this API endpoint, and those permissions settings
do affect the ability to modify the deactivated group and its
membership, the deactivated group itself cannot be mentioned
or used in the value of any permission without first being reactivated.

**Changes**: Prior to Zulip 12.0 (feature level 496), bot
users were not permitted to call this endpoint.

Starting with Zulip 11.0 (feature level 386), this
endpoint can be used to reactivate a user group.

Prior to Zulip 10.0 (feature level 340), only the name field
of deactivated groups could be modified.

### user-topics

Manage user topics

- **`zulip-pp-cli user-topics`** - This endpoint is used to update the personal preferences for a topic,
such as the topic's visibility policy, which is used to implement
[mute a topic](/help/mute-a-topic) and related features.

This endpoint can be used to update the visibility policy for the single
channel and topic pair indicated by the parameters for a user.

**Changes**: New in Zulip 7.0 (feature level 170). Previously,
toggling whether a topic was muted or unmuted was managed by the
[PATCH /users/me/subscriptions/muted_topics](/api/mute-topic) endpoint.

### user-uploads

Manage user uploads

- **`zulip-pp-cli user-uploads get-file-temporary-url`** - Get a temporary URL for access to an [uploaded file](/api/upload-file)
that doesn't require authentication.

The `SIGNED_ACCESS_TOKEN_VALIDITY_IN_SECONDS` server setting controls
the valid length of time for temporary access, which generally is set
to a default of 60 seconds. Consumers of this API are expected to
immediately request the URL that it returns, and should not store it
in any way.

**Changes**: New in Zulip 3.0 (feature level 1).
- **`zulip-pp-cli user-uploads upload-file`** - [Upload](/help/share-and-upload-files) a single file and get the corresponding URL.

Initially, only you will be able to access the link. To share the
uploaded file, you'll need to [send a message][send-message]
containing the resulting link. Users who can already access the link
can reshare it with other users by sending additional Zulip messages
containing the link.

The maximum allowed file size is available in the `max_file_upload_size_mib`
field in the [`POST /register`](/api/register-queue) response. Note that
large files (25MB+) may fail to upload using this API endpoint due to
network-layer timeouts, depending on the quality of your connection to the
Zulip server.

For uploading larger files, `/api/v1/tus` is an endpoint implementing the
[`tus` resumable upload protocol](https://tus.io/protocols/resumable-upload),
which supports uploading arbitrarily large files limited only by the server's
`max_file_upload_size_mib` (Configured via `MAX_FILE_UPLOAD_SIZE` in
`/etc/zulip/settings.py`). Clients which send authenticated credentials
(either via browser-based cookies, or API key via `Authorization` header) may
use this endpoint to upload files.

**Changes**: The `api/v1/tus` endpoint supporting resumable uploads was
introduced in Zulip 10.0 (feature level 296). Previously,
`max_file_upload_size_mib` was typically 25MB.

[uploaded-files]: /help/manage-your-uploaded-files
[send-message]: /api/send-message

### users

Manage users

- **`zulip-pp-cli users add-alert-words`** - Add words (or phrases) to the user's set of configured [alert words][alert-words].

[alert-words]: /help/dm-mention-alert-notifications#alert-words
- **`zulip-pp-cli users add-apns-token`** - This endpoint adds an APNs device token to register for iOS push notifications.

**Changes**: Deprecated in Zulip 11.0 (feature level 406). Clients connecting
to newer servers and with E2EE push notifications support should use the
[Register E2EE push device](/api/register-push-device) endpoint, as this
endpoint will be removed in a future release.
- **`zulip-pp-cli users add-fcm-token`** - This endpoint adds an FCM registration token for push notifications.

**Changes**: Deprecated in Zulip 11.0 (feature level 406). Clients connecting
to newer servers and with E2EE push notifications support should use the
[Register E2EE push device](/api/register-push-device) endpoint, as this
endpoint will be removed in a future release.
- **`zulip-pp-cli users create`** - Create a new user account via the API.

!!! warn ""

    **Note**: On Zulip Cloud, this feature is available only for
    organizations on a [Zulip Cloud Standard](https://zulip.com/plans/)
    or [Zulip Cloud Plus](https://zulip.com/plans/) plan. Administrators
    can request the required `can_create_users` permission for a bot or
    user by contacting [Zulip Cloud support][support] with an
    explanation for why it is needed. Self-hosted installations can
    toggle `can_create_users` on an account using the `manage.py
    change_user_role` [management command][management-commands].

**Changes**: Before Zulip 4.0 (feature level 36), this endpoint was
available to all organization administrators.

[support]: /help/contact-support
[management-commands]: https://zulip.readthedocs.io/en/latest/production/management-commands.html
- **`zulip-pp-cli users deactivate`** - [Deactivates a
user](https://zulip.com/help/deactivate-or-reactivate-a-user)
given their user ID.

Note that any bots controlled by the user will be deactivated
before the user; clients that don't want this behavior are
expected to prompt the user to adjust the bot's owners before
making this API request.
- **`zulip-pp-cli users deactivate-own`** - Deactivates the current user's account. See also the administrative endpoint for
[deactivating another user](/api/deactivate-user).

This endpoint is primarily useful to Zulip clients providing a user settings UI.
- **`zulip-pp-cli users get`** - Retrieve details on users in the organization.

By default, returns all accessible users in the organization.
The `user_ids` query parameter can be used to limit the
results to a specific set of user IDs.

Optionally includes values of [custom profile fields](/help/custom-profile-fields).

You can also [fetch details on a single user](/api/get-user).

**Changes**: In Zulip 12.0 (feature level 437), fixed a bug
dating to feature level 232, which caused guest users to
receive fake backwards-compatibility users in the format
intended for clients using `POST /register` without the
`user_list_incomplete` client capability.

This endpoint did not support unauthenticated
access in organizations using the [public access
option](/help/public-access-option) prior to Zulip 11.0
(feature level 387).
- **`zulip-pp-cli users get-alert-words`** - Get all of the user's configured [alert words][alert-words].

[alert-words]: /help/dm-mention-alert-notifications#alert-words
- **`zulip-pp-cli users get-by-email`** - Fetch details for a single user in the organization given a Zulip
API email address.

You can also fetch details on [all users in the organization](/api/get-users)
or [by user ID](/api/get-user).

Fetching by user ID is generally recommended when possible,
as a user might [change their email address](/help/change-your-email-address)
or change their [email address visibility](/help/configure-email-visibility),
either of which could change the client's ability to look them up by that
email address.

**Changes**: In Zulip 12.0 (feature level 437), fixed a bug
dating to feature level 232, which caused guest users to
receive fake backwards-compatibility users in the format
intended for clients using `POST /register` without the
`user_list_incomplete` client capability.

Starting with Zulip 10.0 (feature level 302), the real email
address can be used in the `email` parameter and will fetch the target user's
data if and only if the target's email visibility setting permits the requester
to see the email address.
The dummy email addresses of the form `user{id}@{realm.host}` still work, and
will now work for **all** users, via identifying them by the embedded user ID.

New in Zulip Server 4.0 (feature level 39).
- **`zulip-pp-cli users get-own`** - Get basic data about the user/bot that requests this endpoint.

**Changes**: Removed `is_billing_admin` field in Zulip 10.0 (feature level 363), as it was
replaced by the `can_manage_billing_group` realm setting.
- **`zulip-pp-cli users get-stream-topics`** - Get all topics the user has access to in a specific channel.

Note that for [private channels with
protected history](/help/channel-permissions#private-channels),
the user will only have access to topics of messages sent after they
[subscribed to](/api/subscribe) the channel. Similarly, a user's
[bot](/help/bots-overview#bot-type) will only have access to messages
sent after the bot was subscribed to the channel, instead of when the
user subscribed.

**Changes**: Before Zulip 12.0 (feature level 480), this
endpoint was not supported for archived channels.
- **`zulip-pp-cli users get-subscriptions`** - Get all channels that the user is subscribed to.
- **`zulip-pp-cli users get-userid`** - Fetch details for a single user in the organization.

You can also fetch details on [all users in the organization](/api/get-users)
or [by a user's Zulip API email](/api/get-user-by-email).

**Changes**: In Zulip 12.0 (feature level 437), fixed a bug
dating to feature level 232, which caused guest users to
receive fake backwards-compatibility users in the format
intended for clients using `POST /register` without the
`user_list_incomplete` client capability.

New in Zulip 3.0 (feature level 1).
- **`zulip-pp-cli users mute`** - [Mute a user](/help/mute-a-user) from the perspective of the requesting
user. Messages sent by muted users will be automatically marked as read
and hidden for the user who muted them.

Muted users should be implemented by clients as follows:

- The server will immediately mark all messages sent by the muted
  user as read. This will automatically clear any existing mobile
  push notifications related to the muted user.
- The server will mark any new messages sent by the muted user as read
  for the requesting user's account, which prevents all email and mobile
  push notifications.
- Clients should exclude muted users from presence lists or other UI
  for viewing or composing one-on-one direct messages. One-on-one direct
  messages sent by muted users should be hidden everywhere in the Zulip UI.
- Channel messages and group direct messages sent by the muted
  user should avoid displaying the content and name/avatar,
  but should display that N messages by a muted user were
  hidden (so that it is possible to interpret the messages by
  other users who are talking with the muted user).
- Group direct message conversations including the muted user
  should display muted users as "Muted user", rather than
  showing their name, in lists of such conversations, along with using
  a blank grey avatar where avatars are displayed.
- Administrative/settings UI elements for showing "All users that exist
  on this channel or realm", e.g. for organization
  administration or showing channel subscribers, should display
  the user's name as normal.

**Changes**: New in Zulip 4.0 (feature level 48).
- **`zulip-pp-cli users mute-topic`** - [Mute or unmute a topic](/help/mute-a-topic) within a channel that
the current user is subscribed to.

**Changes**: Deprecated in Zulip 7.0 (feature level 170). Clients connecting
to newer servers should use the [POST /user_topics](/api/update-user-topic)
endpoint, as this endpoint may be removed in a future release.

Before Zulip 7.0 (feature level 169), this endpoint
returned an error if asked to mute a topic that was already muted
or asked to unmute a topic that had not previously been muted.
- **`zulip-pp-cli users regenerate-api-key`** - !!! warn ""

     **Note**: Users should treat their Zulip API key as
     [carefully as they would their password](/help/protect-your-account).

Generate a new API key for the user making the request.

Changing a user's API key will immediately log them out of Zulip
on devices registered for [mobile push notifications][mobile-push].

**Changes**: Before Zulip 12.0 (feature level 492),
regenerating a user's API key didn't remove all of the user's
[E2EE push device registrations](/api/register-push-device),
so E2EE push notifications could still be sent.

[mobile-push]: https://zulip.readthedocs.io/en/latest/production/mobile-push-notifications.html
- **`zulip-pp-cli users remove-alert-words`** - Remove words (or phrases) from the user's set of configured [alert words][alert-words].

Alert words are case insensitive.

[alert-words]: /help/dm-mention-alert-notifications#alert-words
- **`zulip-pp-cli users remove-apns-token`** - This endpoint removes an APNs device token for iOS push notifications.

**Changes**: Deprecated in Zulip 11.0 (feature level 406) and will be
removed in a future release. Clients connecting to newer servers and
with E2EE push notifications support should delete the account record
in their local accounts table that corresponds to the `push_account_id`
supplied when registering via the [Register E2EE push device](/api/register-push-device)
endpoint, to stop displaying notifications for that registration.
- **`zulip-pp-cli users remove-fcm-token`** - This endpoint removes an FCM registration token for push notifications.

**Changes**: Deprecated in Zulip 11.0 (feature level 406) and will be
removed in a future release. Clients connecting to newer servers and
with E2EE push notifications support should delete the account record
in their local accounts table that corresponds to the `push_account_id`
supplied when registering via the [Register E2EE push device](/api/register-push-device)
endpoint, to stop displaying notifications for that registration.
- **`zulip-pp-cli users remove-profile-data`** - Remove the current user's [profile data](/help/edit-your-profile) for
one or more of the [custom profile fields](/help/custom-profile-fields)
configured in the organization.
- **`zulip-pp-cli users subscribe`** - Subscribe one or more users to one or more channels.

If any of the specified channels do not exist, they are automatically
created. The initial [channel settings](/api/update-stream) will be determined
by the optional parameters, like `invite_only`, detailed below.

Note that the ability to subscribe oneself and/or other users
to a specified channel depends on the [channel's permissions
settings](/help/channel-permissions).

**Changes**: Before Zulip 10.0 (feature level 362),
subscriptions in archived channels could not be modified.

Before Zulip 10.0 (feature level 357), the
`can_subscribe_group` permission, which allows members of the
group to subscribe themselves to the channel, did not exist.

Before Zulip 10.0 (feature level 349), a user cannot subscribe
other users to a private channel without being subscribed
to that channel themselves. Now, If a user is part of
`can_add_subscribers_group`, they can subscribe themselves or other
users to a private channel without being subscribed to that channel.

Removed `stream_post_policy` and `is_announcement_only`
parameters in Zulip 10.0 (feature level 333), as permission to post
in the channel is now controlled by `can_send_message_group`.

Before Zulip 8.0 (feature level 208), if a user specified by the
[`principals`][principals-param] parameter was a deactivated user,
or did not exist, then an HTTP status code of 403 was returned with
`code: "UNAUTHORIZED_PRINCIPAL"` in the error response. As of this
feature level, an HTTP status code of 400 is returned with
`code: "BAD_REQUEST"` in the error response for these cases.

[principals-param]: /api/subscribe#parameter-principals
- **`zulip-pp-cli users unmute`** - [Unmute a user](/help/mute-a-user#see-your-list-of-muted-users)
from the perspective of the requesting user.

**Changes**: New in Zulip 4.0 (feature level 48).
- **`zulip-pp-cli users unsubscribe`** - Unsubscribe yourself or other users from one or more channels.

In addition to managing the current user's subscriptions, this
endpoint can be used to remove other users from channels. This
is possible in 3 situations:

- Organization administrators can remove any user from any
  channel.
- Users can remove a bot that they own from any channel that
  the user [can access](/help/channel-permissions).
- Users can unsubscribe any user from a channel if they [have
  access](/help/channel-permissions) to the channel and are a
  member of the [user group](/api/get-user-groups) specified
  by the [`can_remove_subscribers_group`][can-remove-parameter]
  for the channel.

**Changes**: Before Zulip 10.0 (feature level 362),
subscriptions in archived channels could not be modified.

Before Zulip 8.0 (feature level 208), if a user specified by
the [`principals`][principals-param] parameter was a
deactivated user, or did not exist, then an HTTP status code
of 403 was returned with `code: "UNAUTHORIZED_PRINCIPAL"` in
the error response. As of this feature level, an HTTP status
code of 400 is returned with `code: "BAD_REQUEST"` in the
error response for these cases.

Before Zulip 8.0 (feature level 197),
the `can_remove_subscribers_group` setting
was named `can_remove_subscribers_group_id`.

Before Zulip 7.0 (feature level 161), the
`can_remove_subscribers_group_id` for all channels was always
the system group for organization administrators.

Before Zulip 6.0 (feature level 145), users had no special
privileges for managing bots that they own.

[principals-param]: /api/unsubscribe#parameter-principals
[can-remove-parameter]: /api/subscribe#parameter-can_remove_subscribers_group
- **`zulip-pp-cli users update`** - Administrative endpoint to update the details of another user in the organization.

Supports everything an administrator can do to edit details of another
user's account, including editing full name,
[role](/help/user-roles), and [custom profile
fields](/help/custom-profile-fields).
- **`zulip-pp-cli users update-by-email`** - Administrative endpoint to update the details of another user in the organization by their email address.
Works the same way as [`PATCH /users/{user_id}`](/api/update-user) but fetching the target user by their
real email address.

The requester needs to have permission to view the target user's real email address, subject to the
user's email address visibility setting. Otherwise, the dummy address of the format
`user{id}@{realm.host}` needs be used. This follows the same rules as `GET /users/{email}`.

**Changes**: New in Zulip 10.0 (feature level 313).
- **`zulip-pp-cli users update-presence`** - Update the current user's [presence][availability] and fetch presence data
of other users in the organization.

This endpoint is meant to be used by clients for both:

- Reporting the current user's presence status (`"active"` or `"idle"`)
  to the server.

- Obtaining the presence data of all other users in the organization via
  regular polling.

Accurate user presence is one of the most expensive parts of any
chat application (in terms of bandwidth and other resources). Therefore,
it is important that clients implementing Zulip's user presence system
use the modern [`last_update_id`](#parameter-last_update_id) protocol to
minimize fetching duplicate user presence data.

Client apps implementing presence are recommended to also consume [`presence`
events](/api/get-events#presence)), in order to learn about newly online users
immediately.

The Zulip server is responsible for implementing [invisible mode][invisible],
which disables sharing a user's presence data. Nonetheless, clients
should check the `presence_enabled` field in user objects in order to
display the current user as online or offline based on whether they are
sharing their presence information.

**Changes**: As of Zulip 8.0 (feature level 228), if the
`CAN_ACCESS_ALL_USERS_GROUP_LIMITS_PRESENCE` server-level setting is
`true`, then user presence data in the response is [limited to users
the current user can see/access][limit-visibility].

[limit-visibility]: /help/guest-users#configure-whether-guests-can-see-all-other-users
[invisible]: /help/status-and-availability#invisible-mode
[availability]: /help/status-and-availability#availability
- **`zulip-pp-cli users update-profile-data`** - Update the current user's [profile data](/help/edit-your-profile) for
one or more of the [custom profile fields](/help/custom-profile-fields)
configured in the organization.
- **`zulip-pp-cli users update-status`** - Change your [status](/help/status-and-availability).

A request to this endpoint will only change the parameters passed.
For example, passing just `status_text` requests a change in the status
text, but will leave the status emoji unchanged.

Clients that wish to set the user's status to a specific value should
pass all supported parameters.

**Changes**: In Zulip 5.0 (feature level 86), added support for
`emoji_name`, `emoji_code`, and `reaction_type` parameters.
- **`zulip-pp-cli users update-subscription-property`** - Update the current user's personal settings for a specific channel they
are subscribed to. These settings include [color](/help/change-the-color-of-a-channel),
[muting](/help/mute-a-channel), [pinning](/help/pin-a-channel)
and [per-channel notification settings](/help/channel-notifications).

This is a single channel alternative to the bulk endpoint:
[`POST /users/me/subscriptions/properties`](/api/update-subscription-settings).
- **`zulip-pp-cli users update-subscription-settings`** - Update the current user's personal settings for channels they are
subscribed to. These settings include [color](/help/change-the-color-of-a-channel),
[muting](/help/mute-a-channel), [pinning](/help/pin-a-channel)
and [per-channel notification settings](/help/channel-notifications).

There is a single channel alternative to this bulk endpoint:
[`POST /users/me/subscriptions/{stream_id}`](/api/update-subscription-property).

**Changes**: Prior to Zulip 5.0 (feature level 111), the response object
included the `subscription_data` in the request. The endpoint now returns
the more ergonomic [`ignored_parameters_unsupported`][ignored-parameters]
array instead.

[ignored-parameters]: /api/rest-error-handling#ignored-parameters
- **`zulip-pp-cli users update-subscriptions`** - Update which channels you are subscribed to.

**Changes**: Before Zulip 10.0 (feature level 362),
subscriptions in archived channels could not be modified.

### zulip-export

Manage zulip export

- **`zulip-pp-cli zulip-export get-realm`** - Fetch all the public and standard [data exports][export-data]
of the organization.

**Changes**: Prior to Zulip 10.0 (feature level 304), only
public data exports could be fetched using this endpoint.

New in Zulip 2.1.

[export-data]: /help/export-your-organization#export-data-in-an-importable-format
- **`zulip-pp-cli zulip-export get-realm-consents`** - Fetches which users have [consented](/help/export-your-organization#configure-whether-administrators-can-export-your-private-data)
for their private data to be exported by organization administrators.

**Changes**: Changes in Zulip 12.0 (feature level 430). Added an
integer field `email_address_visibility` to the objects in the
`export_consents` array.

New in Zulip 10.0 (feature level 295).
- **`zulip-pp-cli zulip-export realm`** - Create a public or a standard [data export][export-data] of the organization.

!!! warn ""

    **Note**: If you're the administrator of a self-hosted installation,
    you may be looking for the documentation on [server data export and
    import][data-export] or [server backups][backups].

**Changes**: Prior to Zulip 10.0 (feature level 304), only
public data exports could be created using this endpoint.

New in Zulip 2.1.

[export-data]: /help/export-your-organization#export-data-in-an-importable-format
[data-export]: https://zulip.readthedocs.io/en/stable/production/export-and-import.html#data-export
[backups]: https://zulip.readthedocs.io/en/stable/production/export-and-import.html#backups

### zulip-outgoing-webhook

Manage zulip outgoing webhook

- **`zulip-pp-cli zulip-outgoing-webhook`** - Outgoing webhooks allow you to build or set up Zulip integrations which are
notified when certain types of messages are sent in Zulip.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zulip-pp-cli attachments get

# JSON for scripting and agents
zulip-pp-cli attachments get --json

# Filter to specific fields
zulip-pp-cli attachments get --json --select id,name,status

# Dry run — show the request without sending
zulip-pp-cli attachments get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zulip-pp-cli attachments get --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `ZULIP_SUBDOMAIN` resolves `{subdomain}`

Base URL: `https://{subdomain}.zulipchat.com/api/v1`

## Health Check

```bash
zulip-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/zulip-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
