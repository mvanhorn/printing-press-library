---
name: pp-twitch
description: "Printing Press CLI for Twitch. Twitch Helix API"
author: "ryanc00per"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - twitch-pp-cli
    install:
      - kind: go
        bins: [twitch-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/twitch/cmd/twitch-pp-cli
---

# Twitch — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `twitch-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install twitch --cli-only
   ```
2. Verify: `twitch-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/twitch/cmd/twitch-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Twitch Helix API

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`sync`** — Mirror Twitch Helix data (top games, live streams, global emotes, content labels) into a local SQLite store for offline search and analysis
- **`search`** — Full-text search across locally synced Twitch records
- **`analytics`** — Group-by and count aggregation over locally synced Twitch data without re-hitting the rate-limited API
- **`workflow`** — Chain multiple Helix operations into one agent-friendly command

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Recipes

### Aggregate live streams by game

```bash
twitch-pp-cli analytics --type streams --group-by game_name --limit 10 --json
```

### List Twitch content classification labels

```bash
twitch-pp-cli content-classification-labels get --json
```

### Search synced Twitch data

```bash
twitch-pp-cli search fortnite --json
```

## Command Reference

**authorization** — Manage authorization

- `twitch-pp-cli authorization` — NEW Gets the authorization scopes that the specified user(s) have granted the application.

**bits** — Manage bits

- `twitch-pp-cli bits get-cheermotes` — Gets a list of Cheermotes that users can use to cheer Bits in any Bits-enabled channel's chat room.
- `twitch-pp-cli bits get-extension-products` — Gets the list of Bits products that belongs to the extension.
- `twitch-pp-cli bits get-leaderboard` — Gets the Bits leaderboard for the authenticated broadcaster.
- `twitch-pp-cli bits update-extension-product` — Adds or updates a Bits product that the extension created. If the SKU doesn't exist, the product is added.

**channel-points** — Manage channel points

- `twitch-pp-cli channel-points create-custom-rewards` — Creates a Custom Reward in the broadcaster's channel.
- `twitch-pp-cli channel-points delete-custom-reward` — Deletes a custom reward that the broadcaster created.
- `twitch-pp-cli channel-points get-custom-reward` — Gets a list of custom rewards that the specified broadcaster created.
- `twitch-pp-cli channel-points get-custom-reward-redemption` — Gets a list of redemptions for the specified custom reward.
- `twitch-pp-cli channel-points update-custom-reward` — Updates a custom reward. The app used to create the reward is the only app that may update the reward.
- `twitch-pp-cli channel-points update-redemption-status` — Updates a redemption's status. You may update a redemption only if its status is UNFULFILLED.

**channels** — Manage channels

- `twitch-pp-cli channels add-vip` — Adds the specified user as a VIP in the broadcaster's channel.
- `twitch-pp-cli channels get-ad-schedule` — This endpoint returns ad schedule related information, including snooze, when the last ad was run
- `twitch-pp-cli channels get-editors` — Gets the broadcaster's list editors. __Authorization:__ Requires a [user access token](https://dev.twitch.
- `twitch-pp-cli channels get-followed` — Gets a list of broadcasters that the specified user follows.
- `twitch-pp-cli channels get-followers` — Gets a list of users that follow the specified broadcaster.
- `twitch-pp-cli channels get-information` — Gets information about one or more channels. __Authorization:__ Requires an [app access token](https://dev.twitch.
- `twitch-pp-cli channels get-vips` — Gets a list of the broadcaster's VIPs. __Authorization:__ Requires a [user access token](https://dev.twitch.
- `twitch-pp-cli channels modify-information` — Updates a channel's properties. __Authorization:__ Requires a [user access token](https://dev.twitch.
- `twitch-pp-cli channels remove-vip` — Removes the specified user as a VIP in the broadcaster's channel.
- `twitch-pp-cli channels snooze-next-ad` — If available, pushes back the timestamp of the upcoming automatic mid-roll ad by 5 minutes.
- `twitch-pp-cli channels start-commercial` — Starts a commercial on the specified channel.

**charity** — Manage charity

- `twitch-pp-cli charity get-campaign` — Gets information about the charity campaign that a broadcaster is running.
- `twitch-pp-cli charity get-campaign-donations` — Gets the list of donations that users have made to the broadcaster's active charity campaign.

**chat** — Manage chat

- `twitch-pp-cli chat get-channel-badges` — Gets the broadcaster's list of custom chat badges.
- `twitch-pp-cli chat get-channel-emotes` — Gets the broadcaster's list of custom emotes.
- `twitch-pp-cli chat get-chatters` — Gets the list of users that are connected to the broadcaster's chat session.
- `twitch-pp-cli chat get-emote-sets` — Gets emotes for one or more specified emote sets. An emote set groups emotes that have a similar context.
- `twitch-pp-cli chat get-global-badges` — Gets Twitch's list of chat badges, which users may use in any channel's chat room.
- `twitch-pp-cli chat get-global-emotes` — Gets the list of [global emotes](https://www.twitch.tv/creatorcamp/en/learn-the-basics/emotes/).
- `twitch-pp-cli chat get-settings` — Gets the broadcaster's chat settings.
- `twitch-pp-cli chat get-user-color` — Gets the color used for the user's name in chat. __Authorization:__ Requires an [app access token](https://dev.twitch.
- `twitch-pp-cli chat get-user-emotes` — Retrieves emotes available to the user across all channels.
- `twitch-pp-cli chat send-a-shoutout` — Sends a Shoutout to the specified broadcaster.
- `twitch-pp-cli chat send-announcement` — Sends an announcement to the broadcaster's chat room. **Rate Limits**: One announcement may be sent every 2 seconds.
- `twitch-pp-cli chat send-message` — Sends a message to the broadcaster's chat room.
- `twitch-pp-cli chat update-settings` — Updates the broadcaster's chat settings.
- `twitch-pp-cli chat update-user-color` — Updates the color used for the user's name in chat. __Authorization:__ Requires a [user access token](https://dev.

**clips** — Manage clips

- `twitch-pp-cli clips create` — Creates a clip from the broadcaster's stream. This API captures up to 90 seconds of the broadcaster's stream.
- `twitch-pp-cli clips get` — Gets one or more video clips that were captured from streams.
- `twitch-pp-cli clips get-download` — NEW Provides URLs to download the video file(s) for the specified clips.

**content-classification-labels** — Manage content classification labels

- `twitch-pp-cli content-classification-labels` — Gets information about Twitch content classification labels.

**entitlements** — Manage entitlements

- `twitch-pp-cli entitlements get-drops` — Gets an organization's list of entitlements that have been granted to a game, a user, or both.
- `twitch-pp-cli entitlements update-drops` — Updates the Drop entitlement's fulfillment status.

**eventsub** — Manage eventsub

- `twitch-pp-cli eventsub create-conduits` — Creates a new [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/).
- `twitch-pp-cli eventsub create-subscription` — Creates an EventSub subscription. __Authorization:__ If you use [webhooks to receive events](https://dev.twitch.
- `twitch-pp-cli eventsub delete-conduit` — Deletes a specified [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/).
- `twitch-pp-cli eventsub delete-subscription` — Deletes an EventSub subscription. __Authorization:__ If you use [webhooks to receive events](https://dev.twitch.
- `twitch-pp-cli eventsub get-conduit-shards` — Gets a lists of all shards for a [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/).
- `twitch-pp-cli eventsub get-conduits` — Gets the [conduits](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/) for a client ID.
- `twitch-pp-cli eventsub get-subscriptions` — Gets a list of EventSub subscriptions that the client in the access token created.
- `twitch-pp-cli eventsub update-conduit-shards` — Updates shard(s) for a [conduit](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/).
- `twitch-pp-cli eventsub update-conduits` — Updates a [conduit's](https://dev.twitch.tv/docs/eventsub/handling-conduit-events/) shard count.

**extensions** — Manage extensions

- `twitch-pp-cli extensions create-secret` — Creates a shared secret used to sign and verify JWT tokens.
- `twitch-pp-cli extensions get` — Gets information about an extension.
- `twitch-pp-cli extensions get-configuration-segment` — Gets the specified configuration segment from the specified extension.
- `twitch-pp-cli extensions get-live-channels` — Gets a list of broadcasters that are streaming live and have installed or activated the extension.
- `twitch-pp-cli extensions get-released` — Gets information about a released extension. Returns the extension if its `state` is Released.
- `twitch-pp-cli extensions get-secrets` — Gets an extension's list of shared secrets.
- `twitch-pp-cli extensions get-transactions` — Gets an extension's list of transactions.
- `twitch-pp-cli extensions send-chat-message` — Sends a message to the specified broadcaster's chat room.
- `twitch-pp-cli extensions send-pubsub-message` — Sends a message to one or more viewers.
- `twitch-pp-cli extensions set-configuration-segment` — Updates a configuration segment. The segment is limited to 5 KB.
- `twitch-pp-cli extensions set-required-configuration` — Updates the extension's required_configuration string.

**games** — Manage games

- `twitch-pp-cli games get` — Gets information about specified categories or games.
- `twitch-pp-cli games get-top` — Gets information about all broadcasts on Twitch. __Authorization:__ Requires an [app access token](https://dev.twitch.

**goals** — Manage goals

- `twitch-pp-cli goals` — Gets the broadcaster's list of active goals. Use this endpoint to get the current progress of each goal.

**guest-star** — Manage guest star

- `twitch-pp-cli guest-star assign-slot` — BETA Allows a previously invited user to be assigned a slot within the active Guest Star session
- `twitch-pp-cli guest-star create-session` — BETA Programmatically creates a Guest Star session on behalf of the broadcaster.
- `twitch-pp-cli guest-star delete-invite` — BETA Revokes a previously sent invite for a Guest Star session.
- `twitch-pp-cli guest-star delete-slot` — BETA Allows a caller to remove a slot assignment from a user participating in an active Guest Star session.
- `twitch-pp-cli guest-star end-session` — BETA Programmatically ends a Guest Star session on behalf of the broadcaster.
- `twitch-pp-cli guest-star get-channel-settings` — BETA Gets the channel settings for configuration of the Guest Star feature for a particular host.
- `twitch-pp-cli guest-star get-invites` — BETA Provides the caller with a list of pending invites to a Guest Star session
- `twitch-pp-cli guest-star get-session` — BETA Gets information about an ongoing Guest Star session for a particular channel.
- `twitch-pp-cli guest-star send-invite` — BETA Sends an invite to a specified guest on behalf of the broadcaster for a Guest Star session in progress.
- `twitch-pp-cli guest-star update-channel-settings` — BETA Mutates the channel settings for configuration of the Guest Star feature for a particular host.
- `twitch-pp-cli guest-star update-slot` — BETA Allows a user to update the assigned slot for a particular user within the active Guest Star session.
- `twitch-pp-cli guest-star update-slot-settings` — BETA Allows a user to update slot settings for a particular guest within a Guest Star session

**hypetrain** — Manage hypetrain

- `twitch-pp-cli hypetrain` — NEW Get the status of a Hype Train for the specified broadcaster.

**moderation** — Manage moderation

- `twitch-pp-cli moderation add-blocked-term` — Adds a word or phrase to the broadcaster's list of blocked terms.
- `twitch-pp-cli moderation add-channel-moderator` — Adds a moderator to the broadcaster's chat room.
- `twitch-pp-cli moderation add-suspicious-status-to-chat-user` — NEW Adds a suspicious user status to a chatter on the broadcaster's channel.
- `twitch-pp-cli moderation ban-user` — Bans a user from participating in the specified broadcaster's chat room or puts them in a timeout.
- `twitch-pp-cli moderation check-automod-status` — Checks whether AutoMod would flag the specified message for review.
- `twitch-pp-cli moderation delete-chat-messages` — Removes a single chat message or all chat messages from the broadcaster's chat room.
- `twitch-pp-cli moderation get-automod-settings` — Gets the broadcaster's AutoMod settings.
- `twitch-pp-cli moderation get-banned-users` — Gets all users that the broadcaster banned or put in a timeout.
- `twitch-pp-cli moderation get-blocked-terms` — Gets the broadcaster's list of non-private, blocked words or phrases.
- `twitch-pp-cli moderation get-moderated-channels` — Gets a list of channels that the specified user has moderator privileges in.
- `twitch-pp-cli moderation get-moderators` — Gets all users allowed to moderate the broadcaster's chat room.
- `twitch-pp-cli moderation get-shield-mode-status` — Gets the broadcaster's Shield Mode activation status.
- `twitch-pp-cli moderation get-unban-requests` — Gets a list of unban requests for a broadcaster's channel.
- `twitch-pp-cli moderation manage-held-automod-messages` — Allow or deny the message that AutoMod flagged for review.
- `twitch-pp-cli moderation remove-blocked-term` — Removes the word or phrase from the broadcaster's list of blocked terms.
- `twitch-pp-cli moderation remove-channel-moderator` — Removes a moderator from the broadcaster's chat room.
- `twitch-pp-cli moderation remove-suspicious-status-from-chat-user` — NEW Remove a suspicious user status from a chatter on broadcaster's channel.
- `twitch-pp-cli moderation resolve-unban-requests` — Resolves an unban request by approving or denying it.
- `twitch-pp-cli moderation unban-user` — Removes the ban or timeout that was placed on the specified user. To ban a user, see [Ban user](https://dev.twitch.
- `twitch-pp-cli moderation update-automod-settings` — Updates the broadcaster's AutoMod settings.
- `twitch-pp-cli moderation update-shield-mode-status` — Activates or deactivates the broadcaster's Shield Mode.
- `twitch-pp-cli moderation warn-chat-user` — Warns a user in the specified broadcaster's chat room

**polls** — Manage polls

- `twitch-pp-cli polls create` — Creates a poll that viewers in the broadcaster's channel can vote on. The poll begins as soon as it's created.
- `twitch-pp-cli polls end` — Ends an active poll. You have the option to end it or end it and archive it.
- `twitch-pp-cli polls get` — Gets a list of polls that the broadcaster created. Polls are available for 90 days after they're created.

**predictions** — Manage predictions

- `twitch-pp-cli predictions create` — Creates a Channel Points Prediction.
- `twitch-pp-cli predictions end` — Locks, resolves, or cancels a Channel Points Prediction. __Authorization:__ Requires a [user access token](https://dev.
- `twitch-pp-cli predictions get` — Gets a list of Channel Points Predictions that the broadcaster created.

**raids** — Manage raids

- `twitch-pp-cli raids cancel-a` — Cancel a pending raid.
- `twitch-pp-cli raids start-a` — Raid another channel by sending the broadcaster's viewers to the targeted channel.

**schedule** — Manage schedule

- `twitch-pp-cli schedule create-channel-stream-segment` — Adds a single or recurring broadcast to the broadcaster's streaming schedule.
- `twitch-pp-cli schedule delete-channel-stream-segment` — Removes a broadcast segment from the broadcaster's streaming schedule.
- `twitch-pp-cli schedule get-channel-icalendar` — Gets the broadcaster's streaming schedule as an [iCalendar](https://datatracker.ietf.org/doc/html/rfc5545).
- `twitch-pp-cli schedule get-channel-stream` — Gets the broadcaster's streaming schedule. You can get the entire schedule or specific segments of the schedule.
- `twitch-pp-cli schedule update-channel-stream` — Updates the broadcaster's schedule settings, such as scheduling a vacation.
- `twitch-pp-cli schedule update-channel-stream-segment` — Updates a scheduled broadcast segment.

**shared-chat** — Manage shared chat

- `twitch-pp-cli shared-chat` — NEW Retrieves the active shared chat session for a channel.

**streams** — Manage streams

- `twitch-pp-cli streams create-marker` — Adds a marker to a live stream.
- `twitch-pp-cli streams get` — Gets a list of all streams. The list is in descending order by the number of viewers watching the stream.
- `twitch-pp-cli streams get-followed` — Gets the list of broadcasters that the user follows and that are streaming live.
- `twitch-pp-cli streams get-key` — Gets the channel's stream key. __Authorization:__ Requires a [user access token](https://dev.twitch.
- `twitch-pp-cli streams get-markers` — Gets a list of markers from the user's most recent stream or from the specified VOD/video.
- `twitch-pp-cli streams get-tags` — **IMPORTANT** Twitch is moving from Twitch-defined tags to channel-defined tags.

**subscriptions** — Manage subscriptions

- `twitch-pp-cli subscriptions check-user` — Checks whether the user subscribes to the broadcaster's channel.
- `twitch-pp-cli subscriptions get-broadcaster` — Gets a list of users that subscribe to the specified broadcaster.

**tags** — Manage tags

- `twitch-pp-cli tags` — **IMPORTANT** Twitch is moving from Twitch-defined tags to channel-defined tags.

**teams** — Manage teams

- `twitch-pp-cli teams get` — Gets information about the specified Twitch team. [Read More](https://help.twitch.
- `twitch-pp-cli teams get-channel` — Gets the list of Twitch teams that the broadcaster is a member of.

**twitch-helix-analytics** — Manage twitch helix analytics

- `twitch-pp-cli twitch-helix-analytics get-extension` — Gets an analytics report for one or more extensions.
- `twitch-pp-cli twitch-helix-analytics get-game` — Gets an analytics report for one or more games. The response contains the URLs used to download the reports (CSV files).

**twitch-helix-search** — Manage twitch helix search

- `twitch-pp-cli twitch-helix-search categories` — Gets the games or categories that match the specified query.
- `twitch-pp-cli twitch-helix-search channels` — Gets the channels that match the specified query and have streamed content within the past 6 months.

**users** — Manage users

- `twitch-pp-cli users block` — Blocks the specified user from interacting with or having contact with the broadcaster.
- `twitch-pp-cli users get` — Gets information about one or more users.
- `twitch-pp-cli users get-active-extensions` — Gets the active extensions that the broadcaster has installed for each configuration.
- `twitch-pp-cli users get-block-list` — Gets the list of users that the broadcaster has blocked. [Read More](https://help.twitch.
- `twitch-pp-cli users get-extensions` — Gets a list of all extensions (both active and inactive) that the broadcaster has installed.
- `twitch-pp-cli users unblock` — Removes the user from the broadcaster's list of blocked users.
- `twitch-pp-cli users update` — Updates the specified user's information.
- `twitch-pp-cli users update-extensions` — Updates an installed extension's information. You can update the extension's activation state, ID, and version number.

**videos** — Manage videos

- `twitch-pp-cli videos create-clip-from-vod` — NEW Creates a clip from a broadcaster's VOD on behalf of the broadcaster or an editor of the channel.
- `twitch-pp-cli videos delete` — Deletes one or more videos. You may delete past broadcasts, highlights, or uploads.
- `twitch-pp-cli videos get` — Gets information about one or more published videos. You may get videos by ID, by user, or by game/category.

**whispers** — Manage whispers

- `twitch-pp-cli whispers` — Sends a whisper message to the specified user.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
twitch-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `twitch-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
twitch-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `TWITCH_CLIENT_ID` as an environment variable.

Run `twitch-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  twitch-pp-cli clips get --agent --select id,name,status
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
twitch-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
twitch-pp-cli feedback --stdin < notes.txt
twitch-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/twitch-pp-cli/feedback.jsonl`. They are never POSTed unless `TWITCH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TWITCH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
twitch-pp-cli profile save briefing --json
twitch-pp-cli --profile briefing clips get
twitch-pp-cli profile list --json
twitch-pp-cli profile show briefing
twitch-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `twitch-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/twitch/cmd/twitch-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add twitch-pp-mcp -- twitch-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which twitch-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   twitch-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `twitch-pp-cli <command> --help`.
