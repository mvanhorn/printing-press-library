package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func TestConflictFolderDeleteRefusesSameSizeEditInCopy(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t,
		fixtureRow("/Share", "folder", "", "", 0),
		fixtureRow("/Share/x.txt", "file", "H3", "", 30),
		fixtureRow("/Share (Selective Sync Conflict)", "folder", "", "", 0),
		fixtureRow("/Share (Selective Sync Conflict)/x.txt", "file", "H3", "", 30),
	)
	planPath := filepath.Join(t.TempDir(), "conflicts.json")
	if _, err := runRead(t, "conflicts", "--db", dbPath, "--plan", planPath, "--json"); err != nil {
		t.Fatal(err)
	}
	plan, err := dropbox.ReadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].ExpectPathTreeHash == "" {
		t.Fatalf("copy tree is not pinned: %+v", plan.Ops)
	}
	if data, err := runRead(t, "plan", "check", planPath, "--db", dbPath, "--json"); err != nil {
		t.Fatalf("unchanged plan check: %v: %s", err, data)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB().Exec(`UPDATE dbx_files SET content_hash='edited' WHERE path_lower='/share (selective sync conflict)/x.txt'`); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "plan", "check", planPath, "--db", dbPath, "--json")
	if err == nil || !strings.Contains(string(data), "delete_tree_mismatch") {
		t.Fatalf("same-size copy edit passed plan check: err=%v data=%s", err, data)
	}
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/files/delete_batch") {
			deletes++
		}
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	result, _ := executeApply(context.Background(), c, db, nil, plan, planPath, "pro")
	if deletes != 0 || result.Counts.Delete.Skipped != 1 {
		t.Fatalf("apply deleted an edited copy: deletes=%d result=%+v", deletes, result)
	}
	var reason string
	if err := db.DB().QueryRow(`SELECT COALESCE(error,'') FROM dbx_journal_ops WHERE batch_id=? AND seq=1`, result.BatchID).Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reason, "changed since the plan") {
		t.Fatalf("skip reason = %q", reason)
	}
}

func TestUndoReconcilePollsUnknownOpsWithSavedJobID(t *testing.T) {
	testenv.Isolate(t)
	checks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/files/move_batch/check_v2" {
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		checks++
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "move", FromPath: "/A/f", ToPath: "/B/f", Tag: "file", EntryID: "id-f", Result: "unknown", Error: "poll failed", AsyncJobID: "job-unknown"}}
	db, poster, _ := undoFixture(t, dbPath, ops, "undo-unknown-job")
	defer db.Close()
	updated, err := reconcilePendingApply(context.Background(), db, poster, "original", "", false)
	if err != nil || checks != 1 || len(updated) != 1 || updated[0].Result != "ok" {
		t.Fatalf("unknown op with a saved job id was not polled: checks=%d updated=%+v err=%v", checks, updated, err)
	}
}

func TestUndoReconcileStopsWhenSavedJobStaysUnresolved(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		io.WriteString(w, `{"error_summary":"internal_error/..."}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "move", FromPath: "/A/f", ToPath: "/B/f", Tag: "file", EntryID: "id-f", Result: "unknown", AsyncJobID: "job-running"}}
	db, poster, _ := undoFixture(t, dbPath, ops, "undo-unresolved-job")
	defer db.Close()
	if _, err := reconcilePendingApply(context.Background(), db, poster, "original", "", false); err == nil || !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("undo proceeded past an unresolved job: err=%v", err)
	}
	after, err := db.ListDropboxJournalOps(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if after[0].Result != "unknown" || after[0].AsyncJobID != "job-running" {
		t.Fatalf("unresolved op lost its state: %+v", after[0])
	}
}

func TestUndoMoveInterruptedAfterSubmitResolvesOnRetry(t *testing.T) {
	testenv.Isolate(t)
	submits, checkFails := 0, true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/files/move_batch_v2":
			submits++
			io.WriteString(w, `{".tag":"async_job_id","async_job_id":"undo-job"}`)
		case "/files/move_batch/check_v2":
			if checkFails {
				w.WriteHeader(409)
				io.WriteString(w, `{"error_summary":"internal_error/..."}`)
				return
			}
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	moved := fixtureRow("/B/f.txt", "file", "", "", 1)
	moved.ID = "id-f"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), moved)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "move", FromPath: "/A/f.txt", ToPath: "/B/f.txt", Tag: "file", EntryID: "id-f", Result: "ok"}}
	db, poster, first := undoFixture(t, dbPath, ops, "undo-first")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), first, false); err == nil {
		t.Fatal("first undo reported success after its poll failed")
	}
	undoOps, err := db.ListDropboxJournalOps(context.Background(), "undo-first")
	if err != nil {
		t.Fatal(err)
	}
	if len(undoOps) != 1 || undoOps[0].Result != "unknown" || undoOps[0].AsyncJobID != "undo-job" {
		t.Fatalf("interrupted undo was not journaled with its job id: %+v", undoOps)
	}
	reloaded, err := db.ListDropboxJournalOps(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded[0].UndoResult != "unknown" || reloaded[0].UndoJobID != "undo-job" {
		t.Fatalf("original op lost the undo job: %+v", reloaded[0])
	}

	// Dropbox finished the reverse move; a refresh now shows the entry back home.
	if err := db.MoveDropboxPathPrefix(context.Background(), "/B/f.txt", "/A/f.txt"); err != nil {
		t.Fatal(err)
	}
	checkFails = false
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "undo-second", UndoOf: "original", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	second := &undoResult{BatchID: "undo-second", UndoOf: "original", Counts: map[string]int{}, Warnings: []string{}, Failures: []applyFailure{}}
	if err := executeUndo(context.Background(), db, poster, reloaded, second, false); err != nil || len(second.Failures) != 0 || submits != 1 {
		t.Fatalf("retry did not recognize the completed undo: err=%v result=%+v submits=%d", err, second, submits)
	}
	if !strings.Contains(strings.Join(second.Warnings, "\n"), "undo already completed") {
		t.Fatalf("warnings = %v", second.Warnings)
	}
	final, err := db.ListDropboxJournalOps(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if final[0].UndoResult != "ok" {
		t.Fatalf("original undo_result = %q", final[0].UndoResult)
	}
}

func TestUndoMovePendingWithoutJobIDRecognizesCompletedReverse(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	home := fixtureRow("/A/f.txt", "file", "", "", 1)
	home.ID = "id-f"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), home)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "move", FromPath: "/A/f.txt", ToPath: "/B/f.txt", Tag: "file", EntryID: "id-f", Result: "ok", UndoResult: "pending"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-crashed")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false); err != nil || result.Counts["skipped"] != 1 || len(result.Failures) != 0 {
		t.Fatalf("crashed undo retry reported a mismatch: err=%v result=%+v", err, result)
	}
}

func TestUndoRestoreIndexesFullMetadataUnderItsRoot(t *testing.T) {
	testenv.Isolate(t)
	restoreBody := `{"name":"report.txt","id":"id:restored","path_lower":"/docs/report.txt","path_display":"/Docs/report.txt","rev":"rev-old","size":4096,"content_hash":"hash-restored","server_modified":"2026-01-02T03:04:05Z","client_modified":"2026-01-01T00:00:00Z","is_downloadable":true}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/files/restore" {
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		io.WriteString(w, restoreBody)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/Docs", "folder", "", "", 0))
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/Docs/report.txt", Rev: "rev-old", Tag: "file", EntryID: "id:restored", Result: "ok"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-restore-meta")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false); err != nil || result.Counts["ok"] != 1 {
		t.Fatalf("err=%v result=%+v", err, result)
	}
	var id, hash, modified, root string
	var size int64
	if err := db.DB().QueryRow(`SELECT COALESCE(id,''),COALESCE(content_hash,''),COALESCE(server_modified,''),COALESCE(root,''),COALESCE(size,0) FROM dbx_files WHERE path_lower='/docs/report.txt'`).Scan(&id, &hash, &modified, &root, &size); err != nil {
		t.Fatal(err)
	}
	if id != "id:restored" || hash != "hash-restored" || size != 4096 || modified != "2026-01-02T03:04:05Z" || root != "/docs" {
		t.Fatalf("restored row id=%q hash=%q size=%d modified=%q root=%q", id, hash, size, modified, root)
	}
	if stale, _, err := db.GetDropboxMeta(context.Background(), "index_stale"); err != nil || stale == "1" {
		t.Fatalf("full metadata marked the index stale: %q %v", stale, err)
	}

	restoreBody = `{}`
	if err := db.AddDropboxJournalOp(context.Background(), store.DropboxJournalOp{BatchID: "original", Seq: 2, Op: "delete", Path: "/Docs/other.txt", Rev: "rev-2", Tag: "file", Result: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "undo-restore-bare", UndoOf: "original", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	bare := &undoResult{BatchID: "undo-restore-bare", UndoOf: "original", Counts: map[string]int{}, Warnings: []string{}, Failures: []applyFailure{}}
	if err := executeUndo(context.Background(), db, poster, []store.DropboxJournalOp{{BatchID: "original", Seq: 2, Op: "delete", Path: "/Docs/other.txt", Rev: "rev-2", Tag: "file", Result: "ok"}}, bare, false); err != nil || bare.Counts["ok"] != 1 {
		t.Fatal(err)
	}
	if stale, _, err := db.GetDropboxMeta(context.Background(), "index_stale"); err != nil || stale != "1" {
		t.Fatalf("metadata-less restore did not mark the index stale: %q %v", stale, err)
	}
}

func TestApplyFolderDeleteJournalsEmptySubfolders(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t,
		fixtureRow("/Docs", "folder", "", "", 0),
		fixtureRow("/Docs/old", "folder", "", "", 0),
		fixtureRow("/Docs/old/empty", "folder", "", "", 0),
		fixtureRow("/Docs/old/a.txt", "file", "H", "", 3),
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	result, err := executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/Docs/old"}}}, "test", "pro", applyExecutionOptions{AllowNonemptyDelete: true})
	if err != nil {
		t.Fatal(err)
	}
	ops, err := db.ListDropboxJournalOps(context.Background(), result.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	folders := 0
	for _, op := range ops {
		if op.Op == "delete_child" && op.Tag == "folder" && op.Path == "/Docs/old/empty" && op.Result == "ok" {
			folders++
		}
	}
	if folders != 1 {
		t.Fatalf("empty subfolder was not journaled: %+v", ops)
	}
}

func TestUndoFolderDeleteRecreatesSubfoldersParentFirst(t *testing.T) {
	testenv.Isolate(t)
	calls := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body struct {
			Paths []string `json:"paths"`
			Path  string   `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/files/create_folder_batch":
			calls = append(calls, "mkdir "+strings.Join(body.Paths, ","))
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		case "/files/restore":
			calls = append(calls, "restore "+body.Path)
			io.WriteString(w, `{}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/Docs", "folder", "", "", 0))
	ops := []store.DropboxJournalOp{
		{Seq: 1, Op: "delete", Path: "/Docs/old", Tag: "folder", Result: "ok"},
		{Seq: 2, Op: "delete_child", Path: "/Docs/old/empty", Tag: "folder", Result: "ok"},
		{Seq: 3, Op: "delete_child", Path: "/Docs/old/empty/inner", Tag: "folder", Result: "ok"},
		{Seq: 4, Op: "delete_child", Path: "/Docs/old/a.txt", Tag: "file", Rev: "ra", Result: "ok"},
	}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-subfolders")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false); err != nil || len(result.Failures) != 0 {
		t.Fatalf("err=%v result=%+v", err, result)
	}
	want := "mkdir /Docs/old|mkdir /Docs/old/empty|mkdir /Docs/old/empty/inner|restore /Docs/old/a.txt"
	if got := strings.Join(calls, "|"); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
	var kind string
	if err := db.DB().QueryRow(`SELECT tag FROM dbx_files WHERE path_lower='/docs/old/empty/inner'`).Scan(&kind); err != nil || kind != "folder" {
		t.Fatalf("recreated subfolder not indexed: %q %v", kind, err)
	}
}

func TestOrganizeReadsOnlyRowsUnderTheSourceFolder(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t,
		fixtureRow("/Inbox", "folder", "", "", 0),
		fixtureRow("/Inbox/a.jpg", "file", "", "2021-05-06T00:00:00Z", 1),
		fixtureRow("/Archive", "folder", "", "", 0),
		fixtureRow("/Archive/2021", "folder", "", "", 0),
		fixtureRow("/Archive/2021/a.jpg", "file", "", "2021-05-06T00:00:00Z", 1),
		fixtureRow("/Elsewhere", "folder", "", "", 0),
		fixtureRow("/Elsewhere/b.jpg", "file", "", "2021-05-06T00:00:00Z", 1),
	)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// A row the scan cannot decode: reading it means organize loaded rows
	// outside --under.
	if _, err := db.DB().Exec(`UPDATE dbx_files SET size='not-a-number' WHERE path_lower='/elsewhere/b.jpg'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "organize", "--match", "*.jpg", "--under", "/Inbox", "--to", "/Archive/{year}", "--tz", "UTC", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("organize scanned outside --under: %v", err)
	}
	var got organizeResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Matched != 1 || got.PlannedMoves != 0 || len(got.Collisions) != 1 || got.Collisions[0].Reason != "destination exists" {
		t.Fatalf("result = %+v", got)
	}
}

func TestOrganizeReportsMissingIndexInJSON(t *testing.T) {
	testenv.Isolate(t)
	missing := filepath.Join(t.TempDir(), "absent.db")
	data, err := runRead(t, "organize", "--match", "*.jpg", "--under", "/Inbox", "--to", "/Archive/{year}", "--db", missing, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		IndexMissing bool   `json:"index_missing"`
		Note         string `json:"note"`
		Matched      int    `json:"matched"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !got.IndexMissing || got.Note != missingIndexNote || got.Matched != 0 {
		t.Fatalf("missing index looked like an empty result: %s", data)
	}
}

func TestUndoRetryLeavesReplacementAtOldDestinationAlone(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/files/move_batch/check_v2" {
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	// The earlier undo finished, the user moved the restored file away, and a
	// different file now sits at the old destination.
	replacement := fixtureRow("/B/f.txt", "file", "H-new", "", 9)
	replacement.ID = "id-replacement"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), replacement)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "move", FromPath: "/A/f.txt", ToPath: "/B/f.txt", Tag: "file", EntryID: "id-f", Result: "ok", UndoResult: "unknown", UndoJobID: "undo-job"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-reused-dest")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false); err != nil || len(result.Failures) != 0 {
		t.Fatalf("err=%v result=%+v", err, result)
	}
	var id string
	if err := db.DB().QueryRow(`SELECT COALESCE(id,'') FROM dbx_files WHERE path_lower='/b/f.txt'`).Scan(&id); err != nil || id != "id-replacement" {
		t.Fatalf("replacement row was rewritten: id=%q err=%v", id, err)
	}
	var moved int
	if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files WHERE path_lower='/a/f.txt'`).Scan(&moved); err != nil || moved != 0 {
		t.Fatalf("unrelated row was moved to the original path: count=%d err=%v", moved, err)
	}
}

func TestUndoRetryMarksIndexStaleWhenIdentityUnknown(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), fixtureRow("/B/f.txt", "file", "H", "", 1))
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "move", FromPath: "/A/f.txt", ToPath: "/B/f.txt", Tag: "file", Result: "ok", UndoResult: "unknown", UndoJobID: "undo-job"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-unknown-identity")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files WHERE path_lower='/b/f.txt'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("row without identity was moved: count=%d err=%v", count, err)
	}
	if stale, _, err := db.GetDropboxMeta(context.Background(), "index_stale"); err != nil || stale != "1" {
		t.Fatalf("index not marked stale: %q %v", stale, err)
	}
}
