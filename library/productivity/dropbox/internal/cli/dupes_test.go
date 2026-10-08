package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func fixtureRow(p, tag, hash, date string, size int64) store.DropboxRow {
	parent := path.Dir(p)
	if parent == "/" {
		parent = ""
	}
	return store.DropboxRow{PathLower: strings.ToLower(p), PathDisplay: p, ParentLower: strings.ToLower(parent), Name: path.Base(p), Tag: tag, ContentHash: hash, ClientModified: date, Size: size, Rev: "rev:" + path.Base(p)}
}
func seedIndex(t *testing.T, rows ...store.DropboxRow) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "index.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.EnsureDropboxSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertDropboxEntries(context.Background(), "", rows); err != nil {
		t.Fatal(err)
	}
	if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: "", LastFullAt: time.Now().UTC().Format(time.RFC3339), Complete: true}); err != nil {
		t.Fatal(err)
	}
	roots := map[string]bool{}
	for _, row := range rows {
		root := indexRootForEntry(row.PathLower, row.Tag)
		if root != "" {
			roots[root] = true
		}
	}
	for root := range roots {
		if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: root, LastFullAt: time.Now().UTC().Format(time.RFC3339), Complete: true}); err != nil {
			t.Fatal(err)
		}
	}
	return dbPath
}
func runRead(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err != nil {
		return out.Bytes(), err
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("invalid JSON %q; stderr: %s", out.String(), stderr.String())
	}
	return out.Bytes(), nil
}
func TestDupesAnalysisAndPlan(t *testing.T) {
	testenv.Isolate(t)
	rows := []store.DropboxRow{
		fixtureRow("/A/x.jpg", "file", "H", "2015-01-01T00:00:00Z", 100),
		fixtureRow("/B/x.jpg", "file", "H", "2018-01-01T00:00:00Z", 100),
		fixtureRow("/Camera Uploads/x 1.jpg", "file", "H", "2020-01-01T00:00:00Z", 100),
		fixtureRow("/C/unique.jpg", "file", "U", "2017-01-01T00:00:00Z", 100),
		fixtureRow("/D/empty.txt", "file", "E", "2017-01-01T00:00:00Z", 0),
		fixtureRow("/E/empty.txt", "file", "E", "2017-01-01T00:00:00Z", 0),
	}
	db := seedIndex(t, rows...)
	data, err := runRead(t, "dupes", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got dupesResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalGroups != 1 || len(got.Groups) != 1 || got.Groups[0].Keeper != "/A/x.jpg" || len(got.Groups[0].Duplicates) != 2 || got.TotalReclaimableBytes != 200 {
		t.Fatalf("got %+v", got)
	}
	data, err = runRead(t, "dupes", "--db", db, "--keep", "in:/B", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Groups[0].Keeper != "/B/x.jpg" {
		t.Fatalf("keeper %q", got.Groups[0].Keeper)
	}
	planPath := filepath.Join(t.TempDir(), "dupes.json")
	_, err = runRead(t, "dupes", "--db", db, "--limit", "0", "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := dropbox.ReadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 2 || plan.Ops[0].Rev == "" || plan.Ops[1].Rev == "" {
		t.Fatalf("plan %+v", plan)
	}
	for _, op := range plan.Ops {
		if op.Keeper != "/A/x.jpg" || op.ContentHash != "H" {
			t.Fatalf("delete guard=%+v", op)
		}
	}
}

func TestReadCommandsMissingIndexAndDryRun(t *testing.T) {
	testenv.Isolate(t)
	missing := filepath.Join(t.TempDir(), "missing.db")
	for _, command := range []struct {
		name string
		args []string
	}{{"overview", nil}, {"tree", nil}, {"dupes", nil}, {"conflicts", nil}, {"mess", nil}, {"search", []string{"invoice"}}} {
		t.Run(command.name, func(t *testing.T) {
			args := append([]string{command.name}, command.args...)
			args = append(args, "--db", missing, "--json")
			if _, err := runRead(t, args...); err != nil {
				t.Fatal(err)
			}
			dry := append([]string{command.name}, command.args...)
			dry = append(dry, "--db", missing, "--dry-run", "--json")
			data, err := runRead(t, dry...)
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				DryRun bool `json:"dry_run"`
			}
			if err := json.Unmarshal(data, &payload); err != nil || !payload.DryRun {
				t.Fatalf("dry run %s: %v", data, err)
			}
		})
	}
}

func TestDupesExcludesDevDirectoriesByDefault(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/A/node_modules/x/LICENSE", "file", "dev-hash", "", 10),
		fixtureRow("/B/node_modules/x/LICENSE", "file", "dev-hash", "", 10),
		fixtureRow("/E/node_modules/only.txt", "file", "singleton", "", 7),
		fixtureRow("/C/LICENSE.txt", "file", "normal-hash", "", 20),
		fixtureRow("/D/LICENSE.txt", "file", "normal-hash", "", 20),
	)
	for _, tc := range []struct {
		include               bool
		groups, excludedFiles int
		excludedBytes         int64
	}{
		{false, 1, 2, 20}, {true, 2, 0, 0},
	} {
		args := []string{"dupes", "--db", db, "--json"}
		if tc.include {
			args = append(args, "--include-dev-dirs")
		}
		data, err := runRead(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		var got dupesResult
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.TotalGroups != tc.groups || got.ExcludedDevDirs.Files != tc.excludedFiles || got.ExcludedDevDirs.Bytes != tc.excludedBytes {
			t.Fatalf("include=%t result=%+v", tc.include, got)
		}
	}
}
