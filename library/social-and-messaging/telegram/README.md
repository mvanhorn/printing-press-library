# Telegram Bot CLI

**Every Telegram Bot API method, plus idempotent sends, replace-last status messages, and a local store that knows what your bot said yesterday.**

telegram-pp-cli wraps the full Bot API (sendMessage, sendPhoto, edit/delete/forward, webhooks, chat admin, stickers) with the agent-native floor every Printing Press CLI gets — typed exit codes, --json, --dry-run, doctor — and adds a local SQLite store that powers send dedup, status-message threading, and audit roll-ups no other Telegram CLI offers. Designed for scripts and agents that need to send text and HTML messages to channels and DMs without re-implementing retry semantics every time.

Learn more at [Telegram Bot](https://core.telegram.org/bots/api).

## Install

The recommended path installs both the `telegram-pp-cli` binary and the `pp-telegram` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install telegram
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install telegram --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/cmd/telegram-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/telegram-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-telegram --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-telegram --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-telegram skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-telegram. The skill defines how its required CLI can be installed.
```

## Authentication

Set TELEGRAM_BOT_TOKEN in your environment. The token is appended directly into every API URL (`https://api.telegram.org/bot<TOKEN>/<method>`); no OAuth, no refresh. `doctor` confirms the token by calling getMe and prints the bot identity so you can verify you're posting from the right bot before going live.

## Quick Start

```bash
# Verify TELEGRAM_BOT_TOKEN is valid and print the bot identity. Run this once after setting the env var.
telegram-pp-cli doctor


# Send the smallest possible message to a channel. Exit 0 + JSON message_id confirms delivery.
telegram-pp-cli send --chat @mychan --text 'hello world' --html


# Send a DM that is safe to retry from cron — same key, same outcome, no double-send.
telegram-pp-cli send --chat 1234567 --idempotency-key release-v1.2 --text 'release shipped' --json


# Update the previous status message in place instead of sending a new one.
telegram-pp-cli send --chat 1234567 --replace-last --text 'Step 3/5: build complete' --html


# Sync the inbox, then list everything this bot has said or received with this chat in the last day.
telegram-pp-cli messages list --chat 1234567 --since 1d --mine --json

```

## Unique Features

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

## Usage

Run `telegram-pp-cli --help` for the full command reference and flag list.

## Commands

### add-sticker-to-set

Manage add sticker to set

- **`telegram-pp-cli add-sticker-to-set create`** - Use this method to add a new sticker to a set created by the bot. You **must** use exactly one of the fields *png\_sticker* or *tgs\_sticker*. Animated stickers can be added to animated sticker sets and only to them. Animated sticker sets can have up to 50 stickers. Static sticker sets can have up to 120 stickers. Returns *True* on success.

### answer-callback-query

Manage answer callback query

- **`telegram-pp-cli answer-callback-query create`** - Use this method to send answers to callback queries sent from [inline keyboards](/bots#inline-keyboards-and-on-the-fly-updating). The answer will be displayed to the user as a notification at the top of the chat screen or as an alert. On success, *True* is returned.

Alternatively, the user can be redirected to the specified Game URL. For this option to work, you must first create a game for your bot via [@Botfather](https://t.me/botfather) and accept the terms. Otherwise, you may use links like `t.me/your_bot?start=XXXX` that open your bot with a parameter.

### answer-inline-query

Manage answer inline query

- **`telegram-pp-cli answer-inline-query create`** - Use this method to send answers to an inline query. On success, *True* is returned.  
No more than **50** results per query are allowed.

### answer-pre-checkout-query

Manage answer pre checkout query

- **`telegram-pp-cli answer-pre-checkout-query create`** - Once the user has confirmed their payment and shipping details, the Bot API sends the final confirmation in the form of an [Update](https://core.telegram.org/bots/api/#update) with the field *pre\_checkout\_query*. Use this method to respond to such pre-checkout queries. On success, True is returned. **Note:** The Bot API must receive an answer within 10 seconds after the pre-checkout query was sent.

### answer-shipping-query

Manage answer shipping query

- **`telegram-pp-cli answer-shipping-query create`** - If you sent an invoice requesting a shipping address and the parameter *is\_flexible* was specified, the Bot API will send an [Update](https://core.telegram.org/bots/api/#update) with a *shipping\_query* field to the bot. Use this method to reply to shipping queries. On success, True is returned.

### close

Manage close

- **`telegram-pp-cli close create`** - Use this method to close the bot instance before moving it from one local server to another. You need to delete the webhook before calling this method to ensure that the bot isn't launched again after server restart. The method will return error 429 in the first 10 minutes after the bot is launched. Returns *True* on success. Requires no parameters.

### copy-message

Manage copy message

- **`telegram-pp-cli copy-message create`** - Use this method to copy messages of any kind. The method is analogous to the method [forwardMessages](https://core.telegram.org/bots/api/#forwardmessages), but the copied message doesn't have a link to the original message. Returns the [MessageId](https://core.telegram.org/bots/api/#messageid) of the sent message on success.

### create-new-sticker-set

Manage create new sticker set

- **`telegram-pp-cli create-new-sticker-set create`** - Use this method to create a new sticker set owned by a user. The bot will be able to edit the sticker set thus created. You **must** use exactly one of the fields *png\_sticker* or *tgs\_sticker*. Returns *True* on success.

### delete-chat-photo

Manage delete chat photo

- **`telegram-pp-cli delete-chat-photo create`** - Use this method to delete a chat photo. Photos can't be changed for private chats. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Returns *True* on success.

### delete-chat-sticker-set

Manage delete chat sticker set

- **`telegram-pp-cli delete-chat-sticker-set create`** - Use this method to delete a group sticker set from a supergroup. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Use the field *can\_set\_sticker\_set* optionally returned in [getChat](https://core.telegram.org/bots/api/#getchat) requests to check if the bot can use this method. Returns *True* on success.

### delete-message

Manage delete message

- **`telegram-pp-cli delete-message create`** - Use this method to delete a message, including service messages, with the following limitations:  
\- A message can only be deleted if it was sent less than 48 hours ago.  
\- A dice message in a private chat can only be deleted if it was sent more than 24 hours ago.  
\- Bots can delete outgoing messages in private chats, groups, and supergroups.  
\- Bots can delete incoming messages in private chats.  
\- Bots granted *can\_post\_messages* permissions can delete outgoing messages in channels.  
\- If the bot is an administrator of a group, it can delete any message there.  
\- If the bot has *can\_delete\_messages* permission in a supergroup or a channel, it can delete any message there.  
Returns *True* on success.

### delete-sticker-from-set

Manage delete sticker from set

- **`telegram-pp-cli delete-sticker-from-set create`** - Use this method to delete a sticker from a set created by the bot. Returns *True* on success.

### delete-webhook

Manage delete webhook

- **`telegram-pp-cli delete-webhook create`** - Use this method to remove webhook integration if you decide to switch back to [getUpdates](https://core.telegram.org/bots/api/#getupdates). Returns *True* on success.

### edit-message-caption

Manage edit message caption

- **`telegram-pp-cli edit-message-caption create`** - Use this method to edit captions of messages. On success, if the edited message is not an inline message, the edited [Message](https://core.telegram.org/bots/api/#message) is returned, otherwise *True* is returned.

### edit-message-live-location

Manage edit message live location

- **`telegram-pp-cli edit-message-live-location create`** - Use this method to edit live location messages. A location can be edited until its *live\_period* expires or editing is explicitly disabled by a call to [stopMessageLiveLocation](https://core.telegram.org/bots/api/#stopmessagelivelocation). On success, if the edited message is not an inline message, the edited [Message](https://core.telegram.org/bots/api/#message) is returned, otherwise *True* is returned.

### edit-message-media

Manage edit message media

- **`telegram-pp-cli edit-message-media create`** - Use this method to edit animation, audio, document, photo, or video messages. If a message is part of a message album, then it can be edited only to an audio for audio albums, only to a document for document albums and to a photo or a video otherwise. When an inline message is edited, a new file can't be uploaded. Use a previously uploaded file via its file\_id or specify a URL. On success, if the edited message was sent by the bot, the edited [Message](https://core.telegram.org/bots/api/#message) is returned, otherwise *True* is returned.

### edit-message-reply-markup

Manage edit message reply markup

- **`telegram-pp-cli edit-message-reply-markup create`** - Use this method to edit only the reply markup of messages. On success, if the edited message is not an inline message, the edited [Message](https://core.telegram.org/bots/api/#message) is returned, otherwise *True* is returned.

### edit-message-text

Manage edit message text

- **`telegram-pp-cli edit-message-text create`** - Use this method to edit text and [game](https://core.telegram.org/bots/api/#games) messages. On success, if the edited message is not an inline message, the edited [Message](https://core.telegram.org/bots/api/#message) is returned, otherwise *True* is returned.

### export-chat-invite-link

Manage export chat invite link

- **`telegram-pp-cli export-chat-invite-link create`** - Use this method to generate a new invite link for a chat; any previously generated link is revoked. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Returns the new invite link as *String* on success.

### forward-message

Manage forward message

- **`telegram-pp-cli forward-message create`** - Use this method to forward messages of any kind. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### get-chat

Manage get chat

- **`telegram-pp-cli get-chat create`** - Use this method to get up to date information about the chat (current name of the user for one-on-one conversations, current username of a user, group or channel, etc.). Returns a [Chat](https://core.telegram.org/bots/api/#chat) object on success.

### get-chat-administrators

Manage get chat administrators

- **`telegram-pp-cli get-chat-administrators create`** - Use this method to get a list of administrators in a chat. On success, returns an Array of [ChatMember](https://core.telegram.org/bots/api/#chatmember) objects that contains information about all chat administrators except other bots. If the chat is a group or a supergroup and no administrators were appointed, only the creator will be returned.

### get-chat-member

Manage get chat member

- **`telegram-pp-cli get-chat-member create`** - Use this method to get information about a member of a chat. Returns a [ChatMember](https://core.telegram.org/bots/api/#chatmember) object on success.

### get-chat-members-count

Manage get chat members count

- **`telegram-pp-cli get-chat-members-count create`** - Use this method to get the number of members in a chat. Returns *Int* on success.

### get-file

Manage get file

- **`telegram-pp-cli get-file create`** - Use this method to get basic info about a file and prepare it for downloading. For the moment, bots can download files of up to 20MB in size. On success, a [File](https://core.telegram.org/bots/api/#file) object is returned. The file can then be downloaded via the link `https://api.telegram.org/file/bot<token>/<file_path>`, where `<file_path>` is taken from the response. It is guaranteed that the link will be valid for at least 1 hour. When the link expires, a new one can be requested by calling [getFile](https://core.telegram.org/bots/api/#getfile) again.

### get-game-high-scores

Manage get game high scores

- **`telegram-pp-cli get-game-high-scores create`** - Use this method to get data for high score tables. Will return the score of the specified user and several of their neighbors in a game. On success, returns an *Array* of [GameHighScore](https://core.telegram.org/bots/api/#gamehighscore) objects.

This method will currently return scores for the target user, plus two of their closest neighbors on each side. Will also return the top three users if the user and his neighbors are not among them. Please note that this behavior is subject to change.

### get-me

Manage get me

- **`telegram-pp-cli get-me create`** - A simple method for testing your bot's auth token. Requires no parameters. Returns basic information about the bot in form of a [User](https://core.telegram.org/bots/api/#user) object.

### get-my-commands

Manage get my commands

- **`telegram-pp-cli get-my-commands create`** - Use this method to get the current list of the bot's commands. Requires no parameters. Returns Array of [BotCommand](https://core.telegram.org/bots/api/#botcommand) on success.

### get-sticker-set

Manage get sticker set

- **`telegram-pp-cli get-sticker-set create`** - Use this method to get a sticker set. On success, a [StickerSet](https://core.telegram.org/bots/api/#stickerset) object is returned.

### get-updates

Manage get updates

- **`telegram-pp-cli get-updates create`** - Use this method to receive incoming updates using long polling ([wiki](https://en.wikipedia.org/wiki/Push_technology#Long_polling)). An Array of [Update](https://core.telegram.org/bots/api/#update) objects is returned.

### get-user-profile-photos

Manage get user profile photos

- **`telegram-pp-cli get-user-profile-photos create`** - Use this method to get a list of profile pictures for a user. Returns a [UserProfilePhotos](https://core.telegram.org/bots/api/#userprofilephotos) object.

### get-webhook-info

Manage get webhook info

- **`telegram-pp-cli get-webhook-info create`** - Use this method to get current webhook status. Requires no parameters. On success, returns a [WebhookInfo](https://core.telegram.org/bots/api/#webhookinfo) object. If the bot is using [getUpdates](https://core.telegram.org/bots/api/#getupdates), will return an object with the *url* field empty.

### kick-chat-member

Manage kick chat member

- **`telegram-pp-cli kick-chat-member create`** - Use this method to kick a user from a group, a supergroup or a channel. In the case of supergroups and channels, the user will not be able to return to the group on their own using invite links, etc., unless [unbanned](https://core.telegram.org/bots/api/#unbanchatmember) first. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Returns *True* on success.

### leave-chat

Manage leave chat

- **`telegram-pp-cli leave-chat create`** - Use this method for your bot to leave a group, supergroup or channel. Returns *True* on success.

### log-out

Manage log out

- **`telegram-pp-cli log-out create`** - Use this method to log out from the cloud Bot API server before launching the bot locally. You **must** log out the bot before running it locally, otherwise there is no guarantee that the bot will receive updates. After a successful call, you can immediately log in on a local server, but will not be able to log in back to the cloud Bot API server for 10 minutes. Returns *True* on success. Requires no parameters.

### pin-chat-message

Manage pin chat message

- **`telegram-pp-cli pin-chat-message create`** - Use this method to add a message to the list of pinned messages in a chat. If the chat is not a private chat, the bot must be an administrator in the chat for this to work and must have the 'can\_pin\_messages' admin right in a supergroup or 'can\_edit\_messages' admin right in a channel. Returns *True* on success.

### promote-chat-member

Manage promote chat member

- **`telegram-pp-cli promote-chat-member create`** - Use this method to promote or demote a user in a supergroup or a channel. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Pass *False* for all boolean parameters to demote a user. Returns *True* on success.

### restrict-chat-member

Manage restrict chat member

- **`telegram-pp-cli restrict-chat-member create`** - Use this method to restrict a user in a supergroup. The bot must be an administrator in the supergroup for this to work and must have the appropriate admin rights. Pass *True* for all permissions to lift restrictions from a user. Returns *True* on success.

### send-animation

Manage send animation

- **`telegram-pp-cli send-animation create`** - Use this method to send animation files (GIF or H.264/MPEG-4 AVC video without sound). On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned. Bots can currently send animation files of up to 50 MB in size, this limit may be changed in the future.

### send-audio

Manage send audio

- **`telegram-pp-cli send-audio create`** - Use this method to send audio files, if you want Telegram clients to display them in the music player. Your audio must be in the .MP3 or .M4A format. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned. Bots can currently send audio files of up to 50 MB in size, this limit may be changed in the future.

For sending voice messages, use the [sendVoice](https://core.telegram.org/bots/api/#sendvoice) method instead.

### send-chat-action

Manage send chat action

- **`telegram-pp-cli send-chat-action create`** - Use this method when you need to tell the user that something is happening on the bot's side. The status is set for 5 seconds or less (when a message arrives from your bot, Telegram clients clear its typing status). Returns *True* on success.

Example: The [ImageBot](https://t.me/imagebot) needs some time to process a request and upload the image. Instead of sending a text message along the lines of “Retrieving image, please wait…”, the bot may use [sendChatAction](https://core.telegram.org/bots/api/#sendchataction) with *action* = *upload\_photo*. The user will see a “sending photo” status for the bot.

We only recommend using this method when a response from the bot will take a **noticeable** amount of time to arrive.

### send-contact

Manage send contact

- **`telegram-pp-cli send-contact create`** - Use this method to send phone contacts. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-dice

Manage send dice

- **`telegram-pp-cli send-dice create`** - Use this method to send an animated emoji that will display a random value. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-document

Manage send document

- **`telegram-pp-cli send-document create`** - Use this method to send general files. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned. Bots can currently send files of any type of up to 50 MB in size, this limit may be changed in the future.

### send-game

Manage send game

- **`telegram-pp-cli send-game create`** - Use this method to send a game. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-invoice

Manage send invoice

- **`telegram-pp-cli send-invoice create`** - Use this method to send invoices. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-location

Manage send location

- **`telegram-pp-cli send-location create`** - Use this method to send point on the map. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-media-group

Manage send media group

- **`telegram-pp-cli send-media-group create`** - Use this method to send a group of photos, videos, documents or audios as an album. Documents and audio files can be only grouped in an album with messages of the same type. On success, an array of [Messages](https://core.telegram.org/bots/api/#message) that were sent is returned.

### send-message

Manage send message

- **`telegram-pp-cli send-message create`** - Use this method to send text messages. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-photo

Manage send photo

- **`telegram-pp-cli send-photo create`** - Use this method to send photos. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-poll

Manage send poll

- **`telegram-pp-cli send-poll create`** - Use this method to send a native poll. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-sticker

Manage send sticker

- **`telegram-pp-cli send-sticker create`** - Use this method to send static .WEBP or [animated](https://telegram.org/blog/animated-stickers) .TGS stickers. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-venue

Manage send venue

- **`telegram-pp-cli send-venue create`** - Use this method to send information about a venue. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-video

Manage send video

- **`telegram-pp-cli send-video create`** - Use this method to send video files, Telegram clients support mp4 videos (other formats may be sent as [Document](https://core.telegram.org/bots/api/#document)). On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned. Bots can currently send video files of up to 50 MB in size, this limit may be changed in the future.

### send-video-note

Manage send video note

- **`telegram-pp-cli send-video-note create`** - As of [v.4.0](https://telegram.org/blog/video-messages-and-telescope), Telegram clients support rounded square mp4 videos of up to 1 minute long. Use this method to send video messages. On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned.

### send-voice

Manage send voice

- **`telegram-pp-cli send-voice create`** - Use this method to send audio files, if you want Telegram clients to display the file as a playable voice message. For this to work, your audio must be in an .OGG file encoded with OPUS (other formats may be sent as [Audio](https://core.telegram.org/bots/api/#audio) or [Document](https://core.telegram.org/bots/api/#document)). On success, the sent [Message](https://core.telegram.org/bots/api/#message) is returned. Bots can currently send voice messages of up to 50 MB in size, this limit may be changed in the future.

### set-chat-administrator-custom-title

Manage set chat administrator custom title

- **`telegram-pp-cli set-chat-administrator-custom-title create`** - Use this method to set a custom title for an administrator in a supergroup promoted by the bot. Returns *True* on success.

### set-chat-description

Manage set chat description

- **`telegram-pp-cli set-chat-description create`** - Use this method to change the description of a group, a supergroup or a channel. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Returns *True* on success.

### set-chat-permissions

Manage set chat permissions

- **`telegram-pp-cli set-chat-permissions create`** - Use this method to set default chat permissions for all members. The bot must be an administrator in the group or a supergroup for this to work and must have the *can\_restrict\_members* admin rights. Returns *True* on success.

### set-chat-photo

Manage set chat photo

- **`telegram-pp-cli set-chat-photo create`** - Use this method to set a new profile photo for the chat. Photos can't be changed for private chats. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Returns *True* on success.

### set-chat-sticker-set

Manage set chat sticker set

- **`telegram-pp-cli set-chat-sticker-set create`** - Use this method to set a new group sticker set for a supergroup. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Use the field *can\_set\_sticker\_set* optionally returned in [getChat](https://core.telegram.org/bots/api/#getchat) requests to check if the bot can use this method. Returns *True* on success.

### set-chat-title

Manage set chat title

- **`telegram-pp-cli set-chat-title create`** - Use this method to change the title of a chat. Titles can't be changed for private chats. The bot must be an administrator in the chat for this to work and must have the appropriate admin rights. Returns *True* on success.

### set-game-score

Manage set game score

- **`telegram-pp-cli set-game-score create`** - Use this method to set the score of the specified user in a game. On success, if the message was sent by the bot, returns the edited [Message](https://core.telegram.org/bots/api/#message), otherwise returns *True*. Returns an error, if the new score is not greater than the user's current score in the chat and *force* is *False*.

### set-my-commands

Manage set my commands

- **`telegram-pp-cli set-my-commands create`** - Use this method to change the list of the bot's commands. Returns *True* on success.

### set-passport-data-errors

Manage set passport data errors

- **`telegram-pp-cli set-passport-data-errors create`** - Informs a user that some of the Telegram Passport elements they provided contains errors. The user will not be able to re-submit their Passport to you until the errors are fixed (the contents of the field for which you returned the error must change). Returns *True* on success.

Use this if the data submitted by the user doesn't satisfy the standards your service requires for any reason. For example, if a birthday date seems invalid, a submitted document is blurry, a scan shows evidence of tampering, etc. Supply some details in the error message to make sure the user knows how to correct the issues.

### set-sticker-position-in-set

Manage set sticker position in set

- **`telegram-pp-cli set-sticker-position-in-set create`** - Use this method to move a sticker in a set created by the bot to a specific position. Returns *True* on success.

### set-sticker-set-thumb

Manage set sticker set thumb

- **`telegram-pp-cli set-sticker-set-thumb create`** - Use this method to set the thumbnail of a sticker set. Animated thumbnails can be set for animated sticker sets only. Returns *True* on success.

### set-webhook

Manage set webhook

- **`telegram-pp-cli set-webhook create`** - Use this method to specify a url and receive incoming updates via an outgoing webhook. Whenever there is an update for the bot, we will send an HTTPS POST request to the specified url, containing a JSON-serialized [Update](https://core.telegram.org/bots/api/#update). In case of an unsuccessful request, we will give up after a reasonable amount of attempts. Returns *True* on success.

If you'd like to make sure that the Webhook request comes from Telegram, we recommend using a secret path in the URL, e.g. `https://www.example.com/<token>`. Since nobody else knows your bot's token, you can be pretty sure it's us.

### stop-message-live-location

Manage stop message live location

- **`telegram-pp-cli stop-message-live-location create`** - Use this method to stop updating a live location message before *live\_period* expires. On success, if the message was sent by the bot, the sent [Message](https://core.telegram.org/bots/api/#message) is returned, otherwise *True* is returned.

### stop-poll

Manage stop poll

- **`telegram-pp-cli stop-poll create`** - Use this method to stop a poll which was sent by the bot. On success, the stopped [Poll](https://core.telegram.org/bots/api/#poll) with the final results is returned.

### unban-chat-member

Manage unban chat member

- **`telegram-pp-cli unban-chat-member create`** - Use this method to unban a previously kicked user in a supergroup or channel. The user will **not** return to the group or channel automatically, but will be able to join via link, etc. The bot must be an administrator for this to work. By default, this method guarantees that after the call the user is not a member of the chat, but will be able to join it. So if the user is a member of the chat they will also be **removed** from the chat. If you don't want this, use the parameter *only\_if\_banned*. Returns *True* on success.

### unpin-all-chat-messages

Manage unpin all chat messages

- **`telegram-pp-cli unpin-all-chat-messages create`** - Use this method to clear the list of pinned messages in a chat. If the chat is not a private chat, the bot must be an administrator in the chat for this to work and must have the 'can\_pin\_messages' admin right in a supergroup or 'can\_edit\_messages' admin right in a channel. Returns *True* on success.

### unpin-chat-message

Manage unpin chat message

- **`telegram-pp-cli unpin-chat-message create`** - Use this method to remove a message from the list of pinned messages in a chat. If the chat is not a private chat, the bot must be an administrator in the chat for this to work and must have the 'can\_pin\_messages' admin right in a supergroup or 'can\_edit\_messages' admin right in a channel. Returns *True* on success.

### upload-sticker-file

Manage upload sticker file

- **`telegram-pp-cli upload-sticker-file create`** - Use this method to upload a .PNG file with a sticker for later use in *createNewStickerSet* and *addStickerToSet* methods (can be used multiple times). Returns the uploaded [File](https://core.telegram.org/bots/api/#file) on success.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
telegram-pp-cli add-sticker-to-set --emojis example-value

# JSON for scripting and agents
telegram-pp-cli add-sticker-to-set --emojis example-value --json

# Filter to specific fields
telegram-pp-cli add-sticker-to-set --emojis example-value --json --select id,name,status

# Dry run — show the request without sending
telegram-pp-cli add-sticker-to-set --emojis example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
telegram-pp-cli add-sticker-to-set --emojis example-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-telegram -g
```

Then invoke `/pp-telegram <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/cmd/telegram-pp-mcp@latest
```

Then register it:

```bash
claude mcp add telegram telegram-pp-mcp -e TELEGRAM_BOT_TOKEN=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/telegram-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TELEGRAM_BOT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/cmd/telegram-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "telegram": {
      "command": "telegram-pp-mcp",
      "env": {
        "TELEGRAM_BOT_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
telegram-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/telegram-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `telegram-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TELEGRAM_BOT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Exit 3 / 'unauthorized' from any send command** — TELEGRAM_BOT_TOKEN is unset or invalid. Run `telegram-pp-cli doctor` to confirm the token reaches getMe; re-create the bot via @BotFather if the token was revoked.
- **Exit 4 with 'can't parse entities' on an HTML send** — Run `telegram-pp-cli format html-lint --text "$BODY"` to find the offending offset, or re-send with `--html-escape` to auto-escape user-provided text.
- **Exit 7 / rate-limited (429) on bulk send** — Telegram allows 30 msg/sec globally and 1 msg/sec per chat. Throttle the caller, or replay failed sends from the local store using `messages list --since 1m --json` (custom SQL via `sql 'SELECT \u2026'`) and re-send with backoff.
- **send --replace-last returns 'message to edit not found' as a warning** — Telegram does not allow editing messages older than 48 hours. The CLI automatically falls back to sendMessage in this case \u2014 the new message_id is in the JSON output.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**rahiel/telegram-send**](https://github.com/rahiel/telegram-send) — Python
- [**fabianonline/telegram.sh**](https://github.com/fabianonline/telegram.sh) — Shell
- [**go-telegram/bot**](https://github.com/go-telegram/bot) — Go
- [**go-telegram-bot-api/telegram-bot-api**](https://github.com/go-telegram-bot-api/telegram-bot-api) — Go
- [**guangxiangdebizi/telegram-mcp**](https://github.com/guangxiangdebizi/telegram-mcp) — TypeScript
- [**harnyk/mcp-telegram-notifier**](https://github.com/harnyk/mcp-telegram-notifier) — TypeScript
- [**siavashdelkhosh81/telegram-bot-mcp-server**](https://github.com/siavashdelkhosh81/telegram-bot-mcp-server) — TypeScript
- [**danships/telegram-bot-mgmt-cli**](https://github.com/danships/telegram-bot-mgmt-cli) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
