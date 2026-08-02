# Gmail CLI Competitors — Research Data (agent 1)

## Tools
1. gmailctl (mbrt/gmailctl, ~2.2k, Go) — filters-as-code: init, apply, diff, download, edit, export, debug, test. Jsonnet config. Pain: no retroactive filter application, 7-day testing-mode token expiry, 1500-char filter limit.
2. GAM7 (GAM-team/GAM ~4.2k, Python) — Workspace admin: bulk show/print/modify/trash/delete messages+threads (CSV-driven), labels CRUD incl nested, filters CRUD, delegates, sendas/signature, forwarding, IMAP/POP, vacation, language, export. Pain: 3-credential setup, DWD, Workspace-only.
3. GYB (GAM-team/got-your-back ~3.1k, Python) — backup to mbox/maildir + SQLite label index; incremental; restore, restore-mbox, count, estimate, purge, purge-labels, quota; --search scoped. Pain: slow per-message fetch.
4. lieer/gmi (gauteh/lieer ~648, Python) — gmail<->notmuch sync: init, pull, push, sync, send (sendmail-compatible via API), set, auth. Pain: reserved labels read-only, one-of inbox/spam/trash, muted msgs unsyncable.
5. himalaya (pimalaya ~6.8k, Rust) — unified email CLI, has Gmail REST backend: envelopes list/search/sort, flags, read/write/reply/forward/copy/move/delete/send, attachments. Pain: v2 OAuth regression (external helper), keyring removed.
6. gws (googleworkspace/cli ~30.1k, Rust) — THE momentum leader; user has it installed. Helpers: +send, +reply, +reply-all (auto-threading), +forward, +triage (unread summary), +read (body extraction), +watch (NDJSON stream); full Discovery surface; 100+ agent skills incl gws-gmail SKILL.md; auth setup/login, keyring, service account, headless. Pain: pre-1.0 breaking changes, ~25-scope unverified cap.
7. ezgmail (~272, Python) — send/unread/recent/search/trash/markAsRead/Unread/addLabel/removeLabel/downloadAttachment(s)/summary.
8. simplegmail (~407, Python) — send HTML/plain/attachments/cc/bcc/signature, search, attachment download, star/unstar, archive, labels. Pain: label IDs not names.
9. npm misc: gmail_cli (tiny), gmail-tester (E2E inbox polling), leon123858/gmail-cli (Go, multi-account config, 4 stars). No dominant npm Gmail CLI.
10. Claude plugins: feedtailor/ccskill-gmail (GAS-backed: search/read/draft/labels/trash/archive/star/attachments/body->PDF/multi-account/audit log); WadeWarren/gws-claude-plugin (35 stars, wraps gws, 92 skills); raf.dev blog pattern (inbox/search/read/labels/archive/batch-read/top-senders/send, --json; batchModify 1000/call essential).
IMAP generic: aerc (Go), neomutt (+notmuch/mbsync — lieer replaces this niche).

## Top workflows
1. Filters-as-code (diff/apply, versioned)
2. Inbox-zero triage via agents (unread summary, batch archive, top-senders, batchModify)
3. Bulk cleanup / mass unsubscribe (List-Unsubscribe automation)
4. Backup/export/migration (GYB mbox)
5. Offline sync + local search (lieer/notmuch niche)

## API pain points
1. OAuth setup friction (GCP project, consent screen, testing-mode 7-day token expiry, restricted-scope CASA verification)
2. Quota units (batchModify 1000 IDs/call is the workaround; May 2026: 15,000 units/user/min for new projects)
3. Data model quirks: base64url nested MIME, threads-vs-messages duality, label IDs vs names, system labels read-only, 1500-char filters

## Risks
None API-side (stable official). Policy-side: restricted-scope verification, testing-mode expiry. gws pre-1.0.
