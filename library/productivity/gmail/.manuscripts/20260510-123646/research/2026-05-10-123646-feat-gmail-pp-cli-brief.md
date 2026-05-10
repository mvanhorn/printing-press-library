# Gmail CLI Brief

## API Identity
- Domain: Email / Google Workspace
- Users: Developers, power users, SREs, revenue ops teams — anyone managing Gmail programmatically or wanting terminal-first inbox workflows
- Data profile: Messages (with full MIME bodies + attachments), threads, drafts, labels, settings, history delta. High volume — production inboxes hold 100k+ messages. Key entities: message, thread, label, attachment, filter, draft, history event.

## Reachability Risk
- None. Gmail API is one of Google's flagship REST APIs; fully reachable with OAuth2 token. No bot-protection, no scraping needed.

## Top Workflows
1. **Inbox triage**: search unread messages, archive/label/reply in bulk — the daily GTD ritual
2. **Send/draft programmatically**: shell-based email automation (CI notifications, alerts, templated outreach)
3. **Attachment extraction**: download all attachments matching a query to a local directory (invoices, reports, receipts)
4. **Sender/newsletter analysis**: understand who floods your inbox, find unsubscribe candidates, clean up subscriptions
5. **Sync + offline search**: pull messages into local SQLite, then FTS-query, regex-search, or SQL-aggregate — impossible with live API alone

## Table Stakes (must match every competitor)
- messages list / get / send / delete / trash / modify / batch
- threads list / get / modify / delete / trash
- drafts create / list / get / send / delete
- labels CRUD
- settings: vacation, IMAP, POP, forwarding, filters, delegates, sendAs
- history list (delta feed)
- profile get
- attachment download
- search with Gmail query syntax (from:, to:, subject:, has:attachment, after:, label:, etc.)
- --json output, --dry-run, --agent flag
- OAuth2 browser auth flow with token storage

## Data Layer
- Primary entities: messages, threads, labels, drafts
- Sync strategy: history.list delta (token-based) after full initial sync
- FTS: sqlite fts5 on subject + from + snippet + body_text
- Attachments: metadata in DB, optional download to local dir
- Key for novel features: message headers (List-Unsubscribe, From, To, Date), parsed body_text for search, attachment filenames

## Product Thesis
- Name: gmail-pp-cli
- Why it should exist: Every Gmail MCP tool forces live API calls. Our CLI syncs your mailbox to SQLite and lets you query it like a database — FTS search in milliseconds, sender analytics, newsletter detection, attachment export pipelines, inbox digests — all offline after one sync. Plus the full live API surface for sending, labeling, and managing settings.

## Build Priorities
1. OAuth2 auth flow (browser, device code, or token import) + token storage + refresh
2. Full sync to SQLite (initial + incremental via history.list)
3. FTS5 search across subjects, senders, body snippets
4. Novel commands: senders stats, newsletters, attachments export, inbox digest
5. Full API surface: messages, threads, drafts, labels, settings
6. --agent mode + typed exit codes throughout
