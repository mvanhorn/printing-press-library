// pp:data-source live
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type undoResult struct {
	BatchID  string         `json:"batch_id,omitempty"`
	UndoOf   string         `json:"undo_of"`
	Status   string         `json:"status"`
	Would    map[string]int `json:"would,omitempty"`
	Counts   map[string]int `json:"counts,omitempty"`
	Warnings []string       `json:"warnings"`
	Failures []applyFailure `json:"failures"`
}

func newNovelUndoCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var yes, force, overwrite bool
	cmd := &cobra.Command{Use: "undo <batch-id>", Short: "Reverse successful operations in an applied batch", Long: "Move entries back and restore soft-deleted files from their journaled revisions. Created folders are left in place; revoked links cannot be restored.", Example: strings.Trim(`
  dropbox-pp-cli undo 20261006-153012-abcd --agent
  dropbox-pp-cli undo 20261006-153012-abcd --yes --agent`, "\n"), Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "batch-id=20261006-153012-abcd", "pp:verified-by-proof": "undo-lifecycle.md"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "undo")
		}
		if len(args) != 1 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("undo requires one batch id"))
		}
		if flags.dataSource != "" && flags.dataSource != "auto" && flags.dataSource != "live" {
			return usageErr(fmt.Errorf("undo requires live data source"))
		}
		if yes && cliutil.IsAnyHarness() {
			return refuseHarnessApply(cmd, flags, "undo")
		}
		if dbPath == "" {
			dbPath = defaultDBPath("dropbox-pp-cli")
		}
		ctx := cmd.Context()
		cancel := func() {}
		if cmd.Flags().Changed("timeout") {
			ctx, cancel = boundCtx(ctx, flags)
		}
		defer cancel()
		db, err := store.OpenWithContext(ctx, dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.EnsureDropboxSchema(ctx); err != nil {
			return err
		}
		original, ok, err := db.GetDropboxJournalBatch(ctx, args[0])
		if err != nil {
			return err
		}
		if !ok {
			return usageErr(fmt.Errorf("batch %q not found", args[0]))
		}
		if original.UndoOf != "" {
			return usageErr(fmt.Errorf("batch %q is an undo batch and cannot be undone", args[0]))
		}
		if original.Status == "undone" && !force {
			return usageErr(fmt.Errorf("batch %q is already undone; use --force to retry", args[0]))
		}
		if original.LegacyAccount && !force {
			return usageErr(fmt.Errorf("legacy journal has no account binding; use --force to accept the risk"))
		}
		ops, err := db.ListDropboxJournalOps(ctx, args[0])
		if err != nil {
			return err
		}
		result := undoResult{UndoOf: args[0], Would: map[string]int{}, Counts: map[string]int{}, Warnings: make([]string, 0), Failures: make([]applyFailure, 0)}
		if original.LegacyAccount {
			result.Warnings = append(result.Warnings, "legacy journal has no account binding; --force accepted the risk")
		}
		if at, err := time.Parse(time.RFC3339, original.CreatedAt); err == nil && original.RestoreDays > 0 && time.Since(at) > time.Duration(original.RestoreDays)*24*time.Hour {
			result.Warnings = append(result.Warnings, "Dropbox restore window may have expired")
			fmt.Fprintln(cmd.ErrOrStderr(), result.Warnings[0])
		}
		for _, op := range ops {
			if op.Result != "ok" && op.Result != "unknown" {
				continue
			}
			if op.UndoResult == "ok" && !force {
				continue
			}
			switch op.Op {
			case "move", "delete", "delete_child":
				result.Would[op.Op]++
			case "mkdir":
				result.Would["left_in_place"]++
			case "revoke_link":
				result.Would["not_reversible"]++
			}
		}
		if !yes {
			result.Status = "preview"
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		info, err := verifiedDropboxAccount(ctx, c, db)
		if err != nil {
			return err
		}
		if _, err := checkJournalAccount(original, info.AccountID, force); err != nil {
			return err
		}
		poster := dropboxBatchPoster{c, pathRootHeaders(info)}
		ops, err = reconcilePendingApply(ctx, db, poster, original.ID, info.AccountID, force)
		if err != nil {
			return err
		}
		if err := refreshDropboxIndex(ctx, cmd, c, db, pathRootHeaders(info)); err != nil {
			return err
		}
		result.BatchID = newBatchID()
		result.Status = "complete"
		if err := db.CreateDropboxJournalBatch(ctx, store.DropboxJournalBatch{ID: result.BatchID, CreatedAt: time.Now().UTC().Format(time.RFC3339), Source: "undo", Status: "running", AccountType: info.AccountType, AccountID: info.AccountID, RestoreDays: original.RestoreDays, UndoOf: original.ID}); err != nil {
			return err
		}
		err = executeUndoWithOptions(ctx, db, poster, ops, &result, overwrite)
		if err != nil {
			result.Status = "partial"
		}
		if setErr := db.SetDropboxJournalStatus(ctx, result.BatchID, result.Status); setErr != nil {
			return setErr
		}
		if err == nil {
			if setErr := db.SetDropboxJournalStatus(ctx, original.ID, "undone"); setErr != nil {
				return setErr
			}
		}
		if printErr := printJSONFiltered(cmd.OutOrStdout(), result, flags); printErr != nil {
			return printErr
		}
		return err
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().BoolVar(&yes, "yes", false, "Execute the undo")
	cmd.Flags().BoolVar(&force, "force", false, "Retry an already undone batch")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Allow restoring a deleted file over an occupied path")
	return cmd
}

func executeUndo(ctx context.Context, db *store.Store, poster dropboxBatchPoster, ops []store.DropboxJournalOp, result *undoResult, force bool) error {
	return executeUndoWithOptions(ctx, db, poster, ops, result, false)
}

func executeUndoWithOptions(ctx context.Context, db *store.Store, poster dropboxBatchPoster, ops []store.DropboxJournalOp, result *undoResult, overwrite bool) error {
	u := undoExecution{ctx: ctx, db: db, poster: poster, result: result, overwrite: overwrite, children: undoChildren(ops)}
	for _, op := range orderedUndoOps(ops) {
		if (op.Result != "ok" && op.Result != "unknown") || op.Op == "delete_child" {
			continue
		}
		if op.UndoResult == "ok" && !u.pendingChild(op) {
			continue
		}
		var err error
		switch op.Op {
		case "move":
			err = u.move(op)
		case "delete":
			err = u.delete(op)
		case "mkdir":
			err = u.record(op, "skipped", nil)
			result.Warnings = append(result.Warnings, "created folder left in place: "+op.Path)
		case "revoke_link":
			err = u.record(op, "skipped", nil)
			result.Warnings = append(result.Warnings, "revoked link is not reversible: "+op.URL)
		}
		if err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		return fmt.Errorf("%d undo operations failed", len(result.Failures))
	}
	return nil
}
func indexEntryForUndo(ctx context.Context, db *store.Store, p string) (dropbox.SnapshotEntry, bool, error) {
	var e dropbox.SnapshotEntry
	err := db.DB().QueryRowContext(ctx, `SELECT COALESCE(tag,''),COALESCE(id,'') FROM dbx_files WHERE path_lower=?`, strings.ToLower(p)).Scan(&e.Tag, &e.EntryID)
	if err == nil {
		return e, true, nil
	}
	if err == sql.ErrNoRows {
		return e, false, nil
	}
	return e, false, err
}
func undoMkdir(ctx context.Context, db *store.Store, p dropboxBatchPoster, folder string) error {
	raw, err := dropbox.RunBatchJob(ctx, p, "/files/create_folder_batch", "/files/create_folder_batch/check", map[string]any{"paths": []string{folder}, "autorename": false}, time.Second)
	if err != nil && !dropbox.HasSummaryPrefix(err, "path/conflict/folder") {
		return err
	}
	if err == nil {
		if entryErr := batchEntries(raw, 1)[0]; entryErr != nil && !folderConflictBatch(raw) {
			return entryErr
		}
	}
	if entry, found, err := indexEntryForUndo(ctx, db, folder); err != nil {
		return err
	} else if found && entry.Tag == "folder" {
		return nil
	}
	parent := ""
	if i := strings.LastIndex(folder, "/"); i > 0 {
		parent = strings.ToLower(folder[:i])
	}
	if err := db.UpsertDropboxEntries(ctx, dropbox.IndexRoot(folder), []store.DropboxRow{{PathLower: strings.ToLower(folder), PathDisplay: folder, ParentLower: parent, Name: folder[strings.LastIndex(folder, "/")+1:], Tag: "folder"}}); err != nil {
		return &undoIndexError{err}
	}
	return nil
}
func folderConflictBatch(raw json.RawMessage) bool {
	var response struct {
		Entries []struct {
			Tag     string `json:".tag"`
			Failure struct {
				Tag  string `json:".tag"`
				Path struct {
					Tag      string `json:".tag"`
					Conflict struct {
						Tag string `json:".tag"`
					} `json:"conflict"`
				} `json:"path"`
			} `json:"failure"`
		} `json:"entries"`
	}
	return json.Unmarshal(raw, &response) == nil && len(response.Entries) == 1 &&
		response.Entries[0].Tag == "failure" && response.Entries[0].Failure.Tag == "path" &&
		response.Entries[0].Failure.Path.Tag == "conflict" && response.Entries[0].Failure.Path.Conflict.Tag == "folder"
}

var errPathOccupied = errors.New("path occupied")

type undoIndexError struct{ err error }

func (e *undoIndexError) Error() string { return "index_error: " + e.err.Error() }
func (e *undoIndexError) Unwrap() error { return e.err }

func undoRestore(ctx context.Context, db *store.Store, p dropboxBatchPoster, op store.DropboxJournalOp, overwrite bool) error {
	if _, found, err := indexEntryForUndo(ctx, db, op.Path); err != nil {
		return err
	} else if found && !overwrite {
		return errPathOccupied
	}
	raw, err := p.Write(ctx, "/files/restore", map[string]string{"path": op.Path, "rev": op.Rev})
	if err != nil {
		return err
	}
	row, complete := restoredFileRow(raw, op)
	if err := db.UpsertDropboxEntries(ctx, dropbox.IndexRoot(row.PathLower), []store.DropboxRow{row}); err != nil {
		return &undoIndexError{err}
	}
	if !complete {
		// The row lacks size and hash, so local reports would undercount it
		// until the next index run repairs it.
		if err := db.SetDropboxMeta(context.WithoutCancel(ctx), "index_stale", "1"); err != nil {
			return &undoIndexError{err}
		}
	}
	return nil
}

// restoredFileRow builds the index row from the restore response. complete is
// false when the response did not carry usable file metadata.
func restoredFileRow(raw json.RawMessage, op store.DropboxJournalOp) (store.DropboxRow, bool) {
	var restored struct {
		dropbox.Entry
		SharingInfo struct {
			ParentSharedFolderID string `json:"parent_shared_folder_id"`
		} `json:"sharing_info"`
	}
	if err := json.Unmarshal(raw, &restored); err == nil && restored.PathLower != "" && restored.ID != "" && strings.EqualFold(restored.PathLower, op.Path) {
		e := restored.Entry
		e.Tag = "file"
		e.PathLower = strings.ToLower(e.PathLower)
		if e.PathDisplay == "" {
			e.PathDisplay = op.Path
		}
		if i := strings.LastIndex(e.PathLower, "/"); i > 0 {
			e.ParentLower = e.PathLower[:i]
		}
		e.ParentSharedFolderID = restored.SharingInfo.ParentSharedFolderID
		return dropboxEntryRow(e), true
	}
	parent := ""
	if i := strings.LastIndex(op.Path, "/"); i > 0 {
		parent = strings.ToLower(op.Path[:i])
	}
	return store.DropboxRow{PathLower: strings.ToLower(op.Path), PathDisplay: op.Path, ParentLower: parent, Name: op.Path[strings.LastIndex(op.Path, "/")+1:], Tag: "file", Rev: op.Rev}, false
}
