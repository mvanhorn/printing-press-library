# Zoho Desk CLI — Absorb Manifest

## Absorbed (matched from Zoho's MCP server + REST API)

127 endpoints from the official Zoho Desk OAS, covering:
- Tickets (CRUD, comments, threads, history, tags, attachments, bulk update, mass action, mark-as-spam, move-to-trash, merge, split, close, share)
- Contacts (CRUD, mass action, statistics, accounts mapping, ticket history)
- Accounts (CRUD, contact mapping, contracts, products, contacts under account)
- Agents (CRUD, signatures, presence, profile photo, scheduled reassignment)
- Departments (CRUD, agents-in-department, name-exists check)
- Organizations (list, get, update, logo/favicon)
- Tasks (CRUD, comments)
- Users (end users, help-center users, groups, labels)
- Search across all modules
- Statistics (counts, queue views, agent stats)
- Bulk operations (bulkUpdateAccounts, bulkUpdateContacts, bulkUpdateTickets, mergeAccounts, mergeContacts)

Plus generated automatically:
- OAuth2 refresh-token flow + multi-DC URL detection (.com / .eu / .in / .com.au / .com.cn)
- `--json`, `--select`, `--compact`, `--csv`, `--dry-run` on every command
- Local SQLite sync via `modifiedTime` cursor, FTS over searchable fields
- Rate-limit-aware retries with exponential backoff

## Transcendence (8 features scoring >= 6/10)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | SLA breach radar | `at-risk --within 4h --unassigned` | 9/10 | SQLite: tickets.dueDate vs now() + status filter + assigneeId IS NULL | Brief workflow #3; no native Zoho view combines "unassigned + breach window" |
| 2 | Aging triage | `stale --days 5 --status open` | 8/10 | SQLite: tickets WHERE max(threads.createdTime WHERE author=agent) < now() - N days | Brief priority #3; no competing helpdesk CLI ships this |
| 3 | Conversational FTS | `grep "TLS handshake" --in comments,threads` | 9/10 | SQLite FTS5 index over threads.content + comments.content + ticket subject/description | Brief thesis explicitly cites; Zoho search is keyword-only + rate-limited |
| 4 | Workload fairness | `workload --department "Support" --sort spread` | 8/10 | SQLite group-by: count open tickets per assigneeId, compute mean/stddev/Gini per department | Brief workflow #4; Zoho stats give totals not dispersion |
| 5 | Reopen-pattern detector | `reopens --min-count 2 --window 30d` | 7/10 | SQLite scan of ticket history events (status closed→open) grouped by ticketId | Brief data layer; persona SLA-risk signal |
| 6 | Reassignment helper | `suggest-agent <ticketId>` | 6/10 | SQLite query: agents in same dept, active, rank by ascending open-ticket count | Brief priority #3; team lead pain |
| 7 | Customer 360 view | `contact-history <email>` | 7/10 | SQLite join: contacts × tickets × accounts × ticket-stats | Persona B daily lookup; Zoho UI requires 3-click drill-down |
| 8 | Escalation audit | `escalations --min-reassigns 3` | 6/10 | SQLite scan of history where event=assigneeChange, group by ticketId, count distinct assignees | Persona A governance; not exposed as native Zoho report |

**Customer model:** Maya (Support Ops Mgr — SLA dashboards), Devon (Senior Agent — conversational recall), Priya (Team Lead — workload balance), Sam (RevOps/Integrations — nightly BQ sync). Full personas + 7 killed candidates documented in [`2026-05-15-novel-features-brainstorm-zoho-desk.md`](./2026-05-15-novel-features-brainstorm-zoho-desk.md).

## Killed candidates (audit trail)
- First-response leaderboard (vanity metric, low pain)
- Tag cloud (table-stakes, not transcendent)
- Sentiment trend (LLM dep — fails kill check)
- Daily digest mailer (scope creep — SMTP infra)
- Department backlog TUI (scope creep — descoped value in `stale` + `workload`)
- Auto-merge duplicates (destructive write — kept as read-only `dupes`, then folded into customer-history)
- Business-hours-aware due-date (per-org config gap in spec)
