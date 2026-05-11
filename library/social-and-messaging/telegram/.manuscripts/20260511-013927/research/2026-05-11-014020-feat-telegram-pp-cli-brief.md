# Telegram Bot CLI Brief

## API Identity
- **Domain:** Cross-platform messaging — bots send/receive messages, manage chats/channels/groups, edit messages, handle webhooks, host stickers and inline games.
- **Users:** Operators wiring notifications (cron, CI, scripts), agent runtimes that need to alert humans, support bots, channel publishers, and content automation pipelines.
- **Data profile:** Spec is OpenAPI 3.0.0, 74 endpoints, 103 schemas (Bot API 5.0). Live API is at 9.5 (March 2026); the core send/edit/forward surface used by the user's two scripts is stable across that gap. Auth is one bot token embedded directly in URL path: `https://api.telegram.org/bot<TOKEN>/<method>` — every method is `POST` (a few accept `GET`). No pagination tokens; `getUpdates` uses a long-poll offset.

## Reachability Risk
- **None.** Telegram Bot API is the canonical, well-maintained endpoint. apis.guru spec at 5.0 is older than live (9.5) but the headline send/edit/forward surface is stable. Probe of api.telegram.org returns 200 anonymously for `/getMe` (with a 401 if no token — which is the expected auth-required path).

## Top Workflows
1. **Send text or HTML message to a channel** (the `post_to_telegram.py` caller). Headline command must be one-liner-friendly: `telegram-pp-cli send --chat @mychan --text "..." --parse-mode HTML`.
2. **Send DM alert to a single user** (the `alert_kris.py` caller). Same `send` command, just with a numeric chat id (Telegram user/private chat). Output must be machine-parseable so the script can record the returned `message_id`.
3. **Pre-flight token check** (`doctor`) — verify token is valid, returns the bot identity. Used in CI/agent setup before sending the first message.
4. **Edit / delete the last alert** — re-write or retract a sent message. Critical for status-message patterns where an alert is updated as a job progresses.
5. **Read recent updates** (`get-updates`) — useful for diagnostic and for the rarer "bot receives a message" workflow. Long-poll offset persisted in local store so re-runs don't replay.

## Table Stakes (must match every notable competitor)
- `--text` / `--stdin` / positional text — match `telegram-send` and `telegram.sh`.
- Parse mode flags: `--parse-mode HTML|MarkdownV2|Markdown` (default: plain text). Plus `--html` shorthand for the user's HTML use case.
- Multiple recipients in one invocation (fan-out): `--chat A --chat B "msg"`.
- Auto-split messages >4096 chars into multiple sends — `telegram-send` does this and it's a real footgun otherwise.
- `--disable-web-page-preview`, `--disable-notification` (silent send), `--protect-content`, `--reply-to <message_id>`.
- File attachments: `--photo`, `--document`, `--video`, `--audio`, `--animation`, `--voice`, `--sticker`, `--location <lat,lng>` — all with optional `--caption`.
- `--dry-run` (preview the wire request, don't send).
- `--json` output returning the API response shape so scripts can pull `message_id` etc.
- Typed exit codes: 0=success, 2=usage, 3=auth, 4=API error (4xx), 5=network/5xx, 7=rate-limited.
- `doctor` checks token, network, and `getMe`.

## Data Layer
- **Primary entities:** `bot` (one row, the authenticated bot identity), `chats` (chats this bot has interacted with — populated by `getUpdates` and successful sends), `messages` (every message we've sent or received, with `chat_id`, `message_id`, `date`, `text`, `caption`, `media_type`), `updates` (raw update objects keyed by `update_id` for replay).
- **Sync cursor:** `getUpdates` long-poll `offset` — single integer per bot, persisted between runs so a `sync` doesn't replay.
- **FTS/search:** `messages_fts` over `text` and `caption` enables `telegram-pp-cli search "deploy failed"` to find prior alerts.
- **Why local store matters:** novel features (history-aware alerts, deduped alerts, status-message threading, "what did I send today") all need it.

## Codebase Intelligence
- Catalog notes confirm flat command structure (no sub-resources), 50+ commands, token-in-URL auth. Spec is community-tier, verified 2026-03-24.
- Top Go wrapper: `go-telegram/bot` (zero-deps, Bot API 9.5, active). The classic `go-telegram-bot-api/telegram-bot-api` is also widely used but locked to a slightly older API revision. Our CLI will generate from the spec rather than wrapping either, but the wrappers' surface validates that direct `POST /bot<token>/<method>` is the right transport.

## User Vision
- Two Python callers will shell out to this CLI: `agents/content_agent/execution/post_to_telegram.py` posts to a channel, `agents/joey/execution/alert_kris.py` sends DM alerts.
- Auth is `TELEGRAM_BOT_TOKEN` env var.
- Compact output and typed exit codes are required — the callers will gate on exit codes and parse JSON for `message_id`.
- Deliverables: binary, Claude skill, MCP server.

## Product Thesis
- **Name:** `telegram-pp-cli` (binary), `telegram` (catalog slug + library directory).
- **Tagline:** "Every Telegram Bot API method, plus a local message store and a `send` command that doesn't make you read a docs page."
- **Why it should exist:**
  - `telegram-send` is Python — adds an interpreter dep to environments that don't want one. The user's two scripts already use Python; a *separate* Go binary keeps the agent runtime decoupled from script runtime.
  - `telegram.sh` is shell — no `--json` output, no exit-code taxonomy, no auto-split.
  - Existing MCP servers expose raw endpoint mirrors; we ship a code-orchestration pair (`telegram_search` + `telegram_execute`) so an agent loads ~1K tokens of MCP surface, not 74 raw tools.
  - We have the agent-native floor every printed CLI gets: `--json`, `--select`, `--dry-run`, typed exits, doctor.

## Build Priorities
1. **Foundation (Priority 0):** data layer for `bot`, `chats`, `messages`, `updates`. `sync` populates by calling `getUpdates`. `search` / `sql` against the store.
2. **Headline command (Priority 1):** `send` (alias of `sendMessage`) — ergonomic flags (`--chat`, `--text`, `--stdin`, `--parse-mode`, `--html`, `--disable-web-page-preview`, `--silent`, `--reply-to`, `--protect`, multi-`--chat` fan-out, auto-split, `--dry-run`). Returns `{message_id, chat_id, date, ok}` on `--json`.
3. **Rest of absorbed surface (Priority 1):** every `send*` method, `editMessageText`, `editMessageCaption`, `deleteMessage`, `forwardMessage`, `copyMessage`, `getMe`, `getUpdates`, `setMyCommands`/`getMyCommands`, `setWebhook`/`deleteWebhook`/`getWebhookInfo`, `getChat`, `getChatMember`, `getChatMembersCount`, `sendChatAction`, `answerCallbackQuery`, `setMyName`/`setMyDescription`/`setMyShortDescription`.
4. **Transcendence (Priority 2):** Built in Phase 3 from the absorb manifest's transcendence table — features that require the local store and exist nowhere else.
5. **Polish (Priority 3):** flag-description enrichment for the spec-derived commands, README + SKILL recipes targeted at the two shell-out callers.

## MCP Surface Decision (pre-generation)
- 74 spec endpoints + ~13 framework tools = **~87 MCP tools** by default.
- Recommend the Cloudflare pattern at generate-time:
  - `mcp.transport: [stdio, http]` — agents can reach this remotely.
  - `mcp.orchestration: code` — emit `telegram_search` + `telegram_execute` pair instead of 74 typed mirrors.
  - `mcp.endpoint_tools: hidden` — suppress raw mirrors.
- The user said "MCP server" is a deliverable; agents shelling out via MCP almost exclusively want `send-message` and `get-me`. The orchestration pair covers all 74 endpoints in ~1K tokens of tool-list payload, with `send-message` reachable as a named intent.
