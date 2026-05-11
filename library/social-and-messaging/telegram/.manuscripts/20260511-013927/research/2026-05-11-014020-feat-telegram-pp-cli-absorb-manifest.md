# telegram-pp-cli Absorb Manifest

## API surface summary
Telegram Bot API, OpenAPI 3.0.0 from apis.guru (5.0.0; live 9.5 — send/edit/forward family is stable across that gap). 74 endpoints under `https://api.telegram.org/bot<TOKEN>/<method>`, every method is `POST`. Auth: bot token in URL path, sourced from `TELEGRAM_BOT_TOKEN`.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Send text message | telegram-send (positional arg), telegram.sh (-t/-c/positional) | `send` command (alias of `sendMessage`); positional or `--text`; multi-`--chat` | `--json` returns full API response; typed exit codes |
| 2 | Send via stdin | telegram-send (--stdin) | `send --stdin` | Works with `--json` for capturing `message_id` of piped content |
| 3 | HTML / Markdown / MarkdownV2 formatting | telegram-send (`--format html`), telegram.sh (-M) | `--parse-mode HTML|MarkdownV2|Markdown`, `--html` shorthand | `--html-escape` (see novel #5) safely escapes user input |
| 4 | Send photo / image | telegram-send (--image), telegram.sh (-i) | `sendPhoto` endpoint, `--photo` flag, `--caption` | Generator-provided `--dry-run`, JSON response |
| 5 | Send document / file | telegram-send (--file), telegram.sh (-f) | `sendDocument` endpoint, `--document` flag | Same |
| 6 | Send video | telegram-send (--video), telegram.sh (-V) | `sendVideo` endpoint | Same |
| 7 | Send animation / GIF | telegram-send (--animation) | `sendAnimation` endpoint | Same |
| 8 | Send audio | telegram-send (--audio) | `sendAudio` endpoint | Same |
| 9 | Send voice | gap in shell CLIs (Bot API native) | `sendVoice` endpoint | First-class voice support |
| 10 | Send sticker | telegram-send (--sticker) | `sendSticker` endpoint | Same |
| 11 | Send location | telegram-send (--location) | `sendLocation` endpoint | Same |
| 12 | Send poll | gap in CLIs (Bot API native) | `sendPoll` endpoint | First-class poll support |
| 13 | Caption on media | telegram-send (--caption) | `--caption` flag across all sendX commands | Same |
| 14 | Multi-recipient fan-out | telegram-send (`--config` repeat), telegram.sh (`-c` repeat) | `--chat` repeatable; one wire request per chat | Per-recipient `--json` envelope; failure isolation |
| 15 | Auto-split messages >4096 chars | telegram-send, siavashdelkhosh81/telegram-bot-mcp-server | Generator wires auto-split into the `send` path | Splits at paragraph/sentence boundaries when possible |
| 16 | Disable web page preview | telegram-send (--disable-web-page-preview) | `--disable-web-page-preview` | Same |
| 17 | Silent notification | gap in CLIs (Bot API native) | `--silent` (maps to `disable_notification`) | First-class flag |
| 18 | Protect content | gap (Bot API native) | `--protect` (maps to `protect_content`) | First-class flag |
| 19 | Reply to message | gap (Bot API native) | `--reply-to <message_id>` | First-class flag |
| 20 | Forward message | guangxiangdebizi/telegram-mcp | `forwardMessage` endpoint | JSON response |
| 21 | Copy message | gap in CLIs (Bot API native) | `copyMessage` endpoint | Same |
| 22 | Edit message text | guangxiangdebizi/telegram-mcp, chigwell/telegram-mcp | `editMessageText` endpoint | Combined with novel `--replace-last` |
| 23 | Edit message caption | gap in CLIs (Bot API native) | `editMessageCaption` endpoint | Same |
| 24 | Delete message | guangxiangdebizi/telegram-mcp, harnyk/mcp-telegram-notifier | `deleteMessage` endpoint | Same |
| 25 | List chats | telegram.sh (-l) | `chats list` from local store (after sync) | Filters, JSON output |
| 26 | Get last received message | telegram.sh (-m) | `messages list --inbound --limit 1` | Generalizes to novel `messages list` |
| 27 | getMe (token check) | every MCP, every wrapper | `getMe` endpoint + `doctor` command | Doctor reports auth + network + bot identity |
| 28 | getUpdates / inbox poll | gap in CLIs (Bot API native, several MCPs) | `getUpdates` + `sync` command | Persistent offset across runs |
| 29 | Set webhook | guangxiangdebizi/telegram-mcp, IQAIcom/mcp-telegram | `setWebhook` endpoint | Same |
| 30 | Delete webhook | guangxiangdebizi/telegram-mcp | `deleteWebhook` endpoint | Same |
| 31 | Get webhook info | gap (Bot API native) | `getWebhookInfo` endpoint | Same |
| 32 | Get chat info | guangxiangdebizi/telegram-mcp, chigwell/telegram-mcp | `getChat` endpoint | Combined with novel chats-resolver |
| 33 | Get chat member | gap (Bot API native) | `getChatMember` endpoint | Same |
| 34 | Get chat member count | gap (Bot API native) | `getChatMembersCount` endpoint | Same |
| 35 | Send typing indicator | gap in CLIs (Bot API native) | `sendChatAction` endpoint | Same |
| 36 | Set bot commands | guangxiangdebizi/telegram-mcp | `setMyCommands` endpoint | Same |
| 37 | Get bot commands | gap (Bot API native) | `getMyCommands` endpoint | Same |
| 38 | Set bot name | gap (Bot API native) | `setMyName` endpoint | Same |
| 39 | Set bot description | gap (Bot API native) | `setMyDescription` / `setMyShortDescription` | Same |
| 40 | Answer callback query | guangxiangdebizi/telegram-mcp | `answerCallbackQuery` endpoint | Same |
| 41 | Answer inline query | gap (Bot API native) | `answerInlineQuery` endpoint | Same |
| 42 | Pin / unpin message | guangxiangdebizi/telegram-mcp | `pinChatMessage` / `unpinChatMessage` | Same |
| 43 | Promote / restrict member | guangxiangdebizi/telegram-mcp | `promoteChatMember` / `restrictChatMember` | Same |
| 44 | Ban / unban member | guangxiangdebizi/telegram-mcp | `banChatMember` / `unbanChatMember` | Same |
| 45 | Sticker set management | (Bot API native, MCPs) | `createNewStickerSet` etc. | Same |
| 46 | Inline keyboard / reply markup | telegram.sh README, several CLIs | `--inline-keyboard` JSON flag on `send` | Same |
| 47 | Get / download file | guangxiangdebizi/telegram-mcp | `getFile` endpoint + download helper | Same |
| 48 | Dry-run / preview request | gap among competitors | Generator-provided `--dry-run` | Prints wire request, no API call |
| 49 | JSON output for scripting | gap (telegram-send returns nothing useful; telegram.sh prints curl raw) | Generator-provided `--json`, `--select`, `--compact` | Agent-friendly composability |
| 50 | Typed exit codes | gap among competitors | Generator-provided 0/2/3/4/5/7 | Scripts can branch on auth vs network vs rate-limit |
| 51 | Doctor / health check | gap among CLIs | Generator-provided `doctor` command | Token + network + getMe in one call |
| 52 | Proxy support (HTTPS_PROXY) | telegram-send, telegram.sh | Generator HTTP client honors env | Same |
| 53 | SQL / search over local store | gap | Generator-provided `sql` and `search` | FTS5 over messages.text/caption |
| 54 | Code-orchestration MCP (search+execute pair) | gap (every MCP ships 27+ raw tools) | `mcp.orchestration: code` in spec — emits `telegram_search` + `telegram_execute` pair | ~1K-token MCP payload instead of 87 tools |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Idempotent send | `send --idempotency-key <key> --chat <id> --text "..."` | Hashes `(bot_id, chat_id, key)` into the local store after the first successful POST. Subsequent invocations with the same key short-circuit to the cached `{message_id, chat_id, date, ok}` envelope without calling the API. Resolves cron-retry double-send. No competitor has it. | 9/10 |
| 2 | Replace-last status message | `send --replace-last --chat <id> --text "Step 3/5"` | Looks up most recent outbound `messages` row for this `(bot_id, chat_id)`, calls `editMessageText`. Falls back to `sendMessage` if the message is >48h old or was deleted. The long-running-job pattern is the canonical bot use case and no CLI competitor supports it. | 9/10 |
| 3 | Sent-message history | `messages list --chat <id> [--since 1d] [--mine] [--media-type photo]` | Pure SELECT against the local `messages` table populated by every send and `sync` cycle. No API call. No surveyed Telegram CLI has any local history. | 8/10 |
| 4 | Activity audit | `audit --since today [--chat <id>] [--json]` | Aggregation SELECT: count by chat, by media type, success-vs-error, last-send timestamp. Persona C's weekly "did the agent deliver" check. Generator-provided `sql` can do this but the focused command is one-shot. | 7/10 |
| 5 | HTML preflight + safe-escape | `send --html-escape --text "$RAW"` and `format --html-lint --text "..."` | Allowed-tag whitelist over Telegram's HTML subset; HTML-encodes `<`/`>`/`&` outside allowed tags before sending. `--html-lint` returns the same diagnostic Telegram would on a 400, with byte offsets. Avoids the parse-mode footgun that bites `post_to_telegram.py` today. | 7/10 |
| 6 | Channel publish with manifest | `publish --channel @x --body file.md --record-as my-post`, `publish edit my-post --body file2.md` | First call splits body to ≤4096-char chunks, sends each, records `(slug, [message_ids])` in local store. `publish edit` looks up the slug and calls `editMessageText` per chunk, only re-editing chunks whose content hash changed. Tracks the logical-post -> message-id mapping no competitor maintains. | 8/10 |
| 7 | Chat resolver | `chats resolve @username [--json]` | Local-store lookup of cached `username -> chat_id`; on miss, calls `getChat` and caches. Returns `{chat_id, type, title}`. Solves "the numeric chat_id changed when the group was upgraded to supergroup." | 6/10 |

## Stubs / scope-deferred

None. Every absorbed and transcendence row above is shipping scope.

## Risk notes

- Bot API 5.0 vs live 9.5: spec is missing newer endpoint families (chat-boosts, business-connection, etc.). For the named callers' use case this is irrelevant; flag for retro if a user later asks for boost or business features.
- `--inline-keyboard` JSON flag is the standard escape hatch for advanced features the catalog spec doesn't model cleanly.
