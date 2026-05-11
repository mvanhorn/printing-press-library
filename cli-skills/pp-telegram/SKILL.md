---
name: pp-telegram
description: "Every Telegram Bot API method, plus idempotent sends, replace-last status messages, and a local store that knows... Trigger phrases: `send telegram message`, `post to telegram channel`, `alert me on telegram`, `telegram bot status update`, `edit my last telegram message`, `use telegram`, `run telegram`."
author: "Yung Bing"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - telegram-pp-cli
    install:
      - kind: go
        bins: [telegram-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/cmd/telegram-pp-cli
---

# Telegram Bot — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `telegram-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install telegram --cli-only
   ```
2. Verify: `telegram-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/cmd/telegram-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

telegram-pp-cli wraps the full Bot API (sendMessage, sendPhoto, edit/delete/forward, webhooks, chat admin, stickers) with the agent-native floor every Printing Press CLI gets — typed exit codes, --json, --dry-run, doctor — and adds a local SQLite store that powers send dedup, status-message threading, and audit roll-ups no other Telegram CLI offers. Designed for scripts and agents that need to send text and HTML messages to channels and DMs without re-implementing retry semantics every time.

## When to Use This CLI

Reach for telegram-pp-cli when a script, cron job, or agent runtime needs to send Telegram messages reliably \u2014 especially when retries might fire twice (idempotency-key), when a status message should update in place (--replace-last), or when an operator needs to audit what was sent (messages list, audit). It is the right choice when you want a single Go binary that exposes the full Bot API plus a local store, instead of a Python interpreter dependency.

## When NOT to Use This CLI

This is a Telegram Bot API client. Don't reach for it to drive a personal user account (MTProto / Telethon territory \u2014 bot tokens authenticate bots, not users), to scrape public channels you don't own, or to act as an end-user chat client. If the workflow needs reading arbitrary public channels or impersonating a human, use a Telethon-based tool instead.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`send`** — Skip the API call when a previous send with the same idempotency key already succeeded for this bot and chat — returns the original message_id instead of re-sending.

  _Reach for this whenever a cron job or agent retry might fire the same alert twice — it deduplicates without round-tripping the API._

  ```bash
  telegram-pp-cli send --chat 1234567 --idempotency-key deploy-2026-05-11 --text 'deploy ok' --json
  ```
- **`send`** — Edit the most recent message this bot sent to a chat in place — ideal for long-running job status that updates as steps complete.

  _Use when you would otherwise spam a chat with N status pings — one logical message that mutates over time._

  ```bash
  telegram-pp-cli send --chat 1234567 --replace-last --text 'Step 3/5 complete' --html
  ```
- **`messages list`** — Browse the bot's outbound and inbound history filtered by chat, time window, sender, or media type — entirely from the local store, no API call.

  _Reach for this when an operator asks 'what did the bot say yesterday' — the answer is offline and audit-grade._

  ```bash
  telegram-pp-cli messages list --chat 1234567 --since 1d --mine --json
  ```
- **`audit`** — Roll-up of outbound activity — messages by chat, by media type, error rate, last-send timestamp — one shot, no API call.

  _Use as the weekly 'did the bot actually deliver' check or as the first step when diagnosing a delivery complaint._

  ```bash
  telegram-pp-cli audit --since today --json
  ```

### Reachability mitigation
- **`send`** — Validate or auto-escape HTML before sending — catches Telegram's 'can't parse entities' 400 before it leaves the box.

  _Reach for --html-escape when posting untrusted text, and `format html-lint` when debugging a 400 from a draft post._

  ```bash
  telegram-pp-cli send --chat 1234567 --html --html-escape --text "$RAW_USER_INPUT"
  ```

### Agent-native plumbing
- **`publish`** — Send a long-form post as one logical artifact — auto-split into chunks, recorded under a caller-chosen slug — then edit it later as a single unit.

  _Use for any multi-part broadcast that may be revised — release notes, status pages, weekly digests._

  ```bash
  telegram-pp-cli publish send --channel @mychan --body release-notes.md --record-as v1.2 --html
  ```
- **`chats resolve`** — Resolve an @username to a stable numeric chat_id with local caching — transparently falls back to the API when the cache misses.

  _Use before send when you have a handle but need a numeric id (e.g., webhook payloads, persisted job state)._

  ```bash
  telegram-pp-cli chats resolve @mychan --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**add-sticker-to-set** — Manage add sticker to set

- `telegram-pp-cli add-sticker-to-set` — Use this method to add a new sticker to a set created by the bot. You **must** use exactly one of the fields...

**answer-callback-query** — Manage answer callback query

- `telegram-pp-cli answer-callback-query` — Use this method to send answers to callback queries sent from [inline...

**answer-inline-query** — Manage answer inline query

- `telegram-pp-cli answer-inline-query` — Use this method to send answers to an inline query. On success, *True* is returned. No more than **50** results per...

**answer-pre-checkout-query** — Manage answer pre checkout query

- `telegram-pp-cli answer-pre-checkout-query` — Once the user has confirmed their payment and shipping details, the Bot API sends the final confirmation in the form...

**answer-shipping-query** — Manage answer shipping query

- `telegram-pp-cli answer-shipping-query` — If you sent an invoice requesting a shipping address and the parameter *is_flexible* was specified, the Bot API will...

**close** — Manage close

- `telegram-pp-cli close` — Use this method to close the bot instance before moving it from one local server to another. You need to delete the...

**copy-message** — Manage copy message

- `telegram-pp-cli copy-message` — Use this method to copy messages of any kind. The method is analogous to the method...

**create-new-sticker-set** — Manage create new sticker set

- `telegram-pp-cli create-new-sticker-set` — Use this method to create a new sticker set owned by a user. The bot will be able to edit the sticker set thus...

**delete-chat-photo** — Manage delete chat photo

- `telegram-pp-cli delete-chat-photo` — Use this method to delete a chat photo. Photos can't be changed for private chats. The bot must be an administrator...

**delete-chat-sticker-set** — Manage delete chat sticker set

- `telegram-pp-cli delete-chat-sticker-set` — Use this method to delete a group sticker set from a supergroup. The bot must be an administrator in the chat for...

**delete-message** — Manage delete message

- `telegram-pp-cli delete-message` — Use this method to delete a message, including service messages, with the following limitations: - A message can...

**delete-sticker-from-set** — Manage delete sticker from set

- `telegram-pp-cli delete-sticker-from-set` — Use this method to delete a sticker from a set created by the bot. Returns *True* on success.

**delete-webhook** — Manage delete webhook

- `telegram-pp-cli delete-webhook` — Use this method to remove webhook integration if you decide to switch back to...

**edit-message-caption** — Manage edit message caption

- `telegram-pp-cli edit-message-caption` — Use this method to edit captions of messages. On success, if the edited message is not an inline message, the edited...

**edit-message-live-location** — Manage edit message live location

- `telegram-pp-cli edit-message-live-location` — Use this method to edit live location messages. A location can be edited until its *live_period* expires or editing...

**edit-message-media** — Manage edit message media

- `telegram-pp-cli edit-message-media` — Use this method to edit animation, audio, document, photo, or video messages. If a message is part of a message...

**edit-message-reply-markup** — Manage edit message reply markup

- `telegram-pp-cli edit-message-reply-markup` — Use this method to edit only the reply markup of messages. On success, if the edited message is not an inline...

**edit-message-text** — Manage edit message text

- `telegram-pp-cli edit-message-text` — Use this method to edit text and [game](https://core.telegram.org/bots/api/#games) messages. On success, if the...

**export-chat-invite-link** — Manage export chat invite link

- `telegram-pp-cli export-chat-invite-link` — Use this method to generate a new invite link for a chat; any previously generated link is revoked. The bot must be...

**forward-message** — Manage forward message

- `telegram-pp-cli forward-message` — Use this method to forward messages of any kind. On success, the sent...

**get-chat** — Manage get chat

- `telegram-pp-cli get-chat` — Use this method to get up to date information about the chat (current name of the user for one-on-one conversations,...

**get-chat-administrators** — Manage get chat administrators

- `telegram-pp-cli get-chat-administrators` — Use this method to get a list of administrators in a chat. On success, returns an Array of...

**get-chat-member** — Manage get chat member

- `telegram-pp-cli get-chat-member` — Use this method to get information about a member of a chat. Returns a...

**get-chat-members-count** — Manage get chat members count

- `telegram-pp-cli get-chat-members-count` — Use this method to get the number of members in a chat. Returns *Int* on success.

**get-file** — Manage get file

- `telegram-pp-cli get-file` — Use this method to get basic info about a file and prepare it for downloading. For the moment, bots can download...

**get-game-high-scores** — Manage get game high scores

- `telegram-pp-cli get-game-high-scores` — Use this method to get data for high score tables. Will return the score of the specified user and several of their...

**get-me** — Manage get me

- `telegram-pp-cli get-me` — A simple method for testing your bot's auth token. Requires no parameters. Returns basic information about the bot...

**get-my-commands** — Manage get my commands

- `telegram-pp-cli get-my-commands` — Use this method to get the current list of the bot's commands. Requires no parameters. Returns Array of...

**get-sticker-set** — Manage get sticker set

- `telegram-pp-cli get-sticker-set` — Use this method to get a sticker set. On success, a [StickerSet](https://core.telegram.org/bots/api/#stickerset)...

**get-updates** — Manage get updates

- `telegram-pp-cli get-updates` — Use this method to receive incoming updates using long polling...

**get-user-profile-photos** — Manage get user profile photos

- `telegram-pp-cli get-user-profile-photos` — Use this method to get a list of profile pictures for a user. Returns a...

**get-webhook-info** — Manage get webhook info

- `telegram-pp-cli get-webhook-info` — Use this method to get current webhook status. Requires no parameters. On success, returns a...

**kick-chat-member** — Manage kick chat member

- `telegram-pp-cli kick-chat-member` — Use this method to kick a user from a group, a supergroup or a channel. In the case of supergroups and channels, the...

**leave-chat** — Manage leave chat

- `telegram-pp-cli leave-chat` — Use this method for your bot to leave a group, supergroup or channel. Returns *True* on success.

**log-out** — Manage log out

- `telegram-pp-cli log-out` — Use this method to log out from the cloud Bot API server before launching the bot locally. You **must** log out the...

**pin-chat-message** — Manage pin chat message

- `telegram-pp-cli pin-chat-message` — Use this method to add a message to the list of pinned messages in a chat. If the chat is not a private chat, the...

**promote-chat-member** — Manage promote chat member

- `telegram-pp-cli promote-chat-member` — Use this method to promote or demote a user in a supergroup or a channel. The bot must be an administrator in the...

**restrict-chat-member** — Manage restrict chat member

- `telegram-pp-cli restrict-chat-member` — Use this method to restrict a user in a supergroup. The bot must be an administrator in the supergroup for this to...

**send-animation** — Manage send animation

- `telegram-pp-cli send-animation` — Use this method to send animation files (GIF or H.264/MPEG-4 AVC video without sound). On success, the sent...

**send-audio** — Manage send audio

- `telegram-pp-cli send-audio` — Use this method to send audio files, if you want Telegram clients to display them in the music player. Your audio...

**send-chat-action** — Manage send chat action

- `telegram-pp-cli send-chat-action` — Use this method when you need to tell the user that something is happening on the bot's side. The status is set for...

**send-contact** — Manage send contact

- `telegram-pp-cli send-contact` — Use this method to send phone contacts. On success, the sent [Message](https://core.telegram.org/bots/api/#message)...

**send-dice** — Manage send dice

- `telegram-pp-cli send-dice` — Use this method to send an animated emoji that will display a random value. On success, the sent...

**send-document** — Manage send document

- `telegram-pp-cli send-document` — Use this method to send general files. On success, the sent [Message](https://core.telegram.org/bots/api/#message)...

**send-game** — Manage send game

- `telegram-pp-cli send-game` — Use this method to send a game. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

**send-invoice** — Manage send invoice

- `telegram-pp-cli send-invoice` — Use this method to send invoices. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is...

**send-location** — Manage send location

- `telegram-pp-cli send-location` — Use this method to send point on the map. On success, the sent...

**send-media-group** — Manage send media group

- `telegram-pp-cli send-media-group` — Use this method to send a group of photos, videos, documents or audios as an album. Documents and audio files can be...

**send-message** — Manage send message

- `telegram-pp-cli send-message` — Use this method to send text messages. On success, the sent [Message](https://core.telegram.org/bots/api/#message)...

**send-photo** — Manage send photo

- `telegram-pp-cli send-photo` — Use this method to send photos. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

**send-poll** — Manage send poll

- `telegram-pp-cli send-poll` — Use this method to send a native poll. On success, the sent [Message](https://core.telegram.org/bots/api/#message)...

**send-sticker** — Manage send sticker

- `telegram-pp-cli send-sticker` — Use this method to send static .WEBP or [animated](https://telegram.org/blog/animated-stickers) .TGS stickers. On...

**send-venue** — Manage send venue

- `telegram-pp-cli send-venue` — Use this method to send information about a venue. On success, the sent...

**send-video** — Manage send video

- `telegram-pp-cli send-video` — Use this method to send video files, Telegram clients support mp4 videos (other formats may be sent as...

**send-video-note** — Manage send video note

- `telegram-pp-cli send-video-note` — As of [v.4.0](https://telegram.org/blog/video-messages-and-telescope), Telegram clients support rounded square mp4...

**send-voice** — Manage send voice

- `telegram-pp-cli send-voice` — Use this method to send audio files, if you want Telegram clients to display the file as a playable voice message....

**set-chat-administrator-custom-title** — Manage set chat administrator custom title

- `telegram-pp-cli set-chat-administrator-custom-title` — Use this method to set a custom title for an administrator in a supergroup promoted by the bot. Returns *True* on...

**set-chat-description** — Manage set chat description

- `telegram-pp-cli set-chat-description` — Use this method to change the description of a group, a supergroup or a channel. The bot must be an administrator in...

**set-chat-permissions** — Manage set chat permissions

- `telegram-pp-cli set-chat-permissions` — Use this method to set default chat permissions for all members. The bot must be an administrator in the group or a...

**set-chat-photo** — Manage set chat photo

- `telegram-pp-cli set-chat-photo` — Use this method to set a new profile photo for the chat. Photos can't be changed for private chats. The bot must be...

**set-chat-sticker-set** — Manage set chat sticker set

- `telegram-pp-cli set-chat-sticker-set` — Use this method to set a new group sticker set for a supergroup. The bot must be an administrator in the chat for...

**set-chat-title** — Manage set chat title

- `telegram-pp-cli set-chat-title` — Use this method to change the title of a chat. Titles can't be changed for private chats. The bot must be an...

**set-game-score** — Manage set game score

- `telegram-pp-cli set-game-score` — Use this method to set the score of the specified user in a game. On success, if the message was sent by the bot,...

**set-my-commands** — Manage set my commands

- `telegram-pp-cli set-my-commands` — Use this method to change the list of the bot's commands. Returns *True* on success.

**set-passport-data-errors** — Manage set passport data errors

- `telegram-pp-cli set-passport-data-errors` — Informs a user that some of the Telegram Passport elements they provided contains errors. The user will not be able...

**set-sticker-position-in-set** — Manage set sticker position in set

- `telegram-pp-cli set-sticker-position-in-set` — Use this method to move a sticker in a set created by the bot to a specific position. Returns *True* on success.

**set-sticker-set-thumb** — Manage set sticker set thumb

- `telegram-pp-cli set-sticker-set-thumb` — Use this method to set the thumbnail of a sticker set. Animated thumbnails can be set for animated sticker sets...

**set-webhook** — Manage set webhook

- `telegram-pp-cli set-webhook` — Use this method to specify a url and receive incoming updates via an outgoing webhook. Whenever there is an update...

**stop-message-live-location** — Manage stop message live location

- `telegram-pp-cli stop-message-live-location` — Use this method to stop updating a live location message before *live_period* expires. On success, if the message...

**stop-poll** — Manage stop poll

- `telegram-pp-cli stop-poll` — Use this method to stop a poll which was sent by the bot. On success, the stopped...

**unban-chat-member** — Manage unban chat member

- `telegram-pp-cli unban-chat-member` — Use this method to unban a previously kicked user in a supergroup or channel. The user will **not** return to the...

**unpin-all-chat-messages** — Manage unpin all chat messages

- `telegram-pp-cli unpin-all-chat-messages` — Use this method to clear the list of pinned messages in a chat. If the chat is not a private chat, the bot must be...

**unpin-chat-message** — Manage unpin chat message

- `telegram-pp-cli unpin-chat-message` — Use this method to remove a message from the list of pinned messages in a chat. If the chat is not a private chat,...

**upload-sticker-file** — Manage upload sticker file

- `telegram-pp-cli upload-sticker-file` — Use this method to upload a .PNG file with a sticker for later use in *createNewStickerSet* and *addStickerToSet*...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
telegram-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Cron-safe alert

```bash
telegram-pp-cli send --chat 1234567 --idempotency-key "daily-summary-$(date +%F)" --text 'Backup complete.' --json
```

Bind the idempotency key to a date so a re-run of the same cron invocation is silently deduplicated.

### Long-running job status

```bash
telegram-pp-cli send --chat 1234567 --replace-last --text 'Step 3/5 done' --html
```

Edit the previous status in place; first call falls through to send-new because there is no prior message.

### Audit what the bot did today

```bash
telegram-pp-cli audit --since today --json --select totals,by_chat,errors
```

One-shot aggregation across the local store \u2014 useful for daily ops checks and incident reviews.

### Quiet alerting at night

```bash
telegram-pp-cli send --chat 1234567 --text 'Heartbeat OK' --silent --disable-web-page-preview
```

Send without push notification or link preview \u2014 ideal for periodic health pings that shouldn't wake anyone.

### Resolve a handle before send

```bash
telegram-pp-cli chats resolve @mychan --json --select chat_id,type,title
```

Persist the resolved numeric chat_id in your script's state so later sends don't need the @handle. Cached locally.

## Auth Setup

Set TELEGRAM_BOT_TOKEN in your environment. The token is appended directly into every API URL (`https://api.telegram.org/bot<TOKEN>/<method>`); no OAuth, no refresh. `doctor` confirms the token by calling getMe and prints the bot identity so you can verify you're posting from the right bot before going live.

Run `telegram-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  telegram-pp-cli add-sticker-to-set --emojis example-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
telegram-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
telegram-pp-cli feedback --stdin < notes.txt
telegram-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.telegram-pp-cli/feedback.jsonl`. They are never POSTed unless `TELEGRAM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TELEGRAM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
telegram-pp-cli profile save briefing --json
telegram-pp-cli --profile briefing add-sticker-to-set --emojis example-value
telegram-pp-cli profile list --json
telegram-pp-cli profile show briefing
telegram-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `telegram-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/cmd/telegram-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add telegram-pp-mcp -- telegram-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which telegram-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   telegram-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `telegram-pp-cli <command> --help`.
