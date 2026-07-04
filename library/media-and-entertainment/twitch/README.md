# Twitch CLI

Twitch Helix API

Created by [@coopdogGGs](https://github.com/coopdogGGs) (ryanc00per).

## Install

The recommended path installs both the `twitch-pp-cli` binary and the `pp-twitch` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install twitch
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install twitch --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install twitch --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install twitch --agent claude-code
npx -y @mvanhorn/printing-press-library install twitch --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/twitch/cmd/twitch-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/twitch-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install twitch --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-twitch --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-twitch --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install twitch --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/twitch-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TWITCH_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/twitch/cmd/twitch-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "twitch": {
      "command": "twitch-pp-mcp",
      "env": {
        "TWITCH_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
twitch-pp-cli games get-top --json

twitch-pp-cli streams get --json

twitch-pp-cli sync --resources games-top,streams --json

twitch-pp-cli doctor

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`sync`** — Mirror Twitch Helix data (top games, live streams, global emotes, content labels) into a local SQLite store for offline search and analysis
- **`search`** — Full-text search across locally synced Twitch records
- **`analytics`** — Group-by and count aggregation over locally synced Twitch data without re-hitting the rate-limited API
- **`workflow`** — Chain multiple Helix operations into one agent-friendly command

## Usage

Run `twitch-pp-cli --help` for the full command reference and flag list.

## Commands

### authorization

Manage authorization

- **`twitch-pp-cli authorization`** - NEW Gets the authorization scopes that the specified user(s) have granted the application.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).

### bits

Manage bits

- **`twitch-pp-cli bits get-cheermotes`** - Gets a list of Cheermotes that users can use to cheer Bits in any Bits-enabled channel's chat room. Cheermotes are animated emotes that viewers can assign Bits to.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli bits get-extension-products`** - Gets the list of Bits products that belongs to the extension. The client ID in the app access token identifies the extension.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens). The client ID in the app access token must be the extension's client ID.
- **`twitch-pp-cli bits get-leaderboard`** - Gets the Bits leaderboard for the authenticated broadcaster.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **bits:read** scope.
- **`twitch-pp-cli bits update-extension-product`** - Adds or updates a Bits product that the extension created. If the SKU doesn't exist, the product is added. You may update all fields except the `sku` field.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens). The client ID in the app access token must match the extension's client ID.

### channel-points

Manage channel points

- **`twitch-pp-cli channel-points create-custom-rewards`** - Creates a Custom Reward in the broadcaster's channel. The maximum number of custom rewards per channel is 50, which includes both enabled and disabled rewards.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:redemptions** scope.
- **`twitch-pp-cli channel-points delete-custom-reward`** - Deletes a custom reward that the broadcaster created.

The app used to create the reward is the only app that may delete it. If the reward's redemption status is UNFULFILLED at the time the reward is deleted, its redemption status is marked as FULFILLED.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:redemptions** scope.
- **`twitch-pp-cli channel-points get-custom-reward`** - Gets a list of custom rewards that the specified broadcaster created.

**NOTE**: A channel may offer a maximum of 50 rewards, which includes both enabled and disabled rewards.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:redemptions** or **channel:manage:redemptions** scope.
- **`twitch-pp-cli channel-points get-custom-reward-redemption`** - Gets a list of redemptions for the specified custom reward. The app used to create the reward is the only app that may get the redemptions.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:redemptions** or **channel:manage:redemptions** scope.
- **`twitch-pp-cli channel-points update-custom-reward`** - Updates a custom reward. The app used to create the reward is the only app that may update the reward.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:redemptions** scope.

__Request Body:__

The body of the request should contain only the fields you're updating.
- **`twitch-pp-cli channel-points update-redemption-status`** - Updates a redemption's status. You may update a redemption only if its status is UNFULFILLED. The app used to create the reward is the only app that may update the redemption.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:redemptions** scope.

### channels

Manage channels

- **`twitch-pp-cli channels add-vip`** - Adds the specified user as a VIP in the broadcaster's channel.

**Rate Limits**: The broadcaster may add a maximum of 10 VIPs within a 10-second window.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:vips** scope.
- **`twitch-pp-cli channels get-ad-schedule`** - This endpoint returns ad schedule related information, including snooze, when the last ad was run, when the next ad is scheduled, and if the channel is currently in pre-roll free time. Note that a new ad cannot be run until 8 minutes after running a previous ad.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:ads** scope. The user ID associated with the token must match the `broadcaster_id` in the query parameter.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **channel:read:ads** scope for the user represented by the `broadcaster_id` query parameter.
- **`twitch-pp-cli channels get-editors`** - Gets the broadcaster's list editors.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:editors** scope.
- **`twitch-pp-cli channels get-followed`** - Gets a list of broadcasters that the specified user follows. You can also use this endpoint to see whether a user follows a specific broadcaster.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:read:follows** scope.
- **`twitch-pp-cli channels get-followers`** - Gets a list of users that follow the specified broadcaster. You can also use this endpoint to see whether a specific user follows the broadcaster.

__Authorization:__

* Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:read:followers** scope.
* The ID in the broadcaster\_id query parameter must match the user ID in the access token or the user ID in the access token must be a moderator for the specified broadcaster.

This endpoint will return specific follower information only if both of the above are true. If a scope is not provided or the user isn't the broadcaster or a moderator for the specified channel, only the total follower count will be included in the response.
- **`twitch-pp-cli channels get-information`** - Gets information about one or more channels.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli channels get-vips`** - Gets a list of the broadcaster's VIPs.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:vips** scope. If your app also adds and removes VIP status, you can use the **channel:manage:vips** scope instead.
- **`twitch-pp-cli channels modify-information`** - Updates a channel's properties.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:broadcast** scope.

__Request Body:__

All fields are optional, but you must specify at least one field.
- **`twitch-pp-cli channels remove-vip`** - Removes the specified user as a VIP in the broadcaster's channel.

If the broadcaster is removing the user's VIP status, the ID in the _broadcaster\_id_ query parameter must match the user ID in the access token; otherwise, if the user is removing their VIP status themselves, the ID in the _user\_id_ query parameter must match the user ID in the access token.

**Rate Limits**: The broadcaster may remove a maximum of 10 VIPs within a 10-second window.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:vips** scope.
- **`twitch-pp-cli channels snooze-next-ad`** - If available, pushes back the timestamp of the upcoming automatic mid-roll ad by 5 minutes. This endpoint duplicates the snooze functionality in the creator dashboard's Ads Manager.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:ads** scope. The user ID associated with the token must match the `broadcaster_id` in the query parameter.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **channel:manage:ads** scope for the user represented by the `broadcaster_id` query parameter.
- **`twitch-pp-cli channels start-commercial`** - Starts a commercial on the specified channel.

**NOTE**: Only partners and affiliates may run commercials and they must be streaming live at the time.

**NOTE**: Only the broadcaster may start a commercial; the broadcaster's editors and moderators may not start commercials on behalf of the broadcaster.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:edit:commercial** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **channel:edit:commercial** scope for the user represented by the `broadcaster_id` query parameter.

### charity

Manage charity

- **`twitch-pp-cli charity get-campaign`** - Gets information about the charity campaign that a broadcaster is running. For example, the campaign's fundraising goal and the current amount of donations.

To receive events when progress is made towards the campaign's goal or the broadcaster changes the fundraising goal, subscribe to the [channel.charity\_campaign.progress](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelcharity%5Fcampaignprogress) subscription type.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:charity** scope.
- **`twitch-pp-cli charity get-campaign-donations`** - Gets the list of donations that users have made to the broadcaster's active charity campaign.

To receive events as donations occur, subscribe to the [channel.charity\_campaign.donate](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelcharity%5Fcampaigndonate) subscription type.

__Authorization:__

Requires a user access token that includes the **channel:read:charity** scope.

### chat

Manage chat

- **`twitch-pp-cli chat get-channel-badges`** - Gets the broadcaster's list of custom chat badges. The list is empty if the broadcaster hasn't created custom chat badges. For information about custom badges, see [subscriber badges](https://help.twitch.tv/s/article/subscriber-badge-guide) and [Bits badges](https://help.twitch.tv/s/article/custom-bit-badges-guide).

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli chat get-channel-emotes`** - Gets the broadcaster's list of custom emotes. Broadcasters create these custom emotes for users who subscribe to or follow the channel or cheer Bits in the channel's chat window. [Learn More](https://dev.twitch.tv/docs/irc/emotes)

For information about the custom emotes, see [subscriber emotes](https://help.twitch.tv/s/article/subscriber-emote-guide), [Bits tier emotes](https://help.twitch.tv/s/article/custom-bit-badges-guide?language=bg#slots), and [follower emotes](https://blog.twitch.tv/en/2021/06/04/kicking-off-10-years-with-our-biggest-emote-update-ever/).

**NOTE:** With the exception of custom follower emotes, users may use custom emotes in any Twitch chat.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli chat get-chatters`** - Gets the list of users that are connected to the broadcaster's chat session.

**NOTE**: There is a delay between when users join and leave a chat and when the list is updated accordingly.

To determine whether a user is a moderator or VIP, use the [Get Moderators](https://dev.twitch.tv/docs/api/reference#get-moderators) and [Get VIPs](https://dev.twitch.tv/docs/api/reference#get-vips) endpoints. You can check the roles of up to 100 users.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:read:chatters** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:read:chatters** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli chat get-emote-sets`** - Gets emotes for one or more specified emote sets.

An emote set groups emotes that have a similar context. For example, Twitch places all the subscriber emotes that a broadcaster uploads for their channel in the same emote set.

[Learn More](https://dev.twitch.tv/docs/irc/emotes)

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli chat get-global-badges`** - Gets Twitch's list of chat badges, which users may use in any channel's chat room. For information about chat badges, see [Twitch Chat Badges Guide](https://help.twitch.tv/s/article/twitch-chat-badges-guide).

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

__Request Query Parameters:__

None
- **`twitch-pp-cli chat get-global-emotes`** - Gets the list of [global emotes](https://www.twitch.tv/creatorcamp/en/learn-the-basics/emotes/). Global emotes are Twitch-created emotes that users can use in any Twitch chat.

[Learn More](https://dev.twitch.tv/docs/irc/emotes)

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

__Request Query Parameters:__

None
- **`twitch-pp-cli chat get-settings`** - Gets the broadcaster's chat settings.

For an overview of chat settings, see [Chat Commands for Broadcasters and Moderators](https://help.twitch.tv/s/article/chat-commands#AllMods) and [Moderator Preferences](https://help.twitch.tv/s/article/setting-up-moderation-for-your-twitch-channel#modpreferences).

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli chat get-user-color`** - Gets the color used for the user's name in chat.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli chat get-user-emotes`** - Retrieves emotes available to the user across all channels.

__Authorization:__

* Requires a user access token that includes the **user:read:emotes** scope.
* Query parameter `user_id` must match the `user_id` in the user access token.
- **`twitch-pp-cli chat send-a-shoutout`** - Sends a Shoutout to the specified broadcaster. Typically, you send Shoutouts when you or one of your moderators notice another broadcaster in your chat, the other broadcaster is coming up in conversation, or after they raid your broadcast.

Twitch's Shoutout feature is a great way for you to show support for other broadcasters and help them grow. Viewers who do not follow the other broadcaster will see a pop-up Follow button in your chat that they can click to follow the other broadcaster. [Learn More](https://help.twitch.tv/s/article/shoutouts)

**Rate Limits**: The broadcaster may send a Shoutout once every 2 minutes. They may send the same broadcaster a Shoutout once every 60 minutes.

To receive notifications when a Shoutout is sent or received, subscribe to the [channel.shoutout.create](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelshoutoutcreate) and [channel.shoutout.receive](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelshoutoutreceive) subscription types. The **channel.shoutout.create** event includes cooldown periods that indicate when the broadcaster may send another Shoutout without exceeding the endpoint's rate limit.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:shoutouts** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through prior authorizations, has:

* The **channel:bot** scope for the user represented by the `broadcaster_id` query parameter, and
* The **moderator:manage:shoutouts** and **user:bot** scopes for the user represented by the `moderator_id` in the query parameter.
- **`twitch-pp-cli chat send-announcement`** - Sends an announcement to the broadcaster's chat room.

**Rate Limits**: One announcement may be sent every 2 seconds.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:announcements** scope.
- **`twitch-pp-cli chat send-message`** - Sends a message to the broadcaster's chat room.

**NOTE:** When sending messages to a Shared Chat session, behaviors differ depending on your authentication token type:

* When using an _App Access Token_, messages will only be sent to the source channel (defined by the `broadcaster_id` parameter) by default starting on May 19, 2025\. Messages can be sent to all channels by using the `for_source_only` parameter and setting it to `false`.
* When using a _User Access Token_, messages will be sent to all channels in the shared chat session, including the source channel. This behavior cannot be changed with this token type.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the `user:write:chat` scope. If app access token used, then additionally requires `user:bot` scope from chatting user, and either `channel:bot` scope from broadcaster or moderator status.
- **`twitch-pp-cli chat update-settings`** - Updates the broadcaster's chat settings.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:chat\_settings** scope. The user ID associated with the token must match the `moderator_id` in the query parameter.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:chat\_settings** scope for the user represented by the `moderator_id` query parameter.

__Request Body:__

All fields are optional. Specify only those fields that you want to update.

To set the `slow_mode_wait_time` or `follower_mode_duration` field to its default value, set the corresponding `slow_mode` or `follower_mode` field to **true** (and don't include the `slow_mode_wait_time` or `follower_mode_duration` field).

To set the `slow_mode_wait_time`, `follower_mode_duration`, or `non_moderator_chat_delay_duration` field's value, you must set the corresponding `slow_mode`, `follower_mode`, or `non_moderator_chat_delay` field to **true**.

To remove the `slow_mode_wait_time`, `follower_mode_duration`, or `non_moderator_chat_delay_duration` field's value, set the corresponding `slow_mode`, `follower_mode`, or `non_moderator_chat_delay` field to **false** (and don't include the `slow_mode_wait_time`, `follower_mode_duration`, or `non_moderator_chat_delay_duration` field).
- **`twitch-pp-cli chat update-user-color`** - Updates the color used for the user's name in chat.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:manage:chat\_color** scope.

### clips

Manage clips

- **`twitch-pp-cli clips create`** - Creates a clip from the broadcaster's stream.

This API captures up to 90 seconds of the broadcaster's stream. The 90 seconds spans the point in the stream from when you called the API. For example, if you call the API at the 4:00 minute mark, the API captures from approximately the 2:35 mark to approximately the 4:05 minute mark. Twitch tries its best to capture 90 seconds of the stream, but the actual length may be less. This may occur if you begin capturing the clip near the beginning or end of the stream.

By default, Twitch publishes up to the last 30 seconds of the 90 seconds window and provides a default title for the clip. To specify the title and the portion of the 90 seconds window that's used for the clip, use the URL in the response's `edit_url` field. You can specify a clip that's from 5 seconds to 60 seconds in length. The URL is valid for up to 24 hours or until the clip is published, whichever comes first.

Creating a clip is an asynchronous process that can take a short amount of time to complete. To determine whether the clip was successfully created, call [Get Clips](https://dev.twitch.tv/docs/api/reference#get-clips) using the clip ID that this request returned. If Get Clips returns the clip, the clip was successfully created. If after 15 seconds Get Clips hasn't returned the clip, assume it failed.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **clips:edit** scope.
- **`twitch-pp-cli clips get`** - Gets one or more video clips that were captured from streams. For information about clips, see [How to use clips](https://help.twitch.tv/s/article/how-to-use-clips).

When using pagination for clips, note that the maximum number of results returned over multiple requests will be approximately 1,000\. If additional results are necessary, paginate over different query parameters such as multiple `started_at` and `ended_at` timeframes to refine the search.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

__Request Query Parameters:__

The _id_, _game\_id_, and _broadcaster\_id_ query parameters are mutually exclusive.
- **`twitch-pp-cli clips get-download`** - NEW Provides URLs to download the video file(s) for the specified clips. For information about clips, see [How to use clips](https://help.twitch.tv/s/article/how-to-use-clips).

**Rate Limits**: Limited to 100 requests per minute.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the `editor:manage:clips` or `channel:manage:clips` scope.

### content-classification-labels

Manage content classification labels

- **`twitch-pp-cli content-classification-labels`** - Gets information about Twitch content classification labels.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

### entitlements

Manage entitlements

- **`twitch-pp-cli entitlements get-drops`** - Gets an organization's list of entitlements that have been granted to a game, a user, or both.

**NOTE:** Entitlements returned in the response body data are not guaranteed to be sorted by any field returned by the API. To retrieve **CLAIMED** or **FULFILLED** entitlements, use the `fulfillment_status` query parameter to filter results. To retrieve entitlements for a specific game, use the `game_id` query parameter to filter results.

The following table identifies the request parameters that you may specify based on the type of access token used.

| Access token type | Parameter | Description |
| - | - | - |
| App | None | If you don't specify request parameters, the request returns all entitlements that your organization owns. |
| App | user_id | The request returns all entitlements for any game that the organization granted to the specified user. |
| App | user_id, game_id | The request returns all entitlements that the specified game granted to the specified user. |
| App | game_id | The request returns all entitlements that the specified game granted to all entitled users. |
| User | None | If you don't specify request parameters, the request returns all entitlements for any game that the organization granted to the user identified in the access token. |
| User | user_id | Invalid. |
| User | user_id, game_id | Invalid. |
| User | game_id | The request returns all entitlements that the specified game granted to the user identified in the access token. |


__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens). The Client ID associated with the access token must be owned by a user who is a member of the [organization](https://dev.twitch.tv/docs/docs/companies/) that holds ownership of the game.
- **`twitch-pp-cli entitlements update-drops`** - Updates the Drop entitlement's fulfillment status.

The following table identifies which entitlements are updated based on the type of access token used.

| Access token type | Data that's updated |
| - | - |
| App | Updates all entitlements with benefits owned by the organization in the access token. |
| User | Updates all entitlements owned by the user in the access token and where the benefits are owned by the organization in the access token. |


__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens). The Client ID associated with the access token must be owned by a user who is a member of the [organization](https://dev.twitch.tv/docs/docs/companies/) that holds ownership of the game.

### eventsub

Manage eventsub

- **`twitch-pp-cli eventsub create-conduits`** - Creates a new [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/).

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).
- **`twitch-pp-cli eventsub create-subscription`** - Creates an EventSub subscription.

__Authorization:__

If you use [webhooks to receive events](https://dev.twitch.tv/docs/eventsub/handling-webhook-events), the request must specify an app access token. The request will fail if you use a user access token. If the subscription type requires user authorization, the user must have granted your app (client ID) permissions to receive those events before you subscribe to them. For example, to subscribe to [channel.subscribe](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#channelsubscribe) events, your app must get a user access token that includes the `channel:read:subscriptions` scope, which adds the required permission to your app access token's client ID.

If you use [WebSockets to receive events](https://dev.twitch.tv/docs/eventsub/handling-websocket-events), the request must specify a user access token. The request will fail if you use an app access token. If the subscription type requires user authorization, the token must include the required scope. However, if the subscription type doesn't include user authorization, the token may include any scopes or no scopes.

If you use [Conduits](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/) to receive events, the request must specify an app access token. The request will fail if you use a user access token.
- **`twitch-pp-cli eventsub delete-conduit`** - Deletes a specified [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/). Note that it may take some time for Eventsub subscriptions on a deleted [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/) to show as disabled when calling [Get Eventsub Subscriptions](https://dev.twitch.tv/docs/api/reference/#get-eventsub-subscriptions).

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).
- **`twitch-pp-cli eventsub delete-subscription`** - Deletes an EventSub subscription.

__Authorization:__

If you use [webhooks to receive events](https://dev.twitch.tv/docs/eventsub/handling-webhook-events), the request must specify an app access token. The request will fail if you use a user access token.

If you use [WebSockets to receive events](https://dev.twitch.tv/docs/eventsub/handling-websocket-events), the request must specify a user access token. The request will fail if you use an app access token. The token may include any scopes.
- **`twitch-pp-cli eventsub get-conduit-shards`** - Gets a lists of all shards for a [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/).

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).
- **`twitch-pp-cli eventsub get-conduits`** - Gets the [conduits](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/) for a client ID.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).
- **`twitch-pp-cli eventsub get-subscriptions`** - Gets a list of EventSub subscriptions that the client in the access token created.

__Authorization:__

If you use [Webhooks](https://dev.twitch.tv/docs/eventsub/handling-webhook-events) or [Conduits](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/) to receive events, the request must specify an app access token. The request will fail if you use a user access token.

If you use [WebSockets to receive events](https://dev.twitch.tv/docs/eventsub/handling-websocket-events), the request must specify a user access token. The request will fail if you use an app access token. The token may include any scopes.

__Request Query Parameters:__

Use the _status_, _type_, _user\_id_, and _subscription\_id_ query parameters to filter the list of subscriptions that are returned. The filters are mutually exclusive; the request fails if you specify more than one filter.
- **`twitch-pp-cli eventsub update-conduit-shards`** - Updates shard(s) for a [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/).

**NOTE:** Shard IDs are indexed starting at 0, so a conduit with a `shard_count` of 5 will have shards with IDs 0 through 4.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).
- **`twitch-pp-cli eventsub update-conduits`** - Updates a [conduit's](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/) shard count. To delete shards, update the count to a lower number, and the shards above the count will be deleted. For example, if the existing shard count is 100, by resetting shard count to 50, shards 50-99 are disabled.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).

### extensions

Manage extensions

- **`twitch-pp-cli extensions create-secret`** - Creates a shared secret used to sign and verify JWT tokens. Creating a new secret removes the current secrets from service. Use this function only when you are ready to use the new secret it returns.

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an Extension Backend Service (EBS). For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role`, `user_id`, and `exp` fields (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)). The `role` field must be set to _external_.
- **`twitch-pp-cli extensions get`** - Gets information about an extension.

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an Extension Backend Service (EBS). For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role` field (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)), and the `role` field must be set to _external_.
- **`twitch-pp-cli extensions get-configuration-segment`** - Gets the specified configuration segment from the specified extension.

**Rate Limits**: You may retrieve each segment a maximum of 20 times per minute.

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an Extension Backend Service (EBS). For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role`, `user_id`, and `exp` fields (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)). The `role` field must be set to _external_.
- **`twitch-pp-cli extensions get-live-channels`** - Gets a list of broadcasters that are streaming live and have installed or activated the extension.

It may take a few minutes for the list to include or remove broadcasters that have recently gone live or stopped broadcasting.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli extensions get-released`** - Gets information about a released extension. Returns the extension if its `state` is Released.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli extensions get-secrets`** - Gets an extension's list of shared secrets.

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an Extension Backend Service (EBS). For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role`, `user_id`, and `exp` fields (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)). The `role` field must be set to _external_.
- **`twitch-pp-cli extensions get-transactions`** - Gets an extension's list of transactions. A transaction records the exchange of a currency (for example, Bits) for a digital product.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens).
- **`twitch-pp-cli extensions send-chat-message`** - Sends a message to the specified broadcaster's chat room. The extension's name is used as the username for the message in the chat room. To send a chat message, your extension must enable **Chat Capabilities** (under your extension's **Capabilities** tab).

**Rate Limits**: You may send a maximum of 12 messages per minute per channel.

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an Extension Backend Service (EBS). For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role` and `user_id` fields (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)). The `role` field must be set to _external_.
- **`twitch-pp-cli extensions send-pubsub-message`** - Sends a message to one or more viewers. You can send messages to a specific channel or to all channels where your extension is active. This endpoint uses the same mechanism as the [send](https://dev.twitch.tv/docs/extensions/reference#send) JavaScript helper function used to send messages.

**Rate Limits**: You may send a maximum of 100 messages per minute per combination of extension client ID and broadcaster ID.

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an Extension Backend Service (EBS). For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role`, `user_id`, and `exp` fields (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)) along with the `channel_id` and `pubsub_perms` fields. The `role` field must be set to _external_.

To send the message to a specific channel, set the `channel_id` field in the JWT to the channel's ID and set the `pubsub_perms.send` array to _broadcast_.

```
{
  "exp": 1503343947,
  "user_id": "27419011",
  "role": "external",
  "channel_id": "27419011",
  "pubsub_perms": {
    "send":[
      "broadcast"
    ]
  }
}

```

To send the message to all channels on which your extension is active, set the `channel_id` field to _all_ and set the `pubsub_perms.send` array to _global_.

```
{
  "exp": 1503343947,
  "user_id": "27419011",
  "role": "external",
  "channel_id": "all",
  "pubsub_perms": {
    "send":[
      "global"
    ]
  }
}

```
- **`twitch-pp-cli extensions set-configuration-segment`** - Updates a configuration segment. The segment is limited to 5 KB. Extensions that are active on a channel do not receive the updated configuration.

**Rate Limits**: You may update the configuration a maximum of 20 times per minute.

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an Extension Backend Service (EBS). For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role`, `user_id`, and `exp` fields (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)). The `role` field must be set to _external_.
- **`twitch-pp-cli extensions set-required-configuration`** - Updates the extension's required\_configuration string. Use this endpoint if your extension requires the broadcaster to configure the extension before activating it (to require configuration, you must select **Custom/My Own Service** in Extension [Capabilities](https://dev.twitch.tv/docs/extensions/life-cycle/#capabilities)). For more information, see [Required Configurations](https://dev.twitch.tv/docs/extensions/building#required-configurations) and [Setting Required Configuration](https://dev.twitch.tv/docs/extensions/building#setting-required-configuration-with-the-configuration-service-optional).

__Authorization:__

Requires a signed JSON Web Token (JWT) created by an EBS. For signing requirements, see [Signing the JWT](https://dev.twitch.tv/docs/extensions/building/#signing-the-jwt). The signed JWT must include the `role`, `user_id`, and `exp` fields (see [JWT Schema](https://dev.twitch.tv/docs/extensions/reference/#jwt-schema)). Set the `role` field to _external_ and the `user_id` field to the ID of the user that owns the extension.

### games

Manage games

- **`twitch-pp-cli games get`** - Gets information about specified categories or games.

You may get up to 100 categories or games by specifying their ID or name. You may specify all IDs, all names, or a combination of IDs and names. If you specify a combination of IDs and names, the total number of IDs and names must not exceed 100.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli games get-top`** - Gets information about all broadcasts on Twitch.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

### goals

Manage goals

- **`twitch-pp-cli goals`** - Gets the broadcaster's list of active goals. Use this endpoint to get the current progress of each goal.

Instead of polling for the progress of a goal, consider [subscribing](https://dev.twitch.tv/docs/eventsub/manage-subscriptions) to receive notifications when a goal makes progress using the [channel.goal.progress](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelgoalprogress) subscription type. [Read More](https://dev.twitch.tv/docs/api/goals#requesting-event-notifications)

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:goals** scope.

### guest-star

Manage guest star

- **`twitch-pp-cli guest-star assign-slot`** - BETA Allows a previously invited user to be assigned a slot within the active Guest Star session, once that guest has indicated they are ready to join.

__Authorization:__

* Query parameter `moderator_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star` or `moderator:manage:guest_star`
- **`twitch-pp-cli guest-star create-session`** - BETA Programmatically creates a Guest Star session on behalf of the broadcaster. Requires the broadcaster to be present in the call interface, or the call will be ended automatically.

__Authorization:__

* Query parameter `broadcaster_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star`
- **`twitch-pp-cli guest-star delete-invite`** - BETA Revokes a previously sent invite for a Guest Star session.

__Authorization:__

* Query parameter `moderator_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star` or `moderator:manage:guest_star`
- **`twitch-pp-cli guest-star delete-slot`** - BETA Allows a caller to remove a slot assignment from a user participating in an active Guest Star session. This revokes their access to the session immediately and disables their access to publish or subscribe to media within the session.

__Authorization:__

* Query parameter `moderator_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star` or `moderator:manage:guest_star`
- **`twitch-pp-cli guest-star end-session`** - BETA Programmatically ends a Guest Star session on behalf of the broadcaster. Performs the same action as if the host clicked the "End Call" button in the Guest Star UI.

__Authorization:__

* Query parameter `broadcaster_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star`
- **`twitch-pp-cli guest-star get-channel-settings`** - BETA Gets the channel settings for configuration of the Guest Star feature for a particular host.

__Authorization:__

* Query parameter `moderator_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:read:guest_star`, `channel:manage:guest_star`, `moderator:read:guest_star` or `moderator:manage:guest_star`
- **`twitch-pp-cli guest-star get-invites`** - BETA Provides the caller with a list of pending invites to a Guest Star session, including the invitee's ready status while joining the waiting room.

__Authorization:__

* Query parameter `broadcaster_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:read:guest_star`, `channel:manage:guest_star`, `moderator:read:guest_star` or `moderator:manage:guest_star`
- **`twitch-pp-cli guest-star get-session`** - BETA Gets information about an ongoing Guest Star session for a particular channel.

__Authorization:__

* Requires OAuth Scope: `channel:read:guest_star`, `channel:manage:guest_star`, `moderator:read:guest_star` or `moderator:manage:guest_star`
* Guests must be either invited or assigned a slot within the session
- **`twitch-pp-cli guest-star send-invite`** - BETA Sends an invite to a specified guest on behalf of the broadcaster for a Guest Star session in progress.

__Authorization:__

* Query parameter `moderator_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star` or `moderator:manage:guest_star`
- **`twitch-pp-cli guest-star update-channel-settings`** - BETA Mutates the channel settings for configuration of the Guest Star feature for a particular host.

__Authorization:__

* Query parameter `broadcaster_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star`
- **`twitch-pp-cli guest-star update-slot`** - BETA Allows a user to update the assigned slot for a particular user within the active Guest Star session.

__Authorization:__

* Query parameter `moderator_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star` or `moderator:manage:guest_star`
- **`twitch-pp-cli guest-star update-slot-settings`** - BETA Allows a user to update slot settings for a particular guest within a Guest Star session, such as allowing the user to share audio or video within the call as a host. These settings will be broadcasted to all subscribers which control their view of the guest in that slot. One or more of the optional parameters to this API can be specified at any time.

__Authorization:__

* Query parameter `moderator_id` must match the `user_id` in the [User-Access token](https://dev.twitch.tv/docs/authentication#user-access-tokens)
* Requires OAuth Scope: `channel:manage:guest_star` or `moderator:manage:guest_star`

### hypetrain

Manage hypetrain

- **`twitch-pp-cli hypetrain`** - NEW Get the status of a Hype Train for the specified broadcaster.

__Authorization:__

* Requires an [user access token](https://dev.twitch.tv/docs/authentication/#user-access-tokens).
* Requires OAuth Scope: `channel:read:hype_train`.
* Requires that `broadcaster_id` and `user_id` match in the User-Access token.

### moderation

Manage moderation

- **`twitch-pp-cli moderation add-blocked-term`** - Adds a word or phrase to the broadcaster's list of blocked terms. These are the terms that the broadcaster doesn't want used in their chat room.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:blocked\_terms** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:blocked\_terms** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation add-channel-moderator`** - Adds a moderator to the broadcaster's chat room.

**Rate Limits**: The broadcaster may add a maximum of 10 moderators within a 10-second window.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:moderators** scope.
- **`twitch-pp-cli moderation add-suspicious-status-to-chat-user`** - NEW Adds a suspicious user status to a chatter on the broadcaster's channel.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:suspicious\_users** scope.
- **`twitch-pp-cli moderation ban-user`** - Bans a user from participating in the specified broadcaster's chat room or puts them in a timeout.

For information about banning or putting users in a timeout, see [Ban a User](https://help.twitch.tv/s/article/how-to-manage-harassment-in-chat#TheBanFeature) and [Timeout a User](https://help.twitch.tv/s/article/how-to-manage-harassment-in-chat#TheTimeoutFeature).

If the user is currently in a timeout, you can call this endpoint to change the duration of the timeout or ban them altogether. If the user is currently banned, you cannot call this method to put them in a timeout instead.

To remove a ban or end a timeout, see [Unban user](https://dev.twitch.tv/docs/api/reference#unban-user).

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:banned\_users** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:banned\_users** and **user:bot** scopes for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation check-automod-status`** - Checks whether AutoMod would flag the specified message for review.

AutoMod is a moderation tool that holds inappropriate or harassing chat messages for moderators to review. Moderators approve or deny the messages that AutoMod flags; only approved messages are released to chat. AutoMod detects misspellings and evasive language automatically. For information about AutoMod, see [How to Use AutoMod](https://help.twitch.tv/s/article/how-to-use-automod).

**Rate Limits**: Rates are limited per channel based on the account type rather than per access token.

| Account type | Limit per minute | Limit per hour |
| - | - | - |
| Normal | 5 | 50 |
| Affiliate | 10 | 100 |
| Partner | 30 | 300 |


The above limits are in addition to the standard [Twitch API rate limits](https://dev.twitch.tv/docs/api/guide#twitch-rate-limits). The rate limit headers in the response represent the Twitch rate limits and not the above limits.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderation:read** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderation:read** scope for the user represented by the `broadcaster_id` query parameter.
- **`twitch-pp-cli moderation delete-chat-messages`** - Removes a single chat message or all chat messages from the broadcaster's chat room.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:chat\_messages** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:chat\_messages** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation get-automod-settings`** - Gets the broadcaster's AutoMod settings. The settings are used to automatically block inappropriate or harassing messages from appearing in the broadcaster's chat room.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:read:automod\_settings** or **moderator:manage:automod\_settings** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:read:automod\_settings** or **moderator:manage:automod\_settings** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation get-banned-users`** - Gets all users that the broadcaster banned or put in a timeout.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderation:read** or **moderator:manage:banned\_users** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderation:read** or **moderator:manage:banned\_users** scope for the user represented by the `broadcaster_id` query parameter.
- **`twitch-pp-cli moderation get-blocked-terms`** - Gets the broadcaster's list of non-private, blocked words or phrases. These are the terms that the broadcaster or moderator added manually or that were denied by AutoMod.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:read:blocked\_terms** or **moderator:manage:blocked\_terms** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:read:blocked\_terms** or **moderator:manage:blocked\_terms** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation get-moderated-channels`** - Gets a list of channels that the specified user has moderator privileges in.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:read:moderated\_channels**. The user ID associated with the token must match the `user_id` in the query parameter.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **user:read:moderated\_channels** scope for the user represented by the `user_id` query parameter.
- **`twitch-pp-cli moderation get-moderators`** - Gets all users allowed to moderate the broadcaster's chat room.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderation:read** scope. If your app also adds and removes moderators, you can use the **channel:manage:moderators** scope instead.
- **`twitch-pp-cli moderation get-shield-mode-status`** - Gets the broadcaster's Shield Mode activation status.

To receive notification when the broadcaster activates and deactivates Shield Mode, subscribe to the [channel.shield\_mode.begin](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelshield%5Fmodebegin) and [channel.shield\_mode.end](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelshield%5Fmodeend) subscription types.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:read:shield\_mode** or **moderator:manage:shield\_mode** scope. The user ID associated with the token must match the `moderator_id` in the query parameter.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:read:shield\_mode** or **moderator:manage:shield\_mode** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation get-unban-requests`** - Gets a list of unban requests for a broadcaster's channel.

__Authorization:__

* Requires a user access token that includes the **moderator:read:unban\_requests** or **moderator:manage:unban\_requests** scope.
* Query parameter `moderator_id` must match the `user_id` in the [user access token](https://dev.twitch.tv/docs/authentication/#user-access-tokens).
- **`twitch-pp-cli moderation manage-held-automod-messages`** - Allow or deny the message that AutoMod flagged for review. For information about AutoMod, see [How to Use AutoMod](https://help.twitch.tv/s/article/how-to-use-automod).

To get messages that AutoMod is holding for review, subscribe to the **automod-queue.<moderator\_id>.<channel\_id>** [topic](https://dev.twitch.tv/docs/pubsub#topics) using [PubSub](https://dev.twitch.tv/docs/pubsub). PubSub sends a notification to your app when AutoMod holds a message for review.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:automod** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:automod** scope for the user represented by the `user_id` query parameter.
- **`twitch-pp-cli moderation remove-blocked-term`** - Removes the word or phrase from the broadcaster's list of blocked terms.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:blocked\_terms** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:blocked\_terms** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation remove-channel-moderator`** - Removes a moderator from the broadcaster's chat room.

**Rate Limits**: The broadcaster may remove a maximum of 10 moderators within a 10-second window.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:moderators** scope.
- **`twitch-pp-cli moderation remove-suspicious-status-from-chat-user`** - NEW Remove a suspicious user status from a chatter on broadcaster's channel.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:suspicious\_users** scope.
- **`twitch-pp-cli moderation resolve-unban-requests`** - Resolves an unban request by approving or denying it.

__Authorization:__

* Requires a user access token that includes the **moderator:manage:unban\_requests** scope.
* Query parameter `moderator_id` must match the `user_id` in the[user access token](https://dev.twitch.tv/docs/authentication/#user-access-tokens).
- **`twitch-pp-cli moderation unban-user`** - Removes the ban or timeout that was placed on the specified user.

To ban a user, see [Ban user](https://dev.twitch.tv/docs/api/reference#ban-user).

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:banned\_users** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:banned\_users** and **user:bot** scopes for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation update-automod-settings`** - Updates the broadcaster's AutoMod settings. The settings are used to automatically block inappropriate or harassing messages from appearing in the broadcaster's chat room.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:automod\_settings** scope.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:automod\_settings** scope for the user represented by the `moderator_id` query parameter.

__Request Body:__

Because PUT is an overwrite operation, you must include all the fields that you want set after the operation completes. Typically, you'll send a GET request, update the fields you want to change, and pass that object in the PUT request.

You may set either `overall_level` or the individual settings like `aggression`, but not both.

Setting `overall_level` applies default values to the individual settings. However, setting `overall_level` to 4 does not necessarily mean that it applies 4 to all the individual settings. Instead, it applies a set of recommended defaults to the rest of the settings. For example, if you set `overall_level` to 2, Twitch provides some filtering on discrimination and sexual content, but more filtering on hostility (see the first example response).

If `overall_level` is currently set and you update `swearing` to 3, `overall_level` will be set to **null** and all settings other than `swearing` will be set to 0\. The same is true if individual settings are set and you update `overall_level` to 3 - all the individual settings are updated to reflect the default level.

Note that if you set all the individual settings to values that match what `overall_level` would have set them to, Twitch changes AutoMod to use the default AutoMod level instead of using the individual settings.

Valid values for all levels are from 0 (no filtering) through 4 (most aggressive filtering). These levels affect how aggressively AutoMod holds back messages for moderators to review before they appear in chat or are denied (not shown).
- **`twitch-pp-cli moderation update-shield-mode-status`** - Activates or deactivates the broadcaster's Shield Mode.

Twitch's Shield Mode feature is like a panic button that broadcasters can push to protect themselves from chat abuse coming from one or more accounts. When activated, Shield Mode applies the overrides that the broadcaster configured in the Twitch UX. If the broadcaster hasn't configured Shield Mode, it applies default overrides.

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:shield\_mode** scope. The user ID associated with the token must match the `moderator_id` in the query parameter.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:shield\_mode** scope for the user represented by the `moderator_id` query parameter.
- **`twitch-pp-cli moderation warn-chat-user`** - Warns a user in the specified broadcaster's chat room, preventing them from chat interaction until the warning is acknowledged. New warnings can be issued to a user when they already have a warning in the channel (new warning will replace old warning).

__Authorization:__

Requires one of the following:

* A [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **moderator:manage:warnings** scope. The user ID associated with the token must match the `moderator_id` in the query parameter.
* BETA An [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) where the application, through a prior authorization, has the **moderator:manage:warnings** scope for the user represented by the `moderator_id` query parameter.

### polls

Manage polls

- **`twitch-pp-cli polls create`** - Creates a poll that viewers in the broadcaster's channel can vote on.

The poll begins as soon as it's created. You may run only one poll at a time.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:polls** scope.
- **`twitch-pp-cli polls end`** - Ends an active poll. You have the option to end it or end it and archive it.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:polls** scope.
- **`twitch-pp-cli polls get`** - Gets a list of polls that the broadcaster created.

Polls are available for 90 days after they're created.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:polls** or **channel:manage:polls** scope.

### predictions

Manage predictions

- **`twitch-pp-cli predictions create`** - Creates a Channel Points Prediction.

With a Channel Points Prediction, the broadcaster poses a question and viewers try to predict the outcome. The prediction runs as soon as it's created. The broadcaster may run only one prediction at a time.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:predictions** scope.
- **`twitch-pp-cli predictions end`** - Locks, resolves, or cancels a Channel Points Prediction.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:predictions** scope.
- **`twitch-pp-cli predictions get`** - Gets a list of Channel Points Predictions that the broadcaster created.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:predictions** or **channel:manage:predictions** scope.

### raids

Manage raids

- **`twitch-pp-cli raids cancel-a`** - Cancel a pending raid.

You can cancel a raid at any point up until the broadcaster clicks **Raid Now** in the Twitch UX or the 90-second countdown expires.

**Rate Limit**: The limit is 10 requests within a 10-minute window.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:raids** scope.
- **`twitch-pp-cli raids start-a`** - Raid another channel by sending the broadcaster's viewers to the targeted channel.

When you call the API from a chat bot or extension, the Twitch UX pops up a window at the top of the chat room that identifies the number of viewers in the raid. The raid occurs when the broadcaster clicks **Raid Now** or after the 90-second countdown expires.

To determine whether the raid successfully occurred, you must subscribe to the [Channel Raid](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types#channelraid) event. For more information, see [Get notified when a raid begins](https://dev.twitch.tv/docs/api/raids#get-notified-when-a-raid-begins).

To cancel a pending raid, use the [Cancel a raid](https://dev.twitch.tv/docs/api/reference#cancel-a-raid) endpoint.

**Rate Limit**: The limit is 10 requests within a 10-minute window.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:raids** scope.

### schedule

Manage schedule

- **`twitch-pp-cli schedule create-channel-stream-segment`** - Adds a single or recurring broadcast to the broadcaster's streaming schedule. For information about scheduling broadcasts, see [Stream Schedule](https://help.twitch.tv/s/article/channel-page-setup#Schedule).

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:schedule** scope.
- **`twitch-pp-cli schedule delete-channel-stream-segment`** - Removes a broadcast segment from the broadcaster's streaming schedule.

**NOTE**: For recurring segments, removing a segment removes all segments in the recurring schedule.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:schedule** scope.
- **`twitch-pp-cli schedule get-channel-icalendar`** - Gets the broadcaster's streaming schedule as an [iCalendar](https://datatracker.ietf.org/doc/html/rfc5545).

__Authorization:__

The Client-Id and Authorization headers are not required.

__Response Body:__

The response body contains the iCalendar data (see [RFC5545](https://datatracker.ietf.org/doc/html/rfc5545)).

The Content-Type response header is set to `text/calendar`.
- **`twitch-pp-cli schedule get-channel-stream`** - Gets the broadcaster's streaming schedule. You can get the entire schedule or specific segments of the schedule. [Learn More](https://help.twitch.tv/s/article/channel-page-setup#Schedule)

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli schedule update-channel-stream`** - Updates the broadcaster's schedule settings, such as scheduling a vacation.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:schedule** scope.
- **`twitch-pp-cli schedule update-channel-stream-segment`** - Updates a scheduled broadcast segment.

For recurring segments, updating a segment's title, category, duration, and timezone, changes all segments in the recurring schedule, not just the specified segment.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:schedule** scope.

### shared-chat

Manage shared chat

- **`twitch-pp-cli shared-chat`** - NEW Retrieves the active shared chat session for a channel.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/cli/token-command/#app-access-token) or [user access token](https://dev.twitch.tv/docs/authentication/#user-access-tokens).

### streams

Manage streams

- **`twitch-pp-cli streams create-marker`** - Adds a marker to a live stream. A marker is an arbitrary point in a live stream that the broadcaster or editor wants to mark, so they can return to that spot later to create video highlights (see Video Producer, Highlights in the Twitch UX).

You may not add markers:

* If the stream is not live
* If the stream has not enabled video on demand (VOD)
* If the stream is a premiere (a live, first-viewing event that combines uploaded videos with live chat)
* If the stream is a rerun of a past broadcast, including past premieres.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:broadcast** scope.
- **`twitch-pp-cli streams get`** - Gets a list of all streams. The list is in descending order by the number of viewers watching the stream. Because viewers come and go during a stream, it's possible to find duplicate or missing streams in the list as you page through the results.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli streams get-followed`** - Gets the list of broadcasters that the user follows and that are streaming live.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:read:follows** scope.
- **`twitch-pp-cli streams get-key`** - Gets the channel's stream key.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:stream\_key** scope.
- **`twitch-pp-cli streams get-markers`** - Gets a list of markers from the user's most recent stream or from the specified VOD/video. A marker is an arbitrary point in a live stream that the broadcaster or editor marked, so they can return to that spot later to create video highlights (see Video Producer, Highlights in the Twitch UX).

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:read:broadcast** or **channel:manage:broadcast** scope.
- **`twitch-pp-cli streams get-tags`** - **IMPORTANT** Twitch is moving from Twitch-defined tags to channel-defined tags. **IMPORTANT** As of February 28, 2023, this endpoint returns an empty array. On July 13, 2023, it will return a 410 response. If you use this endpoint, please update your code to use [Get Channel Information](https://dev.twitch.tv/docs/api/reference#get-channel-information).

Gets the list of stream tags that the broadcaster or Twitch added to their channel.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

### subscriptions

Manage subscriptions

- **`twitch-pp-cli subscriptions check-user`** - Checks whether the user subscribes to the broadcaster's channel.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:read:subscriptions** scope.

A Twitch extensions may use an app access token if the broadcaster has granted the **user:read:subscriptions** scope from within the Twitch Extensions manager.
- **`twitch-pp-cli subscriptions get-broadcaster`** - Gets a list of users that subscribe to the specified broadcaster.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:read:subscriptions** scope.

A Twitch extensions may use an app access token if the broadcaster has granted the **channel:read:subscriptions** scope from within the Twitch Extensions manager.

### tags

Manage tags

- **`twitch-pp-cli tags`** - **IMPORTANT** Twitch is moving from Twitch-defined tags to channel-defined tags. **IMPORTANT** As of February 28, 2023, this endpoint returns an empty array. On July 13, 2023, it will return a 410 response.

Gets a list of all stream tags that Twitch defines. The broadcaster may apply any of these to their channel except automatic tags. For an online list of the possible tags, see [List of All Tags](https://www.twitch.tv/directory/all/tags).

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

### teams

Manage teams

- **`twitch-pp-cli teams get`** - Gets information about the specified Twitch team. [Read More](https://help.twitch.tv/s/article/twitch-teams)

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli teams get-channel`** - Gets the list of Twitch teams that the broadcaster is a member of.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

### twitch-helix-analytics

Manage twitch helix analytics

- **`twitch-pp-cli twitch-helix-analytics get-extension`** - Gets an analytics report for one or more extensions. The response contains the URLs used to download the reports (CSV files). [Learn More](https://dev.twitch.tv/docs/insights)

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **analytics:read:extensions** scope.
- **`twitch-pp-cli twitch-helix-analytics get-game`** - Gets an analytics report for one or more games. The response contains the URLs used to download the reports (CSV files). [Learn more](https://dev.twitch.tv/docs/insights)

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **analytics:read:games** scope.

### twitch-helix-search

Manage twitch helix search

- **`twitch-pp-cli twitch-helix-search categories`** - Gets the games or categories that match the specified query.

To match, the category's name must contain all parts of the query string. For example, if the query string is 42, the response includes any category name that contains 42 in the title. If the query string is a phrase like _love computer_, the response includes any category name that contains the words love and computer anywhere in the name. The comparison is case insensitive.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli twitch-helix-search channels`** - Gets the channels that match the specified query and have streamed content within the past 6 months.

The fields that the API uses for comparison depends on the value that the _live\_only_ query parameter is set to. If _live\_only_ is **false**, the API matches on the broadcaster's login name. However, if _live\_only_ is **true**, the API matches on the broadcaster's name and category name.

To match, the beginning of the broadcaster's name or category must match the query string. The comparison is case insensitive. If the query string is angel\_of\_death, it matches all names that begin with angel\_of\_death. However, if the query string is a phrase like _angel of death_, it matches to names starting with angelofdeath or names starting with angel\_of\_death.

By default, the results include both live and offline channels. To get only live channels set the _live\_only_ query parameter to **true**.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

### users

Manage users

- **`twitch-pp-cli users block`** - Blocks the specified user from interacting with or having contact with the broadcaster. The user ID in the OAuth token identifies the broadcaster who is blocking the user.

To learn more about blocking users, see [Block Other Users on Twitch](https://help.twitch.tv/s/article/how-to-manage-harassment-in-chat?language=en%5FUS#BlockWhispersandMessagesfromStrangers).

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:manage:blocked\_users** scope.
- **`twitch-pp-cli users get`** - Gets information about one or more users.

You may look up users using their user ID, login name, or both but the sum total of the number of users you may look up is 100\. For example, you may specify 50 IDs and 50 names or 100 IDs or names, but you cannot specify 100 IDs and 100 names.

If you don't specify IDs or login names, the request returns information about the user in the access token if you specify a user access token.

To include the user's verified email address in the response, you must use a user access token that includes the **user:read:email** scope.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli users get-active-extensions`** - Gets the active extensions that the broadcaster has installed for each configuration.

NOTE: To include extensions that you have under development, you must specify a user access token that includes the **user:read:broadcast** or **user:edit:broadcast** scope.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).
- **`twitch-pp-cli users get-block-list`** - Gets the list of users that the broadcaster has blocked. [Read More](https://help.twitch.tv/s/article/how-to-manage-harassment-in-chat?language=en%5FUS#BlockWhispersandMessagesfromStrangers)

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:read:blocked\_users** scope.
- **`twitch-pp-cli users get-extensions`** - Gets a list of all extensions (both active and inactive) that the broadcaster has installed. The user ID in the access token identifies the broadcaster.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:read:broadcast** or **user:edit:broadcast** scope. To include inactive extensions, you must include the **user:edit:broadcast** scope.
- **`twitch-pp-cli users unblock`** - Removes the user from the broadcaster's list of blocked users. The user ID in the OAuth token identifies the broadcaster who's removing the block.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:manage:blocked\_users** scope.
- **`twitch-pp-cli users update`** - Updates the specified user's information. The user ID in the OAuth token identifies the user whose information you want to update.

To include the user's verified email address in the response, the user access token must also include the **user:read:email** scope.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:edit** scope.
- **`twitch-pp-cli users update-extensions`** - Updates an installed extension's information. You can update the extension's activation state, ID, and version number. The user ID in the access token identifies the broadcaster whose extensions you're updating.

NOTE: If you try to activate an extension under multiple extension types, the last write wins (and there is no guarantee of write order).

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:edit:broadcast** scope.

### videos

Manage videos

- **`twitch-pp-cli videos create-clip-from-vod`** - NEW Creates a clip from a broadcaster's VOD on behalf of the broadcaster or an editor of the channel. Since a live stream is actively creating a VOD, this endpoint can also be used to create a clip from earlier in the current stream.

The duration of a clip can be from 5 seconds to 60 seconds in length, with a default of 30 seconds if not specified.

`vod_offset` indicates where the clip will end. In other words, the clip will start at (`vod_offset` \- `duration`) and end at `vod_offset`. This means that the value of `vod_offset` must greater than or equal to the value of `duration`.

The URL in the response's `edit_url` field allows you to edit the clip's title, feature the clip, create a portrait version of the clip, download the clip media, and share the clip directly to social platforms.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **editor:manage:clips** or **channel:manage:clips** scope.
- **`twitch-pp-cli videos delete`** - Deletes one or more videos. You may delete past broadcasts, highlights, or uploads.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **channel:manage:videos** scope.
- **`twitch-pp-cli videos get`** - Gets information about one or more published videos. You may get videos by ID, by user, or by game/category.

You may apply several filters to get a subset of the videos. The filters are applied as an AND operation to each video. For example, if _language_ is set to 'de' and _game\_id_ is set to 21779, the response includes only videos that show playing League of Legends by users that stream in German. The filters apply only if you get videos by user ID or game ID.

__Authorization:__

Requires an [app access token](https://dev.twitch.tv/docs/authentication#app-access-tokens) or [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens).

### whispers

Manage whispers

- **`twitch-pp-cli whispers`** - Sends a whisper message to the specified user.

NOTE: The user sending the whisper must have a verified phone number (see the **Phone Number** setting in your [Security and Privacy](https://www.twitch.tv/settings/security) settings).

NOTE: The API may silently drop whispers that it suspects of violating Twitch policies. (The API does not indicate that it dropped the whisper; it returns a 204 status code as if it succeeded.)

**Rate Limits**: You may whisper to a maximum of 40 unique recipients per day. Within the per day limit, you may whisper a maximum of 3 whispers per second and a maximum of 100 whispers per minute.

__Authorization:__

Requires a [user access token](https://dev.twitch.tv/docs/authentication#user-access-tokens) that includes the **user:manage:whispers** scope.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
twitch-pp-cli clips get

# JSON for scripting and agents
twitch-pp-cli clips get --json

# Filter to specific fields
twitch-pp-cli clips get --json --select id,name,status

# Dry run — show the request without sending
twitch-pp-cli clips get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
twitch-pp-cli clips get --agent
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
twitch-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/twitch-helix-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TWITCH_CLIENT_ID` | auth_flow_input | Yes |  |
| `TWITCH_CLIENT_SECRET` | auth_flow_input | Yes | Set during initial auth setup. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `twitch-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `twitch-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TWITCH_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
