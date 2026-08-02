# gmail-pp-cli Absorb Manifest

Sources absorbed: gws (googleworkspace/cli, ~30.1k stars), GongRzhe/Gmail-MCP-Server (1.2k), shinzo-labs/gmail-mcp (~64 tools), gmailctl (2.2k), GYB (3.1k), lieer (648), simplegmail (407), ezgmail (272), himalaya (6.8k), raf.dev pattern, feedtailor/ccskill-gmail, Anthropic hosted Gmail connector (read-only gap).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Live search w/ Gmail query operators | gws, GongRzhe search_emails | gmail-pp-cli find | metadata-format batched fetch, table + --json/--select, positional query |
| 2 | Read email w/ decoded body | GongRzhe read_email | gmail-pp-cli read | recursive MIME walk + base64url + HTML-to-text (no competitor converts HTML), headers, attachment listing |
| 3 | Send email (attachments/HTML/cc/bcc) | gws +send, MCP send_email | gmail-pp-cli send | RFC2822 builder, --dry-run, quiet under verify |
| 4 | Reply / reply-all w/ auto-threading | gws +reply/+reply-all | gmail-pp-cli reply | In-Reply-To/References/Re: + quoted original, --all flag |
| 5 | Forward | gws +forward | gmail-pp-cli forward | attachment carry-over |
| 6 | Unread triage summary | gws +triage | gmail-pp-cli triage | grouped by sender + category, counts, --agent output |
| 7 |  Stream new mail | gws +watch |  gmail-pp-cli stream | history.list polling loop, NDJSON, --interval |
| 8 | Drafts lifecycle | shinzo-labs MCP | (generated endpoint) drafts create/list/get/update/delete/send | typed flags, --json |
| 9 | Labels CRUD | all tools | (generated endpoint) labels list/get/create/update/patch/delete | |
| 10 | Label by name + get-or-create | GongRzhe get_or_create_label | (behavior in gmail-pp-cli label) resolve names to IDs, auto-create missing | beats simplegmail's ID-only pain |
| 11 | Bulk modify (archive/mark-read at scale) | raf.dev pattern, shinzo | (generated endpoint) messages batchModify | 1000 ids/call |
| 12 | Bulk delete | shinzo-labs | (generated endpoint) messages batchDelete | --dry-run guard |
| 13 | Attachment download / harvest | ezgmail downloadAllAttachments | gmail-pp-cli attachments save | query-scoped harvest to dirs, base64url decode |
| 14 | Filters CRUD | gmailctl, GongRzhe | (generated endpoint) settings filters create/list/get/delete | |
| 15 | Vacation/sendAs/forwarding/delegates/imap/pop/language | GAM, shinzo-labs | (generated endpoint) settings get*/update* surface | consumer-account friendly |
| 16 | Profile / mailbox counts | GYB quota | (generated endpoint) users get-profile | messagesTotal/threadsTotal/historyId |
| 17 | Trash / untrash message + thread | simplegmail | (generated endpoint) messages/threads trash/untrash | |
| 18 | Thread get/modify/list | shinzo-labs | (generated endpoint) threads list/get/modify/delete | |
| 19 | Offline incremental sync | lieer | (behavior in gmail-pp-cli pull) full-message mirror via history.list cursor, 404 => windowed resync | full-format hydration, quota-aware batching |
| 20 | Local full-text mailbox search | lieer/notmuch niche | (behavior in gmail-pp-cli search) FTS5 over subject/from/to/snippet/body text | offline, instant, no quota |
| 21 | Export message .eml | GYB backup | gmail-pp-cli export | format=raw archival to file |
| 22 | Star/spam/archive/move-to-inbox verbs | simplegmail | (behavior in gmail-pp-cli label) system-label verbs: star = --add STARRED, archive = --remove INBOX, spam = --add SPAM | label-op sugar at 1000 ids/call |
| 23 | History listing (change records) | shinzo-labs | (generated endpoint) history list | sync debugging |
| 24 | Send-as / signature read+update | GAM signature | (generated endpoint) settings sendAs get/update/patch | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|---------|--------------|-------------------------|
| 1 | Scheduled send queue (user-vision) | schedule send --at/--in, schedule list/cancel/edit, schedule run --watch | 9/10 | Sofia (outreach founder) | hand-code | Gmail API has NO schedule-send endpoint (UI-only). Local scheduled_sends SQLite table + due-time processor + idempotency keys builds the missing capability |
| 2 | Sender leaderboard | senders --limit 20, senders --sender user@example.com | 9/10 | Marcus, Ken | hand-code | GROUP BY from_email over synced store (count, unread, sum size, first/last seen) — impossible via live API without O(mailbox) fetches |
| 3 | Storage hogs | storage --group-by sender | 8/10 | Marcus, Ken | hand-code | Local sizeEstimate aggregation by sender/year/attachments; each row emits the exact find query + batchDelete --dry-run to clean it |
| 4 | Follow-up tracker | followups --direction in|out --days 3 | 8/10 | Sofia, Marcus, Priya | hand-code | SQL window query over threads+messages classifying "last word theirs/mine" vs sendAs addresses — a per-thread fetch storm via live API (20 units/msg) |
| 5 | Unsubscribe audit | unsub --min-count 10 | 7/10 | Marcus | hand-code | Joins stored List-Unsubscribe headers with per-sender volume + unread ratio; emits mailto:/URL targets in --json for agent execution |
| 6 | Filters as code | filters diff --file filters.yaml, filters apply | 7/10 | Marcus | hand-code | Three-way diff between flat YAML file and live settings.filters, create/delete plan with --dry-run (gmailctl's niche, minus jsonnet complexity) |

Hand-code commitment: 6/6 transcendence features are hand-code. Absorbed rows 1-7, 10, 13, 19, 21, 22 are also hand-built commands (find, read, send, reply, forward, triage, stream, label, attachments save, export, pull): ~16 hand-written Cobra commands total, each ~50-150 LoC plus hook registration.

No stubs. All rows are shipping scope.
