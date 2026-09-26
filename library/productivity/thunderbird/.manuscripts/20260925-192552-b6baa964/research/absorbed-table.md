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
