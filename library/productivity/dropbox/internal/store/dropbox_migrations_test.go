package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestDropboxSchemaAndPageTransaction(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureDropboxSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureDropboxSchema(ctx); err != nil {
		t.Fatal(err)
	}
	rows := []DropboxRow{{PathLower: "/a_1", Tag: "folder", Name: "A_1", PathDisplay: "/A_1"}, {PathLower: "/a_1/x", Tag: "file", Name: "x", ParentLower: "/a_1"}, {PathLower: "/ab1/y", Tag: "file", Name: "y", ParentLower: "/ab1"}}
	if err := s.UpsertDropboxEntries(ctx, "", rows); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDropboxIndexState(ctx, DropboxIndexState{Root: "", Cursor: "c1", Complete: true, Entries: 3}); err != nil {
		t.Fatal(err)
	}
	state, ok, err := s.GetDropboxIndexState(ctx, "")
	if err != nil || !ok || state.Cursor != "c1" || !state.Complete {
		t.Fatalf("state = %+v, %t, %v", state, ok, err)
	}
	if err := s.SetDropboxMeta(ctx, "account_type", "pro"); err != nil {
		t.Fatal(err)
	}
	value, ok, err := s.GetDropboxMeta(ctx, "account_type")
	if err != nil || !ok || value != "pro" {
		t.Fatalf("meta = %q, %t, %v", value, ok, err)
	}
	deleted, err := s.DeleteDropboxPathPrefix(ctx, "/a_1")
	if err != nil || deleted != 2 {
		t.Fatalf("deleted = %d, %v", deleted, err)
	}
	var n int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM dbx_files WHERE path_lower='/ab1/y'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("escaped prefix deleted sibling: %d, %v", n, err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM dbx_files_fts WHERE dbx_files_fts MATCH 'x'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("FTS stale after delete: %d, %v", n, err)
	}
}

func TestDropboxMovePrefixAndJournal(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureDropboxSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertDropboxEntries(ctx, "", []DropboxRow{{PathLower: "/old", PathDisplay: "/Old", Tag: "folder"}, {PathLower: "/old/child.txt", PathDisplay: "/Old/child.txt", ParentLower: "/old", Tag: "file", Rev: "r"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveDropboxPathPrefix(ctx, "/Old", "/New"); err != nil {
		t.Fatal(err)
	}
	var display, parent string
	if err := s.DB().QueryRowContext(ctx, `SELECT path_display,parent_lower FROM dbx_files WHERE path_lower='/new/child.txt'`).Scan(&display, &parent); err != nil {
		t.Fatal(err)
	}
	if display != "/New/child.txt" || parent != "/new" {
		t.Fatalf("moved child: %q %q", display, parent)
	}
	b := DropboxJournalBatch{ID: "test", CreatedAt: "2026-10-01T00:00:00Z", Status: "running", RestoreDays: 30}
	if err := s.CreateDropboxJournalBatch(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := s.AddDropboxJournalOp(ctx, DropboxJournalOp{BatchID: "test", Seq: 1, Op: "move", Result: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDropboxJournalStatus(ctx, "test", "complete"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetDropboxJournalBatch(ctx, "test")
	if err != nil || !ok || got.Status != "complete" {
		t.Fatalf("batch=%+v %t %v", got, ok, err)
	}
	ops, err := s.ListDropboxJournalOps(ctx, "test")
	if err != nil || len(ops) != 1 || ops[0].Result != "ok" {
		t.Fatalf("ops=%+v %v", ops, err)
	}
}

func TestDropboxMoveUnicodePrefixAndContext(t *testing.T) {
	for _, name := range []string{"Café", "📁"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "index.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if err := s.EnsureDropboxSchema(ctx); err != nil {
				t.Fatal(err)
			}
			from := "/Fotos/" + name
			rows := []DropboxRow{
				{PathLower: strings.ToLower(from), PathDisplay: from, Tag: "folder", Name: name, ParentLower: "/fotos", ParentSharedFolderID: "old-share"},
				{PathLower: strings.ToLower(from) + "/x.jpg", PathDisplay: from + "/x.jpg", Tag: "file", Name: "x.jpg", ParentLower: strings.ToLower(from), ParentSharedFolderID: "old-share"},
			}
			if err := s.UpsertDropboxEntries(ctx, "/fotos", rows[:2]); err != nil {
				t.Fatal(err)
			}
			if err := s.MoveDropboxPathPrefix(ctx, from, "/Dest"); err != nil {
				t.Fatal(err)
			}
			var display, parent, root, share string
			if err := s.DB().QueryRowContext(ctx, `SELECT path_display,parent_lower,root,COALESCE(parent_shared_folder_id,'') FROM dbx_files WHERE path_lower='/dest/x.jpg'`).Scan(&display, &parent, &root, &share); err != nil {
				t.Fatal(err)
			}
			if display != "/Dest/x.jpg" || parent != "/dest" || root != "/dest" || share != "" {
				t.Fatalf("moved child = %q %q %q %q", display, parent, root, share)
			}
		})
	}
}

func TestDropboxMoveRecomputesSharedFolderContext(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureDropboxSchema(ctx); err != nil {
		t.Fatal(err)
	}
	rows := []DropboxRow{
		{PathLower: "/old", PathDisplay: "/Old", Tag: "folder", Name: "Old"},
		{PathLower: "/old/folder", PathDisplay: "/Old/Folder", Tag: "folder", Name: "Folder", ParentLower: "/old", ParentSharedFolderID: "old-share"},
		{PathLower: "/old/folder/a", PathDisplay: "/Old/Folder/a", Tag: "file", Name: "a", ParentLower: "/old/folder", ParentSharedFolderID: "old-share"},
		{PathLower: "/shared", PathDisplay: "/Shared", Tag: "folder", Name: "Shared", SharedFolderID: "new-share"},
	}
	if err := s.UpsertDropboxEntries(ctx, "/old", rows); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveDropboxPathPrefix(ctx, "/Old/Folder", "/Shared/Folder"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/shared/folder", "/shared/folder/a"} {
		var root, share string
		if err := s.DB().QueryRowContext(ctx, `SELECT COALESCE(root,''),COALESCE(parent_shared_folder_id,'') FROM dbx_files WHERE path_lower=?`, path).Scan(&root, &share); err != nil {
			t.Fatal(err)
		}
		if root != "/shared" || share != "new-share" {
			t.Fatalf("%s root=%q share=%q", path, root, share)
		}
	}
}

func TestDropboxDevKindMigrationAndWrites(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.DB().ExecContext(ctx, `CREATE TABLE dbx_files(path_lower TEXT PRIMARY KEY, id TEXT, tag TEXT, name TEXT, path_display TEXT, parent_lower TEXT, rev TEXT, size INTEGER, content_hash TEXT, client_modified TEXT, server_modified TEXT, shared_folder_id TEXT, parent_shared_folder_id TEXT, is_downloadable INTEGER, root TEXT, seen_at TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE VIRTUAL TABLE dbx_files_fts USING fts5(name, path_display, content='dbx_files', content_rowid='rowid')`,
		`CREATE TRIGGER dbx_files_ai AFTER INSERT ON dbx_files BEGIN INSERT INTO dbx_files_fts(rowid,name,path_display) VALUES(new.rowid,new.name,new.path_display); END`,
		`CREATE TRIGGER dbx_files_ad AFTER DELETE ON dbx_files BEGIN INSERT INTO dbx_files_fts(dbx_files_fts,rowid,name,path_display) VALUES('delete',old.rowid,old.name,old.path_display); END`,
		`CREATE TRIGGER dbx_files_au AFTER UPDATE ON dbx_files BEGIN INSERT INTO dbx_files_fts(dbx_files_fts,rowid,name,path_display) VALUES('delete',old.rowid,old.name,old.path_display); INSERT INTO dbx_files_fts(rowid,name,path_display) VALUES(new.rowid,new.name,new.path_display); END`,
	} {
		if _, err := s.DB().ExecContext(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"/repo/node_modules", "/repo/node_modules/a.js", "/notes/node_modules_guide.txt"} {
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO dbx_files(path_lower,tag,name) VALUES(?,'file',?)`, p, dropboxBase(p)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := s.EnsureDropboxSchema(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for path, want := range map[string]string{"/repo/node_modules": "node_modules", "/repo/node_modules/a.js": "node_modules", "/notes/node_modules_guide.txt": ""} {
		var kind sql.NullString
		if err := s.DB().QueryRowContext(ctx, `SELECT dev_kind FROM dbx_files WHERE path_lower=?`, path).Scan(&kind); err != nil {
			t.Fatal(err)
		}
		if kind.String != want || kind.Valid != (want != "") {
			t.Fatalf("%s dev_kind=%v, want %q", path, kind, want)
		}
	}
	if err := s.UpsertDropboxEntries(ctx, "", []DropboxRow{{PathLower: "/repo/.git/head", Tag: "file"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyDropboxPage(ctx, "", []DropboxRow{{PathLower: "/repo/venv/a.py", Tag: "file"}}, nil, DropboxIndexState{}); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveDropboxPathPrefix(ctx, "/repo/.git", "/safe"); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveDropboxPathPrefix(ctx, "/repo/venv", "/repo/Pods"); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"/safe/head": "", "/repo/pods/a.py": "Pods"} {
		var kind sql.NullString
		if err := s.DB().QueryRowContext(ctx, `SELECT dev_kind FROM dbx_files WHERE path_lower=?`, path).Scan(&kind); err != nil {
			t.Fatal(err)
		}
		if kind.String != want || kind.Valid != (want != "") {
			t.Fatalf("moved %s dev_kind=%v, want %q", path, kind, want)
		}
	}
}

func TestDropboxConflictCandidateMigrationAndWrites(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.DB().ExecContext(ctx, `CREATE TABLE dbx_files(path_lower TEXT PRIMARY KEY, id TEXT, tag TEXT, name TEXT, path_display TEXT, parent_lower TEXT, rev TEXT, size INTEGER, content_hash TEXT, client_modified TEXT, server_modified TEXT, shared_folder_id TEXT, parent_shared_folder_id TEXT, is_downloadable INTEGER, root TEXT, seen_at TEXT, dev_kind TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ path, name string }{
		{"/docs/copy.txt", "Report (Dana's conflicted copy 2020-01-01).txt"},
		{"/docs/ordinary.txt", "ordinary.txt"},
	} {
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO dbx_files(path_lower,tag,name) VALUES(?,'file',?)`, row.path, row.name); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := s.EnsureDropboxSchema(ctx); err != nil {
			t.Fatal(err)
		}
	}
	check := func(path string, want int) {
		t.Helper()
		var got int
		if err := s.DB().QueryRowContext(ctx, `SELECT conflict_candidate FROM dbx_files WHERE path_lower=?`, path).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s candidate=%d, want %d", path, got, want)
		}
	}
	check("/docs/copy.txt", 1)
	check("/docs/ordinary.txt", 0)
	if err := s.UpsertDropboxEntries(ctx, "", []DropboxRow{{PathLower: "/docs/sync", Tag: "folder", Name: "Docs (Selective Sync Conflict)"}}); err != nil {
		t.Fatal(err)
	}
	check("/docs/sync", 1)
	if err := s.MoveDropboxPathPrefix(ctx, "/docs/sync", "/docs/plain"); err != nil {
		t.Fatal(err)
	}
	check("/docs/plain", 0)
	if _, err := s.DB().ExecContext(ctx, `UPDATE dbx_files SET name='Case Conflict note' WHERE path_lower='/docs/plain'`); err != nil {
		t.Fatal(err)
	}
	check("/docs/plain", 1)
	for name, fragment := range map[string]string{"dbx_files_duplicate_candidates": "dev_kind", "dbx_files_file_path_size": "content_hash"} {
		var ddl string
		if err := s.DB().QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE name=?`, name).Scan(&ddl); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(ddl, fragment) {
			t.Fatalf("%s is not covering: %s", name, ddl)
		}
	}
}

func TestDropboxJournalMigrationAndCancelledContextWrites(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DB().ExecContext(ctx, `CREATE TABLE dbx_journal_ops(batch_id TEXT, seq INTEGER, op TEXT, path TEXT, from_path TEXT, to_path TEXT, rev TEXT, url TEXT, result TEXT, error TEXT, async_job_id TEXT, PRIMARY KEY(batch_id,seq))`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.EnsureDropboxSchema(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for table, wanted := range map[string][]string{"dbx_journal_batches": {"account_id"}, "dbx_journal_ops": {"job_index", "undo_job_id"}} {
		rows, err := s.DB().QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for rows.Next() {
			var cid, notNull, pk int
			var name, typ string
			var def sql.NullString
			if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
				t.Fatal(err)
			}
			found[name] = true
		}
		rows.Close()
		for _, name := range wanted {
			if !found[name] {
				t.Errorf("%s missing %s", table, name)
			}
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.CreateDropboxJournalBatch(cancelled, DropboxJournalBatch{ID: "b", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddDropboxJournalOp(cancelled, DropboxJournalOp{BatchID: "b", Seq: 1, Op: "delete", Path: "/f", Tag: "file", EntryID: "id", Rev: "rev", Result: "pending"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDropboxJournalAsyncJobID(cancelled, "b", 1, "job", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDropboxJournalOpResult(cancelled, "b", 1, "unknown", "poll failed"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDropboxJournalUndoResult(cancelled, "b", 1, "skipped", "path occupied"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDropboxJournalStatus(cancelled, "b", "partial"); err != nil {
		t.Fatal(err)
	}
	ops, err := s.ListDropboxJournalOps(ctx, "b")
	if err != nil || len(ops) != 1 || ops[0].Tag != "file" || ops[0].EntryID != "id" || ops[0].AsyncJobID != "job" || ops[0].JobIndex == nil || *ops[0].JobIndex != 0 || ops[0].Result != "unknown" || ops[0].UndoResult != "skipped" {
		t.Fatalf("ops=%+v err=%v", ops, err)
	}
}
