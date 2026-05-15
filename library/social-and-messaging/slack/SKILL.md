---
name: pp-slack
description: "Every Slack Web API endpoint plus a local SQLite mirror for offline FTS5 search, cross-source customer timelines, and cron-safe posting. Trigger phrases: `search Slack for`, `what did Slack say about`, `customer intel for`, `prep my 1:1`, `post to Slack`, `use slack-pp-cli`, `run slack-pp-cli`."
author: "Erick Holm"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - slack-pp-cli
---

# Slack — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `slack-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install slack --cli-only
   ```
2. Verify: `slack-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI wraps the full 174-endpoint Slack Web API and keeps a local SQLite mirror of channels, users, messages, threads, reactions, and usergroups. The mirror powers offline FTS5 search and transcendence verbs — customer-intel-deep joins Slack against co-located Attio, Asana, and Fathom mirrors; dm-engagement and goal-channel-pulse compose cross-source volumes; agent-audit reads the CLI's own DM-access log. Every write verb supports --dry-run for cron safety.

## When to Use This CLI

Reach for slack-pp-cli when an agent task needs Slack workspace data composed with history — weekly customer sweeps, L10 prep, pre-1:1 engagement volumes, cron-posted alerts — rather than a single live lookup. Its local mirror makes repeated and offline queries cheap, and its cross-source verbs answer questions no single Slack API call can. For a one-off read of one channel, the Slack MCP is lighter.

## Unique Capabilities

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

## Command Reference

**admin-apps-approve** — Manage admin apps approve

- `slack-pp-cli admin-apps-approve` — Approve an app for installation on a workspace.

**admin-apps-approved-list** — Manage admin apps approved list

- `slack-pp-cli admin-apps-approved-list` — List approved apps for an org or workspace.

**admin-apps-requests-list** — Manage admin apps requests list

- `slack-pp-cli admin-apps-requests-list` — List app requests for a team/workspace.

**admin-apps-restrict** — Manage admin apps restrict

- `slack-pp-cli admin-apps-restrict` — Restrict an app for installation on a workspace.

**admin-apps-restricted-list** — Manage admin apps restricted list

- `slack-pp-cli admin-apps-restricted-list` — List restricted apps for an org or workspace.

**admin-conversations-archive** — Manage admin conversations archive

- `slack-pp-cli admin-conversations-archive` — Archive a public or private channel.

**admin-conversations-convert-to-private** — Manage admin conversations convert to private

- `slack-pp-cli admin-conversations-convert-to-private` — Convert a public channel to a private channel.

**admin-conversations-create** — Manage admin conversations create

- `slack-pp-cli admin-conversations-create` — Create a public or private channel-based conversation.

**admin-conversations-delete** — Manage admin conversations delete

- `slack-pp-cli admin-conversations-delete` — Delete a public or private channel.

**admin-conversations-disconnect-shared** — Manage admin conversations disconnect shared

- `slack-pp-cli admin-conversations-disconnect-shared` — Disconnect a connected channel from one or more workspaces.

**admin-conversations-ekm-list-original-connected-channel-info** — Manage admin conversations ekm list original connected channel info

- `slack-pp-cli admin-conversations-ekm-list-original-connected-channel-info` — List all disconnected channelsâ€”i.e., channels that were once connected to other workspaces and then...

**admin-conversations-get-conversation-prefs** — Manage admin conversations get conversation prefs

- `slack-pp-cli admin-conversations-get-conversation-prefs` — Get conversation preferences for a public or private channel.

**admin-conversations-get-teams** — Manage admin conversations get teams

- `slack-pp-cli admin-conversations-get-teams` — Get all the workspaces a given public or private channel is connected to within this Enterprise org.

**admin-conversations-invite** — Manage admin conversations invite

- `slack-pp-cli admin-conversations-invite` — Invite a user to a public or private channel.

**admin-conversations-rename** — Manage admin conversations rename

- `slack-pp-cli admin-conversations-rename` — Rename a public or private channel.

**admin-conversations-restrict-access-add-group** — Manage admin conversations restrict access add group

- `slack-pp-cli admin-conversations-restrict-access-add-group` — Add an allowlist of IDP groups for accessing a channel

**admin-conversations-restrict-access-list-groups** — Manage admin conversations restrict access list groups

- `slack-pp-cli admin-conversations-restrict-access-list-groups` — List all IDP Groups linked to a channel

**admin-conversations-restrict-access-remove-group** — Manage admin conversations restrict access remove group

- `slack-pp-cli admin-conversations-restrict-access-remove-group` — Remove a linked IDP group linked from a private channel

**admin-conversations-search** — Manage admin conversations search

- `slack-pp-cli admin-conversations-search` — Search for public or private channels in an Enterprise organization.

**admin-conversations-set-conversation-prefs** — Manage admin conversations set conversation prefs

- `slack-pp-cli admin-conversations-set-conversation-prefs` — Set the posting permissions for a public or private channel.

**admin-conversations-set-teams** — Manage admin conversations set teams

- `slack-pp-cli admin-conversations-set-teams` — Set the workspaces in an Enterprise grid org that connect to a public or private channel.

**admin-conversations-unarchive** — Manage admin conversations unarchive

- `slack-pp-cli admin-conversations-unarchive` — Unarchive a public or private channel.

**admin-emoji-add** — Manage admin emoji add

- `slack-pp-cli admin-emoji-add` — Add an emoji.

**admin-emoji-add-alias** — Manage admin emoji add alias

- `slack-pp-cli admin-emoji-add-alias` — Add an emoji alias.

**admin-emoji-list** — Manage admin emoji list

- `slack-pp-cli admin-emoji-list` — List emoji for an Enterprise Grid organization.

**admin-emoji-remove** — Manage admin emoji remove

- `slack-pp-cli admin-emoji-remove` — Remove an emoji across an Enterprise Grid organization

**admin-emoji-rename** — Manage admin emoji rename

- `slack-pp-cli admin-emoji-rename` — Rename an emoji.

**admin-invite-requests-approve** — Manage admin invite requests approve

- `slack-pp-cli admin-invite-requests-approve` — Approve a workspace invite request.

**admin-invite-requests-approved-list** — Manage admin invite requests approved list

- `slack-pp-cli admin-invite-requests-approved-list` — List all approved workspace invite requests.

**admin-invite-requests-denied-list** — Manage admin invite requests denied list

- `slack-pp-cli admin-invite-requests-denied-list` — List all denied workspace invite requests.

**admin-invite-requests-deny** — Manage admin invite requests deny

- `slack-pp-cli admin-invite-requests-deny` — Deny a workspace invite request.

**admin-invite-requests-list** — Manage admin invite requests list

- `slack-pp-cli admin-invite-requests-list` — List all pending workspace invite requests.

**admin-teams-admins-list** — Manage admin teams admins list

- `slack-pp-cli admin-teams-admins-list` — List all of the admins on a given workspace.

**admin-teams-create** — Manage admin teams create

- `slack-pp-cli admin-teams-create` — Create an Enterprise team.

**admin-teams-list** — Manage admin teams list

- `slack-pp-cli admin-teams-list` — List all teams on an Enterprise organization

**admin-teams-owners-list** — Manage admin teams owners list

- `slack-pp-cli admin-teams-owners-list` — List all of the owners on a given workspace.

**admin-teams-settings-info** — Manage admin teams settings info

- `slack-pp-cli admin-teams-settings-info` — Fetch information about settings in a workspace

**admin-teams-settings-set-default-channels** — Manage admin teams settings set default channels

- `slack-pp-cli admin-teams-settings-set-default-channels` — Set the default channels of a workspace.

**admin-teams-settings-set-description** — Manage admin teams settings set description

- `slack-pp-cli admin-teams-settings-set-description` — Set the description of a given workspace.

**admin-teams-settings-set-discoverability** — Manage admin teams settings set discoverability

- `slack-pp-cli admin-teams-settings-set-discoverability` — An API method that allows admins to set the discoverability of a given workspace

**admin-teams-settings-set-icon** — Manage admin teams settings set icon

- `slack-pp-cli admin-teams-settings-set-icon` — Sets the icon of a workspace.

**admin-teams-settings-set-name** — Manage admin teams settings set name

- `slack-pp-cli admin-teams-settings-set-name` — Set the name of a given workspace.

**admin-usergroups-add-channels** — Manage admin usergroups add channels

- `slack-pp-cli admin-usergroups-add-channels` — Add one or more default channels to an IDP group.

**admin-usergroups-add-teams** — Manage admin usergroups add teams

- `slack-pp-cli admin-usergroups-add-teams` — Associate one or more default workspaces with an organization-wide IDP group.

**admin-usergroups-list-channels** — Manage admin usergroups list channels

- `slack-pp-cli admin-usergroups-list-channels` — List the channels linked to an org-level IDP group (user group).

**admin-usergroups-remove-channels** — Manage admin usergroups remove channels

- `slack-pp-cli admin-usergroups-remove-channels` — Remove one or more default channels from an org-level IDP group (user group).

**admin-users-assign** — Manage admin users assign

- `slack-pp-cli admin-users-assign` — Add an Enterprise user to a workspace.

**admin-users-invite** — Manage admin users invite

- `slack-pp-cli admin-users-invite` — Invite a user to a workspace.

**admin-users-list** — Manage admin users list

- `slack-pp-cli admin-users-list` — List users on a workspace

**admin-users-remove** — Manage admin users remove

- `slack-pp-cli admin-users-remove` — Remove a user from a workspace.

**admin-users-session-invalidate** — Manage admin users session invalidate

- `slack-pp-cli admin-users-session-invalidate` — Invalidate a single session for a user by session_id

**admin-users-session-reset** — Manage admin users session reset

- `slack-pp-cli admin-users-session-reset` — Wipes all valid sessions on all devices for a given user

**admin-users-set-admin** — Manage admin users set admin

- `slack-pp-cli admin-users-set-admin` — Set an existing guest, regular user, or owner to be an admin user.

**admin-users-set-expiration** — Manage admin users set expiration

- `slack-pp-cli admin-users-set-expiration` — Set an expiration for a guest user

**admin-users-set-owner** — Manage admin users set owner

- `slack-pp-cli admin-users-set-owner` — Set an existing guest, regular user, or admin user to be a workspace owner.

**admin-users-set-regular** — Manage admin users set regular

- `slack-pp-cli admin-users-set-regular` — Set an existing guest user, admin user, or owner to be a regular user.

**api-test** — Manage api test

- `slack-pp-cli api-test` — Checks API calling code.

**apps-event-authorizations-list** — Manage apps event authorizations list

- `slack-pp-cli apps-event-authorizations-list` — Get a list of authorizations for the given event context. Each authorization represents an app installation that the...

**apps-permissions-info** — Manage apps permissions info

- `slack-pp-cli apps-permissions-info` — Returns list of permissions this app has on a team.

**apps-permissions-request** — Manage apps permissions request

- `slack-pp-cli apps-permissions-request` — Allows an app to request additional scopes

**apps-permissions-resources-list** — Manage apps permissions resources list

- `slack-pp-cli apps-permissions-resources-list` — Returns list of resource grants this app has on a team.

**apps-permissions-scopes-list** — Manage apps permissions scopes list

- `slack-pp-cli apps-permissions-scopes-list` — Returns list of scopes this app has on a team.

**apps-permissions-users-list** — Manage apps permissions users list

- `slack-pp-cli apps-permissions-users-list` — Returns list of user grants and corresponding scopes this app has on a team.

**apps-permissions-users-request** — Manage apps permissions users request

- `slack-pp-cli apps-permissions-users-request` — Enables an app to trigger a permissions modal to grant an app access to a user access scope.

**apps-uninstall** — Manage apps uninstall

- `slack-pp-cli apps-uninstall` — Uninstalls your app from a workspace.

**auth-revoke** — Manage auth revoke

- `slack-pp-cli auth-revoke` — Revokes a token.

**auth-test** — Manage auth test

- `slack-pp-cli auth-test` — Checks authentication & identity.

**bots-info** — Manage bots info

- `slack-pp-cli bots-info` — Gets information about a bot user.

**calls-add** — Manage calls add

- `slack-pp-cli calls-add` — Registers a new Call.

**calls-end** — Manage calls end

- `slack-pp-cli calls-end` — Ends a Call.

**calls-info** — Manage calls info

- `slack-pp-cli calls-info` — Returns information about a Call.

**calls-participants-add** — Manage calls participants add

- `slack-pp-cli calls-participants-add` — Registers new participants added to a Call.

**calls-participants-remove** — Manage calls participants remove

- `slack-pp-cli calls-participants-remove` — Registers participants removed from a Call.

**calls-update** — Manage calls update

- `slack-pp-cli calls-update` — Updates information about a Call.

**chat-delete** — Manage chat delete

- `slack-pp-cli chat-delete` — Deletes a message.

**chat-delete-scheduled-message** — Manage chat delete scheduled message

- `slack-pp-cli chat-delete-scheduled-message` — Deletes a pending scheduled message from the queue.

**chat-get-permalink** — Manage chat get permalink

- `slack-pp-cli chat-get-permalink` — Retrieve a permalink URL for a specific extant message

**chat-me-message** — Manage chat me message

- `slack-pp-cli chat-me-message` — Share a me message into a channel.

**chat-post-ephemeral** — Manage chat post ephemeral

- `slack-pp-cli chat-post-ephemeral` — Sends an ephemeral message to a user in a channel.

**chat-post-message** — Manage chat post message

- `slack-pp-cli chat-post-message` — Sends a message to a channel.

**chat-schedule-message** — Manage chat schedule message

- `slack-pp-cli chat-schedule-message` — Schedules a message to be sent to a channel.

**chat-scheduled-messages-list** — Manage chat scheduled messages list

- `slack-pp-cli chat-scheduled-messages-list` — Returns a list of scheduled messages.

**chat-unfurl** — Manage chat unfurl

- `slack-pp-cli chat-unfurl` — Provide custom unfurl behavior for user-posted URLs

**chat-update** — Manage chat update

- `slack-pp-cli chat-update` — Updates a message.

**conversations-archive** — Manage conversations archive

- `slack-pp-cli conversations-archive` — Archives a conversation.

**conversations-close** — Manage conversations close

- `slack-pp-cli conversations-close` — Closes a direct message or multi-person direct message.

**conversations-create** — Manage conversations create

- `slack-pp-cli conversations-create` — Initiates a public or private channel-based conversation

**conversations-history** — Manage conversations history

- `slack-pp-cli conversations-history` — Fetches a conversation's history of messages and events.

**conversations-info** — Manage conversations info

- `slack-pp-cli conversations-info` — Retrieve information about a conversation.

**conversations-invite** — Manage conversations invite

- `slack-pp-cli conversations-invite` — Invites users to a channel.

**conversations-join** — Manage conversations join

- `slack-pp-cli conversations-join` — Joins an existing conversation.

**conversations-kick** — Manage conversations kick

- `slack-pp-cli conversations-kick` — Removes a user from a conversation.

**conversations-leave** — Manage conversations leave

- `slack-pp-cli conversations-leave` — Leaves a conversation.

**conversations-list** — Manage conversations list

- `slack-pp-cli conversations-list` — Lists all channels in a Slack team.

**conversations-mark** — Manage conversations mark

- `slack-pp-cli conversations-mark` — Sets the read cursor in a channel.

**conversations-members** — Manage conversations members

- `slack-pp-cli conversations-members` — Retrieve members of a conversation.

**conversations-open** — Manage conversations open

- `slack-pp-cli conversations-open` — Opens or resumes a direct message or multi-person direct message.

**conversations-rename** — Manage conversations rename

- `slack-pp-cli conversations-rename` — Renames a conversation.

**conversations-replies** — Manage conversations replies

- `slack-pp-cli conversations-replies` — Retrieve a thread of messages posted to a conversation

**conversations-set-purpose** — Manage conversations set purpose

- `slack-pp-cli conversations-set-purpose` — Sets the purpose for a conversation.

**conversations-set-topic** — Manage conversations set topic

- `slack-pp-cli conversations-set-topic` — Sets the topic for a conversation.

**conversations-unarchive** — Manage conversations unarchive

- `slack-pp-cli conversations-unarchive` — Reverses conversation archival.

**dialog-open** — Manage dialog open

- `slack-pp-cli dialog-open` — Open a dialog with a user

**dnd-end-dnd** — Manage dnd end dnd

- `slack-pp-cli dnd-end-dnd` — Ends the current user's Do Not Disturb session immediately.

**dnd-end-snooze** — Manage dnd end snooze

- `slack-pp-cli dnd-end-snooze` — Ends the current user's snooze mode immediately.

**dnd-info** — Manage dnd info

- `slack-pp-cli dnd-info` — Retrieves a user's current Do Not Disturb status.

**dnd-set-snooze** — Manage dnd set snooze

- `slack-pp-cli dnd-set-snooze` — Turns on Do Not Disturb mode for the current user, or changes its duration.

**dnd-team-info** — Manage dnd team info

- `slack-pp-cli dnd-team-info` — Retrieves the Do Not Disturb status for up to 50 users on a team.

**emoji-list** — Manage emoji list

- `slack-pp-cli emoji-list` — Lists custom emoji for a team.

**files-comments-delete** — Manage files comments delete

- `slack-pp-cli files-comments-delete` — Deletes an existing comment on a file.

**files-delete** — Manage files delete

- `slack-pp-cli files-delete` — Deletes a file.

**files-info** — Manage files info

- `slack-pp-cli files-info` — Gets information about a file.

**files-list** — Manage files list

- `slack-pp-cli files-list` — List for a team, in a channel, or from a user with applied filters.

**files-remote-add** — Manage files remote add

- `slack-pp-cli files-remote-add` — Adds a file from a remote service

**files-remote-info** — Manage files remote info

- `slack-pp-cli files-remote-info` — Retrieve information about a remote file added to Slack

**files-remote-list** — Manage files remote list

- `slack-pp-cli files-remote-list` — Retrieve information about a remote file added to Slack

**files-remote-remove** — Manage files remote remove

- `slack-pp-cli files-remote-remove` — Remove a remote file.

**files-remote-share** — Manage files remote share

- `slack-pp-cli files-remote-share` — Share a remote file into a channel.

**files-remote-update** — Manage files remote update

- `slack-pp-cli files-remote-update` — Updates an existing remote file.

**files-revoke-public-url** — Manage files revoke public url

- `slack-pp-cli files-revoke-public-url` — Revokes public/external sharing access for a file

**files-shared-public-url** — Manage files shared public url

- `slack-pp-cli files-shared-public-url` — Enables a file for public/external sharing.

**files-upload** — Manage files upload

- `slack-pp-cli files-upload` — Uploads or creates a file.

**migration-exchange** — Manage migration exchange

- `slack-pp-cli migration-exchange` — For Enterprise Grid workspaces, map local user IDs to global user IDs

**oauth-access** — Manage oauth access

- `slack-pp-cli oauth-access` — Exchanges a temporary OAuth verifier code for an access token.

**oauth-token** — Manage oauth token

- `slack-pp-cli oauth-token` — Exchanges a temporary OAuth verifier code for a workspace token.

**oauth-v2-access** — Manage oauth v2 access

- `slack-pp-cli oauth-v2-access` — Exchanges a temporary OAuth verifier code for an access token.

**pins-add** — Manage pins add

- `slack-pp-cli pins-add` — Pins an item to a channel.

**pins-list** — Manage pins list

- `slack-pp-cli pins-list` — Lists items pinned to a channel.

**pins-remove** — Manage pins remove

- `slack-pp-cli pins-remove` — Un-pins an item from a channel.

**reactions-add** — Manage reactions add

- `slack-pp-cli reactions-add` — Adds a reaction to an item.

**reactions-get** — Manage reactions get

- `slack-pp-cli reactions-get` — Gets reactions for an item.

**reactions-list** — Manage reactions list

- `slack-pp-cli reactions-list` — Lists reactions made by a user.

**reactions-remove** — Manage reactions remove

- `slack-pp-cli reactions-remove` — Removes a reaction from an item.

**reminders-add** — Manage reminders add

- `slack-pp-cli reminders-add` — Creates a reminder.

**reminders-complete** — Manage reminders complete

- `slack-pp-cli reminders-complete` — Marks a reminder as complete.

**reminders-delete** — Manage reminders delete

- `slack-pp-cli reminders-delete` — Deletes a reminder.

**reminders-info** — Manage reminders info

- `slack-pp-cli reminders-info` — Gets information about a reminder.

**reminders-list** — Manage reminders list

- `slack-pp-cli reminders-list` — Lists all reminders created by or for a given user.

**rtm-connect** — Manage rtm connect

- `slack-pp-cli rtm-connect` — Starts a Real Time Messaging session.

**search-messages** — Manage search messages

- `slack-pp-cli search-messages` — Searches for messages matching a query.

**stars-add** — Manage stars add

- `slack-pp-cli stars-add` — Adds a star to an item.

**stars-list** — Manage stars list

- `slack-pp-cli stars-list` — Lists stars for a user.

**stars-remove** — Manage stars remove

- `slack-pp-cli stars-remove` — Removes a star from an item.

**team-access-logs** — Manage team access logs

- `slack-pp-cli team-access-logs` — Gets the access logs for the current team.

**team-billable-info** — Manage team billable info

- `slack-pp-cli team-billable-info` — Gets billable users information for the current team.

**team-info** — Manage team info

- `slack-pp-cli team-info` — Gets information about the current team.

**team-integration-logs** — Manage team integration logs

- `slack-pp-cli team-integration-logs` — Gets the integration logs for the current team.

**team-profile-get** — Manage team profile get

- `slack-pp-cli team-profile-get` — Retrieve a team's profile.

**usergroups-create** — Manage usergroups create

- `slack-pp-cli usergroups-create` — Create a User Group

**usergroups-disable** — Manage usergroups disable

- `slack-pp-cli usergroups-disable` — Disable an existing User Group

**usergroups-enable** — Manage usergroups enable

- `slack-pp-cli usergroups-enable` — Enable a User Group

**usergroups-list** — Manage usergroups list

- `slack-pp-cli usergroups-list` — List all User Groups for a team

**usergroups-update** — Manage usergroups update

- `slack-pp-cli usergroups-update` — Update an existing User Group

**usergroups-users-list** — Manage usergroups users list

- `slack-pp-cli usergroups-users-list` — List all users in a User Group

**usergroups-users-update** — Manage usergroups users update

- `slack-pp-cli usergroups-users-update` — Update the list of users for a User Group

**users-conversations** — Manage users conversations

- `slack-pp-cli users-conversations` — List conversations the calling user may access.

**users-delete-photo** — Manage users delete photo

- `slack-pp-cli users-delete-photo` — Delete the user profile photo

**users-get-presence** — Manage users get presence

- `slack-pp-cli users-get-presence` — Gets user presence information.

**users-identity** — Manage users identity

- `slack-pp-cli users-identity` — Get a user's identity.

**users-info** — Manage users info

- `slack-pp-cli users-info` — Gets information about a user.

**users-list** — Manage users list

- `slack-pp-cli users-list` — Lists all users in a Slack team.

**users-lookup-by-email** — Manage users lookup by email

- `slack-pp-cli users-lookup-by-email` — Find a user with an email address.

**users-profile-get** — Manage users profile get

- `slack-pp-cli users-profile-get` — Retrieves a user's profile information.

**users-profile-set** — Manage users profile set

- `slack-pp-cli users-profile-set` — Set the profile information for a user.

**users-set-active** — Manage users set active

- `slack-pp-cli users-set-active` — Marked a user as active. Deprecated and non-functional.

**users-set-photo** — Manage users set photo

- `slack-pp-cli users-set-photo` — Set the user profile photo

**users-set-presence** — Manage users set presence

- `slack-pp-cli users-set-presence` — Manually sets user presence.

**views-open** — Manage views open

- `slack-pp-cli views-open` — Open a view for a user.

**views-publish** — Manage views publish

- `slack-pp-cli views-publish` — Publish a static view for a User.

**views-push** — Manage views push

- `slack-pp-cli views-push` — Push a view onto the stack of a root view.

**views-update** — Manage views update

- `slack-pp-cli views-update` — Update an existing view.

**workflows-step-completed** — Manage workflows step completed

- `slack-pp-cli workflows-step-completed` — Indicate that an app's step in a workflow completed execution.

**workflows-step-failed** — Manage workflows step failed

- `slack-pp-cli workflows-step-failed` — Indicate that an app's step in a workflow failed to execute.

**workflows-update-step** — Manage workflows update step

- `slack-pp-cli workflows-update-step` — Update the configuration for a workflow extension step.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
slack-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly customer sweep

```bash
slack-pp-cli customer-intel "Petroautos" --window 7d --agent --select messages.channel,messages.ts,messages.text
```

Narrows the cross-channel customer timeline to just the fields an agent needs, keeping the payload small.

### Pre-1:1 engagement scan

```bash
slack-pp-cli dm-engagement --report all --window 14d --agent
```

One volume row per direct report — DM count, Asana tasks, Fathom calls — for fast 1:1 prep.

### Cron-safe alert post

```bash
slack-pp-cli post --channel "#csm-signals" --text "alert body" --dry-run
```

Shows the exact request a cron would send without sending it; drop --dry-run to post.

### Daily attention pulse

```bash
slack-pp-cli reactions summarize --channel "#the-wolf-of-atom" --window 7d --agent
```

Top messages by reaction count plus emoji-class buckets for the channel.

## Auth Setup

Auth is an xoxp Slack user token. Set SLACK_USER_TOKEN, or let the CLI fall back to ~/.claude/slack-user-token.env. A bot token (xoxb) works for post-only flows via SLACK_BOT_TOKEN.

Run `slack-pp-cli doctor` to verify setup. Use `slack-pp-cli auth status` to see which token source resolved, `slack-pp-cli auth set-token` to store a token, and `slack-pp-cli auth logout` to clear it.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  slack-pp-cli admin-apps-approve --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
slack-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
slack-pp-cli feedback --stdin < notes.txt
slack-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.slack-pp-cli/feedback.jsonl`. They are never POSTed unless `SLACK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SLACK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
slack-pp-cli profile save briefing --json
slack-pp-cli --profile briefing admin-apps-approve
slack-pp-cli profile list --json
slack-pp-cli profile show briefing
slack-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `slack-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add slack-pp-mcp -- slack-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which slack-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   slack-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `slack-pp-cli <command> --help`.
