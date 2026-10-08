package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func undoFixture(t *testing.T, dbPath string, ops []store.DropboxJournalOp, batchID string) (*store.Store, dropboxBatchPoster, *undoResult) {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "original", Status: "complete"}); err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		op.BatchID = "original"
		if err := db.AddDropboxJournalOp(context.Background(), op); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: batchID, UndoOf: "original", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	return db, dropboxBatchPoster{client: c}, &undoResult{BatchID: batchID, UndoOf: "original", Counts: map[string]int{}, Warnings: []string{}, Failures: []applyFailure{}}
}

func TestUndoReverseExecutionOrderAndIdentity(t *testing.T) {
	testenv.Isolate(t)
	calls := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/files/restore":
			io.WriteString(w, `{}`)
		case "/files/move_batch_v2":
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	moved := fixtureRow("/B/f", "file", "", "", 1)
	moved.ID = "id-f"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), moved)
	ops := []store.DropboxJournalOp{
		{Seq: 1, Op: "mkdir", Path: "/A/new", Tag: "folder", Result: "ok"},
		{Seq: 2, Op: "move", FromPath: "/A/f", ToPath: "/B/f", Tag: "file", EntryID: "id-f", Result: "ok"},
		{Seq: 3, Op: "delete", Path: "/A/g", Tag: "file", Rev: "r-g", Result: "ok"},
		{Seq: 4, Op: "revoke_link", URL: "url", Result: "ok"},
	}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-1")
	defer db.Close()
	if err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false); err != nil {
		t.Fatalf("undo: %v %+v", err, result)
	}
	if fmt.Sprint(calls) != "[/files/restore /files/move_batch_v2]" {
		t.Fatalf("calls=%v", calls)
	}
	undoOps, err := db.ListDropboxJournalOps(context.Background(), "undo-1")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, op := range undoOps {
		names = append(names, op.Op)
	}
	if fmt.Sprint(names) != "[revoke_link delete move mkdir]" {
		t.Fatalf("undo order=%v", names)
	}
}

func opsWithBatch(ops []store.DropboxJournalOp) []store.DropboxJournalOp {
	out := append([]store.DropboxJournalOp(nil), ops...)
	for i := range out {
		out[i].BatchID = "original"
	}
	return out
}

func TestUndoSkipsChangedMoveAndOccupiedRestore(t *testing.T) {
	testenv.Isolate(t)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writes++; w.WriteHeader(500) }))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	changed := fixtureRow("/B/f", "file", "", "", 1)
	changed.ID = "replacement"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), changed, fixtureRow("/A/occupied", "file", "", "", 1))
	ops := []store.DropboxJournalOp{
		{Seq: 1, Op: "move", FromPath: "/A/f", ToPath: "/B/f", Tag: "file", EntryID: "original-id", Result: "ok"},
		{Seq: 2, Op: "delete", Path: "/A/occupied", Tag: "file", Rev: "r", Result: "ok"},
	}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-1")
	defer db.Close()
	err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false)
	if err == nil || writes != 0 || result.Counts["skipped"] != 2 {
		t.Fatalf("err=%v writes=%d result=%+v", err, writes, result)
	}
	original, err := db.ListDropboxJournalOps(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(original[0].UndoError, "entry_id") || original[1].UndoError != "path occupied" {
		t.Fatalf("original=%+v", original)
	}
}

func TestUndoFolderConflictRestoresEveryChildAndRetriesOnlyPending(t *testing.T) {
	testenv.Isolate(t)
	failB := true
	calls := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/files/create_folder_batch":
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"failure","failure":{".tag":"path","path":{".tag":"conflict","conflict":{".tag":"folder"}}}}]}`)
		case "/files/restore":
			var body struct {
				Path string `json:"path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Path == "/A/F/b" && failB {
				w.WriteHeader(409)
				io.WriteString(w, `{"error_summary":"path/not_found/..."}`)
			} else {
				io.WriteString(w, `{}`)
			}
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0))
	ops := []store.DropboxJournalOp{
		{Seq: 1, Op: "delete", Path: "/A/F", Tag: "folder", Result: "ok"},
		{Seq: 4, Op: "delete_child", Path: "/A/F/a", Tag: "file", Rev: "ra", Result: "ok"},
		{Seq: 5, Op: "delete_child", Path: "/A/F/b", Tag: "file", Rev: "rb", Result: "ok"},
	}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-1")
	defer db.Close()
	err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false)
	if err == nil || result.Counts["ok"] != 2 || result.Counts["failed"] != 1 || len(result.Failures) != 1 {
		t.Fatalf("err=%v result=%+v", err, result)
	}
	if fmt.Sprint(calls) != "[/files/create_folder_batch /files/restore /files/restore]" {
		t.Fatalf("calls=%v", calls)
	}
	failB = false
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "undo-2", UndoOf: "original", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := db.ListDropboxJournalOps(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	second := &undoResult{BatchID: "undo-2", UndoOf: "original", Counts: map[string]int{}, Warnings: []string{}, Failures: []applyFailure{}}
	if err := executeUndo(context.Background(), db, poster, reloaded, second, false); err != nil {
		t.Fatalf("retry: %v %+v", err, second)
	}
	if second.Counts["ok"] != 1 || len(calls) != 4 || calls[3] != "/files/restore" {
		t.Fatalf("retry calls=%v result=%+v", calls, second)
	}
}

func TestUndoUnknownOpsOnlyWhenIndexShowsApplied(t *testing.T) {
	testenv.Isolate(t)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writes++
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), fixtureRow("/A/still", "file", "", "", 1))
	ops := []store.DropboxJournalOp{
		{Seq: 1, Op: "move", FromPath: "/A/f", ToPath: "/B/f", Tag: "file", EntryID: "id", Result: "unknown"},
		{Seq: 2, Op: "delete", Path: "/A/still", Tag: "file", Rev: "r", Result: "unknown"},
	}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-1")
	defer db.Close()
	_ = executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false)
	if writes != 0 || result.Counts["skipped"] != 2 {
		t.Fatalf("writes=%d result=%+v", writes, result)
	}
}
