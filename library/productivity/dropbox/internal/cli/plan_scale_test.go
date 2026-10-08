package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func TestPlanCheckAndApplyPreviewScale(t *testing.T) {
	if testing.Short() {
		t.Skip("300k-entry index scale test")
	}
	testenv.Isolate(t)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "scale.db")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureDropboxSchema(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO dbx_files(path_lower,id,tag,name,path_display,parent_lower,rev,size,root) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{"/src", "/dst"} {
		if _, err := stmt.ExecContext(ctx, folder, folder, "folder", folder[1:], folder, "", "", int64(0), folder); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 300000; i++ {
		name := fmt.Sprintf("%06d.txt", i)
		path := "/src/" + name
		if _, err := stmt.ExecContext(ctx, path, fmt.Sprintf("id:%d", i), "file", name, path, "/src", "r", int64(1), "/src"); err != nil {
			t.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"", "/src", "/dst"} {
		if err := db.SetDropboxIndexState(ctx, store.DropboxIndexState{Root: root, Complete: true, LastFullAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
	}
	ops := make([]dropbox.Op, 0, 5000)
	for i := 0; i < 5000; i++ {
		from := fmt.Sprintf("/src/%06d.txt", i)
		if i%2 == 0 {
			ops = append(ops, dropbox.Op{Op: "move", From: from, To: fmt.Sprintf("/dst/%06d.txt", i)})
		} else {
			ops = append(ops, dropbox.Op{Op: "delete", Path: from})
		}
	}
	p := dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "scale", Ops: ops}
	planPath := filepath.Join(t.TempDir(), "scale.json")
	if err := dropbox.WritePlan(planPath, p); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	report, err := checkPlanAtIndex(ctx, db, p, dropbox.CheckOptions{MaxOps: 5000})
	checkDuration := time.Since(started)
	if err != nil || !report.OK || checkDuration >= 5*time.Second {
		t.Fatalf("check duration=%s errors=%d err=%v", checkDuration, report.Errors, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	started = time.Now()
	_, err = runRead(t, "apply", planPath, "--db", dbPath, "--json")
	previewDuration := time.Since(started)
	if err != nil || previewDuration >= 5*time.Second {
		t.Fatalf("preview duration=%s err=%v", previewDuration, err)
	}
	db, err = store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err = db.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err = tx.PrepareContext(ctx, `INSERT INTO dbx_files(path_lower,id,tag,name,path_display,parent_lower,rev,size,root) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]numberedOp, 0, 1000)
	for i := 0; i < 1000; i++ {
		folder := fmt.Sprintf("/empty-%04d", i)
		if _, err := stmt.ExecContext(ctx, folder, folder, "folder", folder[1:], folder, "", "", 0, folder); err != nil {
			t.Fatal(err)
		}
		items = append(items, numberedOp{seq: i + 1, op: dropbox.Op{Op: "delete", Path: folder}})
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateDropboxJournalBatch(ctx, store.DropboxJournalBatch{ID: "empty-folder-scale", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	result := &applyResult{BatchID: "empty-folder-scale"}
	runner := applyRun{ctx: ctx, db: db, result: result, nextSeq: len(items) + 1, priorDeletes: map[string]bool{}, failedMoveSources: map[string]bool{}}
	started = time.Now()
	active, children, err := runner.prepareChunk("delete", items)
	prepareDuration := time.Since(started)
	if err != nil || len(active) != len(items) || len(children) != 0 || prepareDuration >= 3*time.Second {
		t.Fatalf("1000 empty folder deletes: active=%d children=%d duration=%s err=%v", len(active), len(children), prepareDuration, err)
	}
	t.Logf("300k entries, 5000 ops: check=%s preview=%s; 1000 empty-folder prepare/child=%s", checkDuration, previewDuration, prepareDuration)
}

func TestApplyFolderDeleteChildJournalingScale(t *testing.T) {
	if testing.Short() {
		t.Skip("40k-child journal scale test")
	}
	testenv.Isolate(t)
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "children.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.EnsureDropboxSchema(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO dbx_files(path_lower,id,tag,name,path_display,parent_lower,rev,size,root) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]numberedOp, 0, 20)
	for folder := 0; folder < 20; folder++ {
		parent := fmt.Sprintf("/folder-%02d", folder)
		if _, err := stmt.ExecContext(ctx, parent, parent, "folder", parent[1:], parent, "", "", 0, parent); err != nil {
			t.Fatal(err)
		}
		items = append(items, numberedOp{seq: folder + 1, op: dropbox.Op{Op: "delete", Path: parent}})
		for child := 0; child < 2000; child++ {
			name := fmt.Sprintf("%04d.txt", child)
			path := parent + "/" + name
			if _, err := stmt.ExecContext(ctx, path, path, "file", name, path, parent, "r", 1, parent); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/delete_batch" {
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		io.WriteString(w, `{".tag":"complete","entries":[`)
		for i := 0; i < 20; i++ {
			if i > 0 {
				io.WriteString(w, ",")
			}
			io.WriteString(w, `{".tag":"success"}`)
		}
		io.WriteString(w, `]}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	result := &applyResult{BatchID: "scale", Failures: []applyFailure{}}
	if err := db.CreateDropboxJournalBatch(ctx, store.DropboxJournalBatch{ID: "scale", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	runner := applyRun{ctx: ctx, db: db, poster: dropboxBatchPoster{client: c}, result: result, nextSeq: 21, priorDeletes: map[string]bool{}, failedMoveSources: map[string]bool{}, allowNonemptyDelete: true}
	started := time.Now()
	active, children, err := runner.prepareChunk("delete", items)
	duration := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 20 || len(children) != 20 {
		t.Fatalf("active=%d children=%d", len(active), len(children))
	}
	if duration >= 3*time.Second {
		t.Fatalf("child journaling took %s", duration)
	}
	if _, err := runner.submitChunk("delete", active, children); err != nil {
		t.Fatal(err)
	}
	t.Logf("journaled 40k delete children in %s", duration)
}
