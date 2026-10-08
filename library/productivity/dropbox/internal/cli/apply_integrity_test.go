package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func integrityPlan(t *testing.T, ops ...dropbox.Op) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(file, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: ops}); err != nil {
		t.Fatal(err)
	}
	return file
}

func integrityJournal(t *testing.T, dbPath, batchID string) []store.DropboxJournalOp {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ops, err := db.ListDropboxJournalOps(context.Background(), batchID)
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

func TestApplyJournalsIntentAndUnknownPoll(t *testing.T) {
	testenv.Isolate(t)
	file := fixtureRow("/A/file.txt", "file", "hash", "", 2)
	file.ID = "id-file"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), file)
	var sawPending bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"}}`)
		case "/files/move_batch_v2":
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				t.Error(err)
				return
			}
			var state, tag, id, rev string
			err = db.DB().QueryRow(`SELECT result,tag,entry_id,rev FROM dbx_journal_ops WHERE op='move'`).Scan(&state, &tag, &id, &rev)
			db.Close()
			if err != nil {
				t.Error(err)
			}
			sawPending = state == "pending" && tag == "file" && id == "id-file" && rev == "rev:file.txt"
			io.WriteString(w, `{".tag":"async_job_id","async_job_id":"job-42"}`)
		case "/files/move_batch/check_v2":
			w.WriteHeader(http.StatusConflict)
			io.WriteString(w, `{"error_summary":"path/not_found/..."}`)
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	data, err := runRead(t, "apply", integrityPlan(t, dropbox.Op{Op: "move", From: "/A/file.txt", To: "/B/file.txt"}), "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err == nil || !strings.Contains(err.Error(), "do not re-apply; run `dropbox-pp-cli index` then inspect `dropbox-pp-cli journal") {
		t.Fatalf("err=%v data=%s", err, data)
	}
	var result applyResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	ops := integrityJournal(t, dbPath, result.BatchID)
	if !sawPending || result.Counts.Move.Unknown != 1 || len(ops) != 1 || ops[0].Result != "unknown" || ops[0].AsyncJobID != "job-42" || ops[0].JobIndex == nil || *ops[0].JobIndex != 0 {
		t.Fatalf("pending=%t result=%+v ops=%+v", sawPending, result, ops)
	}
}

func TestApplyDeleteUsesIndexedRevisionAndIdentity(t *testing.T) {
	testenv.Isolate(t)
	file := fixtureRow("/A/f.txt", "file", "hash", "", 9)
	file.ID = "id-f"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), file)
	var parentRev string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"}}`)
		case "/files/delete_batch":
			var body struct {
				Entries []struct {
					ParentRev string `json:"parent_rev"`
				} `json:"entries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body.Entries) == 1 {
				parentRev = body.Entries[0].ParentRev
			}
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	data, err := runRead(t, "apply", integrityPlan(t, dropbox.Op{Op: "delete", Path: "/A/f.txt"}), "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("err=%v data=%s", err, data)
	}
	var result applyResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	ops := integrityJournal(t, dbPath, result.BatchID)
	if parentRev != "rev:f.txt" || len(ops) != 1 || ops[0].Tag != "file" || ops[0].EntryID != "id-f" || ops[0].Rev != "rev:f.txt" {
		t.Fatalf("parentRev=%s ops=%+v", parentRev, ops)
	}
}

func TestApplyIndexErrorKeepsRemoteSuccessAndForcesRefresh(t *testing.T) {
	testenv.Isolate(t)
	file := fixtureRow("/A/f.txt", "file", "", "", 1)
	file.ID = "id-f"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), file)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`CREATE TRIGGER fail_move BEFORE UPDATE OF path_lower ON dbx_files BEGIN SELECT RAISE(ABORT,'index update failed'); END`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	lists := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"}}`)
		case "/files/move_batch_v2":
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		case "/files/list_folder":
			lists++
			var req struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			switch req.Path {
			case "":
				io.WriteString(w, `{"cursor":"root","has_more":false,"entries":[{".tag":"folder","id":"a","name":"A","path_lower":"/a","path_display":"/A"},{".tag":"folder","id":"b","name":"B","path_lower":"/b","path_display":"/B"}]}`)
			case "/a":
				io.WriteString(w, `{"cursor":"a","has_more":false,"entries":[]}`)
			case "/b":
				io.WriteString(w, `{"cursor":"b","has_more":false,"entries":[{".tag":"file","id":"id-f","name":"f.txt","path_lower":"/b/f.txt","path_display":"/B/f.txt","rev":"rev:f.txt","size":1}]}`)
			}
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	plan := integrityPlan(t, dropbox.Op{Op: "move", From: "/A/f.txt", To: "/B/f.txt"})
	data, err := runRead(t, "apply", plan, "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err == nil {
		t.Fatalf("expected index error: %s", data)
	}
	var result applyResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	ops := integrityJournal(t, dbPath, result.BatchID)
	if len(ops) != 1 || ops[0].Result != "ok" || !strings.Contains(ops[0].Error, "index_error") {
		t.Fatalf("ops=%+v", ops)
	}
	db, err = store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	stale, _, err := db.GetDropboxMeta(context.Background(), "index_stale")
	db.Close()
	if err != nil || stale != "1" {
		t.Fatalf("stale=%q err=%v", stale, err)
	}
	_, err = runRead(t, "apply", plan, "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err == nil || lists == 0 {
		t.Fatalf("forced refresh missing: lists=%d err=%v", lists, err)
	}
}

func TestApplyPreviewListsRequiredFlagsAndIrreversibleRevokes(t *testing.T) {
	testenv.Isolate(t)
	shared := fixtureRow("/shared", "folder", "", "", 0)
	shared.SharedFolderID = "s"
	from := fixtureRow("/source/f", "file", "", "", 1)
	from.ParentSharedFolderID = "s"
	dbPath := seedIndex(t, fixtureRow("/repo", "folder", "", "", 0), fixtureRow("/repo/node_modules", "folder", "", "", 0), fixtureRow("/repo/node_modules/a", "file", "", "", 1), shared, fixtureRow("/source", "folder", "", "", 0), from, fixtureRow("/dest", "folder", "", "", 0))
	plan := integrityPlan(t, dropbox.Op{Op: "delete", Path: "/repo"}, dropbox.Op{Op: "delete", Path: "/shared"}, dropbox.Op{Op: "move", From: "/source/f", To: "/dest/f"}, dropbox.Op{Op: "revoke_link", URL: "url"})
	data, _ := runRead(t, "apply", plan, "--db", dbPath, "--json")
	var preview applyPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--allow-dev-dirs", "--allow-cross-share", "--allow-nonempty-delete", "--allow-unshare"} {
		if !strings.Contains(fmt.Sprint(preview.RequiresFlags), flag) {
			t.Fatalf("missing %s in %+v", flag, preview)
		}
	}
	if fmt.Sprint(preview.Irreversible) != "[revoke_link]" {
		t.Fatalf("irreversible=%v", preview.Irreversible)
	}
}

func TestApplyRetriesRateLimitedBatchCheck(t *testing.T) {
	testenv.Isolate(t)
	file := fixtureRow("/A/f", "file", "", "", 1)
	file.ID = "id-f"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), file)
	checks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"}}`)
		case "/files/move_batch_v2":
			io.WriteString(w, `{".tag":"async_job_id","async_job_id":"job"}`)
		case "/files/move_batch/check_v2":
			checks++
			if checks == 1 {
				w.WriteHeader(429)
				io.WriteString(w, `{"error_summary":"too_many_requests"}`)
			} else {
				io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
			}
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	data, err := runRead(t, "apply", integrityPlan(t, dropbox.Op{Op: "move", From: "/A/f", To: "/B/f"}), "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err != nil || checks < 2 {
		t.Fatalf("checks=%d err=%v data=%s", checks, err, data)
	}
}

func TestApplyNestedDeleteJournalsChildOnce(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/F", "folder", "", "", 0), fixtureRow("/A/F/G", "folder", "", "", 0), fixtureRow("/A/F/G/file", "file", "", "", 1))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/files/delete_batch" {
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"},{".tag":"success"}]}`)
			return
		}
		t.Errorf("unexpected API %s", r.URL.Path)
		w.WriteHeader(404)
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
	result, _ := executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/A/F"}, {Op: "delete", Path: "/A/F/G"}}}, "test", "pro", applyExecutionOptions{AllowNonemptyDelete: true})
	ops := integrityJournal(t, dbPath, result.BatchID)
	children := 0
	for _, op := range ops {
		if op.Op == "delete_child" && op.Path == "/A/F/G/file" {
			children++
		}
	}
	if children != 1 {
		t.Fatalf("children=%d ops=%+v", children, ops)
	}
}
