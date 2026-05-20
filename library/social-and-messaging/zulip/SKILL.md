---
name: pp-zulip
description: "Printing Press CLI for Zulip. Powerful open source group chat"
author: "Haqi Ramadhani"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zulip-pp-cli
---

# Zulip — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zulip-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install zulip --cli-only
   ```
2. Verify: `zulip-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Powerful open source group chat

## Command Reference

**attachments** — Manage attachments

- `zulip-pp-cli attachments get` — Fetch metadata on files uploaded by the requesting user.
- `zulip-pp-cli attachments remove` — Delete an uploaded file given its attachment ID. Note that uploaded files that have been referenced in at least one...

**bot-storage** — Manage bot storage

- `zulip-pp-cli bot-storage get` — !!! warn '' **Note:** This endpoint is only available to [bot user](/help/bots-overview) accounts. Retrieve [data...
- `zulip-pp-cli bot-storage remove` — !!! warn '' **Note:** This endpoint is only available to [bot user](/help/bots-overview) accounts. Delete [data...
- `zulip-pp-cli bot-storage update` — !!! warn '' **Note:** This endpoint is only available to [bot user](/help/bots-overview) accounts. Add or update...

**bots** — Manage bots


**calls** — Manage calls

- `zulip-pp-cli calls create-big-blue-button-video` — Create a video call URL for a BigBlueButton video call. Requires [BigBlueButton 2.4+](/integrations/big-blue-button)...
- `zulip-pp-cli calls create-constructor-groups-video` — Create a video call URL for a Constructor Groups video call. Requires [Constructor...
- `zulip-pp-cli calls create-nextcloud-talk-video` — Create a video call URL for a Nextcloud Talk video call. Requires [Nextcloud Talk](/integrations/nextcloud-talk) to...
- `zulip-pp-cli calls create-webex-video` — Create a video call URL for a Webex video call. Requires [Webex integration](/integrations/webex) to be configured...

**channel-folders** — Manage channel folders

- `zulip-pp-cli channel-folders create` — Create a new [channel folder](/help/channel-folders). **Changes**: New in Zulip 11.0 (feature level 389).
- `zulip-pp-cli channel-folders get` — Fetches all of the [channel folders](/help/channel-folders) in the organization, sorted by the `order` field....
- `zulip-pp-cli channel-folders patch` — Reorder the [channel folders](/help/channel-folders) in the user's organization. Channel folders are displayed in...
- `zulip-pp-cli channel-folders update` — Update the name or description of a [channel folder](/help/channel-folders) with the specified ID. This endpoint is...

**channels** — Manage channels

- `zulip-pp-cli channels` — Create a new [channel](/help/create-channels), and optionally subscribe users to the newly created channel. The...

**default-streams** — Manage default streams

- `zulip-pp-cli default-streams add` — Add a channel to the set of [default channels][default-channels] for new users joining the organization....
- `zulip-pp-cli default-streams remove` — Remove a channel from the set of [default channels][default-channels] for new users joining the organization....

**dev-fetch-api-key** — Manage dev fetch api key

- `zulip-pp-cli dev-fetch-api-key` — For easy testing of mobile apps and other clients and against Zulip development servers, we support fetching a Zulip...

**dev-list-users** — Manage dev list users

- `zulip-pp-cli dev-list-users` — Get a list of all, non-bot users in a [Zulip development server](https://zulip.readthedocs.io/en/latest/development/o...

**drafts** — Manage drafts

- `zulip-pp-cli drafts create` — Create one or more drafts on the server. These drafts will be automatically synchronized to other clients via...
- `zulip-pp-cli drafts delete` — Delete a single draft from the server. The deletion will be automatically synchronized to other clients via a...
- `zulip-pp-cli drafts edit` — Edit a draft on the server. The edit will be automatically synchronized to other clients via `drafts` events.
- `zulip-pp-cli drafts get` — Fetch all drafts for the current user.

**events** — Manage events

- `zulip-pp-cli events delete-queue` — Delete a previously registered queue.
- `zulip-pp-cli events get` — This endpoint allows you to receive new events from [a registered event queue](/api/register-queue). Long-lived...

**fetch-api-key** — Manage fetch api key

- `zulip-pp-cli fetch-api-key` — This API endpoint is used by clients such as the Zulip mobile and terminal apps to implement password-based...

**get-stream-id** — Manage get stream id

- `zulip-pp-cli get-stream-id` — Get the unique ID of a given channel.

**invites** — Manage invites

- `zulip-pp-cli invites create-link` — Create a [reusable invitation link](/help/invite-new-users#create-a-reusable-invitation-link) which can be used to...
- `zulip-pp-cli invites get` — Fetch all unexpired [invitations](/help/invite-new-users) (i.e. email invitations and reusable invitation links)...
- `zulip-pp-cli invites revoke-email` — Revoke an [email invitation](/help/invite-new-users#send-email-invitations). A user can only revoke [invitations...
- `zulip-pp-cli invites revoke-link` — Revoke a [reusable invitation link](/help/invite-new-users#create-a-reusable-invitation-link). A user can only...
- `zulip-pp-cli invites send` — Send [invitations](/help/invite-new-users) to specified email addresses. **Changes**: In Zulip 6.0 (feature level...

**jwt** — Manage jwt

- `zulip-pp-cli jwt` — This API endpoint is used by clients to implement JSON Web Token (JWT) authentication. Given a JWT identifying a...

**mark-all-as-read** — Manage mark all as read

- `zulip-pp-cli mark-all-as-read` — Marks all of the current user's unread messages as read. Because this endpoint marks messages as read in batches, it...

**mark-stream-as-read** — Manage mark stream as read

- `zulip-pp-cli mark-stream-as-read` — Mark all the unread messages in a channel as read. **Changes**: Deprecated; clients should use the [update personal...

**mark-topic-as-read** — Manage mark topic as read

- `zulip-pp-cli mark-topic-as-read` — Mark all the unread messages in a topic as read. **Changes**: Deprecated; clients should use the [update personal...

**messages** — Manage messages

- `zulip-pp-cli messages check-match-narrow` — Check whether a set of messages match a [narrow](/api/construct-narrow). For many common narrows (e.g. a topic),...
- `zulip-pp-cli messages delete` — Permanently delete a message. This API corresponds to the [delete a message completely][delete-completely] feature...
- `zulip-pp-cli messages get` — This endpoint is the primary way to fetch a messages. It is used by all official Zulip clients (e.g. the web,...
- `zulip-pp-cli messages get-messageid` — Given a message ID, return the message object. Additionally, a `raw_content` field is included. This field is useful...
- `zulip-pp-cli messages render` — Render a message to HTML.
- `zulip-pp-cli messages send` — Send a [channel message](/help/introduction-to-topics) or a [direct message](/help/direct-messages).
- `zulip-pp-cli messages update` — Update the content, topic, or channel of the message with the specified ID. You can [resolve...
- `zulip-pp-cli messages update-flags` — Add or remove personal message flags like `read` and `starred` on a collection of message IDs. See also the endpoint...
- `zulip-pp-cli messages update-flags-for-narrow` — Add or remove personal message flags like `read` and `starred` on a range of messages within a narrow. See also [the...

**mobile-push** — Manage mobile push

- `zulip-pp-cli mobile-push e2ee-test-notify` — Trigger sending an end-to-end encrypted (E2EE) test push notification to the user's selected mobile device or all of...
- `zulip-pp-cli mobile-push register-push-device` — Register a device to receive end-to-end encrypted mobile push notifications, or update such a registration. To...
- `zulip-pp-cli mobile-push test-notify` — Trigger sending a test push notification to the user's selected mobile device or all of their mobile devices....

**navigation-views** — Manage navigation views

- `zulip-pp-cli navigation-views add` — Adds a new custom left sidebar navigation view configuration for the current user. This can be used both to...
- `zulip-pp-cli navigation-views edit` — Update the details of an existing configured navigation view, such as its name or whether it's pinned. **Changes**:...
- `zulip-pp-cli navigation-views get` — Fetch all configured custom navigation views for the current user. **Changes**: New in Zulip 11.0 (feature level 390).
- `zulip-pp-cli navigation-views remove` — Remove a navigation view. **Changes**: New in Zulip 11.0 (feature level 390).

**real-time** — Manage real time

- `zulip-pp-cli real-time` — (Ignored)

**realm** — Manage realm

- `zulip-pp-cli realm add-code-playground` — Configure [code playgrounds](/help/code-blocks#code-playgrounds) for the organization. **Changes**: New in Zulip 4.0...
- `zulip-pp-cli realm add-domain` — Add a domain to the set of allowed domains configured in the organization for [user account email...
- `zulip-pp-cli realm add-linkifier` — Configure [linkifiers](/help/add-a-custom-linkifier), regular expression patterns that are automatically linkified...
- `zulip-pp-cli realm create-custom-profile-field` — [Create a custom profile field](/help/custom-profile-fields#add-a-custom-profile-field) in the user's organization.
- `zulip-pp-cli realm deactivate-custom-emoji` — [Deactivate a custom emoji](/help/custom-emoji#deactivate-custom-emoji) from the user's organization. Users can only...
- `zulip-pp-cli realm delete-domain` — Remove the specified domain from the set of allowed domains configured in the organization for [user account email...
- `zulip-pp-cli realm get-custom-emoji` — Get all the custom emoji in the user's organization.
- `zulip-pp-cli realm get-custom-profile-fields` — Get all the [custom profile fields](/help/custom-profile-fields) configured for the user's organization.
- `zulip-pp-cli realm get-domains` — Get the set of allowed domains configured in the organization for user account email addresses. As each Zulip user...
- `zulip-pp-cli realm get-linkifiers` — List all of an organization's configured [linkifiers](/help/add-a-custom-linkifier), regular expression patterns...
- `zulip-pp-cli realm get-presence` — Get the presence information of all the users in an organization. If the...
- `zulip-pp-cli realm patch-domain` — Update whether subdomains are allowed in [user account email addresses](/help/restrict-account-creation) for the...
- `zulip-pp-cli realm remove-code-playground` — Remove a [code playground](/help/code-blocks#code-playgrounds) previously configured for an organization....
- `zulip-pp-cli realm remove-linkifier` — Remove [linkifiers](/help/add-a-custom-linkifier), regular expression patterns that are automatically linkified when...
- `zulip-pp-cli realm reorder-custom-profile-fields` — Reorder the custom profile fields in the user's organization. Custom profile fields are displayed in Zulip UI...
- `zulip-pp-cli realm reorder-linkifiers` — Change the order that the regular expression patterns in the organization's...
- `zulip-pp-cli realm test-welcome-bot-custom-message` — Sends a test Welcome Bot custom message to the acting administrator. This allows administrators to preview how the...
- `zulip-pp-cli realm update-linkifier` — Update a [linkifier](/help/add-a-custom-linkifier), regular expression patterns that are automatically linkified...
- `zulip-pp-cli realm update-user-settings-defaults` — Change the [default values of settings][new-user-defaults] for new users joining the organization. Essentially all...
- `zulip-pp-cli realm upload-custom-emoji` — This endpoint is used to upload a custom emoji for use in the user's organization. Access to this endpoint depends...

**register** — Manage register

- `zulip-pp-cli register` — This powerful endpoint can be used to register a Zulip 'event queue' (subscribed to certain types of 'events', or...

**register-client-device** — Manage register client device

- `zulip-pp-cli register-client-device` — Logged-in mobile devices use this endpoint as an initial step to register themselves, before registering for E2EE...

**reminders** — Manage reminders

- `zulip-pp-cli reminders create-message` — Schedule a reminder to be sent to the current user at the specified time. The reminder will link the relevant...
- `zulip-pp-cli reminders delete` — Delete, and therefore cancel sending, a previously [scheduled reminder](/help/schedule-a-reminder). **Changes**: New...
- `zulip-pp-cli reminders get` — Fetch all [reminders](/help/schedule-a-reminder) for the current user. Reminders are messages the user has scheduled...

**remotes** — Manage remotes

- `zulip-pp-cli remotes` — Register a push device to bouncer to receive end-to-end encrypted mobile push notifications. Self-hosted servers use...

**remove-client-device** — Manage remove client device

- `zulip-pp-cli remove-client-device` — Mobile devices use this endpoint to remove their device record registered using [`POST...

**rest-error-handling** — Manage rest error handling

- `zulip-pp-cli rest-error-handling` — Common error to many endpoints

**saved-snippets** — Manage saved snippets

- `zulip-pp-cli saved-snippets create` — Create a new saved snippet for the current user. **Changes**: New in Zulip 10.0 (feature level 297).
- `zulip-pp-cli saved-snippets delete` — Delete a saved snippet. **Changes**: New in Zulip 10.0 (feature level 297).
- `zulip-pp-cli saved-snippets edit` — Edit a saved snippet for the current user. **Changes**: New in Zulip 10.0 (feature level 368).
- `zulip-pp-cli saved-snippets get` — Fetch all the saved snippets for the current user. **Changes**: New in Zulip 10.0 (feature level 297).

**scheduled-messages** — Manage scheduled messages

- `zulip-pp-cli scheduled-messages create` — Create a new [scheduled message](/help/schedule-a-message). **Changes**: In Zulip 7.0 (feature level 184), moved...
- `zulip-pp-cli scheduled-messages delete` — Delete, and therefore cancel sending, a previously [scheduled message](/help/schedule-a-message). **Changes**: New...
- `zulip-pp-cli scheduled-messages get` — Fetch all [scheduled messages](/help/schedule-a-message) for the current user. Scheduled messages are messages the...
- `zulip-pp-cli scheduled-messages update` — Edit an existing [scheduled message](/help/schedule-a-message). **Changes**: New in Zulip 7.0 (feature level 184).

**server-settings** — Manage server settings

- `zulip-pp-cli server-settings` — Fetch global settings for a Zulip server. **Note:** this endpoint does not require any authentication at all, and...

**settings** — Manage settings

- `zulip-pp-cli settings` — This endpoint is used to edit the current user's settings. When invoked by a realm admin, it supports bulk updates...

**streams** — Manage streams

- `zulip-pp-cli streams archive` — [Archive the channel](/help/archive-a-channel) with the ID `stream_id`.
- `zulip-pp-cli streams get` — Get all channels that the user [has access to](/help/channel-permissions).
- `zulip-pp-cli streams get-by-id` — Fetch details for the channel with the ID `stream_id`. **Changes**: Before Zulip 12.0 (feature level 480), this...
- `zulip-pp-cli streams update` — Configure the channel with the ID `stream_id`. This endpoint supports an organization administrator editing any...

**thumbnail** — Manage thumbnail

- `zulip-pp-cli thumbnail <realm_id_str> <filename>` — Check whether a thumbnail exists for a specific file uploaded by a user. This endpoint is intended to be polled by...

**typing** — Manage typing

- `zulip-pp-cli typing` — Notify other users whether the current user is [typing a message][help-typing]. Clients implementing Zulip's typing...

**user-groups** — Manage user groups

- `zulip-pp-cli user-groups create` — Create a new [user group](/help/user-groups). **Changes**: Prior to Zulip 12.0 (feature level 496), bot users were...
- `zulip-pp-cli user-groups get` — Fetches all of the user groups in the organization. !!! warn '' **Note**: This endpoint is not available to [guest...
- `zulip-pp-cli user-groups update` — Update the name, description or any of the permission settings of a [user group](/help/user-groups). This endpoint...

**user-topics** — Manage user topics

- `zulip-pp-cli user-topics` — This endpoint is used to update the personal preferences for a topic, such as the topic's visibility policy, which...

**user-uploads** — Manage user uploads

- `zulip-pp-cli user-uploads get-file-temporary-url` — Get a temporary URL for access to an [uploaded file](/api/upload-file) that doesn't require authentication. The...
- `zulip-pp-cli user-uploads upload-file` — [Upload](/help/share-and-upload-files) a single file and get the corresponding URL. Initially, only you will be able...

**users** — Manage users

- `zulip-pp-cli users add-alert-words` — Add words (or phrases) to the user's set of configured [alert words][alert-words]. [alert-words]:...
- `zulip-pp-cli users add-apns-token` — This endpoint adds an APNs device token to register for iOS push notifications. **Changes**: Deprecated in Zulip...
- `zulip-pp-cli users add-fcm-token` — This endpoint adds an FCM registration token for push notifications. **Changes**: Deprecated in Zulip 11.0 (feature...
- `zulip-pp-cli users create` — Create a new user account via the API. !!! warn '' **Note**: On Zulip Cloud, this feature is available only for...
- `zulip-pp-cli users deactivate` — [Deactivates a user](https://zulip.com/help/deactivate-or-reactivate-a-user) given their user ID. Note that any bots...
- `zulip-pp-cli users deactivate-own` — Deactivates the current user's account. See also the administrative endpoint for [deactivating another...
- `zulip-pp-cli users get` — Retrieve details on users in the organization. By default, returns all accessible users in the organization. The...
- `zulip-pp-cli users get-alert-words` — Get all of the user's configured [alert words][alert-words]. [alert-words]:...
- `zulip-pp-cli users get-by-email` — Fetch details for a single user in the organization given a Zulip API email address. You can also fetch details on...
- `zulip-pp-cli users get-own` — Get basic data about the user/bot that requests this endpoint. **Changes**: Removed `is_billing_admin` field in...
- `zulip-pp-cli users get-stream-topics` — Get all topics the user has access to in a specific channel. Note that for [private channels with protected...
- `zulip-pp-cli users get-subscriptions` — Get all channels that the user is subscribed to.
- `zulip-pp-cli users get-userid` — Fetch details for a single user in the organization. You can also fetch details on [all users in the...
- `zulip-pp-cli users mute` — [Mute a user](/help/mute-a-user) from the perspective of the requesting user. Messages sent by muted users will be...
- `zulip-pp-cli users mute-topic` — [Mute or unmute a topic](/help/mute-a-topic) within a channel that the current user is subscribed to. **Changes**:...
- `zulip-pp-cli users regenerate-api-key` — !!! warn '' **Note**: Users should treat their Zulip API key as [carefully as they would their...
- `zulip-pp-cli users remove-alert-words` — Remove words (or phrases) from the user's set of configured [alert words][alert-words]. Alert words are case...
- `zulip-pp-cli users remove-apns-token` — This endpoint removes an APNs device token for iOS push notifications. **Changes**: Deprecated in Zulip 11.0...
- `zulip-pp-cli users remove-fcm-token` — This endpoint removes an FCM registration token for push notifications. **Changes**: Deprecated in Zulip 11.0...
- `zulip-pp-cli users remove-profile-data` — Remove the current user's [profile data](/help/edit-your-profile) for one or more of the [custom profile...
- `zulip-pp-cli users subscribe` — Subscribe one or more users to one or more channels. If any of the specified channels do not exist, they are...
- `zulip-pp-cli users unmute` — [Unmute a user](/help/mute-a-user#see-your-list-of-muted-users) from the perspective of the requesting user....
- `zulip-pp-cli users unsubscribe` — Unsubscribe yourself or other users from one or more channels. In addition to managing the current user's...
- `zulip-pp-cli users update` — Administrative endpoint to update the details of another user in the organization. Supports everything an...
- `zulip-pp-cli users update-by-email` — Administrative endpoint to update the details of another user in the organization by their email address. Works the...
- `zulip-pp-cli users update-presence` — Update the current user's [presence][availability] and fetch presence data of other users in the organization. This...
- `zulip-pp-cli users update-profile-data` — Update the current user's [profile data](/help/edit-your-profile) for one or more of the [custom profile...
- `zulip-pp-cli users update-status` — Change your [status](/help/status-and-availability). A request to this endpoint will only change the parameters...
- `zulip-pp-cli users update-subscription-property` — Update the current user's personal settings for a specific channel they are subscribed to. These settings include...
- `zulip-pp-cli users update-subscription-settings` — Update the current user's personal settings for channels they are subscribed to. These settings include...
- `zulip-pp-cli users update-subscriptions` — Update which channels you are subscribed to. **Changes**: Before Zulip 10.0 (feature level 362), subscriptions in...

**zulip-export** — Manage zulip export

- `zulip-pp-cli zulip-export get-realm` — Fetch all the public and standard [data exports][export-data] of the organization. **Changes**: Prior to Zulip 10.0...
- `zulip-pp-cli zulip-export get-realm-consents` — Fetches which users have [consented](/help/export-your-organization#configure-whether-administrators-can-export-your-...
- `zulip-pp-cli zulip-export realm` — Create a public or a standard [data export][export-data] of the organization. !!! warn '' **Note**: If you're the...

**zulip-outgoing-webhook** — Manage zulip outgoing webhook

- `zulip-pp-cli zulip-outgoing-webhook` — Outgoing webhooks allow you to build or set up Zulip integrations which are notified when certain types of messages...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zulip-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `zulip-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zulip-pp-cli attachments get --agent --select id,name,status
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
zulip-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zulip-pp-cli feedback --stdin < notes.txt
zulip-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.zulip-pp-cli/feedback.jsonl`. They are never POSTed unless `ZULIP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZULIP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zulip-pp-cli profile save briefing --json
zulip-pp-cli --profile briefing attachments get
zulip-pp-cli profile list --json
zulip-pp-cli profile show briefing
zulip-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `zulip-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add zulip-pp-mcp -- zulip-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zulip-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zulip-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zulip-pp-cli <command> --help`.
