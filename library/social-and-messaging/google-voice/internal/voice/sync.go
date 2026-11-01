package voice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func containsMessage(messages []Message, id string) bool {
	for _, m := range messages {
		if m.ID == id {
			return true
		}
	}
	return false
}
func latestMessageID(messages []Message) string {
	var id string
	var ts int64
	for _, m := range messages {
		if m.TimestampMS > ts || (m.TimestampMS == ts && m.ID > id) {
			id = m.ID
			ts = m.TimestampMS
		}
	}
	return id
}
func ceilMillis(t time.Time) int64 {
	ms := t.UnixMilli()
	if t.Nanosecond()%int(time.Millisecond) != 0 {
		ms++
	}
	return ms
}

type pageJob struct {
	kind, scope, cursor string
	last                int64
}

func (m *Mirror) nextJob(ctx context.Context, folders []string, runID int64) (pageJob, bool, error) {
	var best pageJob
	found := false
	for _, folder := range folders {
		var cursor string
		var complete bool
		var last, observedRun int64
		e := m.DB.QueryRowContext(ctx, `SELECT i.cursor,i.complete,COALESCE((SELECT MAX(rowid) FROM voice_pages WHERE kind='inventory' AND scope=i.folder),0),COALESCE((SELECT run_id FROM voice_pages WHERE kind='inventory' AND scope=i.folder ORDER BY rowid DESC LIMIT 1),0) FROM voice_inventory i WHERE folder=?`, folder).Scan(&cursor, &complete, &last, &observedRun)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return best, false, e
		}
		if complete && observedRun == runID {
			continue
		}
		if complete {
			cursor = ""
		}
		job := pageJob{"inventory", folder, cursor, last}
		if !found || job.last < best.last {
			best = job
			found = true
		}
	}
	var id, older, gap, lastKind string
	var refresh, complete bool
	var last int64
	e := m.DB.QueryRowContext(ctx, `SELECT t.id,t.older_cursor,t.refresh_cursor,t.needs_refresh,t.history_complete,t.last_kind,COALESCE((SELECT MAX(rowid) FROM voice_pages p WHERE p.kind<>'inventory' AND p.scope=t.id),0) AS worked FROM voice_threads t WHERE t.needs_refresh=1 OR t.history_complete=0 OR t.refresh_cursor<>'' ORDER BY worked,t.id LIMIT 1`).Scan(&id, &older, &gap, &refresh, &complete, &lastKind, &last)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return best, false, e
	}
	if e == nil && (!found || last < best.last) {
		kind, cursor := "refresh", ""
		if gap != "" {
			kind = "refresh_gap"
			cursor = gap
		} else if older != "" && !complete && (!refresh || lastKind == "refresh") {
			kind = "history"
			cursor = older
		}
		best = pageJob{kind, id, cursor, last}
		found = true
	}
	return best, found, nil
}
func (m *Mirror) Sync(ctx context.Context, r *Remote, folders []string, maxPages int) (result SyncResult, err error) {
	if maxPages < 1 || maxPages > 1000 || len(folders) == 0 {
		return result, fmt.Errorf("--max-pages must be 1 through 1000 and --folders must not be empty")
	}
	seen := map[string]bool{}
	unique := []string{}
	for _, f := range folders {
		if f != "texts" && f != "archived" {
			return result, fmt.Errorf("--folders accepts texts,archived")
		}
		if !seen[f] {
			unique = append(unique, f)
			seen[f] = true
		}
	}
	err = mirrorLock(ctx, m.Path+".sync", func() error { var e error; result, e = m.syncLocked(ctx, r, unique, maxPages); return e })
	return result, err
}
func (m *Mirror) syncLocked(ctx context.Context, r *Remote, folders []string, maxPages int) (result SyncResult, err error) {
	a, err := r.Account(ctx)
	if err != nil {
		return result, err
	}
	if a.ID != m.AccountID {
		return result, fmt.Errorf("Voice account does not match this mirror")
	}
	if _, err = m.DB.ExecContext(ctx, "UPDATE voice_runs SET status='interrupted',finished_at=?,error='previous writer ended before terminal status; committed pages remain resumable' WHERE status='running'", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return result, err
	}
	row, e := m.DB.ExecContext(ctx, "INSERT INTO voice_runs(started_at,status) VALUES(?,'running')", time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return result, e
	}
	result.RunID, e = row.LastInsertId()
	if e != nil {
		return result, e
	}
	result.Status = "bounded"
	defer func() {
		status := result.Status
		reason := ""
		if err != nil {
			status = "failed"
			reason = "read or page commit failed; resume saved checkpoints"
		}
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_, e := m.DB.ExecContext(finishCtx, "UPDATE voice_runs SET finished_at=?,status=?,error=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), status, reason, result.RunID)
		if err == nil {
			err = e
		}
	}()
	for result.Pages < maxPages {
		job, ok, e := m.nextJob(ctx, folders, result.RunID)
		if e != nil {
			return result, e
		}
		if !ok {
			break
		}
		var p Page
		if job.kind == "inventory" {
			p, e = r.List(ctx, job.scope, job.cursor)
		} else {
			p, e = r.Get(ctx, job.scope, job.cursor, 100)
		}
		if e != nil {
			return result, e
		}
		if p.NextCursor != "" && p.NextCursor == job.cursor {
			return result, fmt.Errorf("Voice continuation cursor did not advance")
		}
		n, e := m.commit(ctx, result.RunID, job.kind, job.scope, job.cursor, p.NextCursor, p.Threads)
		if e != nil {
			return result, e
		}
		result.NewMessages += n
		result.Pages++
	}
	if e = m.DB.QueryRowContext(ctx, "SELECT count(*) FROM voice_threads WHERE needs_refresh=1 OR history_complete=0 OR refresh_cursor<>''").Scan(&result.PendingThreads); e != nil {
		return result, e
	}
	result.InventoryComplete = true
	for _, folder := range folders {
		var exhausted bool
		e = m.DB.QueryRowContext(ctx, "SELECT complete FROM voice_inventory WHERE folder=?", folder).Scan(&exhausted)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return result, e
		}
		if e != nil || !exhausted {
			result.InventoryComplete = false
		}
	}
	if result.PendingThreads == 0 && result.InventoryComplete {
		result.Status = "observed_exhausted"
	}
	return result, nil
}
