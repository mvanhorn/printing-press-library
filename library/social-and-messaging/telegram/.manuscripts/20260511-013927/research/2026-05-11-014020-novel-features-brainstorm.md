# Novel Features Brainstorm — telegram-pp-cli

## Customer model

**Persona A — Joey the alert-dispatcher agent (DM lane)**
- Today: Shells out to a Python `alert_kris.py` script that uses `python-telegram-bot` and hand-rolled retry/exit-code logic; failure modes leak as 2xx-with-error-body that the agent treats as success.
- Weekly ritual: Fires ~10-50 DM alerts/week to one chat ID — job-finished pings, "needs human review," failure traces, follow-up status edits as a long-running job progresses.
- Frustration: Has no machine-clean way to (a) confirm a previously-sent alert went through, (b) edit/replace the last "in-progress" message, or (c) avoid double-spamming Kris when retries fire after a transient 5xx.

**Persona B — Content-agent channel publisher (broadcast lane)**
- Today: `post_to_telegram.py` calls `bot.sendMessage(chat_id=@mychan, ...)`. Long-form posts get cut at 4096 chars; HTML escaping errors trash the message; there's no record of what was published.
- Weekly ritual: Publishes a handful of long-form HTML posts to one or more channels per week; sometimes follows up with an edit when the post has a typo.
- Frustration: No local record of what the bot has published — when an edit is needed, has to scroll Telegram to find the `message_id`. HTML escape bugs surface as `400: can't parse entities` only after the post fails halfway through a multi-part split.

**Persona C — Kris-the-operator (CI/cron + agent-runtime owner)**
- Today: Hand-edits `.env` files, runs the Python scripts from cron, debugs token problems by reading Python tracebacks at 2am.
- Weekly ritual: Sets up a new bot once a quarter; rotates a token; tweaks the channel allow-list; checks "did the agent actually deliver yesterday's alerts" once a week.
- Frustration: Pre-flight verification is ad-hoc (`curl https://api.telegram.org/bot$TOK/getMe | jq`); no audit trail of outbound messages; no idempotency when a cron rerun retries the same alert.

## Survivors

| # | Feature | Command | Score | Persona | Notes |
|---|---------|---------|-------|---------|-------|
| 1 | Idempotency-key send | `send --idempotency-key <key> --chat <id> --text "..."` | 9/10 | A, C | Hashes (bot_id, chat_id, key) into local store on first successful sendMessage; subsequent same-key invocations skip the API call and return cached `{message_id, chat_id, date, ok}` |
| 2 | Replace-last status message | `send --replace-last --chat <id> --text "Step 3/5"` | 9/10 | A | Reads most recent outbound message from local store, calls editMessageText; falls back to send-new on 400 (>48h) |
| 3 | Sent-message history | `messages list --chat <id> [--since 1d] [--mine]` | 8/10 | B, C | Pure SELECT against local messages table |
| 4 | Audit roll-up | `audit --since today [--chat <id>]` | 7/10 | C | Aggregation SELECT — count by chat, media type, errors, last-send |
| 5 | HTML preflight + safe-escape | `send --html-escape ...` and `format --html-lint ...` | 7/10 | B | Allowed-tag whitelist + offset-pointing lint for Telegram's HTML parse mode |
| 6 | Channel publish with manifest | `publish --channel @x --body file.md --record-as my-post`, `publish edit my-post --body file2.md` | 8/10 | B | Tracks (slug -> [message_ids]) for logical multi-part posts; re-edit only changed chunks |
| 7 | Chat resolver | `chats resolve @username` | 6/10 | A, B, C | Local-store cache of @handle -> chat_id, falls back to getChat |

## Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| Updates replay | Quarterly use; `sql` over store already covers it | — |
| doctor --send-test | Setup-time, not weekly | doctor (absorbed) |
| Webhook adopt/release | Quarterly; raw setWebhook/deleteWebhook suffice | absorbed |
| Rate-limit queue | Scope creep (daemon); in-invocation variant is generator behavior | — |
| Chats-file fan-out | Too close to absorbed multi-`--chat`; shell xargs substitutes | absorbed |
| last-sent introspection | Subsumed by --replace-last (users want the action) | #2 |
| MarkdownV2 lint | Same shape as #5 for a parse mode the named callers don't use | #5 |
| md-to-html converter | Overlap with #5 escape path | #5 |
