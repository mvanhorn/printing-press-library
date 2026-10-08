package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

const deleteFolderCountsQuery = `SELECT count(*),COALESCE(sum(size),0) FROM dbx_files INDEXED BY dbx_files_file_path_size WHERE tag='file' AND path_lower>=? AND path_lower<?`
const deleteFolderChildrenQuery = `SELECT path_display,COALESCE(rev,''),COALESCE(id,'') FROM dbx_files INDEXED BY dbx_files_file_path_size WHERE tag='file' AND path_lower>=? AND path_lower<? ORDER BY path_lower`

// Descendant folders are journaled too: restoring files recreates only the
// folders that hold them, so empty subfolders would otherwise stay deleted.
const deleteFolderSubfoldersQuery = `SELECT COALESCE(NULLIF(path_display,''),path_lower),COALESCE(id,'') FROM dbx_files WHERE tag='folder' AND path_lower>=? AND path_lower<? ORDER BY path_lower`

type applyRun struct {
	ctx                 context.Context
	db                  *store.Store
	poster              dropboxBatchPoster
	result              *applyResult
	nextSeq             int
	priorDeletes        map[string]bool
	skippedDeletes      map[string]bool
	failedMoveSources   map[string]bool
	allowNonemptyDelete bool
}

func (a *applyRun) skip(item numberedOp, kind, reason string) error {
	if err := a.journal(item, "skipped"); err != nil {
		return err
	}
	if err := a.update(item, "skipped", fmt.Errorf("%s", reason)); err != nil {
		return err
	}
	if kind == "delete" {
		if a.skippedDeletes == nil {
			a.skippedDeletes = make(map[string]bool)
		}
		a.skippedDeletes[dropbox.PathKey(item.op.Path)] = true
	}
	a.result.record(kind, item.seq, "skipped", nil)
	return nil
}

func (a *applyRun) journal(item numberedOp, status string) error {
	return a.db.AddDropboxJournalOp(a.ctx, store.DropboxJournalOp{BatchID: a.result.BatchID, Seq: item.seq, Op: item.op.Op,
		Path: item.op.Path, FromPath: item.op.From, ToPath: item.op.To, Rev: item.rev, URL: item.op.URL,
		Tag: item.tag, EntryID: item.entryID, Result: status})
}
func (a *applyRun) update(item numberedOp, status string, detail error) error {
	message := ""
	if detail != nil {
		message = detail.Error()
	}
	return a.db.SetDropboxJournalOpResult(a.ctx, a.result.BatchID, item.seq, status, message)
}
func (a *applyRun) finish() error {
	ok := a.result.Counts.Mkdir.OK + a.result.Counts.Move.OK + a.result.Counts.Delete.OK + a.result.Counts.Revoke.OK
	if len(a.result.Failures) > 0 {
		a.result.Status = "partial"
		if ok == 0 {
			a.result.Status = "failed"
		}
	}
	return a.db.SetDropboxJournalStatus(a.ctx, a.result.BatchID, a.result.Status)
}
func applyChunkEnd(items []numberedOp, kind string, start int) int {
	step := 1000
	if kind == "mkdir" {
		step = 1
	}
	end := start + step
	if end > len(items) {
		end = len(items)
	}
	if kind == "move" {
		if caseOnlyMove(items[start].op) {
			return start + 1
		}
		for i := start + 1; i < end; i++ {
			if caseOnlyMove(items[i].op) {
				return i
			}
		}
	}
	return end
}
func (a *applyRun) runGroups(p dropbox.Plan) error {
	for _, kind := range []string{"mkdir", "move", "delete"} {
		items := groupedOps(p, kind)
		for start := 0; start < len(items); {
			end := applyChunkEnd(items, kind, start)
			if err := a.runChunk(kind, items[start:end]); err != nil {
				return err
			}
			start = end
		}
	}
	return nil
}
func (a *applyRun) runChunk(kind string, chunk []numberedOp) error {
	active, children, err := a.prepareChunk(kind, chunk)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		return nil
	}
	entryErrs, batchErr := a.submitChunk(kind, active, children)
	return a.recordChunk(kind, active, children, entryErrs, batchErr)
}
func (a *applyRun) prepareChunk(kind string, chunk []numberedOp) ([]numberedOp, map[int][]int, error) {
	active := make([]numberedOp, 0, len(chunk))
	children := map[int][]int{}
	for _, item := range chunk {
		if kind == "delete" {
			coveredReason := ""
			for parent := dropbox.PathKey(item.op.Path); parent != ""; parent, _ = dropbox.ParentBase(parent) {
				if a.skippedDeletes[parent] {
					coveredReason = "inside skipped delete"
					break
				}
				if a.priorDeletes[parent] {
					coveredReason = "covered by an earlier delete"
					break
				}
			}
			if coveredReason != "" {
				if err := a.skip(item, kind, coveredReason); err != nil {
					return nil, nil, err
				}
				continue
			}
			dependent := false
			for from := range a.failedMoveSources {
				if dropbox.PathWithin(item.op.Path, from) || dropbox.PathWithin(from, item.op.Path) {
					dependent = true
					break
				}
			}
			if dependent {
				if err := a.skip(item, kind, "depends on failed move"); err != nil {
					return nil, nil, err
				}
				continue
			}
		}
		var err error
		item, err = resolveApplyItem(a.ctx, a.db, item)
		if err != nil {
			return nil, nil, err
		}
		if kind == "mkdir" {
			var exists int
			if err := a.db.DB().QueryRowContext(a.ctx, `SELECT count(*) FROM dbx_files WHERE path_lower=?`, dropbox.PathKey(item.op.Path)).Scan(&exists); err != nil {
				return nil, nil, err
			}
			if exists > 0 {
				if err := a.journal(item, "skipped"); err != nil {
					return nil, nil, err
				}
				a.result.record(kind, item.seq, "skipped", nil)
				continue
			}
		}
		if kind == "delete" && item.tag == "folder" {
			lower, upper := descendantRange(dropbox.PathKey(item.op.Path))
			var files int
			var bytes int64
			if err := a.db.DB().QueryRowContext(a.ctx, deleteFolderCountsQuery, lower, upper).Scan(&files, &bytes); err != nil {
				return nil, nil, err
			}
			if item.op.ExpectFiles != nil || item.op.ExpectBytes != nil {
				if item.op.ExpectFiles == nil || item.op.ExpectBytes == nil || files != *item.op.ExpectFiles || bytes != *item.op.ExpectBytes {
					if err := a.skip(item, kind, fmt.Sprintf("folder delete attestation changed: %d files (%d bytes)", files, bytes)); err != nil {
						return nil, nil, err
					}
					continue
				}
			} else if files > 0 && !a.allowNonemptyDelete {
				if err := a.skip(item, kind, fmt.Sprintf("folder delete now contains %d files (%d bytes)", files, bytes)); err != nil {
					return nil, nil, err
				}
				continue
			}
			if item.op.ExpectPathTreeHash != "" {
				treeFiles, err := conflictTreeFiles(a.ctx, a.db.DB(), dropbox.PathKey(item.op.Path))
				if err != nil {
					return nil, nil, err
				}
				if conflictTreeHash(treeFiles) != item.op.ExpectPathTreeHash {
					if err := a.skip(item, kind, "folder delete contents changed since the plan was written"); err != nil {
						return nil, nil, err
					}
					continue
				}
			}
		}
		if kind == "delete" {
			a.priorDeletes[dropbox.PathKey(item.op.Path)] = true
		}
		if err := a.journal(item, "pending"); err != nil {
			return nil, nil, err
		}
		if kind == "delete" && item.tag == "folder" {
			if err := a.journalDeleteChildren(item, children); err != nil {
				return nil, nil, err
			}
		}
		active = append(active, item)
	}
	return active, children, nil
}
func (a *applyRun) journalDeleteChildren(item numberedOp, children map[int][]int) error {
	lower, upper := descendantRange(dropbox.PathKey(item.op.Path))
	pending := make([]store.DropboxJournalOp, 0)
	folders, err := a.db.DB().QueryContext(a.ctx, deleteFolderSubfoldersQuery, lower, upper)
	if err != nil {
		return err
	}
	for folders.Next() {
		child := store.DropboxJournalOp{BatchID: a.result.BatchID, Op: "delete_child", Tag: "folder", Result: "pending", Seq: a.nextSeq}
		if err := folders.Scan(&child.Path, &child.EntryID); err != nil {
			_ = folders.Close()
			return err
		}
		a.nextSeq++
		pending = append(pending, child)
	}
	err = folders.Err()
	_ = folders.Close()
	if err != nil {
		return err
	}
	rows, err := a.db.DB().QueryContext(a.ctx, deleteFolderChildrenQuery, lower, upper)
	if err != nil {
		return err
	}
	for rows.Next() {
		var child store.DropboxJournalOp
		if err := rows.Scan(&child.Path, &child.Rev, &child.EntryID); err != nil {
			_ = rows.Close()
			return err
		}
		child.BatchID = a.result.BatchID
		child.Op = "delete_child"
		child.Tag = "file"
		child.Result = "pending"
		child.Seq = a.nextSeq
		a.nextSeq++
		pending = append(pending, child)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if err := a.db.AddDropboxJournalOps(a.ctx, pending); err != nil {
		return err
	}
	for _, child := range pending {
		children[item.seq] = append(children[item.seq], child.Seq)
	}
	return nil
}
func (a *applyRun) submitChunk(kind string, active []numberedOp, children map[int][]int) ([]error, error) {
	entryErrs := make([]error, len(active))
	if kind == "move" && len(active) == 1 && caseOnlyMove(active[0].op) {
		_, err := a.poster.Write(a.ctx, "/files/move_v2", map[string]any{"from_path": active[0].op.From, "to_path": active[0].op.To, "autorename": false})
		entryErrs[0] = err
		return entryErrs, err
	}
	var body any
	var submit, check string
	switch kind {
	case "mkdir":
		paths := make([]string, 0, len(active))
		for _, item := range active {
			paths = append(paths, item.op.Path)
		}
		body = map[string]any{"paths": paths, "autorename": false}
		submit, check = "/files/create_folder_batch", "/files/create_folder_batch/check"
	case "move":
		entries := make([]map[string]string, 0, len(active))
		for _, item := range active {
			entries = append(entries, map[string]string{"from_path": item.op.From, "to_path": item.op.To})
		}
		body = map[string]any{"entries": entries, "autorename": false}
		submit, check = "/files/move_batch_v2", "/files/move_batch/check_v2"
	case "delete":
		entries := make([]map[string]string, 0, len(active))
		for _, item := range active {
			e := map[string]string{"path": item.op.Path}
			if item.tag == "file" && item.rev != "" {
				e["parent_rev"] = item.rev
			}
			entries = append(entries, e)
		}
		body = map[string]any{"entries": entries}
		submit, check = "/files/delete_batch", "/files/delete_batch/check"
	}
	raw, err := dropbox.RunBatchJobObserved(a.ctx, a.poster, submit, check, body, time.Second, func(id string) error {
		for i, item := range active {
			if err := a.db.SetDropboxJournalAsyncJobID(a.ctx, a.result.BatchID, item.seq, id, i); err != nil {
				return err
			}
			for _, seq := range children[item.seq] {
				if err := a.db.SetDropboxJournalAsyncJobID(a.ctx, a.result.BatchID, seq, id); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		for i := range entryErrs {
			entryErrs[i] = err
		}
		return entryErrs, err
	}
	return batchEntries(raw, len(active)), nil
}
func (a *applyRun) indexSuccess(kind string, item numberedOp) error {
	switch kind {
	case "mkdir":
		name := item.op.Path[strings.LastIndex(item.op.Path, "/")+1:]
		parent := ""
		if i := strings.LastIndex(item.op.Path, "/"); i > 0 {
			parent = dropbox.PathKey(item.op.Path[:i])
		}
		root := dropbox.IndexRoot(item.op.Path)
		return a.db.UpsertDropboxEntries(a.ctx, root, []store.DropboxRow{{PathLower: dropbox.PathKey(item.op.Path), PathDisplay: item.op.Path, ParentLower: parent, Name: name, Tag: "folder"}})
	case "move":
		return a.db.MoveDropboxPathPrefix(a.ctx, item.op.From, item.op.To)
	case "delete":
		_, err := a.db.DeleteDropboxPathPrefix(a.ctx, item.op.Path)
		return err
	}
	return nil
}
func (a *applyRun) recordChunk(kind string, active []numberedOp, children map[int][]int, entryErrs []error, batchErr error) error {
	for i, item := range active {
		remoteErr := entryErrs[i]
		state := "ok"
		if remoteErr != nil {
			state = "failed"
			if batchErr != nil {
				var failed *dropbox.BatchFailedError
				if !errors.As(batchErr, &failed) {
					state = "unknown"
				}
			}
		}
		detail := remoteErr
		if remoteErr == nil {
			if indexErr := a.indexSuccess(kind, item); indexErr != nil {
				detail = fmt.Errorf("index_error: %w", indexErr)
				if err := a.db.SetDropboxMeta(context.WithoutCancel(a.ctx), "index_stale", "1"); err != nil {
					return err
				}
				a.result.Failures = append(a.result.Failures, applyFailure{Seq: item.seq, Op: kind, Error: detail.Error()})
			}
		}
		if err := a.update(item, state, detail); err != nil {
			return err
		}
		if kind == "move" && state != "ok" {
			a.failedMoveSources[dropbox.PathKey(item.op.From)] = true
		}
		for _, seq := range children[item.seq] {
			if err := a.db.SetDropboxJournalOpResult(a.ctx, a.result.BatchID, seq, state, ""); err != nil {
				return err
			}
		}
		a.result.record(kind, item.seq, state, remoteErr)
	}
	return nil
}
func (a *applyRun) revokeLinks(p dropbox.Plan) error {
	for _, item := range groupedOps(p, "revoke_link") {
		if err := a.journal(item, "pending"); err != nil {
			return err
		}
		metadata, metadataErr := a.poster.Read(a.ctx, "/sharing/get_shared_link_metadata", map[string]string{"url": item.op.URL})
		if metadataErr != nil {
			if err := a.update(item, "unknown", metadataErr); err != nil {
				return err
			}
			a.result.record("revoke", item.seq, "unknown", metadataErr)
			continue
		}
		if item.op.ExpectDangling {
			var link struct {
				PathLower string `json:"path_lower"`
			}
			if err := json.Unmarshal(metadata, &link); err != nil {
				return err
			}
			if link.PathLower != "" {
				exists, err := a.linkedTargetExists(link.PathLower)
				if err != nil {
					if updateErr := a.update(item, "unknown", err); updateErr != nil {
						return updateErr
					}
					a.result.record("revoke", item.seq, "unknown", err)
					continue
				}
				if exists {
					if err := a.update(item, "skipped", fmt.Errorf("dangling link now resolves to %s", link.PathLower)); err != nil {
						return err
					}
					a.result.record("revoke", item.seq, "skipped", nil)
					continue
				}
			}
		}
		_, err := a.poster.Write(a.ctx, "/sharing/revoke_shared_link", map[string]string{"url": item.op.URL})
		state := "ok"
		if err != nil {
			state = "unknown"
		} else {
			_, indexErr := a.db.DB().ExecContext(a.ctx, `DELETE FROM dbx_shared_links WHERE url=?`, item.op.URL)
			if indexErr != nil {
				err = fmt.Errorf("index_error: %w", indexErr)
				if metaErr := a.db.SetDropboxMeta(context.WithoutCancel(a.ctx), "index_stale", "1"); metaErr != nil {
					return metaErr
				}
				a.result.Failures = append(a.result.Failures, applyFailure{Seq: item.seq, Op: "revoke", Error: err.Error()})
			}
		}
		if updateErr := a.update(item, state, err); updateErr != nil {
			return updateErr
		}
		if state == "ok" {
			a.result.record("revoke", item.seq, state, nil)
		} else {
			a.result.record("revoke", item.seq, state, err)
		}
	}
	return nil
}

func (a *applyRun) linkedTargetExists(path string) (bool, error) {
	raw, err := a.poster.Read(a.ctx, "/files/get_metadata", map[string]string{"path": path})
	if err != nil {
		if dropbox.HasSummaryPrefix(err, "path/not_found") {
			return false, nil
		}
		return false, err
	}
	var metadata struct {
		Tag string `json:".tag"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return false, err
	}
	return metadata.Tag != "deleted", nil
}
