package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func TestReadCommandsScale(t *testing.T) {
	if testing.Short() {
		t.Skip("large SQLite fixture")
	}
	testenv.Isolate(t)
	quotaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"used":20000000,"allocation":{".tag":"individual","allocated":100000000}}`))
	}))
	defer quotaServer.Close()
	t.Setenv("DROPBOX_BASE_URL", quotaServer.URL)
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
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO dbx_files(path_lower,tag,name,path_display,parent_lower,rev,size,content_hash,client_modified,shared_folder_id) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5000; i++ {
		folder := fmt.Sprintf("/Folder%04d", i)
		if _, err := stmt.ExecContext(ctx, strings.ToLower(folder), "folder", folder[1:], folder, "", "", 0, "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 200000; i++ {
		folderIndex := i % 5000
		name := fmt.Sprintf("file%06d.txt", i)
		switch {
		case i < 200:
			folderIndex = i
			name = fmt.Sprintf("report%03d (Dana's conflicted copy 2020-01-01).txt", i)
		case i < 400:
			folderIndex = i - 200
			name = fmt.Sprintf("report%03d.txt", i-200)
		}
		folder := fmt.Sprintf("/Folder%04d", folderIndex)
		path := folder + "/" + name
		hash := fmt.Sprintf("unique%06d", i)
		if i >= 400 && i < 2400 {
			hash = fmt.Sprintf("duplicate%04d", (i-400)/2)
		}
		if _, err := stmt.ExecContext(ctx, strings.ToLower(path), "file", name, path, strings.ToLower(folder), "r", 100, hash, "2020-01-01T00:00:00Z", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{"/Dependency", "/Dependency/node_modules"} {
		parent := ""
		var devKind any
		if folder != "/Dependency" {
			parent = "/dependency"
			devKind = "node_modules"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO dbx_files(path_lower,tag,name,path_display,parent_lower,dev_kind) VALUES(?,'folder',?,?,?,?)`, strings.ToLower(folder), filepath.Base(folder), folder, parent, devKind); err != nil {
			t.Fatal(err)
		}
	}
	devStmt, err := tx.PrepareContext(ctx, `INSERT INTO dbx_files(path_lower,tag,name,path_display,parent_lower,rev,size,content_hash,dev_kind) VALUES(?,'file',?,?,?,?,?,?,'node_modules')`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50000; i++ {
		name := fmt.Sprintf("dep%05d.js", i)
		path := "/Dependency/node_modules/" + name
		if _, err := devStmt.ExecContext(ctx, strings.ToLower(path), name, path, "/dependency/node_modules", "r", 1, fmt.Sprintf("dev%04d", i/2)); err != nil {
			t.Fatal(err)
		}
	}
	if err := devStmt.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO dbx_index_state(root,last_full_at,complete) VALUES('',?,1)`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO dbx_index_state(root,last_full_at,complete) SELECT path_lower,?,1 FROM dbx_files WHERE tag='folder' AND parent_lower=''`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	check := func(name string, args []string, validate func([]byte)) {
		t.Helper()
		started := time.Now()
		data, err := runRead(t, args...)
		elapsed := time.Since(started)
		t.Logf("%s: %s", name, elapsed)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if elapsed >= 3*time.Second {
			t.Errorf("%s took %s (limit 3s)", name, elapsed)
		}
		validate(data)
	}
	check("overview", []string{"overview", "--db", dbPath, "--json"}, func(data []byte) {
		var r overviewResult
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		if r.Totals.Files != 250000 || r.Totals.Folders != 5002 || r.Duplicates.Groups != 1000 || r.ConflictedCopies != 200 || len(r.DevDirs) != 1 || r.DevDirs[0].Files != 50000 {
			t.Fatalf("overview counts: %+v", r)
		}
	})
	check("dupes", []string{"dupes", "--db", dbPath, "--limit", "10", "--json"}, func(data []byte) {
		var r dupesResult
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		if r.TotalGroups != 1000 || len(r.Groups) != 10 || r.ExcludedDevDirs.Files != 50000 {
			t.Fatalf("dupes counts: %+v", r)
		}
	})
	check("conflicts", []string{"conflicts", "--db", dbPath, "--json"}, func(data []byte) {
		var r conflictsResult
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		if len(r.Pairs) != 200 || r.Counts.Identical != 0 || r.Counts.Different != 200 {
			t.Fatalf("conflicts counts: %+v", r.Counts)
		}
	})
	check("mess", []string{"mess", "--db", dbPath, "--limit", "10", "--json"}, func(data []byte) {
		var r messResult
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		if r.EmptyFolders.Count != 0 || r.RootFiles.Count != 0 || r.SingleFileFolders.Count != 0 || r.ExcludedDevDirs.Files != 50000 {
			t.Fatalf("mess counts: %+v", r)
		}
	})
	check("tree", []string{"tree", "/", "--depth", "2", "--db", dbPath, "--json"}, func(data []byte) {
		var r treeNode
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		if r.Files != 250000 || len(r.Children) != 20 || r.More != 4981 {
			t.Fatalf("tree counts: files=%d children=%d more=%d", r.Files, len(r.Children), r.More)
		}
	})

	// Add a large folder pair after the other commands' baseline assertions.
	// The second conflicts run must stay within the same three-second budget.
	db, err = store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = db.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err = tx.PrepareContext(ctx, `INSERT INTO dbx_files(path_lower,tag,name,path_display,parent_lower,rev,size,content_hash) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{"/Scale", "/Scale (Selective Sync Conflict)"} {
		if _, err := stmt.ExecContext(ctx, strings.ToLower(folder), "folder", folder[1:], folder, "", "", 0, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO dbx_index_state(root,last_full_at,complete) VALUES(?,?,1)`, strings.ToLower(folder), time.Now().UTC().Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5000; i++ {
			name := fmt.Sprintf("file%04d.txt", i)
			p := folder + "/" + name
			if _, err := stmt.ExecContext(ctx, strings.ToLower(p), "file", name, p, strings.ToLower(folder), "r", 100, fmt.Sprintf("tree%04d", i)); err != nil {
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
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	check("conflicts with 5,000-file folder pair", []string{"conflicts", "--db", dbPath, "--json"}, func(data []byte) {
		var r conflictsResult
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		if len(r.Pairs) != 201 || r.Counts.IdenticalTree != 1 || r.Pairs[200].Class != "identical_tree" {
			t.Fatalf("large conflict pair = %+v", r.Counts)
		}
	})
}

func TestScaleQueryPlansUseIndexes(t *testing.T) {
	ctx := context.Background()
	dbPath := seedIndex(t, fixtureRow("/Docs/a.txt", "file", "h", "", 1), fixtureRow("/Docs/b.txt", "file", "h", "", 1))
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	candidate, args := duplicateCandidateSQL("", 1, false)
	for _, tc := range []struct {
		name, query, want string
		args              []any
	}{
		{"dupes", candidate, "dbx_files_duplicate_candidates", args},
		{"tree", `SELECT COUNT(*) FROM dbx_files WHERE path_lower>=? AND path_lower<?`, "SEARCH dbx_files", []any{"/docs/", "/docs0"}},
		{"conflict tree", `SELECT path_lower,COALESCE(content_hash,''),COALESCE(size,0) FROM dbx_files WHERE path_lower>=? AND path_lower<? AND tag='file' ORDER BY path_lower`, "SEARCH dbx_files", []any{"/docs/", "/docs0"}},
	} {
		rows, err := db.DB().QueryContext(ctx, "EXPLAIN QUERY PLAN "+tc.query, tc.args...)
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
		if !strings.Contains(strings.Join(details, "; "), tc.want) || strings.Contains(strings.Join(details, "; "), "SCAN dbx_files") && tc.name == "tree" {
			t.Fatalf("%s plan: %v", tc.name, details)
		}
	}
}
