package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

type undoExecution struct {
	ctx       context.Context
	db        *store.Store
	poster    dropboxBatchPoster
	result    *undoResult
	overwrite bool
	seq       int
	children  map[string][]store.DropboxJournalOp
}

func undoChildren(ops []store.DropboxJournalOp) map[string][]store.DropboxJournalOp {
	parents := map[string]string{}
	for _, op := range ops {
		if op.Op == "delete" && (op.Result == "ok" || op.Result == "unknown") {
			parents[strings.ToLower(op.Path)] = op.Path
		}
	}
	children := map[string][]store.DropboxJournalOp{}
	for _, op := range ops {
		if op.Op != "delete_child" || (op.Result != "ok" && op.Result != "unknown") {
			continue
		}
		for p, _ := dropbox.ParentBase(strings.ToLower(op.Path)); p != ""; p, _ = dropbox.ParentBase(p) {
			if parent, ok := parents[p]; ok {
				children[parent] = append(children[parent], op)
				break
			}
		}
	}
	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool { return children[parent][i].Seq > children[parent][j].Seq })
	}
	return children
}

// childRestoreOrder recreates descendant folders parent-first, then restores
// files in journal order.
func childRestoreOrder(children []store.DropboxJournalOp) []store.DropboxJournalOp {
	ordered := make([]store.DropboxJournalOp, 0, len(children))
	for _, child := range children {
		if child.Tag == "folder" {
			ordered = append(ordered, child)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return strings.ToLower(ordered[i].Path) < strings.ToLower(ordered[j].Path) })
	for _, child := range children {
		if child.Tag != "folder" {
			ordered = append(ordered, child)
		}
	}
	return ordered
}
func undoPriority(op string) int {
	switch op {
	case "revoke_link":
		return 0
	case "delete":
		return 1
	case "move":
		return 2
	case "mkdir":
		return 3
	default:
		return 4
	}
}
func orderedUndoOps(ops []store.DropboxJournalOp) []store.DropboxJournalOp {
	ordered := append([]store.DropboxJournalOp(nil), ops...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := undoPriority(ordered[i].Op), undoPriority(ordered[j].Op)
		if a != b {
			return a < b
		}
		return ordered[i].Seq > ordered[j].Seq
	})
	return ordered
}
func (u *undoExecution) record(op store.DropboxJournalOp, status string, detail error) error {
	u.seq++
	return u.settle(op, u.seq, status, detail, true)
}

// begin journals a pending reverse operation before its request is sent, so a
// crash or poll failure after Dropbox accepts it still leaves a record.
func (u *undoExecution) begin(op store.DropboxJournalOp) (int, error) {
	u.seq++
	if err := u.db.AddDropboxJournalOp(u.ctx, store.DropboxJournalOp{BatchID: u.result.BatchID, Seq: u.seq, Op: op.Op, Path: op.Path,
		FromPath: op.ToPath, ToPath: op.FromPath, Rev: op.Rev, URL: op.URL, Tag: op.Tag, EntryID: op.EntryID, Result: "pending"}); err != nil {
		return 0, err
	}
	return u.seq, u.db.SetDropboxJournalUndoResult(u.ctx, op.BatchID, op.Seq, "pending", "")
}

func (u *undoExecution) settle(op store.DropboxJournalOp, seq int, status string, detail error, insert bool) error {
	message := ""
	if detail != nil {
		message = detail.Error()
	}
	benign := status == "skipped" && (detail == nil || strings.Contains(message, "unknown move did not occur") ||
		strings.Contains(message, "unknown delete did not occur") || strings.Contains(message, "already restored by parent") ||
		strings.Contains(message, "undo already completed"))
	if benign {
		if message != "" {
			u.result.Warnings = append(u.result.Warnings, message)
		}
	} else if detail != nil {
		u.result.Failures = append(u.result.Failures, applyFailure{Seq: op.Seq, Op: op.Op, Error: message})
	}
	u.result.Counts[status]++
	if insert {
		if err := u.db.AddDropboxJournalOp(u.ctx, store.DropboxJournalOp{BatchID: u.result.BatchID, Seq: seq, Op: op.Op, Path: op.Path,
			FromPath: op.ToPath, ToPath: op.FromPath, Rev: op.Rev, URL: op.URL, Tag: op.Tag, EntryID: op.EntryID, Result: status, Error: message}); err != nil {
			return err
		}
	} else if err := u.db.SetDropboxJournalOpResult(u.ctx, u.result.BatchID, seq, status, message); err != nil {
		return err
	}
	undoState := status
	if benign {
		undoState = "ok"
	}
	return u.db.SetDropboxJournalUndoResult(u.ctx, op.BatchID, op.Seq, undoState, message)
}

// priorUndoAttempted reports that an earlier undo sent this reversal but never
// recorded its outcome.
func priorUndoAttempted(op store.DropboxJournalOp) bool {
	return op.UndoResult == "pending" || op.UndoResult == "unknown"
}

// resolvePriorUndoMove polls the job an interrupted undo saved. done is true
// when that job already moved the entry back.
func (u *undoExecution) resolvePriorUndoMove(op store.DropboxJournalOp) (done bool, err error) {
	if !priorUndoAttempted(op) || op.UndoJobID == "" {
		return false, nil
	}
	raw, pollErr := pollExistingJob(u.ctx, u.poster, "/files/move_batch/check_v2", op.UndoJobID)
	if pollErr != nil {
		var failed *dropbox.BatchFailedError
		if errors.As(pollErr, &failed) || asyncJobGone(pollErr) {
			return false, nil
		}
		return false, fmt.Errorf("earlier undo job %s is unresolved: %w", op.UndoJobID, pollErr)
	}
	return batchEntries(raw, 1)[0] == nil, nil
}
func (u *undoExecution) classify(err error) (string, error) {
	if err == nil {
		return "ok", nil
	}
	var indexErr *undoIndexError
	if errors.As(err, &indexErr) {
		if markErr := u.db.SetDropboxMeta(context.WithoutCancel(u.ctx), "index_stale", "1"); markErr != nil {
			err = fmt.Errorf("%w; stale marker failed: %v", err, markErr)
		}
		return "ok", err
	}
	if errors.Is(err, errPathOccupied) {
		return "skipped", err
	}
	return "failed", err
}
func (u *undoExecution) pendingChild(op store.DropboxJournalOp) bool {
	if op.Op != "delete" || op.Tag != "folder" {
		return false
	}
	for _, child := range u.children[op.Path] {
		if child.UndoResult != "ok" {
			return true
		}
	}
	return false
}
func (u *undoExecution) move(op store.DropboxJournalOp) error {
	if done, err := u.resolvePriorUndoMove(op); err != nil {
		return u.record(op, "unknown", err)
	} else if done {
		// Only rewrite the row that the earlier undo moved. A path reused by
		// another file keeps its row; an unconfirmable identity marks the
		// index stale so the next refresh settles it.
		if current, atTo, lookupErr := indexEntryForUndo(u.ctx, u.db, op.ToPath); lookupErr != nil {
			return lookupErr
		} else if atTo && op.EntryID != "" && current.EntryID == op.EntryID {
			if indexErr := u.db.MoveDropboxPathPrefix(u.ctx, op.ToPath, op.FromPath); indexErr != nil {
				status, detail := u.classify(&undoIndexError{indexErr})
				return u.record(op, status, detail)
			}
		} else if atTo && (op.EntryID == "" || current.EntryID == "") {
			if err := u.db.SetDropboxMeta(context.WithoutCancel(u.ctx), "index_stale", "1"); err != nil {
				return err
			}
		}
		return u.record(op, "skipped", fmt.Errorf("undo already completed by an earlier attempt"))
	}
	entry, found, err := indexEntryForUndo(u.ctx, u.db, op.ToPath)
	if err != nil {
		return err
	}
	if !found || op.EntryID == "" || entry.EntryID != op.EntryID {
		if priorUndoAttempted(op) && op.EntryID != "" {
			original, backAtFrom, lookupErr := indexEntryForUndo(u.ctx, u.db, op.FromPath)
			if lookupErr != nil {
				return lookupErr
			}
			if backAtFrom && original.EntryID == op.EntryID {
				return u.record(op, "skipped", fmt.Errorf("undo already completed by an earlier attempt"))
			}
		}
		if op.Result == "unknown" && op.EntryID != "" {
			original, stillAtFrom, lookupErr := indexEntryForUndo(u.ctx, u.db, op.FromPath)
			if lookupErr != nil {
				return lookupErr
			}
			if stillAtFrom && original.EntryID == op.EntryID {
				return u.record(op, "skipped", fmt.Errorf("unknown move did not occur"))
			}
		}
		return u.record(op, "skipped", fmt.Errorf("destination entry_id differs from journal"))
	}
	caseOnly := strings.EqualFold(op.FromPath, op.ToPath) && op.FromPath != op.ToPath
	if _, occupied, err := indexEntryForUndo(u.ctx, u.db, op.FromPath); err != nil {
		return err
	} else if occupied && !caseOnly {
		return u.record(op, "skipped", fmt.Errorf("path occupied"))
	}
	seq, err := u.begin(op)
	if err != nil {
		return err
	}
	ambiguous := false
	if caseOnly {
		_, err = u.poster.Write(u.ctx, "/files/move_v2", map[string]any{"from_path": op.ToPath, "to_path": op.FromPath, "autorename": false})
	} else {
		body := map[string]any{"entries": []map[string]string{{"from_path": op.ToPath, "to_path": op.FromPath}}, "autorename": false}
		var raw json.RawMessage
		jobSaved := false
		raw, err = dropbox.RunBatchJobObserved(u.ctx, u.poster, "/files/move_batch_v2", "/files/move_batch/check_v2", body, time.Second, func(id string) error {
			if err := u.db.SetDropboxJournalAsyncJobID(u.ctx, u.result.BatchID, seq, id, 0); err != nil {
				return err
			}
			jobSaved = true
			return u.db.SetDropboxJournalUndoJobID(u.ctx, op.BatchID, op.Seq, id)
		})
		var failed *dropbox.BatchFailedError
		ambiguous = err != nil && jobSaved && !errors.As(err, &failed)
		if err == nil {
			err = batchEntries(raw, 1)[0]
		}
	}
	if err == nil {
		if indexErr := u.db.MoveDropboxPathPrefix(u.ctx, op.ToPath, op.FromPath); indexErr != nil {
			err = &undoIndexError{indexErr}
		}
	}
	status, detail := u.classify(err)
	if ambiguous {
		status = "unknown"
	}
	return u.settle(op, seq, status, detail, false)
}
func (u *undoExecution) restoreChild(child store.DropboxJournalOp) error {
	if child.UndoResult == "ok" {
		return nil
	}
	if child.Tag == "folder" {
		if current, exists, err := indexEntryForUndo(u.ctx, u.db, child.Path); err != nil {
			return err
		} else if exists && current.Tag == "folder" {
			return u.record(child, "skipped", fmt.Errorf("child folder already restored by parent"))
		}
		status, detail := u.classify(undoMkdir(u.ctx, u.db, u.poster, child.Path))
		return u.record(child, status, detail)
	}
	if current, exists, err := indexEntryForUndo(u.ctx, u.db, child.Path); err != nil {
		return err
	} else if exists && child.EntryID != "" && current.EntryID == child.EntryID {
		return u.record(child, "skipped", fmt.Errorf("child already restored by parent"))
	}
	err := undoRestore(u.ctx, u.db, u.poster, child, u.overwrite)
	status, detail := u.classify(err)
	return u.record(child, status, detail)
}
func (u *undoExecution) delete(op store.DropboxJournalOp) error {
	entry, found, err := indexEntryForUndo(u.ctx, u.db, op.Path)
	if err != nil {
		return err
	}
	if op.Result == "unknown" && op.UndoResult != "ok" && found && op.EntryID != "" && entry.EntryID == op.EntryID {
		return u.record(op, "skipped", fmt.Errorf("unknown delete did not occur"))
	}
	switch op.Tag {
	case "folder":
		if op.UndoResult != "ok" {
			err = undoMkdir(u.ctx, u.db, u.poster, op.Path)
			status, detail := u.classify(err)
			if err := u.record(op, status, detail); err != nil {
				return err
			}
		}
		for _, child := range childRestoreOrder(u.children[op.Path]) {
			if err := u.restoreChild(child); err != nil {
				return err
			}
		}
		return nil
	case "file":
		err = undoRestore(u.ctx, u.db, u.poster, op, u.overwrite)
		status, detail := u.classify(err)
		return u.record(op, status, detail)
	default:
		return u.record(op, "skipped", fmt.Errorf("journal tag is missing or unsupported"))
	}
}
