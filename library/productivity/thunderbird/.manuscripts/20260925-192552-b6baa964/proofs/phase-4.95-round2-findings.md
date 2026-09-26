# Phase 4.95: round 2 findings (paths relative to the module root)

## Correctness
- **R2-C1. MED-HIGH: the dead-mbox prune deletes rows that were just re-ingested** (tb_sync.go:410-421).
  - How it happens: message ids do not depend on the path, while mbox states are keyed by path. When the same folder is reached through a different path spelling, the old state counts as dead, and `pruneFolderKey(st.FolderKey)` wipes the fresh rows. Examples: `--profile .\`, different case, or a moved directory.
  - Fix, part 1: restrict the dead-state prune to rows with `folder_key = st.FolderKey AND json_extract(data,'$.mbox_path') = st.Path`, and delete attachments by the message_ids of those rows. Alternatively, skip the prune when st.FolderKey is live in this run. In either case, still call DeleteMboxState.
  - Fix, part 2: normalise the profile dir with filepath.Abs in tbprofile.Resolve (profile.go:165).
  - Test: sync with an absolute path, then with a relative or differently spelled path. The message count must stay unchanged.
- **R2-C2. LOW: the key-change prune (C5, tb_sync.go:592-596) can delete another directory's fresh rows.** Use the same mbox_path-restricted prune.
- **R2-C3. LOW: keepPrefix mixes up bytes and characters** (tb_sync.go:236).
  - Go `len(prefix)` counts bytes, but SQLite `substr` counts characters, so non-ASCII subtree names are not protected.
  - Fix: use `instr(id, ?) = 1` or filter in Go. Test with a fictional non-ASCII .sbd name.
- **R2-C4. LOW: the lastID re-read prune is skipped on the cancelled and capped branches** (tb_sync.go:670-702).
  - Fix: run the lastID prune before the switch whenever `lastID != ""` and the re-read message was processed (`lastStart >= startAt`).
- **R2-C5. LOW: a cancelled or capped reparse of a known folder can leave a checkpoint that passes validation even though it is stale** (tb_sync.go:673-675).
  - Fix: save a state that forces a reparse on the next run, for example `Size: math.MaxInt64`, for both cancelled and capped reparses.
- **R2-C6. LOW: the re-read prune can delete a row shared by two copies with the same Message-ID** (the later copy expunged in place).
  - Fix: when the re-read message is expunged, fall back to a folder reparse instead of pruning lastID.

## Security
- **R2-S1. LOW: tbSafeFilename (tb_attachments.go:264-271) maps only C0 and DEL, not C1** (0x80-0x9f).
  - Effect: C1 characters reach the terminal through the attachments save dry-run/saved lines (:320-324) and through the clash error (:170), and also end up in the name on disk.
  - Fix: map r in 0x7f-0x9f to `_`.
  - Test: add an attachments save case to TestTBHumanOutputSanitized.
- **R2-S2. LOW: build/stage/bin binaries predate round 1.**
  - Fix: rebuild both the CLI and MCP staged binaries at the end (go build -o build/stage/bin/...), or delete build/stage.
- **R2-S3. LOW-MED: drafts new --body-file / --attach are reachable over MCP.**
  - Arbitrary local files can be read back into the JSON result, or attached to a compose window when --open is used.
  - Fix: `cmd.Flags().MarkHidden("body-file")` and `MarkHidden("attach")`. cobratree drops hidden flags from the tool schema and from the allowed args, so MCP rejects them.
  - Keep both flags working from the CLI and documented in Long/Example. Check that verify-skill and README/SKILL examples still pass.
  - Test through the cobratree handlers, as in the S1 test.
- **R2-S4. LOW, docs only: --csv and --plain output (generated helpers.go) carries raw C0 and C1 characters, not only C1.**
  - Correct the residual-risk note in the build log.
  - Record it as a generator retro candidate. Do NOT patch helpers.go.

## Maintainability
- **R2-M1. MED: the human-output sanitizer is applied per field and is already inconsistent.**
  - Raw fields: tb_filters.go:139 (AccountName, Name, Target), filters_audit.go:258 (target), tb_stats.go:74 (AccountName), tb_contacts.go:96 (c.Book).
  - Fix: sanitize once at the sink. Wrap the human-mode writer in a `tbSafeWriter` that applies the tbSafe rune map, keeping tab and newline. Remove the per-field tbSafe calls, and keep tbTrunc for truncation only.
- **R2-M2. LOW: ingestFolder (tb_sync.go:579-711) has grown too much.**
  - Return a tbFolderResult struct.
  - Add one pruneMsgs helper to replace the 3 prune variants (this ties in with R2-C1/C2/C4).
  - Save the state in one place.
- **R2-M3. LOW: tbMessageRow.Outgoing (tb_messages.go:36,49) is address-based and contradicts `direction` in the same JSON.**
  - Drop `outgoing` from tbMessageRow and keep it in the stored doc.
- **R2-M4. LOW: two places build the own-address set and the account fields.**
  - tbIdentityEmails (tb_sync.go:369) duplicates tbOwnAddresses (tb_accounts.go:42).
  - syncAccounts (tb_sync.go:311-332) duplicates tbAccountRowFromPrefs.
  - Unify both.
- **R2-M5. LOW: the book id is derived twice** (tb_sync.go:480 and tbprofile/sqlite.go:77). Export tbprofile.BookName(path) and use it in both places.
- **R2-M6. LOW: exported mutable global `var ReadDir = os.ReadDir`** (tbprofile/folders.go:40-41). Pass the directory reader to DiscoverFolders as a parameter or option instead.
- **R2-M7. LOW: two-line comment at tb_message_detail.go:29-30.** Reduce it to one line.
