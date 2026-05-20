# Novel Features Brainstorm — outlook-email

## Customer model

Personas served by this CLI:

1. **Inbox-zero operator (primary)** — Personal MSA owner with 10k–500k messages who wants to close loops. Wants to know what's flagged, what's old-unread, what's awaiting a reply they sent, what arrived since last check. Today scrolls Outlook UI or pays $30/seat for Copilot. Pain: Outlook UI shows no aggregate views; Copilot is gated on work tenants.
2. **Sender-pruner / unsubscriber** — Drowning in newsletters, automated alerts, and one-off senders. Wants to see "who sent me the most this month," "which senders blew up the inbox," "what can I bulk-archive or rule away." Outlook's UI groups by date, not by sender volume.
3. **Agent-driving power user** — Drives the inbox from Claude/scripts. Needs JSON, exit codes, deterministic flags, no interactive prompts, MCP server. Wants the local store so agents can ask cross-folder/cross-time questions without paging Graph.
4. **Follow-up tracker** — Sales/PM/recruiter pattern: "emails I sent more than N days ago that the recipient never replied to." Graph has no endpoint for this; today done by hand or with brittle Power Automate flows.
5. **Attachments-hygiene auditor** — Mailbox is at quota. Wants to find old attachments, large attachments, attachments by sender. Outlook web UI sort-by-size is buried and per-folder.

## Candidates (pre-cut)

(Full Pass 2 list — see survivors and kills sections for verdicts.)

1. followup — joins messages-from-me against later messages-from-recipient via conversation_id/internet_message_id. (a)(e). KEEP.
2. senders — SQL GROUP BY from_email with count/unread/last-received. (a)(b). KEEP.
3. quiet — senders with ≥K in prior window and 0 in trailing. (b)(e). KEEP.
4. since — received_at > T grouped by inference_classification/from/folder. (a). KEEP.
5. flagged — open flagged-todo report with overdue. (a). KEEP.
6. stale-unread — unread older than N days by folder. (a). KEEP.
7. attachments stale — attachment metadata join with received_at. (a). KEEP.
8. dedup — group by conversation_id / internet_message_id / (subject, from, to). (a)(b). KEEP cautious.
9. digest — daily summary aggregations. (a)(b). KEEP.
10. rules-suggest — heuristic inbox-rule proposals. (a). KILL (verifiability).
11. conversations — top conversations by message count / unread tail. (a)(b). KEEP.
12. waiting-on-me — last-message-not-from-me, unread/unanswered for N days. (b)(e). KEEP.
13. categories report — aggregation by color category. (a). KILL (niche).
14. importance audit — importance field roll-up. (b). KILL (speculative).
15. bulk-archive — plan-then-execute via absorbed move endpoint. (a)(b). KEEP w/ safety.
16. send-window — when-do-I-send rollup. (b). KILL (speculative).

Kill-check sweep: no LLM dependency, no external service, all auth in-scope, no reimplementation (all read from internal/store populated by sync). Verifiability flag on quiet, dedup, rules-suggest, send-window — the strongest three kept, rules-suggest killed.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona served | Buildability proof |
|---|---------|---------|-------|----------------|--------------------|
| 1 | Unanswered sent mail | `followup --to <person> --days 7` | 9/10 | Follow-up tracker, inbox-zero operator | Joins local `messages` where `from_email = me` against later `messages` to/cc'ing the recipient using `internet_message_id`/`conversation_id`; no Graph endpoint exists for this. |
| 2 | Sender volume rollup | `senders --window 30d --min 5` | 9/10 | Sender-pruner, inbox-zero operator | SQL `GROUP BY from_email` over local `messages` with count/unread-count/last-received/dominant-folder aggregates. |
| 3 | What arrived since | `since <timestamp\|relative>` | 8/10 | Inbox-zero operator, agent | SQL filter `received_at > T` over local `messages`, grouped by `inference_classification`/`from_email`/`parent_folder_id`. |
| 4 | Open flagged todos | `flagged --overdue` | 8/10 | Inbox-zero operator | SQL filter `flag_status = 'flagged' AND complete_at IS NULL` over local `messages` with due-date diff in select list. |
| 5 | Stale unread report | `stale-unread --days 14` | 8/10 | Inbox-zero operator | SQL filter `is_read = 0 AND received_at < now - N` grouped by `parent_folder_id` over local `messages`. |
| 6 | Waiting-on-me conversations | `waiting --days 3` | 8/10 | Follow-up tracker, inbox-zero operator | Window-function over local `messages` partitioned by `conversation_id` selecting the last message; filter `from_email != me AND is_read = 0`. |
| 7 | Top conversations | `conversations --top 20 --window 30d` | 7/10 | Agent, inbox-zero operator | SQL `GROUP BY conversation_id` over local `messages` ranked by message-count and unread-tail length. |
| 8 | Quiet senders | `quiet --baseline 90d --silent 30d` | 7/10 | Sender-pruner, follow-up tracker | SQL self-join over local `messages` flagging senders with ≥K messages in baseline window and 0 in trailing window. |
| 9 | Daily digest | `digest [--date YYYY-MM-DD]` | 7/10 | Inbox-zero operator, agent | Aggregations over local `messages` for the date: received/sent/unread/flagged counts, top 5 senders, top 5 conversations, focused/other ratio. |
| 10 | Stale attachments | `attachments stale --days 90 --min-mb 1` | 7/10 | Attachments-hygiene auditor | Joins local `attachments` (size/name/content_type captured at sync) with `messages.received_at`; data-model addition: persist attachment metadata rows during sync. |
| 11 | Likely duplicates | `dedup [--by conversation\|message-id\|subject-sender]` | 6/10 | Inbox-zero operator | SQL `GROUP BY` over local `messages` on `internet_message_id`, `conversation_id`, or `(normalized_subject, from_email, to_emails)` returning groups with >1 row. |
| 12 | Plan-then-execute bulk archive | `bulk-archive --from-senders <file>` (prints plan; `--execute` required) | 6/10 | Sender-pruner | Reads sender list from stdin/file, resolves matching message ids from local `messages`, prints `move` plan; with `--execute` calls absorbed POST `/me/messages/{id}/move`. Side-effect command: print-by-default, `cliutil.IsVerifyEnv()` short-circuit on `--execute`. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| categories report | Niche persona fit; many MSA users don't use color categories; `sql`/`search` builtins cover the ad-hoc case. | `senders` |
| importance audit | `importance` field rarely set on inbound personal MSA mail; user-pain speculative. | `flagged` |
| rules-suggest | Verifiability fails — "good rule" is subjective. `senders` + `quiet` give the user the raw signal to author rules themselves. | `senders` |
| send-window | Domain fit OK but user-pain speculative; better as ad-hoc `sql` query. | `digest` |
