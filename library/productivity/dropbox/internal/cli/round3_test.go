package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func TestApplySkipsDeleteDependingOnFailedMove(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t, fixtureRow("/Inbox", "folder", "", "", 0), fixtureRow("/Archive", "folder", "", "", 0), fixtureRow("/Inbox/a", "file", "", "", 3), fixtureRow("/Inbox/b", "file", "", "", 4))
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/files/move_batch_v2":
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"failure","failure":{".tag":"from_lookup"}},{".tag":"success"}]}`)
		case "/files/delete_batch":
			deletes++
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	c, _ := (&rootFlags{}).newClient()
	db, _ := store.OpenWithContext(context.Background(), dbPath)
	defer db.Close()
	zero, noBytes := 0, int64(0)
	p := dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "move", From: "/Inbox/a", To: "/Archive/a"}, {Op: "move", From: "/Inbox/b", To: "/Archive/b"}, {Op: "delete", Path: "/Inbox", ExpectFiles: &zero, ExpectBytes: &noBytes}}}
	result, _ := executeApply(context.Background(), c, db, nil, p, "test", "pro")
	if deletes != 0 || result.Counts.Delete.Skipped != 1 {
		t.Fatalf("deletes=%d result=%+v", deletes, result)
	}
	ops, _ := db.ListDropboxJournalOps(context.Background(), result.BatchID)
	if len(ops) != 3 || ops[2].Result != "skipped" || !strings.Contains(ops[2].Error, "depends on failed move") {
		t.Fatalf("ops=%+v", ops)
	}
	var count int
	if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files WHERE path_lower='/inbox/a'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("file trashed: count=%d err=%v", count, err)
	}
}

func TestAllowedNonemptyDeletePreviewIsProminent(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t, fixtureRow("/F", "folder", "", "", 0), fixtureRow("/F/child", "file", "", "", 7))
	plan := integrityPlan(t, dropbox.Op{Op: "delete", Path: "/F"})
	data, err := runRead(t, "apply", plan, "--allow-nonempty-delete", "--db", dbPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var preview applyPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Check.OK || len(preview.IrreversibleOrLarge) != 1 || !strings.Contains(preview.IrreversibleOrLarge[0], "delete_nonempty_allowed") {
		t.Fatalf("preview=%+v", preview)
	}
}

func TestJournalBindingSurvivesIndexRebindAndLegacyRequiresForce(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	dbPath := seedIndex(t)
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CreateDropboxJournalBatch(ctx, store.DropboxJournalBatch{ID: "bound", AccountID: "acct-a", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := bindDropboxIndexAccount(ctx, db, accountInfo{AccountID: "acct-b"}, true); err != nil {
		t.Fatal(err)
	}
	bound, _, err := db.GetDropboxJournalBatch(ctx, "bound")
	if err != nil || bound.AccountID != "acct-a" {
		t.Fatalf("batch=%+v err=%v", bound, err)
	}
	if _, err := checkJournalAccount(bound, "acct-b", true); err == nil || !strings.Contains(err.Error(), "stop and ask the user") || !strings.Contains(err.Error(), "in a terminal") {
		t.Fatalf("mismatch=%v", err)
	}
	if _, err := reconcilePendingApply(ctx, db, dropboxBatchPoster{}, "bound", "acct-b", true); err == nil || !strings.Contains(err.Error(), "this index/journal belongs") {
		t.Fatalf("reconcile mismatch=%v", err)
	}
	if _, err := db.DB().ExecContext(ctx, `INSERT INTO dbx_journal_batches(id,status) VALUES('legacy','complete')`); err != nil {
		t.Fatal(err)
	}
	legacy, _, err := db.GetDropboxJournalBatch(ctx, "legacy")
	if err != nil || !legacy.LegacyAccount {
		t.Fatalf("legacy=%+v err=%v", legacy, err)
	}
	if _, err := checkJournalAccount(legacy, "acct-b", false); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("legacy refusal=%v", err)
	}
	if warning, err := checkJournalAccount(legacy, "acct-b", true); err != nil || !strings.Contains(warning, "legacy") {
		t.Fatalf("warning=%q err=%v", warning, err)
	}
}

func TestUndoRefusesJournalFromPriorAccountAfterRebind(t *testing.T) {
	testenv.Isolate(t)
	accountID := "acct-a"
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			fmt.Fprintf(w, `{"account_id":%q,"account_type":{".tag":"pro"}}`, accountID)
		case "/files/delete_batch":
			mutations++
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		default:
			mutations++
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/F", "folder", "", "", 0), fixtureRow("/F/file", "file", "", "", 1))
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetDropboxMeta(context.Background(), "account_id", "acct-a"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	data, err := runRead(t, "apply", integrityPlan(t, dropbox.Op{Op: "delete", Path: "/F/file"}), "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("apply: %v %s", err, data)
	}
	var applied applyResult
	if err := json.Unmarshal(data, &applied); err != nil {
		t.Fatal(err)
	}
	db, err = store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err := db.GetDropboxJournalBatch(context.Background(), applied.BatchID)
	if err != nil || batch.AccountID != "acct-a" {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	accountID = "acct-b"
	if err := bindDropboxIndexAccount(context.Background(), db, accountInfo{AccountID: "acct-b"}, true); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, err = runRead(t, "undo", applied.BatchID, "--yes", "--db", dbPath, "--json")
	if err == nil || !strings.Contains(err.Error(), "this index/journal belongs") || mutations != 1 {
		t.Fatalf("undo err=%v mutations=%d", err, mutations)
	}
}

func TestIndexRebindNeedsYesAndIndexIsNotLocalWrite(t *testing.T) {
	testenv.Isolate(t)
	if newIndexCmd(&rootFlags{}).Annotations["mcp:local-write"] != "" {
		t.Fatal("index reads the live Dropbox API")
	}
	_, err := runRead(t, "index", "--rebind", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if err == nil || !strings.Contains(err.Error(), "index --rebind clears the local index; run it yourself in a terminal") {
		t.Fatalf("rebind without yes: %v", err)
	}
	_, err = runRead(t, "index", "--rebind", "--yes", "--db", filepath.Join(t.TempDir(), "script.db"), "--json")
	if err == nil || !strings.Contains(err.Error(), "index --rebind clears the local index; run it yourself in a terminal") {
		t.Fatalf("non-terminal rebind with yes: %v", err)
	}
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	_, err = runRead(t, "index", "--rebind", "--yes", "--db", filepath.Join(t.TempDir(), "other.db"), "--json")
	if err == nil || !strings.Contains(err.Error(), "index --rebind clears the local index; run it yourself in a terminal") {
		t.Fatalf("harness rebind: %v", err)
	}
}

func TestApplySkipsDeleteInsideSkippedFolder(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/b.txt", "file", "", "", 7))
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deletes++
		io.WriteString(w, `{ ".tag":"complete", "entries":[{ ".tag":"success" }] }`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	wantFiles, wantBytes := 1, int64(7)
	p := dropbox.Plan{Source: "hand-written", Ops: []dropbox.Op{{Op: "delete", Path: "/A", ExpectFiles: &wantFiles, ExpectBytes: &wantBytes}, {Op: "delete", Path: "/A/b.txt"}}}
	check, err := checkPlanAtIndex(ctx, db, p, dropbox.CheckOptions{})
	if err != nil || !check.OK || len(check.Results) < 2 || check.Results[len(check.Results)-1].Status != "covered" {
		t.Fatalf("plan check=%+v err=%v", check, err)
	}
	if _, err := db.DB().ExecContext(ctx, `UPDATE dbx_files SET size=8 WHERE path_lower='/a/b.txt'`); err != nil {
		t.Fatal(err)
	}
	result, err := executeApply(ctx, c, db, nil, p, "test", "pro")
	if err != nil || deletes != 0 || result.Counts.Delete.Skipped != 2 {
		t.Fatalf("result=%+v deletes=%d err=%v", result, deletes, err)
	}
	ops, err := db.ListDropboxJournalOps(ctx, result.BatchID)
	if err != nil || len(ops) != 2 || ops[0].Result != "skipped" || ops[1].Result != "skipped" || !strings.Contains(ops[1].Error, "inside skipped delete") {
		t.Fatalf("journal=%+v err=%v", ops, err)
	}
	var count int
	if err := db.DB().QueryRowContext(ctx, `SELECT count(*) FROM dbx_files WHERE path_lower='/a/b.txt'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("child count=%d err=%v", count, err)
	}
}

func TestApplyFolderDeleteQueriesSearchPathIndex(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/b.txt", "file", "", "", 8))
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, tc := range []struct{ name, query string }{{"counts", deleteFolderCountsQuery}, {"children", deleteFolderChildrenQuery}} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := db.DB().QueryContext(ctx, "EXPLAIN QUERY PLAN "+tc.query, "/a/", "/a0")
			if err != nil {
				t.Fatal(err)
			}
			var details []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					rows.Close()
					t.Fatal(err)
				}
				details = append(details, detail)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				t.Fatal(err)
			}
			plan := strings.Join(details, "; ")
			if !strings.Contains(plan, "SEARCH dbx_files USING") || !strings.Contains(plan, "INDEX dbx_files_file_path_size") || strings.Contains(plan, "SCAN dbx_files") {
				t.Fatalf("query plan: %s", plan)
			}
			t.Logf("EXPLAIN %s: %s", tc.name, plan)
		})
	}
}

func TestUploadRejectsSharedPlatformStoreAndLegacyConfigHardlink(t *testing.T) {
	testenv.Isolate(t)
	base := t.TempDir()
	for _, env := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(env, filepath.Join(base, env))
	}
	paths, err := platform.PathsFor("upload-guard", "dropbox-pp-cli", "dropbox")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Dir(filepath.Dir(filepath.Dir(paths.ConfigFile))), filepath.Dir(filepath.Dir(filepath.Dir(paths.DataFile))), filepath.Dir(filepath.Dir(paths.StateDir)), filepath.Dir(filepath.Dir(filepath.Dir(paths.CacheDir)))} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "ordinary.txt")
		if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := safeUploadSource(file, &rootFlags{}); err == nil {
			t.Errorf("accepted %s", file)
		}
	}
	legacy, err := config.LegacyConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	hardlink := filepath.Join(t.TempDir(), "ordinary.txt")
	if err := os.Link(legacy, hardlink); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(hardlink)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := validateUploadHandle(f, hardlink, &rootFlags{}); err == nil {
		t.Fatal("accepted legacy config hardlink")
	}
}

func TestApplySkipsFolderDeleteWhenPostMoveCountDiffers(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t, fixtureRow("/Inbox", "folder", "", "", 0), fixtureRow("/Inbox/a", "file", "", "", 3))
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deletes++
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	c, _ := (&rootFlags{}).newClient()
	db, _ := store.OpenWithContext(context.Background(), dbPath)
	defer db.Close()
	zero, noBytes := 0, int64(0)
	result, _ := executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/Inbox", ExpectFiles: &zero, ExpectBytes: &noBytes}}}, "test", "pro")
	if deletes != 0 || result.Counts.Delete.Skipped != 1 {
		t.Fatalf("deletes=%d result=%+v", deletes, result)
	}
}

func TestUndoUnknownFolderRetryRestoresPendingChildren(t *testing.T) {
	testenv.Isolate(t)
	fails := map[string]bool{"/A/F/2": true, "/A/F/4": true}
	restored := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/files/create_folder_batch":
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		case "/files/restore":
			var body struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			restored[body.Path]++
			if fails[body.Path] {
				w.WriteHeader(409)
				io.WriteString(w, `{"error_summary":"path/not_found/..."}`)
			} else {
				io.WriteString(w, `{}`)
			}
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0))
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/A/F", Tag: "folder", EntryID: "old-folder", Result: "unknown"}}
	for i := 1; i <= 5; i++ {
		ops = append(ops, store.DropboxJournalOp{Seq: i + 1, Op: "delete_child", Path: fmt.Sprintf("/A/F/%d", i), Tag: "file", Rev: fmt.Sprintf("r%d", i), EntryID: fmt.Sprintf("old-%d", i), Result: "unknown"})
	}
	db, poster, first := undoFixture(t, dbPath, ops, "undo-1")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), first, false); err == nil {
		t.Fatalf("first undo should be partial: %+v", first)
	}
	if first.Counts["failed"] != 2 {
		t.Fatalf("first=%+v", first)
	}
	if _, err := db.DB().Exec(`UPDATE dbx_files SET id='recreated-folder' WHERE path_lower='/a/f'`); err != nil {
		t.Fatal(err)
	}
	folder, found, err := indexEntryForUndo(context.Background(), db, "/A/F")
	if err != nil || !found || folder.EntryID == "old-folder" {
		t.Fatalf("recreated folder=%+v found=%t err=%v", folder, found, err)
	}
	for k := range fails {
		fails[k] = false
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "undo-2", UndoOf: "original", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := db.ListDropboxJournalOps(context.Background(), "original")
	second := &undoResult{BatchID: "undo-2", UndoOf: "original", Counts: map[string]int{}, Warnings: []string{}, Failures: []applyFailure{}}
	if err := executeUndo(context.Background(), db, poster, reloaded, second, false); err != nil {
		t.Fatalf("retry: %v %+v", err, second)
	}
	if second.Counts["ok"] != 2 || restored["/A/F/2"] != 2 || restored["/A/F/4"] != 2 || restored["/A/F/1"] != 1 {
		t.Fatalf("retry=%+v restored=%v", second, restored)
	}
}

func TestUndoUnknownChildWithDifferentIdentityRemainsPending(t *testing.T) {
	testenv.Isolate(t)
	different := fixtureRow("/A/F/child", "file", "", "", 1)
	different.ID = "new-id"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/F", "folder", "", "", 0), different)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/A/F", Tag: "folder", EntryID: "old-folder", Result: "unknown", UndoResult: "ok"}, {Seq: 2, Op: "delete_child", Path: "/A/F/child", Tag: "file", EntryID: "old-id", Rev: "old-rev", Result: "unknown"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-child-identity")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false); err == nil || len(result.Failures) != 1 {
		t.Fatalf("different child must remain pending: err=%v result=%+v", err, result)
	}
	reloaded, err := db.ListDropboxJournalOps(context.Background(), "original")
	if err != nil || reloaded[1].UndoResult == "ok" || reloaded[1].UndoError != "path occupied" {
		t.Fatalf("child=%+v err=%v", reloaded[1], err)
	}
}

func TestReconcilePartiallyRecordedJobPreservesEntryOrder(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/delete_batch/check" {
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"},{".tag":"failure","failure":{".tag":"path"}},{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	fx := []store.DropboxJournalOp{
		{Seq: 1, Op: "delete", Path: "/A/1", Result: "ok", AsyncJobID: "job"},
		{Seq: 2, Op: "delete", Path: "/A/2", Result: "pending", AsyncJobID: "job"},
		{Seq: 3, Op: "delete", Path: "/A/3", Result: "pending", AsyncJobID: "job"},
	}
	db, poster, _ := undoFixture(t, dbPath, fx, "undo-reconcile-round3")
	defer db.Close()
	updated, err := reconcilePendingApply(context.Background(), db, poster, "original", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint([]string{updated[0].Result, updated[1].Result, updated[2].Result}) != "[ok failed ok]" {
		t.Fatalf("results=%+v", updated)
	}
}

func TestReconcileUsesPersistedJobIndexWhenEarlierIDWasNotSaved(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"},{".tag":"failure","failure":{".tag":"path"}}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	index := 1
	ops := []store.DropboxJournalOp{{Seq: 2, Op: "delete", Path: "/A/2", Result: "pending", AsyncJobID: "job", JobIndex: &index}}
	db, poster, _ := undoFixture(t, dbPath, ops, "undo-sparse-job")
	defer db.Close()
	got, err := reconcilePendingApply(context.Background(), db, poster, "original", "", false)
	if err != nil || len(got) != 1 || got[0].Result != "failed" {
		t.Fatalf("result=%+v err=%v", got, err)
	}
}
