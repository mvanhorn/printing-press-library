# Thunderbird (local profile) — Absorb Manifest

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Health/environment check | avikalpa/thunderbird-cli `tb doctor` | thunderbird-pp-cli doctor | Reports resolved profile, lock state, offline coverage per folder; no network |
| 2 | List profiles | avikalpa `tb mail profiles` | thunderbird-pp-cli profiles | Resolves [Install*] default correctly; --profile override |
| 3 | List accounts / identities | mudler thunderbird-mcp list_accounts | thunderbird-pp-cli accounts | From prefs.js, no creds read |
| 4 | List folders with counts | vitalio email_folders / mudler list_folders | thunderbird-pp-cli folders | Unread/total/size/offline-yes-no per folder, localized names |
| 5 | Account totals + unread | vitalio email_stats | thunderbird-pp-cli stats | Offline, --json |
| 6 | Hydrate/refresh local cache | avikalpa `tb mail fetch/sync` | thunderbird-pp-cli sync | Incremental mbox tail-parse by byte offset; snapshot-safe while TB runs |
| 7 | Keyword search across mail | avikalpa search/find/q, vitalio email_search, mudler search_messages | thunderbird-pp-cli search | Own FTS5 (gloda FTS unusable), filters from/to/folder/account/date/unread/has-attachment |
| 8 | Recent messages / folder listing | avikalpa recent/list/tail/head, vitalio email_list, mudler list_recent | thunderbird-pp-cli messages list | Paginated, --since, --folder, --unread, --select |
| 9 | Read a message | avikalpa `tb mail show`, vitalio email_read, mudler get_message | thunderbird-pp-cli messages show | Text body (html->text fallback), headers, attachments meta, --raw |
| 10 | Full thread | vitalio email_thread | thunderbird-pp-cli threads show | References/In-Reply-To reconstruction across folders incl. Sent |
| 11 | List/download attachments | vitalio email_attachments | thunderbird-pp-cli attachments list / attachments save | Decode from mbox offline; never writes to profile |
| 12 | Contacts search/get | mudler search_contacts/get_contact | thunderbird-pp-cli contacts search / contacts show | abook EAV flattened + gloda correspondents |
| 13 | Calendars / events list | mudler list_calendars/list_events | thunderbird-pp-cli calendar events | Local calendar-data sqlite, read-only |
| 14 | Export messages as .eml | lanterieur/thunderbird-sqlite-eml | thunderbird-pp-cli messages export | .eml / JSON / mbox slice from search results |
| 15 | Auth-header check (SPF/DKIM results) | avikalpa `tb mail authcheck` | thunderbird-pp-cli messages auth | Parses Authentication-Results headers offline |
| 16 | Send/compose/reply/forward/move/flag/delete | avikalpa, vitalio, mudler (IMAP/SMTP + decrypted key4.db) | (stub) out of scope — credential-bearing, read-only CLI by design | n/a |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Awaiting reply | awaiting-reply | hand-code | 9/10 | Joins reconstructed threads, all identities from prefs.js, localized Sent folders and the X-Mozilla-Status replied bit; no competitor has it | Use this command to find threads where a reply is pending, from you or (with --theirs) from the other side. Do NOT use it to list recent or unread mail; use 'messages list' instead. |
| 2 | Correspondents ranking | contacts top | hand-code | 9/10 | Aggregates messages per counterpart in both directions and joins address-book cards (flags frequent people missing from abook) | Use this command to rank people by how much and how recently you exchanged mail. Do NOT use it to look up a known person's card; use 'contacts search' / 'contacts show' instead. |
| 3 | Filters audit | filters audit | hand-code | 8/10 | Parses msgFilterRules.dat and evaluates supported header conditions against the local store: hits, disabled, missing target folder, duplicates, unevaluated | none |
| 4 | Largest items | largest | hand-code | 7/10 | Cross-account ranking of messages by mbox byte span or attachments by decoded size | Use this command to find the individual messages or attachments using the most space. Do NOT use it for per-folder totals; use 'folders' instead. |
| 5 | Newsletters / bulk senders | newsletters | hand-code | 6/10 | Groups by List-Id / List-Unsubscribe with unread share from X-Mozilla-Status and last-read date | Use this command for mailing lists and bulk senders ranked by volume and unread share. Do NOT use it for plain top-sender counts; use 'analytics --type messages --group-by from_addr' instead. |

### Stubs
- Row 16 (send/reply/forward/move/flag/delete): out of scope by design, not shipped as a command. Read-only CLI; would require decrypting key4.db/logins.json.

### Killed candidates (audit trail)
senders top (dup of analytics), volume (dup of analytics), coverage (in doctor/folders), tags (IMAP keywords only reliable in .msf/server), duplicates (speculative), junk review (junk score in Mork .msf), inbox digest (thin wrapper), identities usage (column of contacts top), since-sync (dup of messages list).

## Customer model (subagent Pass 1)
- P1 Terminal-first power user (2 IMAP accounts + Local Folders, ~900 MB, Italian folder names): finds past mail by sender/subject/attachment several times a week; frustration: search only inside GUI, no cross-account shell search, no view of threads owing a reply.
- P2 AI agent acting for the user: needs credential-free mbox ground truth; frustration: no "awaiting reply" or "correspondent history" primitive.
- P3 Inbox triager: weekly offline triage (top senders, unread, largest, stale); frustration: bulk mail across two accounts, never-read lists invisible.
- P4 Filter maintainer: monthly; frustration: can't see which rules match, are disabled, or target missing folders.
