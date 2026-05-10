# Gmail CLI Novel Features Brainstorm

## Customer model

**Persona 1: Kavitha, Senior SRE at a fintech startup**

Today (without this CLI): Kavitha has a Gmail account that receives thousands of automated alerts, CI failure notifications, and vendor invoices per week. She uses web Gmail search to find specific threads, manually downloads PDF invoices from the browser one at a time, and has a cronjob that pipes `curl` calls against the Gmail API to a Python script. She has three browser tabs open: Gmail, the Gmail API reference, and a Python REPL. She cannot answer "show me all failed-build emails from last Monday between 2am and 4am" without a multi-step script.

Weekly ritual: Every Monday morning she queries Gmail for alerts from the weekend, archives batches of resolved ones, downloads any invoice attachments to a local folder for accounting, and verifies CI notifications were delivered.

Frustration: Bulk-downloading all attachments matching a search query requires writing a script from scratch every time — the API returns attachment IDs that require separate GET calls per message per attachment part, and saving them to a directory with sane filenames is 60 lines of Python.

---

**Persona 2: Marcus, Indie hacker running a one-person SaaS**

Today (without this CLI): Marcus's Gmail inbox has 40,000+ messages. He uses Gmail's web UI to manually unsubscribe from newsletters when they annoy him, has a "newsletters" label he applies inconsistently, and occasionally pipes `msmtp` to send transactional emails. He does not know which 5 senders account for 60% of his unread mail. He has no systematic way to find List-Unsubscribe links across multiple senders at once.

Weekly ritual: Friday cleanup — archive noise, unsubscribe from newsletters that leaked in during the week, reply to leads from his SaaS signup form.

Frustration: He can't see a ranked table of his top senders by volume and identify which ones have unsubscribe links — Gmail's UI shows one thread at a time, and there's no "sender report" view.

---

**Persona 3: Dana, Revenue Ops analyst at a mid-size B2B company**

Today (without this CLI): Dana's team uses Gmail for all customer communication. She needs to audit how many emails went to certain accounts in a date range for compliance, export threads to CSV for a spreadsheet, and verify that auto-forwarding and delegate rules are configured correctly across team accounts. She does this through Google Admin Console (slow) and manual export (once-a-month Takeout).

Weekly ritual: Pulls a weekly count of outbound emails per account, checks for any settings drift (vacation responder left on, forwarding changed), and exports any complaint-thread messages to a shared drive folder.

Frustration: She can't run a single command that tells her how many emails were sent to a given domain last week — she has to export all mail and count in a spreadsheet.

---

**Persona 4: Priya, Developer who automates personal workflows**

Today (without this CLI): Priya sends herself reminders via email, uses Gmail filters to sort things into labels, and writes Python scripts to parse her inbox for receipts. She has a local SQLite database she manually populates by running scripts. She cannot do full-text search across her entire mailbox body text without loading messages one at a time from the API (expensive, slow, rate-limited).

Weekly ritual: Sync new mail to local SQLite, run SQL queries across subject + body to find receipts, extract totals, write to a spreadsheet for expense reporting.

Frustration: A full initial sync of 80,000 messages takes hours using the API's `messages.list` + `messages.get` pattern, and she has no way to resume after a rate-limit interruption.

---

## Candidates (pre-cut)

[Full candidate list with pass/fail verdicts and scores documented in survivors table below]

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Bulk attachment export | `gmail attachments export --query "..." --dir ~/invoices` | 10/10 | Reads attachment metadata from local SQLite, then calls `gmail.users.messages.attachments.get` per ID, saves to --dir with `<date>-<sender>-<filename>` naming | Brief Workflow 3 explicitly names this; Gmail API requires separate attachment.get per ID making multi-step script the only prior art |
| 2 | Sender leaderboard | `gmail senders top --limit 20 --period 30d --unsubscribe` | 10/10 | SQL GROUP BY from_address ORDER BY count DESC on synced messages; joins with list_unsubscribe column parsed at sync time | Brief Workflow 4 explicitly names sender/newsletter analysis; no competitor MCP exposes this aggregation |
| 3 | Newsletter unsubscribe list | `gmail newsletters list` | 8/10 | Queries messages table for list_unsubscribe IS NOT NULL, groups by from_domain, returns sender + URL + count from local SQLite | Brief Workflow 4 names newsletter detection; List-Unsubscribe is RFC 2369 parseable at sync time |
| 4 | Inbox digest | `gmail digest --since yesterday --label IMPORTANT` | 10/10 | Joins threads + messages + message_labels in local SQLite, groups threads by label, returns from/subject/snippet/unread-count ordered by latest message date | Brief Workflow 1 names inbox triage as the daily ritual; brief explicitly lists digest as build priority |
| 5 | Attachment inventory | `gmail attachments list --type pdf --query "from:vendor@..."` | 9/10 | Queries local attachments table joined with messages; filters by MIME type / FTS query; returns filename/size/sender/date with zero API calls | Gmail MIME structure requires multi-step fetch making local inventory uniquely valuable; brief lists attachments in data layer |
| 6 | Sync status | `gmail stale` | 8/10 | Reads sync_state table (last sync timestamp, history token age, total synced count, last error) and reports freshness | Brief names resumable sync as build priority; silent token expiry is primary failure mode of history.list-based sync |
| 7 | Unread age histogram | `gmail inbox age --label INBOX` | 8/10 | SQL GROUP BY date_bucket on unread messages in local SQLite, bucketed into today/1-7d/8-30d/30-90d/90d+ with per-label breakdown | High-volume inboxes (100k+ messages per brief) make age distribution uniquely actionable; no live API call returns this aggregate |
| 8 | Outbound volume by domain | `gmail sent stats --to-domain acmecorp.com --period 7d` | 7/10 | Queries synced messages in SENT label, parses To-header domains, counts by date range in local SQLite | Dana's compliance audit workflow; no single API endpoint returns sent-volume-by-recipient-domain |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Sync with --resume (standalone) | Infrastructure behavior absorbed into sync command implementation | Stale / sync status (#6) |
| Label hierarchy audit | Fails weekly ritual test — one-off hygiene, not a repeating workflow | Newsletter list (#3) |
| Sender-to-label correlation | Fails weekly ritual test — useful once during filter setup | Sender leaderboard (#2) |
| Filter coverage audit | Fails weekly ritual test — set-and-forget hygiene | Sender leaderboard (#2) |
| Thread shape analyzer | Subsumed by C3; higher verifiability risk on participant-count metrics | Inbox digest (#4) |
| Thread response time by label | Narrow persona fit (Dana only); MIME Date parsing error-prone; displaced by outbound stats | Outbound volume (#8) |
| Attachment sender map | Merged as gmail attachments subcommand; standalone value covered by sender leaderboard + attachment inventory | Attachment inventory (#5) |
| Importance digest (standalone B1) | Merged into unified gmail digest command with --label flag | Inbox digest (#4) |
