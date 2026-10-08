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
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func TestRoundTwoApplyCaseRenameAndAbort(t *testing.T) {
	testenv.Isolate(t)
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/files/move_v2":
			io.WriteString(w, `{"metadata":{".tag":"file","id":"id-f"}}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	row := fixtureRow("/A/Name.txt", "file", "", "", 1)
	row.ID = "id-f"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), row)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	result, err := executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "move", From: "/A/Name.txt", To: "/A/name.txt"}}}, "plan", "pro")
	if err != nil || fmt.Sprint(paths) != "[/files/move_v2]" {
		t.Fatalf("result=%+v err=%v paths=%v", result, err, paths)
	}
	var display string
	if err := db.DB().QueryRow(`SELECT path_display FROM dbx_files WHERE path_lower='/a/name.txt'`).Scan(&display); err != nil || display != "/A/name.txt" {
		t.Fatalf("display=%q err=%v", display, err)
	}
	_, err = executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/missing"}}}, "plan", "pro")
	if err == nil || !strings.Contains(err.Error(), "do not re-apply") {
		t.Fatalf("abort err=%v", err)
	}
	var status string
	if err := db.DB().QueryRow(`SELECT status FROM dbx_journal_batches ORDER BY rowid DESC LIMIT 1`).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestRoundTwoApplyRevokeRechecksDangling(t *testing.T) {
	testenv.Isolate(t)
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sharing/get_shared_link_metadata":
			io.WriteString(w, `{"url":"url","path_lower":"/a/f"}`)
		case "/files/get_metadata":
			io.WriteString(w, `{".tag":"file","id":"id-f"}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0))
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	result, err := executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "revoke_link", URL: "url", ExpectDangling: true}}}, "plan", "pro")
	if err != nil || result.Counts.Revoke.Skipped != 1 || fmt.Sprint(paths) != "[/sharing/get_shared_link_metadata /files/get_metadata]" {
		t.Fatalf("result=%+v err=%v paths=%v", result, err, paths)
	}
}

func TestRoundTwoDanglingMetadataPathMissingCanRevoke(t *testing.T) {
	testenv.Isolate(t)
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sharing/get_shared_link_metadata":
			io.WriteString(w, `{"url":"url","path_lower":"/gone"}`)
		case "/files/get_metadata":
			w.WriteHeader(409)
			io.WriteString(w, `{"error_summary":"path/not_found/..."}`)
		case "/sharing/revoke_shared_link":
			io.WriteString(w, `{}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	result, err := executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "revoke_link", URL: "url", ExpectDangling: true}}}, "plan", "pro")
	if err != nil || result.Counts.Revoke.OK != 1 || fmt.Sprint(paths) != "[/sharing/get_shared_link_metadata /files/get_metadata /sharing/revoke_shared_link]" {
		t.Fatalf("result=%+v err=%v paths=%v", result, err, paths)
	}
}

func TestRoundTwoApplyPublicRevokeUsesMetadataFirst(t *testing.T) {
	testenv.Isolate(t)
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sharing/get_shared_link_metadata":
			io.WriteString(w, `{"url":"url","path_lower":"/a/f"}`)
		case "/sharing/revoke_shared_link":
			io.WriteString(w, `{}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	result, err := executeApply(context.Background(), c, db, nil, dropbox.Plan{Source: "test", Ops: []dropbox.Op{{Op: "revoke_link", URL: "url"}}}, "plan", "pro")
	if err != nil || result.Counts.Revoke.OK != 1 || fmt.Sprint(paths) != "[/sharing/get_shared_link_metadata /sharing/revoke_shared_link]" {
		t.Fatalf("result=%+v err=%v paths=%v", result, err, paths)
	}
}

func TestRoundTwoNoIndexAndPreviewFlags(t *testing.T) {
	testenv.Isolate(t)
	missing := filepath.Join(t.TempDir(), "missing.db")
	plan := integrityPlan(t, dropbox.Op{Op: "delete", Path: "/A"})
	for _, args := range [][]string{
		{"apply", plan, "--db", missing, "--json"},
		{"dupes", "--db", missing, "--plan", filepath.Join(t.TempDir(), "d.json"), "--json"},
		{"organize", "--match", "*.txt", "--under", "/A", "--to", "/B", "--db", missing, "--plan", filepath.Join(t.TempDir(), "o.json"), "--json"},
		{"links", "audit", "--db", missing, "--plan", filepath.Join(t.TempDir(), "l.json"), "--json"},
	} {
		data, err := runRead(t, args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("%v: err=%v data=%s", args, err, data)
		}
	}
	db := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/f", "file", "", "", 7))
	files, bytes := 1, int64(7)
	plan = integrityPlan(t, dropbox.Op{Op: "delete", Path: "/A", ExpectFiles: &files, ExpectBytes: &bytes})
	data, err := runRead(t, "apply", plan, "--db", db, "--allow-nonempty-delete", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var preview applyPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.RequiresFlags) != 0 || !strings.Contains(strings.Join(preview.IrreversibleOrLarge, ","), "delete_nonempty_attested") {
		t.Fatalf("preview=%+v", preview)
	}
	unattested := integrityPlan(t, dropbox.Op{Op: "delete", Path: "/A"})
	data, err = runRead(t, "apply", unattested, "--db", db, "--allow-nonempty-delete", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Check.OK || len(preview.RequiresFlags) != 0 {
		t.Fatalf("allowed preview=%+v", preview)
	}
}

func TestRoundTwoIncompleteRootAndCoveredPlan(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/f", "file", "", "", 1))
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: "/a", Complete: false}); err != nil {
		t.Fatal(err)
	}
	p := dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "move", From: "/A/f", To: "/B/f"}}}
	report, err := checkPlanAtIndex(context.Background(), db, p, dropbox.CheckOptions{RequireComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range report.Results {
		if r.Code == "index_incomplete" {
			found = true
		}
	}
	if !found {
		t.Fatalf("report=%+v", report)
	}
	out := filepath.Join(t.TempDir(), "covered.json")
	if _, err := outputPlan(out, "test", []dropbox.Op{{Op: "delete", Path: "/A"}, {Op: "delete", Path: "/A/f"}}, false, false); err != nil {
		t.Fatal(err)
	}
	written, err := dropbox.ReadPlan(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(written.Ops) != 1 {
		t.Fatalf("ops=%+v", written.Ops)
	}
	filtered := withoutCoveredDeletes([]dropbox.Op{{Op: "delete", Path: "/A/f"}, {Op: "delete", Path: "/A"}})
	if len(filtered) != 1 || filtered[0].Path != "/A" {
		t.Fatalf("child-first covered ops=%+v", filtered)
	}
}

func TestRoundTwoApplyNoRefreshRefusesIncompleteRoot(t *testing.T) {
	testenv.Isolate(t)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/users/get_current_account" {
			io.WriteString(w, `{"account_type":{".tag":"pro"}}`)
			return
		}
		writes++
		w.WriteHeader(500)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/f", "file", "", "", 1))
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: "/a", Complete: false}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	p := integrityPlan(t, dropbox.Op{Op: "delete", Path: "/A/f"})
	data, err := runRead(t, "apply", p, "--yes", "--no-refresh", "--db", dbPath, "--json")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(string(data), "index_incomplete") || writes != 0 {
		t.Fatalf("err=%v writes=%d data=%s", err, writes, data)
	}
}

func TestRoundTwoUploadNamesAndDownloadAnnotation(t *testing.T) {
	for _, name := range []string{"id_rsa", "id_rsa.pub", "id_ed25519", ".netrc", ".git-credentials", "cert.p12", "cert.pfx", "/tmp/.aws/credentials", "/tmp/.ssh/config"} {
		if !credentialLikeUploadName(name) {
			t.Errorf("allowed %s", name)
		}
	}
	cmd := newFilesDownloadCmd(&rootFlags{})
	if cmd.Annotations["mcp:hidden"] != "true" {
		t.Fatal("download not hidden over MCP")
	}
	tmp := filepath.Join(t.TempDir(), "safe.txt")
	if err := os.WriteFile(tmp, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(t.TempDir(), "config-home", "settings.data")
	if err := os.MkdirAll(filepath.Dir(protected), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protected, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "ordinary.txt")
	if err := os.Link(protected, alias); err != nil {
		t.Fatal(err)
	}
	handle, err := os.Open(alias)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err := validateUploadHandle(handle, alias, &rootFlags{configPath: protected}); err == nil {
		t.Fatal("hard link to config file accepted")
	}
	stateDir, err := cliutil.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(stateDir, "ordinary.txt")
	if err := os.WriteFile(stateFile, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := safeUploadSource(stateFile, &rootFlags{}); err == nil {
		t.Fatal("state directory file accepted")
	}
}

func TestRoundTwoLegacyAccountBindingAndRebind(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/f", "file", "", "", 1))
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SetDropboxMeta(context.Background(), "root_namespace_id", "root"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "kept", Status: "complete"}); err != nil {
		t.Fatal(err)
	}
	info := accountInfo{AccountID: "acct", RootNamespaceID: "root"}
	if err := bindDropboxIndexAccount(context.Background(), db, info, false); err != nil {
		t.Fatal(err)
	}
	bound, _, err := db.GetDropboxMeta(context.Background(), "account_id")
	if err != nil || bound != "acct" {
		t.Fatalf("bound=%q err=%v", bound, err)
	}
	if err := bindDropboxIndexAccount(context.Background(), db, accountInfo{AccountID: "other", RootNamespaceID: "other"}, true); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := db.DB().QueryRow("SELECT count(*) FROM dbx_files").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("files=%d err=%v", rows, err)
	}
	if err := db.DB().QueryRow("SELECT count(*) FROM dbx_index_state").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("state=%d err=%v", rows, err)
	}
	if err := db.DB().QueryRow("SELECT count(*) FROM dbx_journal_batches").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("journals=%d err=%v", rows, err)
	}
}

func TestRoundTwoUndoBenignUnknownMoveAndCaseRename(t *testing.T) {
	testenv.Isolate(t)
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/files/move_v2" {
			io.WriteString(w, `{"metadata":{".tag":"file","id":"id-case"}}`)
			return
		}
		t.Errorf("unexpected %s", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	unchanged := fixtureRow("/A/still", "file", "", "", 1)
	unchanged.ID = "id-still"
	renamed := fixtureRow("/A/name.txt", "file", "", "", 1)
	renamed.ID = "id-case"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), unchanged, renamed)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "move", FromPath: "/A/still", ToPath: "/B/still", EntryID: "id-still", Tag: "file", Result: "unknown"},
		{Seq: 2, Op: "move", FromPath: "/A/Name.txt", ToPath: "/A/name.txt", EntryID: "id-case", Tag: "file", Result: "ok"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-round2")
	defer db.Close()
	err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false)
	if err != nil || result.Counts["skipped"] != 1 || result.Counts["ok"] != 1 || fmt.Sprint(paths) != "[/files/move_v2]" {
		t.Fatalf("err=%v result=%+v paths=%v", err, result, paths)
	}
}

func TestRoundTwoUndoForceDoesNotOverwrite(t *testing.T) {
	testenv.Isolate(t)
	occupied := fixtureRow("/A/file", "file", "", "", 1)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), occupied)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/A/file", Rev: "old", Tag: "file", Result: "ok"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-force")
	defer db.Close()
	err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, true)
	if err == nil || result.Counts["skipped"] != 1 {
		t.Fatalf("force overwrote occupied path: err=%v result=%+v", err, result)
	}
}

func TestRoundTwoUndoOverwriteRestoresOccupiedFile(t *testing.T) {
	testenv.Isolate(t)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writes++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/files/restore" {
			io.WriteString(w, `{}`)
			return
		}
		t.Errorf("unexpected %s", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/file", "file", "", "", 1))
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/A/file", Rev: "old", Tag: "file", Result: "ok"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-overwrite")
	defer db.Close()
	err := executeUndoWithOptions(context.Background(), db, poster, opsWithBatch(ops), result, true)
	if err != nil || result.Counts["ok"] != 1 || writes != 1 {
		t.Fatalf("err=%v result=%+v writes=%d", err, result, writes)
	}
}

func TestRoundTwoUndoReconcilesPendingJobs(t *testing.T) {
	testenv.Isolate(t)
	checks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checks++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/files/delete_batch/check" {
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
			return
		}
		t.Errorf("unexpected %s", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/A/f", Tag: "file", Rev: "r", Result: "pending", AsyncJobID: "job"}}
	db, poster, _ := undoFixture(t, dbPath, ops, "undo-reconcile")
	defer db.Close()
	if err := db.SetDropboxJournalStatus(context.Background(), "original", "running"); err != nil {
		t.Fatal(err)
	}
	updated, err := reconcilePendingApply(context.Background(), db, poster, "original", "", false)
	if err != nil || checks != 1 || len(updated) != 1 || updated[0].Result != "ok" {
		t.Fatalf("checks=%d updated=%+v err=%v", checks, updated, err)
	}
}

func TestRoundTwoUndoAlreadyRestoredChildIsBenign(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/files/create_folder_batch" {
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
			return
		}
		t.Errorf("unexpected %s", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	child := fixtureRow("/A/F/f", "file", "", "", 1)
	child.ID = "id-child"
	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), child)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/A/F", Tag: "folder", Result: "ok"},
		{Seq: 2, Op: "delete_child", Path: "/A/F/f", Tag: "file", EntryID: "id-child", Rev: "r", Result: "ok"}}
	db, poster, result := undoFixture(t, dbPath, ops, "undo-child")
	defer db.Close()
	err := executeUndo(context.Background(), db, poster, opsWithBatch(ops), result, false)
	if err != nil || result.Counts["skipped"] != 1 || len(result.Failures) != 0 || len(result.Warnings) == 0 {
		t.Fatalf("err=%v result=%+v", err, result)
	}
}

func TestRoundTwoUndoGoneAsyncJobBecomesUnknown(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		io.WriteString(w, `{"error_summary":"async_job_id/not_found/..."}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := seedIndex(t)
	ops := []store.DropboxJournalOp{{Seq: 1, Op: "delete", Path: "/A/f", Tag: "file", Rev: "r", Result: "pending", AsyncJobID: "gone"}}
	db, poster, _ := undoFixture(t, dbPath, ops, "undo-gone")
	defer db.Close()
	updated, err := reconcilePendingApply(context.Background(), db, poster, "original", "", false)
	if err != nil || len(updated) != 1 || updated[0].Result != "unknown" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
}
