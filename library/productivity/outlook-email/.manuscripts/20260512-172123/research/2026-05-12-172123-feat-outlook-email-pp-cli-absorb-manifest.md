# Outlook Email CLI Absorb Manifest

Generated 2026-05-12. Sole API: Microsoft Graph v1.0 mail surface. Auth: OAuth 2.0 device-code against `/common` (personal MSA). Mirror of the `outlook-calendar-pp-cli` pattern.

## Source tools surveyed

| Tool | Source | Role |
|------|--------|------|
| msgcli (skylarbpayne/msgcli) | https://github.com/skylarbpayne/msgcli | Top competitor; agent-first; mail + calendar |
| outpost (signalclaude/outpost) | https://github.com/signalclaude/outpost | Multi-product (mail/cal/tasks/teams); 39-tool MCP |
| outlook-mcp (sajadghawami) | https://github.com/sajadghawami/outlook-mcp | 23 MCP tools; categories, folders, rules |
| outlook-mcp (XenoXilus) | https://github.com/XenoXilus/outlook-mcp | Attachments + SharePoint integration |
| outlook-mcp (ryaker) | https://github.com/ryaker/outlook-mcp | Email + calendar + OneDrive |
| OutlookMCPServer (Norcim133) | https://github.com/Norcim133/OutlookMCPServer | Compose/respond/sort/search/filter MCP |
| cowork-outlook-plugin (brendanerofeev) | https://github.com/brendanerofeev/cowork-outlook-plugin | Drafts-only safe-send pattern |
| outlook-cli (mhattingpete) | https://github.com/mhattingpete/outlook-cli | Work-account-only basic CLI |
| outlook-skill (cristiandan) | https://github.com/cristiandan/outlook-skill | Claude skill, no binary |
| agentbuilder-outlook-mcp (jayozer) | https://github.com/jayozer/agentbuilder-outlook-mcp | Single-tool send-only MCP |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | List inbox messages | msgcli mail list | GET /me/mailFolders/inbox/messages | Offline FTS, --select, --csv |
| 2 | List across folders | sajadghawami list-emails | GET /me/messages | OData composability, JSON |
| 3 | Get message by id | msgcli mail get | GET /me/messages/{id} | --select, attachment metadata included |
| 4 | Server-side search ($search) | msgcli --query | GET /me/messages?$search | Quoted phrases, OData operators |
| 5 | Offline FTS search | (none) | local `search` builtin | Regex, AND-NOT, cross-folder |
| 6 | SQL query against local store | (none) | local `sql` builtin | Power-user composition |
| 7 | Send new message | msgcli mail send | POST /me/sendMail | Markdown body, --stdin, --dry-run |
| 8 | Reply to message | msgcli mail reply | POST /me/messages/{id}/reply | Reply chain |
| 9 | Reply-all | msgcli reply --all | POST /me/messages/{id}/replyAll | Same |
| 10 | Forward message | sajadghawami | POST /me/messages/{id}/forward | --to multi, --comment |
| 11 | Delete message | msgcli mail delete | DELETE /me/messages/{id} | --hard skip Deleted Items |
| 12 | Move message | msgcli mail move | POST /me/messages/{id}/move | --by-name folder resolution |
| 13 | Copy message | sajadghawami | POST /me/messages/{id}/copy | --to-folder |
| 14 | Mark read/unread | sajadghawami | PATCH /me/messages/{id} | --stdin batch |
| 15 | Flag with due-date | (Graph only) | PATCH /me/messages/{id} flag | --due, --start |
| 16 | Complete flag | (Graph only) | PATCH /me/messages/{id} flag | --complete |
| 17 | List mail folders | msgcli mail folders | GET /me/mailFolders | Hierarchical render |
| 18 | Get folder | sajadghawami | GET /me/mailFolders/{id} | Child folders, totals |
| 19 | Create folder | sajadghawami create-folder | POST /me/mailFolders | --parent |
| 20 | Update folder (rename) | (Graph only) | PATCH /me/mailFolders/{id} | --name |
| 21 | Delete folder | (Graph only) | DELETE /me/mailFolders/{id} | --dry-run |
| 22 | List folder messages | sajadghawami | GET /me/mailFolders/{id}/messages | $filter, $top, $select |
| 23 | List attachments (metadata) | XenoXilus | GET /me/messages/{id}/attachments | metadata only by default |
| 24 | Download attachment | XenoXilus | GET .../attachments/{id}/$value | --output, --all |
| 25 | List color categories | sajadghawami | GET /me/outlook/masterCategories | Color rendered |
| 26 | Create category | sajadghawami | POST /me/outlook/masterCategories | --color enum |
| 27 | Delete category | sajadghawami | DELETE /me/outlook/masterCategories/{id} | Same |
| 28 | Apply/remove category on message | (Graph only) | PATCH /me/messages/{id} | --add / --remove deltas |
| 29 | Focused/Other filter | (Graph only) | GET /me/messages?$filter=inferenceClassification | --focused / --other |
| 30 | List inference overrides | (Graph only) | GET /me/inferenceClassification/overrides | Render |
| 31 | Pin sender Focused/Other | (Graph only) | POST/DEL /me/inferenceClassification/overrides | --sender --to |
| 32 | Mailbox settings read | (Graph only) | GET /me/mailboxSettings | Timezone, language, auto-reply |
| 33 | Set auto-reply | (Graph only) | PATCH /me/mailboxSettings | --start --end --internal --external |
| 34 | Delta-sync messages | (Graph only) | /me/messages/delta() | Cursor in local store |
| 35 | Delta-sync folders | (Graph only) | /me/mailFolders/delta() | Same |
| 36 | List inbox rules | sajadghawami list-rules | GET /me/mailFolders/inbox/messageRules | Render conditions |
| 37 | Create inbox rule | sajadghawami create-rule | POST /me/mailFolders/inbox/messageRules | YAML rule spec |
| 38 | Update inbox rule | sajadghawami edit-rule-sequence | PATCH /me/mailFolders/inbox/messageRules/{id} | --priority N |
| 39 | Delete inbox rule | (Graph only) | DELETE /me/mailFolders/inbox/messageRules/{id} | --dry-run |
| 40 | Create draft | brendanerofeev | POST /me/messages | --no-send |
| 41 | Send saved draft | (Graph only) | POST /me/messages/{id}/send | Same |
| 42 | Conversation thread fetch | (Graph $filter) | GET /me/messages?$filter=conversationId | Local reconstruction |
| 43 | Local sync (delta into SQLite) | (none) | `sync` subcommand | offline-first |
| 44 | MCP server (cobratree mirror) | sajadghawami/Norcim133 | generator-emitted MCP | Read-only annotations on safe tools |
| 45 | Doctor / health-check | (none) | `doctor` | Token, scopes, mailbox reachability |
| 46 | OAuth device-code login | msgcli auth | `auth login --device-code` | Persistent refresh tokens |
| 47 | Auth refresh | msgcli auth | `auth refresh` | Manual + automatic |
| 48 | Auth status | msgcli auth status | `auth status` | Token validity, expiry |
| 49 | JSON / --agent / --select / --csv | msgcli/outpost | universal flag layer | --quiet, --compact |
| 50 | Multi-account profiles | msgcli auth add | `auth login --profile <name>` | Switch via `--profile` |
| 51 | Agent context (cli-tree intro) | (none) | `agent-context` builtin | First-tool for agents |

## Transcendence (only possible with our approach)

Generated by the Phase 1.5c.5 novel-features-subagent. Full audit trail in `2026-05-12-172123-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Persona served | Why only we can do this |
|---|---------|---------|-------|----------------|--------------------------|
| 1 | Unanswered sent mail | `followup --to <person> --days 7` | 9/10 | Follow-up tracker, inbox-zero operator | Joins local `messages` where `from_email = me` against later `messages` to/cc'ing the recipient using `internet_message_id`/`conversation_id`. No Graph endpoint exists. |
| 2 | Sender volume rollup | `senders --window 30d --min 5` | 9/10 | Sender-pruner, inbox-zero operator | SQL `GROUP BY from_email` over local `messages` with count/unread-count/last-received/dominant-folder. |
| 3 | What arrived since | `since <timestamp\|relative>` | 8/10 | Inbox-zero operator, agent | SQL filter `received_at > T` over local store, grouped by focused/other/sender/folder. |
| 4 | Open flagged todos | `flagged --overdue` | 8/10 | Inbox-zero operator | `flag_status = 'flagged' AND complete_at IS NULL` with due-date diff. |
| 5 | Stale unread report | `stale-unread --days 14` | 8/10 | Inbox-zero operator | `is_read = 0 AND received_at < now - N` grouped by folder. |
| 6 | Waiting-on-me conversations | `waiting --days 3` | 8/10 | Follow-up tracker, inbox-zero operator | Window-fn over `messages` partitioned by `conversation_id` last-message; filter `from_email != me AND is_read = 0`. |
| 7 | Top conversations | `conversations --top 20 --window 30d` | 7/10 | Agent, inbox-zero operator | `GROUP BY conversation_id` ranked by message-count and unread-tail. |
| 8 | Quiet senders | `quiet --baseline 90d --silent 30d` | 7/10 | Sender-pruner, follow-up tracker | Self-join: ≥K messages in baseline, 0 in trailing. |
| 9 | Daily digest | `digest [--date YYYY-MM-DD]` | 7/10 | Inbox-zero operator, agent | Aggregations over date: counts, top senders, top conversations, focused/other ratio. |
| 10 | Stale attachments | `attachments stale --days 90 --min-mb 1` | 7/10 | Attachments-hygiene auditor | Join `attachments` (size captured at sync) with `messages.received_at`. |
| 11 | Likely duplicates | `dedup [--by conversation\|message-id\|subject-sender]` | 6/10 | Inbox-zero operator | `GROUP BY` on `internet_message_id`/`conversation_id`/(subject,from,to). |
| 12 | Plan-then-execute bulk archive | `bulk-archive --from-senders <file>` (`--execute`) | 6/10 | Sender-pruner | Resolves matching ids locally, prints `move` plan, executes via absorbed POST /move. Side-effect command: print by default, `cliutil.IsVerifyEnv()` short-circuit. |

## Stubs

None planned. Every absorbed and transcendence row above is shipping-scope.

## Killed candidates (audit trail)

| Feature | Kill reason |
|---------|-------------|
| categories report | Niche; `sql`/`search` builtins cover ad-hoc. |
| importance audit | `importance` rarely set on inbound MSA mail. |
| rules-suggest | "Good rule" is subjective. `senders`/`quiet` give the raw signal. |
| send-window | Speculative user pain; better as ad-hoc `sql`. |
