# Gmail CLI Absorb Manifest

## Sources Researched
- **shinzo-labs/gmail-mcp** (TypeScript, MCP) — 64 MCP tools; full Gmail API coverage; file-based OAuth2
- **GongRzhe/Gmail-MCP-Server** (TypeScript, MCP) — 14 tools; Claude Desktop focused; batch ops
- **googleworkspace/cli** (TypeScript, CLI) — dynamic Discovery-based CLI; +send, +triage, +reply helpers
- **cmdg** (Go, TUI) — curses-style Gmail client; keyboard-driven; read-oriented
- **davidtkeane/gmail-multi-cli** (Python, CLI) — multi-account; read/send/forward; credential handling
- **@googleapis/gmail** (TypeScript npm) — official SDK; all API methods

## Absorbed Features (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | messages list + search | All MCPs | `messages list --query "..."` | FTS5 local search, works offline |
| 2 | message get (full) | shinzo-labs | `messages get <id>` | --select for field pruning |
| 3 | message send | googleworkspace/cli, shinzo-labs | `messages send --to --subject --body` | --dry-run, --stdin, --template |
| 4 | message reply | googleworkspace/cli | `messages reply <id>` | threading auto-handled |
| 5 | message reply-all | googleworkspace/cli | `messages reply-all <id>` | threading auto-handled |
| 6 | message forward | googleworkspace/cli | `messages forward <id> --to` | |
| 7 | message delete / trash / untrash | shinzo-labs | `messages delete/trash/untrash <id>` | --dry-run |
| 8 | message modify (add/remove labels) | shinzo-labs | `messages modify <id> --add-labels --remove-labels` | |
| 9 | batch delete messages | shinzo-labs, GongRzhe | `messages batch-delete --query` | query-driven bulk |
| 10 | batch modify messages | shinzo-labs, GongRzhe | `messages batch-modify --query --add-labels` | query-driven bulk |
| 11 | attachment get | shinzo-labs | `messages attachments get <msg-id> <att-id>` | |
| 12 | threads list | shinzo-labs | `threads list --query` | |
| 13 | thread get | shinzo-labs | `threads get <id>` | full conversation view |
| 14 | thread modify | shinzo-labs | `threads modify <id> --add-labels` | |
| 15 | thread delete / trash / untrash | shinzo-labs | `threads delete/trash/untrash <id>` | |
| 16 | drafts list | shinzo-labs | `drafts list` | |
| 17 | draft create | shinzo-labs | `drafts create --to --subject --body` | --stdin |
| 18 | draft get | shinzo-labs | `drafts get <id>` | |
| 19 | draft update | shinzo-labs | `drafts update <id>` | |
| 20 | draft send | shinzo-labs | `drafts send <id>` | --dry-run |
| 21 | draft delete | shinzo-labs | `drafts delete <id>` | |
| 22 | labels list | shinzo-labs | `labels list` | |
| 23 | label create | shinzo-labs | `labels create --name --color` | |
| 24 | label get | shinzo-labs | `labels get <id>` | |
| 25 | label update / patch | shinzo-labs | `labels update <id>` | |
| 26 | label delete | shinzo-labs | `labels delete <id>` | |
| 27 | history list (delta feed) | API only | `history list --start-history-id` | JSON stream for webhooks |
| 28 | profile get | shinzo-labs | `profile` | |
| 29 | settings get/update vacation | shinzo-labs | `settings vacation get/set` | |
| 30 | settings get/update IMAP | shinzo-labs | `settings imap get/set` | |
| 31 | settings get/update POP | shinzo-labs | `settings pop get/set` | |
| 32 | settings get/update forwarding | shinzo-labs | `settings forwarding get/set` | |
| 33 | settings filters list/create/delete | shinzo-labs | `settings filters list/create/delete` | |
| 34 | settings delegates list/add/remove | shinzo-labs | `settings delegates list/add/remove` | |
| 35 | settings sendAs list/create/verify | shinzo-labs | `settings send-as list/create/verify` | |
| 36 | inbox triage view | googleworkspace/cli | `triage` | FTS-backed, customizable columns |
| 37 | watch / stop mailbox push | shinzo-labs | `watch start/stop` | |

## Transcendence Features (only possible with local SQLite store)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Offline FTS search | `search "<query>"` | Instant regex/FTS5 search across synced subjects+body — no API call; works on a plane |
| 2 | Sender analytics | `senders stats` | Aggregate synced messages by From: header; compute reply rate, volume, days-since-last-replied — impossible via single API call |
| 3 | Newsletter / subscription detector | `newsletters list` | Parse List-Unsubscribe headers across all synced messages; group by domain; surface top candidates |
| 4 | One-command bulk unsubscribe | `newsletters unsubscribe` | Follow List-Unsubscribe mailto: or URL from locally-stored headers; trash all messages from sender |
| 5 | Attachment pipeline export | `attachments export` | SQL-join messages + attachment metadata in local store; batch-download all matching a --query to a local dir |
| 6 | Inbox digest | `inbox digest` | Aggregate unread messages from local store by label/sender; print daily summary in one command |
| 7 | Thread timeline | `threads timeline <id>` | Show conversation arc: participants, duration, message count, sentiment delta across synced thread messages |
| 8 | Stale threads detector | `threads stale` | Find threads with no activity in N days — the "which conversations went dark?" command |
