// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
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

// TestNovelApplyHelpWires smoke-tests that the apply command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelApplyHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"apply", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "apply"} {
		if !strings.Contains(help, want) {
			t.Fatalf("apply --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestApplyPreviewBatchesAndNoWrites(t *testing.T) {
	testenv.Isolate(t)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writes++; w.WriteHeader(500) }))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	rows := []store.DropboxRow{fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0)}
	ops := make([]dropbox.Op, 0, 2500)
	for i := 0; i < 2500; i++ {
		from := fmt.Sprintf("/A/%04d.txt", i)
		to := fmt.Sprintf("/B/%04d.txt", i)
		rows = append(rows, fixtureRow(from, "file", "", "", 1))
		ops = append(ops, dropbox.Op{Op: "move", From: from, To: to})
	}
	db := seedIndex(t, rows...)
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: ops}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "apply", planPath, "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var preview applyPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(preview.Would.MoveBatches) != "[1000 1000 500]" || writes != 0 {
		t.Fatalf("preview=%+v writes=%d", preview, writes)
	}
}

func TestApplyYesRefusesDogfoodWithoutRequests(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "mkdir", Path: "/Photos/2020"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "apply", planPath, "--yes", "--json")
	var refusal struct {
		Refused bool `json:"refused"`
	}
	if jsonErr := json.Unmarshal(data, &refusal); err != nil || jsonErr != nil || requests != 0 || !refusal.Refused {
		t.Fatalf("apply: %v, requests=%d, output=%s", err, requests, data)
	}
}

func TestApplyMkdirUsesTopLevelIndexRoot(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"},"root_info":{"root_namespace_id":"root","home_namespace_id":"home"}}`)
		case "/files/create_folder_batch":
			io.WriteString(w, `{ ".tag":"complete", "entries":[{".tag":"success"}] }`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/Photos", "folder", "", "", 0), fixtureRow("/Photos/2020", "folder", "", "", 0))
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "mkdir", Path: "/Photos/2020/07"}}}); err != nil {
		t.Fatal(err)
	}
	if data, err := runRead(t, "apply", planPath, "--yes", "--no-refresh", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("apply: %v %s", err, data)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var root string
	if err := db.DB().QueryRow(`SELECT root FROM dbx_files WHERE path_lower='/photos/2020/07'`).Scan(&root); err != nil || root != "/photos" {
		t.Fatalf("root=%q err=%v", root, err)
	}
}

func TestApplyPartialBatchAndUndo(t *testing.T) {
	testenv.Isolate(t)
	var moveBodies [][]byte
	checks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"},"root_info":{"root_namespace_id":"root","home_namespace_id":"home"}}`)
		case "/files/move_batch_v2":
			if r.Header.Get("Dropbox-API-Path-Root") == "" {
				t.Error("missing path root")
			}
			moveBodies = append(moveBodies, body)
			io.WriteString(w, `{".tag":"async_job_id","async_job_id":"job"}`)
		case "/files/move_batch/check_v2":
			checks++
			if checks%2 == 1 {
				io.WriteString(w, `{".tag":"in_progress"}`)
			} else if len(moveBodies) == 1 {
				io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"},{".tag":"failure","failure":{".tag":"from_lookup"}}]}`)
			} else {
				io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
			}
		case "/files/list_folder":
			var request struct {
				Path string `json:"path"`
			}
			_ = json.Unmarshal(body, &request)
			switch request.Path {
			case "":
				io.WriteString(w, `{"cursor":"root","has_more":false,"entries":[{".tag":"folder","id":"id-a","name":"A","path_lower":"/a","path_display":"/A"},{".tag":"folder","id":"id-b","name":"B","path_lower":"/b","path_display":"/B"}]}`)
			case "/a":
				io.WriteString(w, `{"cursor":"a","has_more":false,"entries":[{".tag":"file","id":"id-fail","name":"fail.txt","path_lower":"/a/fail.txt","path_display":"/A/fail.txt","rev":"rev:fail.txt","size":1}]}`)
			case "/b":
				io.WriteString(w, `{"cursor":"b","has_more":false,"entries":[{".tag":"folder","id":"id-folder","name":"Folder","path_lower":"/b/folder","path_display":"/B/Folder"},{".tag":"file","id":"id-child","name":"child.txt","path_lower":"/b/folder/child.txt","path_display":"/B/Folder/child.txt","rev":"rev:child.txt","size":1}]}`)
			default:
				t.Errorf("unexpected list path %q", request.Path)
			}
		default:
			t.Errorf("unexpected API %s body %s", r.URL.Path, body)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	folder := fixtureRow("/A/Folder", "folder", "", "", 0)
	folder.ID = "id-folder"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), folder, fixtureRow("/A/Folder/child.txt", "file", "", "", 1), fixtureRow("/A/fail.txt", "file", "", "", 1))
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "move", From: "/A/Folder", To: "/B/Folder"}, {Op: "move", From: "/A/fail.txt", To: "/B/fail.txt"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "apply", planPath, "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err == nil {
		t.Fatal("expected partial failure")
	}
	var result applyResult
	if jerr := json.Unmarshal(data, &result); jerr != nil {
		t.Fatalf("result %q: %v", data, jerr)
	}
	if result.Status != "partial" || result.Counts.Move.OK != 1 || result.Counts.Move.Failed != 1 {
		t.Fatalf("result=%+v", result)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var path string
	if err := db.DB().QueryRow(`SELECT path_display FROM dbx_files WHERE path_lower='/b/folder/child.txt'`).Scan(&path); err != nil || path != "/B/Folder/child.txt" {
		t.Fatalf("moved descendant: %q %v", path, err)
	}
	data, err = runRead(t, "undo", result.BatchID, "--db", dbPath, "--json")
	if err != nil || len(moveBodies) != 1 {
		t.Fatalf("undo preview made writes: %v, %d, %s", err, len(moveBodies), data)
	}
	data, err = runRead(t, "undo", result.BatchID, "--yes", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("undo: %v output %s", err, data)
	}
	if len(moveBodies) != 2 || !bytes.Contains(moveBodies[1], []byte(`"from_path":"/B/Folder"`)) || !bytes.Contains(moveBodies[1], []byte(`"to_path":"/A/Folder"`)) {
		t.Fatalf("move bodies: %s", moveBodies)
	}
	b, ok, err := db.GetDropboxJournalBatch(context.Background(), result.BatchID)
	if err != nil || !ok || b.Status != "undone" {
		t.Fatalf("original batch=%+v %t %v", b, ok, err)
	}
	_, err = runRead(t, "undo", result.BatchID, "--db", dbPath, "--json")
	if err == nil || !strings.Contains(err.Error(), "already undone") {
		t.Fatalf("second undo: %v", err)
	}
}

func TestFolderDeleteJournalsChildRevisionAndUndoRestores(t *testing.T) {
	testenv.Isolate(t)
	var restores []string
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"},"root_info":{"root_namespace_id":"root","home_namespace_id":"home"}}`)
		case "/files/delete_batch", "/files/create_folder_batch":
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		case "/files/restore":
			var body struct{ Path, Rev string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			restores = append(restores, body.Path+":"+body.Rev)
			io.WriteString(w, `{}`)
		case "/files/list_folder":
			var request struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			if request.Path == "" {
				io.WriteString(w, `{"cursor":"root","has_more":false,"entries":[{".tag":"folder","id":"id-a","name":"A","path_lower":"/a","path_display":"/A"}]}`)
			} else {
				io.WriteString(w, `{"cursor":"a","has_more":false,"entries":[]}`)
			}
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/Folder", "folder", "", "", 0), fixtureRow("/A/Folder/child.txt", "file", "", "", 1))
	planPath := filepath.Join(t.TempDir(), "delete.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/A/Folder"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "apply", planPath, "--yes", "--allow-nonempty-delete", "--no-refresh", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("apply: %v %s", err, data)
	}
	var result applyResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ops, err := db.ListDropboxJournalOps(context.Background(), result.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, op := range ops {
		if op.Op == "delete_child" && op.Path == "/A/Folder/child.txt" && op.Rev == "rev:child.txt" && op.Result == "ok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("journal ops: %+v", ops)
	}
	data, err = runRead(t, "undo", result.BatchID, "--yes", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("undo: %v %s", err, data)
	}
	if fmt.Sprint(restores) != "[/A/Folder/child.txt:rev:child.txt]" {
		t.Fatalf("restores: %v", restores)
	}
	if fmt.Sprint(calls[len(calls)-2:]) != "[/files/create_folder_batch /files/restore]" {
		t.Fatalf("call order: %v", calls)
	}
}

func TestApplyRequiresAllowDevDirsBeforeWrites(t *testing.T) {
	testenv.Isolate(t)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"}}`)
		case "/files/delete_batch":
			writes++
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	db := seedIndex(t, fixtureRow("/repo/node_modules/a.js", "file", "", "", 3))
	planPath := filepath.Join(t.TempDir(), "dev-delete.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/repo/node_modules/a.js"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "plan", "check", planPath, "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var report dropbox.CheckReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Warnings != 1 || report.Results[0].Code != "dev_dir" {
		t.Fatalf("check report: %+v", report)
	}
	data, err = runRead(t, "apply", planPath, "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var preview applyPreview
	if err := json.Unmarshal(data, &preview); err != nil || preview.Check.Warnings != 1 {
		t.Fatalf("preview: %s %v", data, err)
	}
	data, err = runRead(t, "apply", planPath, "--yes", "--no-refresh", "--db", db, "--json")
	if err == nil || !strings.Contains(err.Error(), "--allow-dev-dirs") || writes != 0 {
		t.Fatalf("refusal err=%v writes=%d report=%s", err, writes, data)
	}
	if err := json.Unmarshal(data, &report); err != nil || report.Warnings != 1 {
		t.Fatalf("refusal report: %s %v", data, err)
	}
	data, err = runRead(t, "apply", planPath, "--yes", "--allow-dev-dirs", "--no-refresh", "--db", db, "--json")
	if err != nil || writes != 1 {
		t.Fatalf("allowed apply err=%v writes=%d output=%s", err, writes, data)
	}
}
