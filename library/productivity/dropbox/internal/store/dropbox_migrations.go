package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
)

type DropboxRow struct {
	PathLower, ID, Tag, Name, PathDisplay, ParentLower, Rev string
	Size                                                    int64
	ContentHash, ClientModified, ServerModified             string
	SharedFolderID, ParentSharedFolderID                    string
	IsDownloadable                                          bool
	DevKind                                                 string
}

type DropboxIndexState struct {
	Root              string
	Cursor            string
	LastFullAt        string
	LastIncrementalAt string
	Entries           int
	Complete          bool
}

// Keep this expression identical to the broad SQL prefilter used by conflicts.
// The Go parser still decides whether a matching name is a real conflict.
const dropboxConflictCandidateExpr = `CASE WHEN (name LIKE '%conflicted copy%' COLLATE NOCASE OR name LIKE '%selective sync conflict%' COLLATE NOCASE OR name LIKE '%case conflict%' COLLATE NOCASE OR name LIKE '%invalid files%' COLLATE NOCASE) THEN 1 ELSE 0 END`

func (s *Store) EnsureDropboxSchema(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	updateFTSTrigger := `CREATE TRIGGER IF NOT EXISTS dbx_files_au AFTER UPDATE OF name,path_display ON dbx_files BEGIN INSERT INTO dbx_files_fts(dbx_files_fts,rowid,name,path_display) VALUES('delete',old.rowid,old.name,old.path_display); INSERT INTO dbx_files_fts(rowid,name,path_display) VALUES(new.rowid,new.name,new.path_display); END`
	statements := []string{
		`CREATE TABLE IF NOT EXISTS dbx_files(path_lower TEXT PRIMARY KEY, id TEXT, tag TEXT, name TEXT, path_display TEXT, parent_lower TEXT, rev TEXT, size INTEGER, content_hash TEXT, client_modified TEXT, server_modified TEXT, shared_folder_id TEXT, parent_shared_folder_id TEXT, is_downloadable INTEGER, root TEXT, seen_at TEXT, dev_kind TEXT, conflict_candidate INTEGER GENERATED ALWAYS AS (` + dropboxConflictCandidateExpr + `) VIRTUAL)`,
		`CREATE INDEX IF NOT EXISTS dbx_files_parent ON dbx_files(parent_lower)`,
		`CREATE INDEX IF NOT EXISTS dbx_files_hash ON dbx_files(content_hash)`,
		`CREATE INDEX IF NOT EXISTS dbx_files_tag_size ON dbx_files(tag,size)`,
		`CREATE INDEX IF NOT EXISTS dbx_files_duplicate_candidates ON dbx_files(content_hash,size,path_lower) WHERE tag='file' AND content_hash<>''`,
		`CREATE INDEX IF NOT EXISTS dbx_files_parent_name_lower ON dbx_files(parent_lower,lower(name))`,
		`CREATE INDEX IF NOT EXISTS dbx_files_file_parent ON dbx_files(parent_lower) WHERE tag='file'`,
		`CREATE INDEX IF NOT EXISTS dbx_files_file_path_size ON dbx_files(path_lower,content_hash,size) WHERE tag='file'`,
		`CREATE INDEX IF NOT EXISTS dbx_files_root ON dbx_files(root)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS dbx_files_fts USING fts5(name, path_display, content='dbx_files', content_rowid='rowid')`,
		`CREATE TRIGGER IF NOT EXISTS dbx_files_ai AFTER INSERT ON dbx_files BEGIN INSERT INTO dbx_files_fts(rowid,name,path_display) VALUES(new.rowid,new.name,new.path_display); END`,
		`CREATE TRIGGER IF NOT EXISTS dbx_files_ad AFTER DELETE ON dbx_files BEGIN INSERT INTO dbx_files_fts(dbx_files_fts,rowid,name,path_display) VALUES('delete',old.rowid,old.name,old.path_display); END`,
		updateFTSTrigger,
		`CREATE TABLE IF NOT EXISTS dbx_index_state(root TEXT PRIMARY KEY, cursor TEXT, last_full_at TEXT, last_incremental_at TEXT, entries INTEGER, complete INTEGER)`,
		`CREATE TABLE IF NOT EXISTS dbx_meta(key TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TABLE IF NOT EXISTS dbx_shared_links(url TEXT PRIMARY KEY, path_lower TEXT, name TEXT, visibility TEXT, expires TEXT, link_type TEXT, id TEXT, seen_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS dbx_journal_batches(id TEXT PRIMARY KEY, created_at TEXT, source TEXT, plan_path TEXT, status TEXT, account_type TEXT, restore_days INTEGER, undo_of TEXT, account_id TEXT)`,
		`CREATE TABLE IF NOT EXISTS dbx_journal_ops(batch_id TEXT, seq INTEGER, op TEXT, path TEXT, from_path TEXT, to_path TEXT, rev TEXT, url TEXT, result TEXT, error TEXT, async_job_id TEXT, tag TEXT, entry_id TEXT, undo_result TEXT, undo_error TEXT, job_index INTEGER, undo_job_id TEXT, PRIMARY KEY(batch_id,seq))`,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("Dropbox schema: %w", err)
		}
	}
	columns, err := tx.QueryContext(ctx, `PRAGMA table_xinfo(dbx_files)`)
	if err != nil {
		return err
	}
	hasDevKind := false
	hasConflictCandidate := false
	for columns.Next() {
		var cid, notNull, primaryKey, hidden int
		var name, dataType string
		var defaultValue sql.NullString
		if err := columns.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey, &hidden); err != nil {
			_ = columns.Close()
			return err
		}
		hasDevKind = hasDevKind || name == "dev_kind"
		hasConflictCandidate = hasConflictCandidate || name == "conflict_candidate"
	}
	err = columns.Err()
	_ = columns.Close()
	if err != nil {
		return err
	}
	if !hasDevKind {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE dbx_files ADD COLUMN dev_kind TEXT`); err != nil {
			return err
		}
		// Old schemas updated FTS for every column change. The backfill only
		// changes dev_kind, so narrow the trigger before updating many rows.
		if _, err := tx.ExecContext(ctx, `DROP TRIGGER IF EXISTS dbx_files_au`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, updateFTSTrigger); err != nil {
			return err
		}
		if err := backfillDropboxDevKinds(ctx, tx); err != nil {
			return err
		}
	}
	if !hasConflictCandidate {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE dbx_files ADD COLUMN conflict_candidate INTEGER GENERATED ALWAYS AS (`+dropboxConflictCandidateExpr+`) VIRTUAL`); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS dbx_files_dev_kind ON dbx_files(dev_kind)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS dbx_files_dev_path ON dbx_files(path_lower) WHERE dev_kind IS NOT NULL`); err != nil {
		return err
	}
	journalColumns, err := tx.QueryContext(ctx, `PRAGMA table_info(dbx_journal_ops)`)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for journalColumns.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := journalColumns.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = journalColumns.Close()
			return err
		}
		present[name] = true
	}
	err = journalColumns.Err()
	_ = journalColumns.Close()
	if err != nil {
		return err
	}
	for _, name := range []string{"tag", "entry_id", "undo_result", "undo_error", "job_index", "undo_job_id"} {
		if !present[name] {
			columnType := "TEXT"
			if name == "job_index" {
				columnType = "INTEGER"
			}
			if _, err := tx.ExecContext(ctx, `ALTER TABLE dbx_journal_ops ADD COLUMN `+name+` `+columnType); err != nil {
				return err
			}
		}
	}
	batchColumns, err := tx.QueryContext(ctx, `PRAGMA table_info(dbx_journal_batches)`)
	if err != nil {
		return err
	}
	hasAccountID := false
	for batchColumns.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := batchColumns.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = batchColumns.Close()
			return err
		}
		hasAccountID = hasAccountID || name == "account_id"
	}
	err = batchColumns.Err()
	_ = batchColumns.Close()
	if err != nil {
		return err
	}
	if !hasAccountID {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE dbx_journal_batches ADD COLUMN account_id TEXT`); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS dbx_files_conflict_candidate ON dbx_files(conflict_candidate,path_lower) WHERE conflict_candidate=1`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS dbx_files_visible_duplicates ON dbx_files(content_hash,size,path_lower) WHERE tag='file' AND content_hash<>'' AND dev_kind IS NULL`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS dbx_files_dev_file_size ON dbx_files(size) WHERE tag='file' AND dev_kind IS NOT NULL`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS dbx_files_visible_file_parent ON dbx_files(parent_lower) WHERE tag='file' AND dev_kind IS NULL`); err != nil {
		return err
	}
	for _, index := range []struct{ name, covering, ddl string }{
		{"dbx_files_duplicate_candidates", "dev_kind", `CREATE INDEX dbx_files_duplicate_candidates ON dbx_files(content_hash,size,dev_kind,path_lower) WHERE tag='file' AND content_hash<>''`},
		{"dbx_files_file_path_size", "content_hash", `CREATE INDEX dbx_files_file_path_size ON dbx_files(path_lower,content_hash,size) WHERE tag='file'`},
	} {
		var definition string
		if err := tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, index.name).Scan(&definition); err != nil {
			return err
		}
		if !strings.Contains(definition, index.covering) {
			if _, err := tx.ExecContext(ctx, `DROP INDEX `+index.name); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, index.ddl); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Backfill only paths that may contain a dev segment, then confirm segment
// boundaries in Go. A rowid cursor bounds each read and update batch.
func backfillDropboxDevKinds(ctx context.Context, tx *sql.Tx) error {
	parts := make([]string, 0, len(dropbox.DevDirNames))
	args := make([]any, 0, len(dropbox.DevDirNames)+1)
	for _, name := range dropbox.DevDirNames {
		parts = append(parts, `lower(path_lower) LIKE ? ESCAPE '\'`)
		args = append(args, "%"+dropbox.EscapeLike(strings.ToLower(name))+"%")
	}
	query := `SELECT rowid,path_lower FROM dbx_files WHERE rowid>? AND (` + strings.Join(parts, " OR ") + `) ORDER BY rowid LIMIT 2000` // #nosec G202 -- parts holds only fixed LIKE clauses; every value is a bound parameter
	stmt, err := tx.PrepareContext(ctx, `UPDATE dbx_files SET dev_kind=? WHERE rowid=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	var cursor int64
	for {
		batchArgs := append([]any{cursor}, args...)
		rows, err := tx.QueryContext(ctx, query, batchArgs...)
		if err != nil {
			return err
		}
		type candidate struct {
			id   int64
			kind string
		}
		batch := make([]candidate, 0, 2000)
		n := 0
		for rows.Next() {
			var id int64
			var path string
			if err := rows.Scan(&id, &path); err != nil {
				_ = rows.Close()
				return err
			}
			cursor = id
			n++
			if kind, ok := dropbox.DevDirKind(path); ok {
				batch = append(batch, candidate{id, kind})
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		for _, item := range batch {
			if _, err := stmt.ExecContext(ctx, item.kind, item.id); err != nil {
				return err
			}
		}
		if n < 2000 {
			return nil
		}
	}
}

func upsertDropboxEntriesTx(ctx context.Context, tx *sql.Tx, root string, entries []DropboxRow) error {
	seen := time.Now().UTC().Format(time.RFC3339)
	for _, e := range entries {
		if e.PathLower == "" {
			return fmt.Errorf("Dropbox row path_lower is empty")
		}
		kind, ok := dropbox.DevDirKind(e.PathLower)
		var devKind any
		if ok {
			devKind = kind
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO dbx_files(path_lower,id,tag,name,path_display,parent_lower,rev,size,content_hash,client_modified,server_modified,shared_folder_id,parent_shared_folder_id,is_downloadable,root,seen_at,dev_kind) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(path_lower) DO UPDATE SET id=excluded.id,tag=excluded.tag,name=excluded.name,path_display=excluded.path_display,parent_lower=excluded.parent_lower,rev=excluded.rev,size=excluded.size,content_hash=excluded.content_hash,client_modified=excluded.client_modified,server_modified=excluded.server_modified,shared_folder_id=excluded.shared_folder_id,parent_shared_folder_id=excluded.parent_shared_folder_id,is_downloadable=excluded.is_downloadable,root=excluded.root,seen_at=excluded.seen_at,dev_kind=excluded.dev_kind`, e.PathLower, e.ID, e.Tag, e.Name, e.PathDisplay, e.ParentLower, e.Rev, e.Size, e.ContentHash, e.ClientModified, e.ServerModified, e.SharedFolderID, e.ParentSharedFolderID, e.IsDownloadable, root, seen, devKind)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpsertDropboxEntries(ctx context.Context, root string, entries []DropboxRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertDropboxEntriesTx(ctx, tx, root, entries); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteDropboxPrefixTx(ctx context.Context, tx *sql.Tx, pathLower string) (int64, error) {
	if pathLower == "" {
		return 0, fmt.Errorf("empty Dropbox path prefix")
	}
	p := strings.ToLower(pathLower)
	result, err := tx.ExecContext(ctx, `DELETE FROM dbx_files WHERE path_lower=? OR path_lower LIKE ? ESCAPE '\'`, p, dropbox.EscapeLike(p)+"/%")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) DeleteDropboxPathPrefix(ctx context.Context, pathLower string) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	n, err := deleteDropboxPrefixTx(ctx, tx, pathLower)
	if err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

func (s *Store) ClearDropboxRoot(ctx context.Context, root string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM dbx_files WHERE root=?`, root)
	return err
}

// RebindDropboxIndex clears only account-scoped index data; journals remain auditable.
func (s *Store) RebindDropboxIndex(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM dbx_files`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dbx_index_state`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dbx_shared_links`); err != nil {
		return err
	}
	return tx.Commit()
}

func setDropboxIndexStateTx(ctx context.Context, tx *sql.Tx, state DropboxIndexState) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO dbx_index_state(root,cursor,last_full_at,last_incremental_at,entries,complete) VALUES(?,?,?,?,?,?) ON CONFLICT(root) DO UPDATE SET cursor=excluded.cursor,last_full_at=excluded.last_full_at,last_incremental_at=excluded.last_incremental_at,entries=excluded.entries,complete=excluded.complete`, state.Root, state.Cursor, state.LastFullAt, state.LastIncrementalAt, state.Entries, state.Complete)
	return err
}

func (s *Store) ApplyDropboxPage(ctx context.Context, root string, entries []DropboxRow, deleted []string, state DropboxIndexState) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var count int64
	for _, p := range deleted {
		n, err := deleteDropboxPrefixTx(ctx, tx, p)
		if err != nil {
			return 0, err
		}
		count += n
		_, err = tx.ExecContext(ctx, `DELETE FROM dbx_index_state WHERE root=? OR root LIKE ? ESCAPE '\'`, strings.ToLower(p), dropbox.EscapeLike(strings.ToLower(p))+"/%")
		if err != nil {
			return 0, err
		}
	}
	if err := upsertDropboxEntriesTx(ctx, tx, root, entries); err != nil {
		return 0, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM dbx_files WHERE root=?`, root).Scan(&state.Entries); err != nil {
		return 0, err
	}
	if err := setDropboxIndexStateTx(ctx, tx, state); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}

func (s *Store) GetDropboxIndexState(ctx context.Context, root string) (DropboxIndexState, bool, error) {
	var st DropboxIndexState
	err := s.db.QueryRowContext(ctx, `SELECT root,COALESCE(cursor,''),COALESCE(last_full_at,''),COALESCE(last_incremental_at,''),COALESCE(entries,0),COALESCE(complete,0) FROM dbx_index_state WHERE root=?`, root).Scan(&st.Root, &st.Cursor, &st.LastFullAt, &st.LastIncrementalAt, &st.Entries, &st.Complete)
	if err == sql.ErrNoRows {
		return DropboxIndexState{}, false, nil
	}
	return st, err == nil, err
}

func (s *Store) SetDropboxIndexState(ctx context.Context, state DropboxIndexState) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := setDropboxIndexStateTx(ctx, tx, state); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteDropboxIndexState(ctx context.Context, root string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM dbx_index_state WHERE root=?`, root)
	return err
}

func (s *Store) GetDropboxMeta(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM dbx_meta WHERE key=?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) SetDropboxMeta(ctx context.Context, key, value string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO dbx_meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) ListDropboxTopFolders(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path_lower FROM dbx_files WHERE parent_lower='' AND tag='folder' ORDER BY path_lower`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roots := make([]string, 0)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		roots = append(roots, p)
	}
	return roots, rows.Err()
}

// MoveDropboxPathPrefix rewrites a moved entry and all its indexed descendants.
func (s *Store) MoveDropboxPathPrefix(ctx context.Context, from, to string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	fromLower, toLower := strings.ToLower(from), strings.ToLower(to)
	if fromLower == "" || toLower == "" {
		return fmt.Errorf("move paths must be nonempty")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	toParent, _ := dropbox.ParentBase(toLower)
	_, displayBase := dropbox.ParentBase(to)
	root := dropbox.IndexRoot(toLower)
	childRoot := root
	if childRoot == "" {
		childRoot = toLower
	}
	_, err = tx.ExecContext(ctx, `UPDATE dbx_files SET
		path_lower=? || substr(path_lower,length(?)+1),
		path_display=? || substr(COALESCE(NULLIF(path_display,''),path_lower),length(?)+1),
		parent_lower=CASE WHEN path_lower=? THEN ? ELSE ? || substr(parent_lower,length(?)+1) END,
		name=CASE WHEN path_lower=? THEN ? ELSE name END,
		root=CASE WHEN path_lower=? THEN ? ELSE ? END
		WHERE path_lower=? OR path_lower LIKE ? ESCAPE '\'`,
		toLower, fromLower, to, from, fromLower, toParent, toLower, fromLower, fromLower, displayBase, fromLower, root, childRoot, fromLower, dropbox.EscapeLike(fromLower)+"/%")
	if err != nil {
		return err
	}
	if err := refreshMovedDropboxShares(ctx, tx, toLower, toParent); err != nil {
		return err
	}
	if err := refreshMovedDropboxDevKinds(ctx, tx, toLower); err != nil {
		return err
	}
	return tx.Commit()
}

func refreshMovedDropboxShares(ctx context.Context, tx *sql.Tx, prefix, parent string) error {
	contextByPath := map[string]string{}
	if parent != "" {
		var shared, inherited string
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(shared_folder_id,''),COALESCE(parent_shared_folder_id,'') FROM dbx_files WHERE path_lower=?`, parent).Scan(&shared, &inherited)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == sql.ErrNoRows {
			return nil
		}
		if err == nil {
			if shared != "" {
				contextByPath[parent] = shared
			} else {
				contextByPath[parent] = inherited
			}
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT path_lower,COALESCE(shared_folder_id,''),COALESCE(parent_shared_folder_id,'') FROM dbx_files WHERE path_lower=? OR (path_lower>=? AND path_lower<?) ORDER BY length(path_lower),path_lower`, prefix, prefix+"/", prefix+"0")
	if err != nil {
		return err
	}
	type shareChange struct{ path, inherited string }
	changes := make([]shareChange, 0)
	for rows.Next() {
		var path, shared, old string
		if err := rows.Scan(&path, &shared, &old); err != nil {
			_ = rows.Close()
			return err
		}
		p, _ := dropbox.ParentBase(path)
		inherited := contextByPath[p]
		if old != inherited {
			changes = append(changes, shareChange{path, inherited})
		}
		if shared != "" {
			contextByPath[path] = shared
		} else {
			contextByPath[path] = inherited
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, change := range changes {
		if _, err := tx.ExecContext(ctx, `UPDATE dbx_files SET parent_shared_folder_id=? WHERE path_lower=?`, change.inherited, change.path); err != nil {
			return err
		}
	}
	return nil
}

func refreshMovedDropboxDevKinds(ctx context.Context, tx *sql.Tx, prefix string) error {
	rows, err := tx.QueryContext(ctx, `SELECT rowid,path_lower,COALESCE(dev_kind,'') FROM dbx_files WHERE path_lower=? OR (path_lower>=? AND path_lower<?)`, prefix, prefix+"/", prefix+"0")
	if err != nil {
		return err
	}
	type change struct {
		id   int64
		kind any
	}
	changes := make([]change, 0)
	for rows.Next() {
		var id int64
		var path, oldKind string
		if err := rows.Scan(&id, &path, &oldKind); err != nil {
			_ = rows.Close()
			return err
		}
		kind, ok := dropbox.DevDirKind(path)
		if kind != oldKind {
			var value any
			if ok {
				value = kind
			}
			changes = append(changes, change{id, value})
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `UPDATE dbx_files SET dev_kind=? WHERE rowid=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range changes {
		if _, err := stmt.ExecContext(ctx, item.kind, item.id); err != nil {
			return err
		}
	}
	return nil
}

func dropboxBase(p string) string {
	_, base := dropbox.ParentBase(p)
	return base
}

type DropboxJournalBatch struct {
	ID            string `json:"id"`
	CreatedAt     string `json:"created_at"`
	Source        string `json:"source"`
	PlanPath      string `json:"plan_path"`
	Status        string `json:"status"`
	AccountType   string `json:"account_type"`
	RestoreDays   int    `json:"restore_days"`
	UndoOf        string `json:"undo_of,omitempty"`
	AccountID     string `json:"account_id,omitempty"`
	LegacyAccount bool   `json:"-"`
}
type DropboxJournalOp struct {
	BatchID    string `json:"batch_id"`
	Seq        int    `json:"seq"`
	Op         string `json:"op"`
	Path       string `json:"path,omitempty"`
	FromPath   string `json:"from,omitempty"`
	ToPath     string `json:"to,omitempty"`
	Rev        string `json:"rev,omitempty"`
	URL        string `json:"url,omitempty"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
	AsyncJobID string `json:"async_job_id,omitempty"`
	JobIndex   *int   `json:"job_index,omitempty"`
	Tag        string `json:"tag,omitempty"`
	EntryID    string `json:"entry_id,omitempty"`
	UndoResult string `json:"undo_result,omitempty"`
	UndoError  string `json:"undo_error,omitempty"`
	UndoJobID  string `json:"undo_job_id,omitempty"`
}

func (s *Store) CreateDropboxJournalBatch(ctx context.Context, b DropboxJournalBatch) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO dbx_journal_batches(id,created_at,source,plan_path,status,account_type,restore_days,undo_of,account_id) VALUES(?,?,?,?,?,?,?,?,?)`, b.ID, b.CreatedAt, b.Source, b.PlanPath, b.Status, b.AccountType, b.RestoreDays, b.UndoOf, b.AccountID)
	return err
}
func (s *Store) SetDropboxJournalStatus(ctx context.Context, id, status string) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE dbx_journal_batches SET status=? WHERE id=?`, status, id)
	return err
}
func (s *Store) AddDropboxJournalOp(ctx context.Context, o DropboxJournalOp) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO dbx_journal_ops(batch_id,seq,op,path,from_path,to_path,rev,url,result,error,async_job_id,tag,entry_id,undo_result,undo_error,job_index) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, o.BatchID, o.Seq, o.Op, o.Path, o.FromPath, o.ToPath, o.Rev, o.URL, o.Result, o.Error, o.AsyncJobID, o.Tag, o.EntryID, o.UndoResult, o.UndoError, o.JobIndex)
	return err
}
func (s *Store) AddDropboxJournalOps(ctx context.Context, ops []DropboxJournalOp) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO dbx_journal_ops(batch_id,seq,op,path,from_path,to_path,rev,url,result,error,async_job_id,tag,entry_id,undo_result,undo_error,job_index) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, o := range ops {
		if _, err := stmt.ExecContext(ctx, o.BatchID, o.Seq, o.Op, o.Path, o.FromPath, o.ToPath, o.Rev, o.URL, o.Result, o.Error, o.AsyncJobID, o.Tag, o.EntryID, o.UndoResult, o.UndoError, o.JobIndex); err != nil {
			_ = stmt.Close()
			return err
		}
	}
	if err := stmt.Close(); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) SetDropboxJournalOpResult(ctx context.Context, batchID string, seq int, result, detail string) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE dbx_journal_ops SET result=?,error=? WHERE batch_id=? AND seq=?`, result, detail, batchID, seq)
	return err
}
func (s *Store) SetDropboxPendingJournalOpResult(ctx context.Context, batchID string, seq int, result, detail string) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE dbx_journal_ops SET result=?,error=? WHERE batch_id=? AND seq=? AND result='pending'`, result, detail, batchID, seq)
	return err
}
func (s *Store) SetDropboxJournalAsyncJobID(ctx context.Context, batchID string, seq int, jobID string, jobIndex ...int) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var index any
	if len(jobIndex) > 0 {
		index = jobIndex[0]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE dbx_journal_ops SET async_job_id=?,job_index=? WHERE batch_id=? AND seq=?`, jobID, index, batchID, seq)
	return err
}
func (s *Store) SetDropboxJournalUndoResult(ctx context.Context, batchID string, seq int, result, detail string) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE dbx_journal_ops SET undo_result=?,undo_error=? WHERE batch_id=? AND seq=?`, result, detail, batchID, seq)
	return err
}

// SetDropboxJournalUndoJobID records the async job that reverses an
// operation, so an interrupted undo can resolve it on retry.
func (s *Store) SetDropboxJournalUndoJobID(ctx context.Context, batchID string, seq int, jobID string) error {
	ctx = context.WithoutCancel(ctx)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE dbx_journal_ops SET undo_job_id=? WHERE batch_id=? AND seq=?`, jobID, batchID, seq)
	return err
}
func (s *Store) ListDropboxJournalBatches(ctx context.Context, limit int) ([]DropboxJournalBatch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,COALESCE(created_at,''),COALESCE(source,''),COALESCE(plan_path,''),COALESCE(status,''),COALESCE(account_type,''),COALESCE(restore_days,0),COALESCE(undo_of,''),account_id FROM dbx_journal_batches ORDER BY created_at DESC,id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DropboxJournalBatch, 0)
	for rows.Next() {
		var b DropboxJournalBatch
		var account sql.NullString
		if err := rows.Scan(&b.ID, &b.CreatedAt, &b.Source, &b.PlanPath, &b.Status, &b.AccountType, &b.RestoreDays, &b.UndoOf, &account); err != nil {
			return nil, err
		}
		b.AccountID, b.LegacyAccount = account.String, !account.Valid
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Store) GetDropboxJournalBatch(ctx context.Context, id string) (DropboxJournalBatch, bool, error) {
	var b DropboxJournalBatch
	var account sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,COALESCE(created_at,''),COALESCE(source,''),COALESCE(plan_path,''),COALESCE(status,''),COALESCE(account_type,''),COALESCE(restore_days,0),COALESCE(undo_of,''),account_id FROM dbx_journal_batches WHERE id=?`, id).Scan(&b.ID, &b.CreatedAt, &b.Source, &b.PlanPath, &b.Status, &b.AccountType, &b.RestoreDays, &b.UndoOf, &account)
	if err == sql.ErrNoRows {
		return b, false, nil
	}
	b.AccountID, b.LegacyAccount = account.String, !account.Valid
	return b, err == nil, err
}
func (s *Store) ListDropboxJournalOps(ctx context.Context, id string) ([]DropboxJournalOp, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT batch_id,seq,COALESCE(op,''),COALESCE(path,''),COALESCE(from_path,''),COALESCE(to_path,''),COALESCE(rev,''),COALESCE(url,''),COALESCE(result,''),COALESCE(error,''),COALESCE(async_job_id,''),COALESCE(tag,''),COALESCE(entry_id,''),COALESCE(undo_result,''),COALESCE(undo_error,''),job_index,COALESCE(undo_job_id,'') FROM dbx_journal_ops WHERE batch_id=? ORDER BY seq`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DropboxJournalOp, 0)
	for rows.Next() {
		var o DropboxJournalOp
		var jobIndex sql.NullInt64
		if err := rows.Scan(&o.BatchID, &o.Seq, &o.Op, &o.Path, &o.FromPath, &o.ToPath, &o.Rev, &o.URL, &o.Result, &o.Error, &o.AsyncJobID, &o.Tag, &o.EntryID, &o.UndoResult, &o.UndoError, &jobIndex, &o.UndoJobID); err != nil {
			return nil, err
		}
		if jobIndex.Valid {
			index := int(jobIndex.Int64)
			o.JobIndex = &index
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
