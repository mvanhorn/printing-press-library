# Thunderbird (local profile) CLI Brief

## API Identity
- Domain: Mozilla Thunderbird desktop mail client — **on-disk profile**, not a network API (user choice: "disco").
- Users: people/agents who want to search, read, audit and export their mail, contacts and filters without launching the GUI and without credentials.
- Data profile (probed on the operator machine, counts only):
  - `profiles.ini` + `installs.ini`: the active profile is the `[Install*] Default=` entry; a `[ProfileN] Default=1` may point at an empty profile. Resolve Install first, then Profile Default=1, then `--profile` / `THUNDERBIRD_PROFILE` override.
  - mbox stores under `ImapMail/<host>/` and `Mail/Local Folders/` (localized folder names with spaces, e.g. `Posta inviata`, `Cestino`; some folders only have `.msf` = not offline-synced). ~900 MB across 2 IMAP accounts. **Ground truth for message content.** `X-Mozilla-Status` / `X-Mozilla-Status2` headers carry read/flagged/replied/expunged (0x0008 => skip).
  - `global-messages-db.sqlite` (gloda): tables messages, conversations, contacts, identities, folderLocations, messageAttributes; `messagesText` is FTS3 with the `mozporter` tokenizer => virtual table unusable outside Thunderbird, but `messagesText_content` (body, subject, attachmentNames, author, recipients) is plain. Incomplete (3202 text rows vs 3941 messages) => accelerator only.
  - `abook.sqlite`: EAV `properties(card,name,value)` + `lists`, `list_cards`.
  - `calendar-data/local.sqlite`: cal_events/cal_todos (empty on operator machine; low priority).
  - `prefs.js`: `mail.account.*`, `mail.identity.*`, `mail.server.*` => accounts, identities, server dirs.
  - `ImapMail/<host>/msgFilterRules.dat`: message filter rules (plain text).
  - Thunderbird runs concurrently (`Locked=1`); SQLite files are WAL. Read by copying db+wal+shm to a temp dir, or read-only open; never `immutable=1`; **never write inside the profile**.

## Reachability Risk
- None for network. Local risk: profile lock/WAL (mitigated by snapshot copy), large mbox (stream parse, byte offsets), missing offline stores (report, don't error).

## Top Workflows
1. "Find that mail": full-text search across all accounts/folders (subject/from/to/body/attachment names) with date/folder/account filters => ids, then `show`.
2. Read a message or a whole thread from the terminal (headers, text body, attachment list), and extract an attachment to disk.
3. Inbox triage/analytics offline: top senders, volume by month, unread counts per folder, largest messages/attachments, stale threads awaiting reply.
4. Contacts: search the address book + gloda-derived correspondents (who emails me most / last contact date).
5. Export: message(s) to .eml / JSON / CSV for agents and archiving; filter rules audit.

## Table Stakes (from competitors)
- avikalpa/thunderbird-cli (Go): profiles, doctor, fetch/sync cache (SQLite), search/find/q, show, recent/list/tail/head, folders; compose/send/reply/move via IMAP/SMTP with NSS-decrypted creds.
- mudler/thunderbird-mcp (Go): list_accounts, list_folders, search_messages (gloda), get_message, list_recent, contacts search/get, calendars list/events; writes/send via IMAP gated by env.
- TKasperczyk / U-C4N thunderbird-mcp: drive a running Thunderbird via extension bridge (112 tools).
- lanterieur/thunderbird-sqlite-eml: export gloda to .eml.

## Out of scope (deliberate)
- Send / reply / move / flag / delete over IMAP/SMTP: requires decrypting `key4.db`/`logins.json` credentials and touches remote state. Listed in absorb manifest as "out of scope (credential-bearing)". CLI is strictly read-only against the profile.

## Data Layer
- Primary entities: accounts, folders, messages (header fields + status flags + mbox offset), threads (by References/In-Reply-To, fallback gloda conversationID), attachments (metadata), contacts, filters.
- Sync cursor: per-mbox file size + mtime + last byte offset (append-only mbox => incremental tail parse; rescan on shrink/compaction).
- FTS/search: own FTS5 table in the CLI store (modernc sqlite, pure Go, no CGO — confirmed in generated go.mod) over subject/from/to/body text/attachment names.

## Product Thesis
- Name: thunderbird-pp-cli
- Why it should exist: agent-native, credential-free, read-only window onto the mail Thunderbird already downloaded — incremental local index with FTS5, thread reconstruction and offline analytics that no competitor ships together, safe to run while Thunderbird is open.

## Build Priorities
1. Profile resolution + `doctor` (profile, accounts, folders, offline coverage, lock state).
2. `sync`: incremental mbox ingest (headers, flags, text body, attachment meta) + abook + filters into the store; gloda as optional enrichment.
3. `search` (FTS5 + filters), `messages list/show`, `threads show`, `attachments list/save`, `export` (.eml/json).
4. `accounts`, `folders` (counts, unread, size, offline yes/no), `contacts search`.
5. Novel analytics: `stats senders|volume|folders`, `awaiting-reply`, `largest`, `filters audit`.
- Generator scaffolding (store, output formats, MCP, learn loop) via internal spec `source: local-sqlite`; HTTP endpoint commands are replaced by hand-built local implementations in Phase 11.

## Privacy plan
- Tests/examples/README use a synthetic fixture profile (tiny mbox with X-Mozilla-Status, abook, gloda-shaped db). Real-profile dogfood proofs record exit codes and counts only — no subjects, bodies or addresses.

## Reachability Gate
- Decision: PASS (carve-out)
- Reason: local-on-disk-profile-no-http-origin
- Evidence: default profile resolved from installs.ini/profiles.ini; prefs.js, abook.sqlite and ImapMail mbox stores readable; thunderbird.exe present for drafts compose handoff.
