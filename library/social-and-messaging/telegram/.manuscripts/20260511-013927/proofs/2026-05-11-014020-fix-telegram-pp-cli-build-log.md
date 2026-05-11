# telegram-pp-cli Build Log

## What was built

### Foundation (Priority 0) — generator-emitted
- 74 spec-derived endpoint commands (sendMessage, sendPhoto, edit/delete/forward/copy, getMe, getUpdates, setWebhook, getChat, chat admin, sticker management, etc.).
- Local SQLite store + generic resources table, sync_state, FTS5 mirror.
- Standard agent-native floor: `--json`, `--select`, `--compact`, `--csv`, `--dry-run`, `--agent`, typed exit codes (0/2/3/4/5/7/10), `doctor`.
- MCP server scaffold (cobratree mirror of every Cobra command) + bundled `.mcpb` for Claude Desktop.

### Auth patch (Priority 0)
The apis-guru spec models Telegram's bot-token-in-URL as an OpenAPI server variable, which is the standard convention for token-in-path APIs but is not a SecurityScheme the generator can wire to env vars. Generator emitted `BaseURL: "https://api.telegram.org/bot<example-token>"` with the example token hardcoded — patched `internal/config/config.go` to:
- Default `BaseURL` to `https://api.telegram.org` (no token).
- After reading `TELEGRAM_BOT_TOKEN` from env, rebuild `BaseURL = "https://api.telegram.org/bot" + token`.
- Honor `TELEGRAM_BASE_URL` as a final override (preserves verify/mock-test code path).

Verified: with token unset, doctor reports `auth: not configured`; with `TELEGRAM_BOT_TOKEN=111:fake`, `doctor --json` reports `base_url: https://api.telegram.org/bot111:fake` and a dry-run of `send-message` correctly emits `POST https://api.telegram.org/bot111:fake/sendMessage`.

### Novel features (Priority 2) — hand-built
All 7 transcendence features from the absorb manifest are built and registered in `root.go`:

| # | File | Command | Status |
|---|------|---------|--------|
| 1 | `internal/cli/novel_send.go` | `send --idempotency-key <key>` (with `lookupIdempotent` / `saveIdempotent`) | shipping |
| 2 | `internal/cli/novel_send.go` | `send --replace-last` (with `lookupLastOutbound` + editMessageText fallback) | shipping |
| 3 | `internal/cli/novel_messages.go` | `messages list` | shipping |
| 4 | `internal/cli/novel_audit.go` | `audit` | shipping |
| 5 | `internal/cli/novel_send.go` + `internal/cli/novel_format.go` | `send --html-escape`, `format html-lint` | shipping |
| 6 | `internal/cli/novel_publish.go` | `publish send` / `publish edit` / `publish list` | shipping |
| 7 | `internal/cli/novel_chats_resolve.go` | `chats resolve` | shipping |

Auxiliary tables (`telegram_messages`, `telegram_idempotency`, `telegram_chats_resolved`) are declared in `internal/cli/novel_store.go` and created on first use via `CREATE TABLE IF NOT EXISTS` — the generator's `internal/store/store.go` is marked DO NOT EDIT, so novel-feature schema attaches lazily from the novel-command openers without touching the generated migration list.

### Tests
Pure-logic tests in `internal/cli/novel_send_test.go` cover: `resolveSendText`, `telegramHTMLEscape`, `splitMessage` (incl. no-spaces fallback), `lintTelegramHTML`, `parseSince`, `contentHash`. All pass.

## What was intentionally deferred
- `--inline-keyboard` builder helper (the `--reply-markup` JSON flag already supports the full Telegram InlineKeyboardMarkup shape; a friendlier builder would be polish).
- Per-error tagging in `telegram_messages.error` column (currently nullable, populated only by future paths that explicitly record failures). The column exists; the `audit` report already reads it.

## Skipped body fields
None of the 74 endpoints had body fields skipped by the generator. The spec is mostly flat primitives + InlineKeyboardMarkup / InputMedia objects which the generator emits as `--reply-markup <JSON>` / `--media <JSON>` flags.

## Generator limitations encountered

1. **Bot-token-in-URL auth is not first-class.** Telegram models this via an OpenAPI 3.0 server variable (`servers[0].url = "https://api.telegram.org/bot{token}"`). The generator emits the server URL as a literal default in `Config.BaseURL` and does not interpolate server variables from env at request time. Worked around with a 1-line config-loading patch; flagged as a candidate retro item — any APIs that put credentials in the URL path (Telegram, some legacy webhooks, AWS SDK signed URLs) would benefit from generator-level handling.

2. **`x-pp-mcp` extension not recognized.** The generator's stdout warns about the >50-tool surface even when the spec contains `x-pp-mcp: {transport, orchestration, endpoint_tools}`. I included the extension in the enriched spec at `$RESEARCH_DIR/telegram-spec-enriched.json`; if the canonical key is `mcp:` instead of `x-pp-mcp:` for OpenAPI inputs, the spec extension docs and the generator warning are mismatched. Flag for retro.

3. **`narrative.recipes[].command` containing `…` literal.** The research.json I wrote includes `…` (escaped ellipsis) inside one troubleshoot string. Generator passed it through to README/SKILL — JSON-valid, but renders as `…` not `…` in markdown viewers, which is fine. Noting for completeness.

## Lock heartbeat trail
build-p0 → build-p1 → build-p2 → build-complete (lock acquired at generation; will be released via `lock promote` at Phase 5.6 once shipcheck and dogfood pass).
