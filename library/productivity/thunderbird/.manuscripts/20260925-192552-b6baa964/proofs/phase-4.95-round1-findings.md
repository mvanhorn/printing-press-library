# Phase 4.95: round 1 findings (paths relative to the module root)

## Correctness
- **C1. HIGH: compaction is not detected when the file size stays equal or grows** (tb_sync.go:469-476).
  - The sync state saves mtime but never reads it.
  - When mtime changed, verify the resume boundary before continuing: the bytes at LastOffset must be EOF, a blank line, or a "From " line. Also read back the last stored message at its offset and compare its Message-ID, or store a hash of the tail region.
  - On any mismatch, do a full re-parse of the folder.
  - In tbReadRaw (tb_messages.go:371-372), also reject when the stored message has no Message-ID and the bytes are not a plausible message start.
- **C2. MED-HIGH: a read error followed by a prune deletes the whole source.**
  - Affected paths: DiscoverFolders error per account (tb_sync.go:247-250, then prune 286-301); ReadAddressBook error (342-345, then prune 366); ParseFilterRules error (379-381, then prune 403).
  - Keep failed-account and failed-book sets and exclude them from pruning, following the events block pattern (409-432).
  - folders.go:101-102: a ReadDir error in one .sbd subfolder should not fail the whole account. Warn and skip only that subtree, and keep that subtree's paths out of the prune.
- **C3. MED: mime.ParseMediaType errors drop all params** (message.go:277-285).
  - Example that fails: `boundary=----=_Part_1` (unquoted).
  - Fall back to a manual split on ';' and recover boundary, charset and name, trimming quotes.
- **C4. MED: a message caught mid-write is stored truncated for good** (mbox.go:104-109, tb_sync.go:533).
  - For uncapped scans, set LastOffset to the start of the last emitted message's From line, so that message is re-read and upserted on every run (the id is stable).
- **C5. MED-LOW: an account key change over the same directory leaves duplicates.**
  - When `ok && st.FolderKey != key`, also prune messages and attachments with folder_key = st.FolderKey.
- **C6. LOW-MED: thread roots split when References is missing** (mbox.go:180-190, tb_sync.go:540-545).
  - Resolve roots transitively: look up the stored parent's thread_id through in_reply_to or the last reference, or union-find at sync end.
- **C7. LOW: declared utf-8 text with one invalid byte is decoded wholesale as windows-1252** (message.go:385-389, 76-82).
  - For a declared utf-8 charset, or data that is mostly valid UTF-8, use `strings.ToValidUTF8(data, "�")`.
- **C8. LOW: a cancelled or timed-out sync saves no checkpoint** (tb_sync.go:480-514).
  - Flush, then save LastOffset = resume without pruning, then return the error.
- **C9. LOW: prevBlank at resume starts as true** (mbox.go:66,84).
  - Seed it from the byte(s) before start, so incremental and full scans split messages identically.

## Security
- **S1. MED: MCP clients can choose the output dir** (`attachments save --out`, `messages export --out`).
  - cobratree blocks only o/output/db, and cobratree is generator-reserved, so do NOT edit it.
  - Rename the CLI flag to `--output` with shorthand `-o` in both commands, keeping `--out` as a hidden deprecated alias for CLI users.
  - Check that the MCP shell-out then blocks it.
  - Update README/SKILL/research.json examples that use --out.
- **S2. MED: control characters from email reach the terminal in human output.**
  - Affected: messages_get.go:136-160, tb_messages.go:228-230, tb_attachments.go:91, tb_drafts.go:518-529, and any other human table of mail-derived fields.
  - Add one sanitizer: replace C0 except \n and \t, plus DEL and C1 U+0080-U+009F, with U+FFFD. Apply it in human mode only; JSON is already escaped.
- **S3. LOW: quadratic header folding plus no header cap** (message.go:110).
  - Collect continuation parts and join them once. Cap the header block at 1 MB, and treat anything beyond it as body.
- **S4. LOW: collision rename can overwrite within one save** (tb_attachments.go:237-241).
  - Loop until the name is not in `used`.
- **S5. LOW: exported/saved mail uses 0755/0644 permissions** (tb_attachments.go:172,176, tb_messages_export.go:113,117).
  - Use 0700/0600.
- **S6. LOW: `sync` and `workflow archive` are annotated mcp:read-only=true but write the store** (sync.go:74, tb_local_only.go:264).
  - Use mcp:local-write=true instead, matching cobratree/classify.go labels.
- **S7. LOW: walkParts copies nested parts at every level** (message.go:293-299).
  - Use sub-slices of the original body instead of io.ReadAll per level, or cap total bytes per message.

## Maintainability
- **M1. MED: hand-written helpers live in files with a "Generated ... DO NOT EDIT" header.**
  - Examples: tbAccountMatches (promoted_folders.go:15-19); tbAccountRow and tbAccountRowFromPrefs (promoted_accounts.go:16-40); messages_list.go:34-75; messages_get.go; tbResourceTypes (sync.go:1665-1671).
  - Move the shared helpers into hand-owned tb_*.go files.
  - For command bodies that must stay in generated files, prefer the tbReplaceCommand override pattern already used in tb_local_only.go:61 (so the generated file can go back to pristine), or record them in .printing-press-patches/ as AGENTS.md:88 requires.
- **M2. LOW: time-window flags are inconsistent.** awaiting-reply and filters audit use --days, the other commands use --since with tbParseTimeBound.
  - Add --since to both, with defaults 14d and 90d, and keep --days as a hidden alias.
- **M3. LOW: runTBSync is about 280 lines with 5 repeated add/flush/prune blocks.**
  - Extract a tbReplaceResource helper and per-resource functions. Do this together with C2.
- **M4. LOW: two meanings of "sent by me".**
  - threads show (tb_threads.go:78) and tbGetMessage (tb_messages.go:129) use `outgoing`; the novel commands use the folder-based sentByMe. Make threads show use the folder-based direction.
  - Share one tbOwnAddresses helper, which tb_drafts.go:302-318 duplicates.
- **M5. LOW: tbScopeWhere (largest.go:41-53) duplicates messages_list.go:45-51, and the two have already drifted.** Reuse tbScopeWhere.
- **M6. LOW: filters_audit.go:237-253 and tbThreadSizes duplicate the IN-clause building.** Add tbInClause, and a withBody option for tbQueryMessages.
- **M7. LOW: small duplications.** Add tbRecipients(d). Add tbContactName(c) and remove tbFirstNonEmpty. Use tbSender in tb_drafts.go:361-364.
- **M8. LOW: two redundancies.** tbLoadMailbox checks `f.SentFolder || IsSentFolderName(...)`: use only f.SentFolder, but keep the query-time fallback if older stores lack the field. tbAccountRef duplicates tbAccountRow.
- **M9. LOW: comments break the one-line rule.**
  - Multi-line: tb_novel.go:22-24, 80-81; filters_audit.go:72-73; tb_sync.go:453-455; tb_messages.go:72-73.
  - Name-restating: tb_novel.go:113; tb_messages.go:141,161; largest.go:40.
