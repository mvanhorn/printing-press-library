# Slack CLI

**Every Slack Web API endpoint, plus a local SQLite mirror that unlocks compound queries and cross-source customer timelines no other Slack tool can produce.**

This CLI wraps the full 174-endpoint Slack Web API and keeps a local SQLite mirror of channels, users, messages, threads, reactions, and usergroups. The mirror powers offline FTS5 search and transcendence verbs — customer-intel-deep joins Slack against co-located Attio, Asana, and Fathom mirrors; dm-engagement and goal-channel-pulse compose cross-source volumes; agent-audit reads the CLI's own DM-access log. Every write verb supports --dry-run for cron safety.

Learn more at [Slack](https://api.slack.com/support).

## Install

The recommended path installs both the `slack-pp-cli` binary and the `pp-slack` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install slack
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install slack --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/slack-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-slack --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-slack --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-slack skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-slack. The skill defines how its required CLI can be installed.
```

## Authentication

Auth is an xoxp Slack user token. Set SLACK_USER_TOKEN, or let the CLI fall back to ~/.claude/slack-user-token.env. A bot token (xoxb) works for post-only flows via SLACK_BOT_TOKEN.

## Quick Start

```bash
# Confirm the token resolves and the workspace is reachable before anything else.
slack-pp-cli doctor


# Hydrate the local SQLite mirror — channels, users, messages, threads, reactions, usergroups.
slack-pp-cli sync mirror


# Offline FTS5 search across every synced channel — no rate limit.
slack-pp-cli who-said "Sonria" --window 14d


# The headline verb — a cross-source timeline for one customer.
slack-pp-cli customer-intel-deep "Sonria" --window 14d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-source mirror joins
- **`customer-intel-deep`** — A one-screen, time-ordered timeline for a customer joining Slack mentions, the Attio deal stage at that moment, open Asana tasks, and Fathom call action items — every line cited with a permalink.

  _Reach for this when an alert fires and you need the full cross-source history of a customer in one call, not Slack chatter alone._

  ```bash
  slack-pp-cli customer-intel-deep "Sonria" --window 14d --agent
  ```
- **`dm-engagement`** — One volume row per direct report: DM count with you, Asana tasks created and completed, and Fathom calls attended over a time window.

  _Use before 1:1s or scorecard reviews to see real engagement volumes per report instead of scrolling DMs._

  ```bash
  slack-pp-cli dm-engagement --report all --window 14d --agent
  ```
- **`action-followthrough`** — For each Fathom call action item assigned to a CSM, whether that person mentioned the company in Slack within the window — with the matching permalink when they did.

  _Use in L10 prep to audit whether commitments made on calls were actually acted on._

  ```bash
  slack-pp-cli action-followthrough --report marjorie --window 14d --agent
  ```
- **`goal-channel-pulse`** — Per active Asana Rock in the current quarter, the mapped Slack channel's 7-day message count, unique participants, total reactions, and a stalled flag when discussion is zero.

  _Use in L10 prep to see whether the team is actually talking about each Rock, not just whether tasks moved._

  ```bash
  slack-pp-cli goal-channel-pulse --quarter current --agent
  ```

### Local-mirror aggregation
- **`reactions summarize`** — Aggregates emoji reactions across every message in a channel over a window: top messages by reaction count, emoji distribution, and a fixed emoji-class bucket count.

  _Use for a daily attention pulse on which messages drew the most team reaction._

  ```bash
  slack-pp-cli reactions summarize --channel "#the-wolf-of-atom" --window 7d --agent
  ```
- **`unreads`** — An inventory of unread DMs, at-mentions, and involved threads, bucketed by priority — DM over partner channel over internal channel over broadcast — with per-bucket counts.

  _Use as the morning triage of what actually needs reading right now._

  ```bash
  slack-pp-cli unreads --priority --agent
  ```
- **`usergroups list`** — Lists usergroups with handle and members, and renders subteam mentions in all output as readable @handles instead of raw <!subteam^S...> IDs.

  _Use so digests and channel reads render subteam mentions as human handles, not opaque IDs._

  ```bash
  slack-pp-cli usergroups list --agent
  ```

### Privacy governance
- **`agent-audit`** — Shows which agents and cron jobs read which Slack DMs and channels over a window, sourced from the CLI's own append-only DM-read audit log.

  _Use for privacy governance — to know which automated agents accessed DM content._

  ```bash
  slack-pp-cli agent-audit --window 7d --agent
  ```

## Usage

Run `slack-pp-cli --help` for the full command reference and flag list.

## Commands

### admin-apps-approve

Manage admin apps approve

- **`slack-pp-cli admin-apps-approve admin_apps_approve`** - Approve an app for installation on a workspace.

### admin-apps-approved-list

Manage admin apps approved list

- **`slack-pp-cli admin-apps-approved-list admin_apps_approved_list`** - List approved apps for an org or workspace.

### admin-apps-requests-list

Manage admin apps requests list

- **`slack-pp-cli admin-apps-requests-list admin_apps_requests_list`** - List app requests for a team/workspace.

### admin-apps-restrict

Manage admin apps restrict

- **`slack-pp-cli admin-apps-restrict admin_apps_restrict`** - Restrict an app for installation on a workspace.

### admin-apps-restricted-list

Manage admin apps restricted list

- **`slack-pp-cli admin-apps-restricted-list admin_apps_restricted_list`** - List restricted apps for an org or workspace.

### admin-conversations-archive

Manage admin conversations archive

- **`slack-pp-cli admin-conversations-archive admin_conversations_archive`** - Archive a public or private channel.

### admin-conversations-convert-to-private

Manage admin conversations convert to private

- **`slack-pp-cli admin-conversations-convert-to-private admin_conversations_convert_to_private`** - Convert a public channel to a private channel.

### admin-conversations-create

Manage admin conversations create

- **`slack-pp-cli admin-conversations-create admin_conversations_create`** - Create a public or private channel-based conversation.

### admin-conversations-delete

Manage admin conversations delete

- **`slack-pp-cli admin-conversations-delete admin_conversations_delete`** - Delete a public or private channel.

### admin-conversations-disconnect-shared

Manage admin conversations disconnect shared

- **`slack-pp-cli admin-conversations-disconnect-shared admin_conversations_disconnect_shared`** - Disconnect a connected channel from one or more workspaces.

### admin-conversations-ekm-list-original-connected-channel-info

Manage admin conversations ekm list original connected channel info

- **`slack-pp-cli admin-conversations-ekm-list-original-connected-channel-info admin_conversations_ekm_list_original_connected_channel_info`** - List all disconnected channelsâ€”i.e., channels that were once connected to other workspaces and then disconnectedâ€”and the corresponding original channel IDs for key revocation with EKM.

### admin-conversations-get-conversation-prefs

Manage admin conversations get conversation prefs

- **`slack-pp-cli admin-conversations-get-conversation-prefs admin_conversations_get_conversation_prefs`** - Get conversation preferences for a public or private channel.

### admin-conversations-get-teams

Manage admin conversations get teams

- **`slack-pp-cli admin-conversations-get-teams admin_conversations_get_teams`** - Get all the workspaces a given public or private channel is connected to within this Enterprise org.

### admin-conversations-invite

Manage admin conversations invite

- **`slack-pp-cli admin-conversations-invite admin_conversations_invite`** - Invite a user to a public or private channel.

### admin-conversations-rename

Manage admin conversations rename

- **`slack-pp-cli admin-conversations-rename admin_conversations_rename`** - Rename a public or private channel.

### admin-conversations-restrict-access-add-group

Manage admin conversations restrict access add group

- **`slack-pp-cli admin-conversations-restrict-access-add-group admin_conversations_restrict_access_add_group`** - Add an allowlist of IDP groups for accessing a channel

### admin-conversations-restrict-access-list-groups

Manage admin conversations restrict access list groups

- **`slack-pp-cli admin-conversations-restrict-access-list-groups admin_conversations_restrict_access_list_groups`** - List all IDP Groups linked to a channel

### admin-conversations-restrict-access-remove-group

Manage admin conversations restrict access remove group

- **`slack-pp-cli admin-conversations-restrict-access-remove-group admin_conversations_restrict_access_remove_group`** - Remove a linked IDP group linked from a private channel

### admin-conversations-search

Manage admin conversations search

- **`slack-pp-cli admin-conversations-search admin_conversations_search`** - Search for public or private channels in an Enterprise organization.

### admin-conversations-set-conversation-prefs

Manage admin conversations set conversation prefs

- **`slack-pp-cli admin-conversations-set-conversation-prefs admin_conversations_set_conversation_prefs`** - Set the posting permissions for a public or private channel.

### admin-conversations-set-teams

Manage admin conversations set teams

- **`slack-pp-cli admin-conversations-set-teams admin_conversations_set_teams`** - Set the workspaces in an Enterprise grid org that connect to a public or private channel.

### admin-conversations-unarchive

Manage admin conversations unarchive

- **`slack-pp-cli admin-conversations-unarchive admin_conversations_unarchive`** - Unarchive a public or private channel.

### admin-emoji-add

Manage admin emoji add

- **`slack-pp-cli admin-emoji-add admin_emoji_add`** - Add an emoji.

### admin-emoji-add-alias

Manage admin emoji add alias

- **`slack-pp-cli admin-emoji-add-alias admin_emoji_add_alias`** - Add an emoji alias.

### admin-emoji-list

Manage admin emoji list

- **`slack-pp-cli admin-emoji-list admin_emoji_list`** - List emoji for an Enterprise Grid organization.

### admin-emoji-remove

Manage admin emoji remove

- **`slack-pp-cli admin-emoji-remove admin_emoji_remove`** - Remove an emoji across an Enterprise Grid organization

### admin-emoji-rename

Manage admin emoji rename

- **`slack-pp-cli admin-emoji-rename admin_emoji_rename`** - Rename an emoji.

### admin-invite-requests-approve

Manage admin invite requests approve

- **`slack-pp-cli admin-invite-requests-approve admin_invite_requests_approve`** - Approve a workspace invite request.

### admin-invite-requests-approved-list

Manage admin invite requests approved list

- **`slack-pp-cli admin-invite-requests-approved-list admin_invite_requests_approved_list`** - List all approved workspace invite requests.

### admin-invite-requests-denied-list

Manage admin invite requests denied list

- **`slack-pp-cli admin-invite-requests-denied-list admin_invite_requests_denied_list`** - List all denied workspace invite requests.

### admin-invite-requests-deny

Manage admin invite requests deny

- **`slack-pp-cli admin-invite-requests-deny admin_invite_requests_deny`** - Deny a workspace invite request.

### admin-invite-requests-list

Manage admin invite requests list

- **`slack-pp-cli admin-invite-requests-list admin_invite_requests_list`** - List all pending workspace invite requests.

### admin-teams-admins-list

Manage admin teams admins list

- **`slack-pp-cli admin-teams-admins-list admin_teams_admins_list`** - List all of the admins on a given workspace.

### admin-teams-create

Manage admin teams create

- **`slack-pp-cli admin-teams-create admin_teams_create`** - Create an Enterprise team.

### admin-teams-list

Manage admin teams list

- **`slack-pp-cli admin-teams-list admin_teams_list`** - List all teams on an Enterprise organization

### admin-teams-owners-list

Manage admin teams owners list

- **`slack-pp-cli admin-teams-owners-list admin_teams_owners_list`** - List all of the owners on a given workspace.

### admin-teams-settings-info

Manage admin teams settings info

- **`slack-pp-cli admin-teams-settings-info admin_teams_settings_info`** - Fetch information about settings in a workspace

### admin-teams-settings-set-default-channels

Manage admin teams settings set default channels

- **`slack-pp-cli admin-teams-settings-set-default-channels admin_teams_settings_set_default_channels`** - Set the default channels of a workspace.

### admin-teams-settings-set-description

Manage admin teams settings set description

- **`slack-pp-cli admin-teams-settings-set-description admin_teams_settings_set_description`** - Set the description of a given workspace.

### admin-teams-settings-set-discoverability

Manage admin teams settings set discoverability

- **`slack-pp-cli admin-teams-settings-set-discoverability admin_teams_settings_set_discoverability`** - An API method that allows admins to set the discoverability of a given workspace

### admin-teams-settings-set-icon

Manage admin teams settings set icon

- **`slack-pp-cli admin-teams-settings-set-icon admin_teams_settings_set_icon`** - Sets the icon of a workspace.

### admin-teams-settings-set-name

Manage admin teams settings set name

- **`slack-pp-cli admin-teams-settings-set-name admin_teams_settings_set_name`** - Set the name of a given workspace.

### admin-usergroups-add-channels

Manage admin usergroups add channels

- **`slack-pp-cli admin-usergroups-add-channels admin_usergroups_add_channels`** - Add one or more default channels to an IDP group.

### admin-usergroups-add-teams

Manage admin usergroups add teams

- **`slack-pp-cli admin-usergroups-add-teams admin_usergroups_add_teams`** - Associate one or more default workspaces with an organization-wide IDP group.

### admin-usergroups-list-channels

Manage admin usergroups list channels

- **`slack-pp-cli admin-usergroups-list-channels admin_usergroups_list_channels`** - List the channels linked to an org-level IDP group (user group).

### admin-usergroups-remove-channels

Manage admin usergroups remove channels

- **`slack-pp-cli admin-usergroups-remove-channels admin_usergroups_remove_channels`** - Remove one or more default channels from an org-level IDP group (user group).

### admin-users-assign

Manage admin users assign

- **`slack-pp-cli admin-users-assign admin_users_assign`** - Add an Enterprise user to a workspace.

### admin-users-invite

Manage admin users invite

- **`slack-pp-cli admin-users-invite admin_users_invite`** - Invite a user to a workspace.

### admin-users-list

Manage admin users list

- **`slack-pp-cli admin-users-list admin_users_list`** - List users on a workspace

### admin-users-remove

Manage admin users remove

- **`slack-pp-cli admin-users-remove admin_users_remove`** - Remove a user from a workspace.

### admin-users-session-invalidate

Manage admin users session invalidate

- **`slack-pp-cli admin-users-session-invalidate admin_users_session_invalidate`** - Invalidate a single session for a user by session_id

### admin-users-session-reset

Manage admin users session reset

- **`slack-pp-cli admin-users-session-reset admin_users_session_reset`** - Wipes all valid sessions on all devices for a given user

### admin-users-set-admin

Manage admin users set admin

- **`slack-pp-cli admin-users-set-admin admin_users_set_admin`** - Set an existing guest, regular user, or owner to be an admin user.

### admin-users-set-expiration

Manage admin users set expiration

- **`slack-pp-cli admin-users-set-expiration admin_users_set_expiration`** - Set an expiration for a guest user

### admin-users-set-owner

Manage admin users set owner

- **`slack-pp-cli admin-users-set-owner admin_users_set_owner`** - Set an existing guest, regular user, or admin user to be a workspace owner.

### admin-users-set-regular

Manage admin users set regular

- **`slack-pp-cli admin-users-set-regular admin_users_set_regular`** - Set an existing guest user, admin user, or owner to be a regular user.

### api-test

Manage api test

- **`slack-pp-cli api-test test`** - Checks API calling code.

### apps-event-authorizations-list

Manage apps event authorizations list

- **`slack-pp-cli apps-event-authorizations-list apps_event_authorizations_list`** - Get a list of authorizations for the given event context. Each authorization represents an app installation that the event is visible to.

### apps-permissions-info

Manage apps permissions info

- **`slack-pp-cli apps-permissions-info apps_permissions_info`** - Returns list of permissions this app has on a team.

### apps-permissions-request

Manage apps permissions request

- **`slack-pp-cli apps-permissions-request apps_permissions_request`** - Allows an app to request additional scopes

### apps-permissions-resources-list

Manage apps permissions resources list

- **`slack-pp-cli apps-permissions-resources-list apps_permissions_resources_list`** - Returns list of resource grants this app has on a team.

### apps-permissions-scopes-list

Manage apps permissions scopes list

- **`slack-pp-cli apps-permissions-scopes-list apps_permissions_scopes_list`** - Returns list of scopes this app has on a team.

### apps-permissions-users-list

Manage apps permissions users list

- **`slack-pp-cli apps-permissions-users-list apps_permissions_users_list`** - Returns list of user grants and corresponding scopes this app has on a team.

### apps-permissions-users-request

Manage apps permissions users request

- **`slack-pp-cli apps-permissions-users-request apps_permissions_users_request`** - Enables an app to trigger a permissions modal to grant an app access to a user access scope.

### apps-uninstall

Manage apps uninstall

- **`slack-pp-cli apps-uninstall apps_uninstall`** - Uninstalls your app from a workspace.

### auth-revoke

Manage auth revoke

- **`slack-pp-cli auth-revoke auth_revoke`** - Revokes a token.

### auth-test

Manage auth test

- **`slack-pp-cli auth-test auth_test`** - Checks authentication & identity.

### bots-info

Manage bots info

- **`slack-pp-cli bots-info bots_info`** - Gets information about a bot user.

### calls-add

Manage calls add

- **`slack-pp-cli calls-add calls_add`** - Registers a new Call.

### calls-end

Manage calls end

- **`slack-pp-cli calls-end calls_end`** - Ends a Call.

### calls-info

Manage calls info

- **`slack-pp-cli calls-info calls_info`** - Returns information about a Call.

### calls-participants-add

Manage calls participants add

- **`slack-pp-cli calls-participants-add calls_participants_add`** - Registers new participants added to a Call.

### calls-participants-remove

Manage calls participants remove

- **`slack-pp-cli calls-participants-remove calls_participants_remove`** - Registers participants removed from a Call.

### calls-update

Manage calls update

- **`slack-pp-cli calls-update calls_update`** - Updates information about a Call.

### chat-delete

Manage chat delete

- **`slack-pp-cli chat-delete chat_delete`** - Deletes a message.

### chat-delete-scheduled-message

Manage chat delete scheduled message

- **`slack-pp-cli chat-delete-scheduled-message chat_delete_scheduled_message`** - Deletes a pending scheduled message from the queue.

### chat-get-permalink

Manage chat get permalink

- **`slack-pp-cli chat-get-permalink chat_get_permalink`** - Retrieve a permalink URL for a specific extant message

### chat-me-message

Manage chat me message

- **`slack-pp-cli chat-me-message chat_me_message`** - Share a me message into a channel.

### chat-post-ephemeral

Manage chat post ephemeral

- **`slack-pp-cli chat-post-ephemeral chat_post_ephemeral`** - Sends an ephemeral message to a user in a channel.

### chat-post-message

Manage chat post message

- **`slack-pp-cli chat-post-message chat_post_message`** - Sends a message to a channel.

### chat-schedule-message

Manage chat schedule message

- **`slack-pp-cli chat-schedule-message chat_schedule_message`** - Schedules a message to be sent to a channel.

### chat-scheduled-messages-list

Manage chat scheduled messages list

- **`slack-pp-cli chat-scheduled-messages-list chat_scheduled_messages_list`** - Returns a list of scheduled messages.

### chat-unfurl

Manage chat unfurl

- **`slack-pp-cli chat-unfurl chat_unfurl`** - Provide custom unfurl behavior for user-posted URLs

### chat-update

Manage chat update

- **`slack-pp-cli chat-update chat_update`** - Updates a message.

### conversations-archive

Manage conversations archive

- **`slack-pp-cli conversations-archive conversations_archive`** - Archives a conversation.

### conversations-close

Manage conversations close

- **`slack-pp-cli conversations-close conversations_close`** - Closes a direct message or multi-person direct message.

### conversations-create

Manage conversations create

- **`slack-pp-cli conversations-create conversations_create`** - Initiates a public or private channel-based conversation

### conversations-history

Manage conversations history

- **`slack-pp-cli conversations-history conversations_history`** - Fetches a conversation's history of messages and events.

### conversations-info

Manage conversations info

- **`slack-pp-cli conversations-info conversations_info`** - Retrieve information about a conversation.

### conversations-invite

Manage conversations invite

- **`slack-pp-cli conversations-invite conversations_invite`** - Invites users to a channel.

### conversations-join

Manage conversations join

- **`slack-pp-cli conversations-join conversations_join`** - Joins an existing conversation.

### conversations-kick

Manage conversations kick

- **`slack-pp-cli conversations-kick conversations_kick`** - Removes a user from a conversation.

### conversations-leave

Manage conversations leave

- **`slack-pp-cli conversations-leave conversations_leave`** - Leaves a conversation.

### conversations-list

Manage conversations list

- **`slack-pp-cli conversations-list conversations_list`** - Lists all channels in a Slack team.

### conversations-mark

Manage conversations mark

- **`slack-pp-cli conversations-mark conversations_mark`** - Sets the read cursor in a channel.

### conversations-members

Manage conversations members

- **`slack-pp-cli conversations-members conversations_members`** - Retrieve members of a conversation.

### conversations-open

Manage conversations open

- **`slack-pp-cli conversations-open conversations_open`** - Opens or resumes a direct message or multi-person direct message.

### conversations-rename

Manage conversations rename

- **`slack-pp-cli conversations-rename conversations_rename`** - Renames a conversation.

### conversations-replies

Manage conversations replies

- **`slack-pp-cli conversations-replies conversations_replies`** - Retrieve a thread of messages posted to a conversation

### conversations-set-purpose

Manage conversations set purpose

- **`slack-pp-cli conversations-set-purpose conversations_set_purpose`** - Sets the purpose for a conversation.

### conversations-set-topic

Manage conversations set topic

- **`slack-pp-cli conversations-set-topic conversations_set_topic`** - Sets the topic for a conversation.

### conversations-unarchive

Manage conversations unarchive

- **`slack-pp-cli conversations-unarchive conversations_unarchive`** - Reverses conversation archival.

### dialog-open

Manage dialog open

- **`slack-pp-cli dialog-open dialog_open`** - Open a dialog with a user

### dnd-end-dnd

Manage dnd end dnd

- **`slack-pp-cli dnd-end-dnd dnd_end_dnd`** - Ends the current user's Do Not Disturb session immediately.

### dnd-end-snooze

Manage dnd end snooze

- **`slack-pp-cli dnd-end-snooze dnd_end_snooze`** - Ends the current user's snooze mode immediately.

### dnd-info

Manage dnd info

- **`slack-pp-cli dnd-info dnd_info`** - Retrieves a user's current Do Not Disturb status.

### dnd-set-snooze

Manage dnd set snooze

- **`slack-pp-cli dnd-set-snooze dnd_set_snooze`** - Turns on Do Not Disturb mode for the current user, or changes its duration.

### dnd-team-info

Manage dnd team info

- **`slack-pp-cli dnd-team-info dnd_team_info`** - Retrieves the Do Not Disturb status for up to 50 users on a team.

### emoji-list

Manage emoji list

- **`slack-pp-cli emoji-list emoji_list`** - Lists custom emoji for a team.

### files-comments-delete

Manage files comments delete

- **`slack-pp-cli files-comments-delete files_comments_delete`** - Deletes an existing comment on a file.

### files-delete

Manage files delete

- **`slack-pp-cli files-delete files_delete`** - Deletes a file.

### files-info

Manage files info

- **`slack-pp-cli files-info files_info`** - Gets information about a file.

### files-list

Manage files list

- **`slack-pp-cli files-list files_list`** - List for a team, in a channel, or from a user with applied filters.

### files-remote-add

Manage files remote add

- **`slack-pp-cli files-remote-add files_remote_add`** - Adds a file from a remote service

### files-remote-info

Manage files remote info

- **`slack-pp-cli files-remote-info files_remote_info`** - Retrieve information about a remote file added to Slack

### files-remote-list

Manage files remote list

- **`slack-pp-cli files-remote-list files_remote_list`** - Retrieve information about a remote file added to Slack

### files-remote-remove

Manage files remote remove

- **`slack-pp-cli files-remote-remove files_remote_remove`** - Remove a remote file.

### files-remote-share

Manage files remote share

- **`slack-pp-cli files-remote-share files_remote_share`** - Share a remote file into a channel.

### files-remote-update

Manage files remote update

- **`slack-pp-cli files-remote-update files_remote_update`** - Updates an existing remote file.

### files-revoke-public-url

Manage files revoke public url

- **`slack-pp-cli files-revoke-public-url files_revoke_public_url`** - Revokes public/external sharing access for a file

### files-shared-public-url

Manage files shared public url

- **`slack-pp-cli files-shared-public-url files_shared_public_url`** - Enables a file for public/external sharing.

### files-upload

Manage files upload

- **`slack-pp-cli files-upload files_upload`** - Uploads or creates a file.

### migration-exchange

Manage migration exchange

- **`slack-pp-cli migration-exchange migration_exchange`** - For Enterprise Grid workspaces, map local user IDs to global user IDs

### oauth-access

Manage oauth access

- **`slack-pp-cli oauth-access oauth_access`** - Exchanges a temporary OAuth verifier code for an access token.

### oauth-token

Manage oauth token

- **`slack-pp-cli oauth-token oauth_token`** - Exchanges a temporary OAuth verifier code for a workspace token.

### oauth-v2-access

Manage oauth v2 access

- **`slack-pp-cli oauth-v2-access oauth_v2_access`** - Exchanges a temporary OAuth verifier code for an access token.

### pins-add

Manage pins add

- **`slack-pp-cli pins-add pins_add`** - Pins an item to a channel.

### pins-list

Manage pins list

- **`slack-pp-cli pins-list pins_list`** - Lists items pinned to a channel.

### pins-remove

Manage pins remove

- **`slack-pp-cli pins-remove pins_remove`** - Un-pins an item from a channel.

### reactions-add

Manage reactions add

- **`slack-pp-cli reactions-add reactions_add`** - Adds a reaction to an item.

### reactions-get

Manage reactions get

- **`slack-pp-cli reactions-get reactions_get`** - Gets reactions for an item.

### reactions-list

Manage reactions list

- **`slack-pp-cli reactions-list reactions_list`** - Lists reactions made by a user.

### reactions-remove

Manage reactions remove

- **`slack-pp-cli reactions-remove reactions_remove`** - Removes a reaction from an item.

### reminders-add

Manage reminders add

- **`slack-pp-cli reminders-add reminders_add`** - Creates a reminder.

### reminders-complete

Manage reminders complete

- **`slack-pp-cli reminders-complete reminders_complete`** - Marks a reminder as complete.

### reminders-delete

Manage reminders delete

- **`slack-pp-cli reminders-delete reminders_delete`** - Deletes a reminder.

### reminders-info

Manage reminders info

- **`slack-pp-cli reminders-info reminders_info`** - Gets information about a reminder.

### reminders-list

Manage reminders list

- **`slack-pp-cli reminders-list reminders_list`** - Lists all reminders created by or for a given user.

### rtm-connect

Manage rtm connect

- **`slack-pp-cli rtm-connect rtm_connect`** - Starts a Real Time Messaging session.

### search-messages

Manage search messages

- **`slack-pp-cli search-messages search_messages`** - Searches for messages matching a query.

### stars-add

Manage stars add

- **`slack-pp-cli stars-add stars_add`** - Adds a star to an item.

### stars-list

Manage stars list

- **`slack-pp-cli stars-list stars_list`** - Lists stars for a user.

### stars-remove

Manage stars remove

- **`slack-pp-cli stars-remove stars_remove`** - Removes a star from an item.

### team-access-logs

Manage team access logs

- **`slack-pp-cli team-access-logs team_access_logs`** - Gets the access logs for the current team.

### team-billable-info

Manage team billable info

- **`slack-pp-cli team-billable-info team_billable_info`** - Gets billable users information for the current team.

### team-info

Manage team info

- **`slack-pp-cli team-info team_info`** - Gets information about the current team.

### team-integration-logs

Manage team integration logs

- **`slack-pp-cli team-integration-logs team_integration_logs`** - Gets the integration logs for the current team.

### team-profile-get

Manage team profile get

- **`slack-pp-cli team-profile-get team_profile_get`** - Retrieve a team's profile.

### usergroups-create

Manage usergroups create

- **`slack-pp-cli usergroups-create usergroups_create`** - Create a User Group

### usergroups-disable

Manage usergroups disable

- **`slack-pp-cli usergroups-disable usergroups_disable`** - Disable an existing User Group

### usergroups-enable

Manage usergroups enable

- **`slack-pp-cli usergroups-enable usergroups_enable`** - Enable a User Group

### usergroups-list

Manage usergroups list

- **`slack-pp-cli usergroups-list usergroups_list`** - List all User Groups for a team

### usergroups-update

Manage usergroups update

- **`slack-pp-cli usergroups-update usergroups_update`** - Update an existing User Group

### usergroups-users-list

Manage usergroups users list

- **`slack-pp-cli usergroups-users-list usergroups_users_list`** - List all users in a User Group

### usergroups-users-update

Manage usergroups users update

- **`slack-pp-cli usergroups-users-update usergroups_users_update`** - Update the list of users for a User Group

### users-conversations

Manage users conversations

- **`slack-pp-cli users-conversations users_conversations`** - List conversations the calling user may access.

### users-delete-photo

Manage users delete photo

- **`slack-pp-cli users-delete-photo users_delete_photo`** - Delete the user profile photo

### users-get-presence

Manage users get presence

- **`slack-pp-cli users-get-presence users_get_presence`** - Gets user presence information.

### users-identity

Manage users identity

- **`slack-pp-cli users-identity users_identity`** - Get a user's identity.

### users-info

Manage users info

- **`slack-pp-cli users-info users_info`** - Gets information about a user.

### users-list

Manage users list

- **`slack-pp-cli users-list users_list`** - Lists all users in a Slack team.

### users-lookup-by-email

Manage users lookup by email

- **`slack-pp-cli users-lookup-by-email users_lookup_by_email`** - Find a user with an email address.

### users-profile-get

Manage users profile get

- **`slack-pp-cli users-profile-get users_profile_get`** - Retrieves a user's profile information.

### users-profile-set

Manage users profile set

- **`slack-pp-cli users-profile-set users_profile_set`** - Set the profile information for a user.

### users-set-active

Manage users set active

- **`slack-pp-cli users-set-active users_set_active`** - Marked a user as active. Deprecated and non-functional.

### users-set-photo

Manage users set photo

- **`slack-pp-cli users-set-photo users_set_photo`** - Set the user profile photo

### users-set-presence

Manage users set presence

- **`slack-pp-cli users-set-presence users_set_presence`** - Manually sets user presence.

### views-open

Manage views open

- **`slack-pp-cli views-open views_open`** - Open a view for a user.

### views-publish

Manage views publish

- **`slack-pp-cli views-publish views_publish`** - Publish a static view for a User.

### views-push

Manage views push

- **`slack-pp-cli views-push views_push`** - Push a view onto the stack of a root view.

### views-update

Manage views update

- **`slack-pp-cli views-update views_update`** - Update an existing view.

### workflows-step-completed

Manage workflows step completed

- **`slack-pp-cli workflows-step-completed workflows_step_completed`** - Indicate that an app's step in a workflow completed execution.

### workflows-step-failed

Manage workflows step failed

- **`slack-pp-cli workflows-step-failed workflows_step_failed`** - Indicate that an app's step in a workflow failed to execute.

### workflows-update-step

Manage workflows update step

- **`slack-pp-cli workflows-update-step workflows_update_step`** - Update the configuration for a workflow extension step.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
slack-pp-cli admin-apps-approve

# JSON for scripting and agents
slack-pp-cli admin-apps-approve --json

# Filter to specific fields
slack-pp-cli admin-apps-approve --json --select id,name,status

# Dry run — show the request without sending
slack-pp-cli admin-apps-approve --dry-run

# Agent mode — JSON + compact + no prompts in one flag
slack-pp-cli admin-apps-approve --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-slack -g
```

Then invoke `/pp-slack <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add slack slack-pp-mcp -e SLACK_USER_TOKEN=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/slack-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SLACK_USER_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "slack": {
      "command": "slack-pp-mcp",
      "env": {
        "SLACK_USER_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
slack-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/slack-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SLACK_USER_TOKEN` | per_call | Yes | Set to your API credential. |
| `SLACK_BOT_TOKEN` | per_call | No | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `slack-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SLACK_USER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **not_authed or invalid_auth on every call** — Confirm SLACK_USER_TOKEN holds a valid xoxp token; run slack-pp-cli doctor to see which source resolved.
- **conversations history returns far fewer messages than --limit requested** — Slack's May-2025 page-size cap may be active for the token; the CLI logs a silent-truncation warning — re-run sync which paginates with Retry-After respect.
- **customer-intel-deep reports a missing sibling mirror** — It needs the pp-attio / pp-asana / pp-fathom SQLite mirrors co-located; run their sync commands or pass --skip-missing to degrade gracefully.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**modelcontextprotocol/servers (slack, archived)**](https://github.com/modelcontextprotocol/servers-archived) — TypeScript
- [**korotovsky/slack-mcp-server**](https://github.com/korotovsky/slack-mcp-server) — Go
- [**rusq/slackdump**](https://github.com/rusq/slackdump) — Go
- [**piekstra/slack-mcp-server**](https://github.com/piekstra/slack-mcp-server) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
