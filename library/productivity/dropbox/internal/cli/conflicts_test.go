// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

// TestNovelConflictsHelpWires smoke-tests that the conflicts command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelConflictsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"conflicts", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("conflicts --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "conflicts"} {
		if !strings.Contains(help, want) {
			t.Fatalf("conflicts --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestConflictsClassifyAndPlan(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Docs/report.docx", "file", "H1", "2016-01-01T00:00:00Z", 100),
		fixtureRow("/Docs/report (Dana's conflicted copy 2017-03-02).docx", "file", "H1", "2017-03-02T00:00:00Z", 100),
		fixtureRow("/Docs/notes.txt", "file", "H3", "2018-01-01T00:00:00Z", 50),
		fixtureRow("/Docs/notes (Sam's conflicted copy 2019-01-05).txt", "file", "H2", "2019-01-05T00:00:00Z", 50),
		fixtureRow("/Docs/lost (Dana's conflicted copy 2020-02-02).pdf", "file", "H4", "2020-02-02T00:00:00Z", 80),
		fixtureRow("/Docs/copy of things.txt", "file", "H5", "2020-01-01T00:00:00Z", 30),
	)
	planPath := filepath.Join(t.TempDir(), "conflicts.json")
	data, err := runRead(t, "conflicts", "--db", db, "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got conflictsResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pairs) != 3 || got.Counts.Identical != 1 || got.Counts.Different != 1 || got.Counts.Orphan != 1 || got.Counts.Folder != 0 {
		t.Fatalf("got %+v", got)
	}
	plan, err := dropbox.ReadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Path != "/Docs/report (Dana's conflicted copy 2017-03-02).docx" {
		t.Fatalf("plan %+v", plan)
	}
}

func TestConflictsRealWorldFoldersAndPlanCheck(t *testing.T) {
	testenv.Isolate(t)
	rows := []store.DropboxRow{
		fixtureRow("/Projects", "folder", "", "", 0),
		fixtureRow("/Projects/a.txt", "file", "H1", "", 10),
		fixtureRow("/Projects/sub", "folder", "", "", 0),
		fixtureRow("/Projects/sub/b.txt", "file", "H2", "", 20),
		fixtureRow("/Projects (Selective Sync Conflict)", "folder", "", "", 0),
		fixtureRow("/Projects (Selective Sync Conflict)/a.txt", "file", "H1", "", 10),
		fixtureRow("/Projects (Selective Sync Conflict) (Selective Sync Conflict)", "folder", "", "", 0),
		fixtureRow("/Projects (Selective Sync Conflict) (Selective Sync Conflict)/a.txt", "file", "H1", "", 10),
		fixtureRow("/Share", "folder", "", "", 0),
		fixtureRow("/Share/x", "file", "H3", "", 30),
		fixtureRow("/Share (Selective Sync Conflict)", "folder", "", "", 0),
		fixtureRow("/Share (Selective Sync Conflict)/x", "file", "H3", "", 30),
		fixtureRow("/Footage", "folder", "", "", 0),
		fixtureRow("/Footage/v", "file", "H4", "", 40),
		fixtureRow("/Footage (Selective Sync Conflict)", "folder", "", "", 0),
		fixtureRow("/Footage (Selective Sync Conflict)/v", "file", "H5", "", 40),
		fixtureRow("/1 (MacBook-Pro.local's invalid files)", "folder", "", "", 0),
		fixtureRow("/1 (MacBook-Pro.local's invalid files)/bad.txt", "file", "H6", "", 60),
		fixtureRow("/Zed", "folder", "", "", 0),
		fixtureRow("/Zed/z", "file", "H7", "", 70),
		fixtureRow("/Zed (MacBook-Pro.local's conflicted copy 2024-02-19)", "folder", "", "", 0),
		fixtureRow("/Zed (MacBook-Pro.local's conflicted copy 2024-02-19)/z", "file", "H7", "", 70),
		fixtureRow("/Cold", "folder", "", "", 0),
		fixtureRow("/Cold/a", "file", "H8", "", 80),
		fixtureRow("/Cold (Selective Sync Conflict)", "folder", "", "", 0),
		fixtureRow("/Cold (Selective Sync Conflict)/a", "file", "H8", "", 80),
	}
	dbPath := seedIndex(t, rows...)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE dbx_files SET root='/cold' WHERE path_lower='/cold' OR path_lower='/cold/a' OR path_lower='/cold (selective sync conflict)' OR path_lower='/cold (selective sync conflict)/a'`); err != nil {
		t.Fatal(err)
	}
	if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: "/cold", LastFullAt: time.Now().UTC().Format(time.RFC3339), Complete: false}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "conflicts.json")
	data, err := runRead(t, "conflicts", "--db", dbPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got conflictsResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]conflictPair)
	for _, pair := range got.Pairs {
		byPath[pair.Copy] = pair
	}
	for path, class := range map[string]string{
		"/Projects (Selective Sync Conflict)":                           "subset_tree",
		"/Projects (Selective Sync Conflict) (Selective Sync Conflict)": "subset_tree",
		"/Share (Selective Sync Conflict)":                              "identical_tree",
		"/Footage (Selective Sync Conflict)":                            "diverged_tree",
		"/1 (MacBook-Pro.local's invalid files)":                        "invalid_files",
		"/Zed (MacBook-Pro.local's conflicted copy 2024-02-19)":         "identical_tree",
		"/Cold (Selective Sync Conflict)":                               "index_incomplete",
	} {
		if byPath[path].Class != class {
			t.Fatalf("%s class = %q, want %q", path, byPath[path].Class, class)
		}
	}
	if got.Counts.SubsetTree != 2 || got.Counts.IdenticalTree != 2 || got.Counts.DivergedTree != 1 || got.Counts.InvalidFiles != 1 || got.Counts.IndexIncomplete != 1 {
		t.Fatalf("counts = %+v", got.Counts)
	}
	if nested := byPath["/Projects (Selective Sync Conflict) (Selective Sync Conflict)"]; nested.Original != "/Projects" || nested.Depth != 2 {
		t.Fatalf("nested pair = %+v", nested)
	}
	if diverged := byPath["/Footage (Selective Sync Conflict)"]; diverged.DifferingCount != 1 || len(diverged.DifferingPaths) != 1 || diverged.DifferingPaths[0] != "v" {
		t.Fatalf("diverged pair = %+v", diverged)
	}
	if invalid := byPath["/1 (MacBook-Pro.local's invalid files)"]; invalid.FileCount != 1 || invalid.Bytes != 60 {
		t.Fatalf("invalid-files pair = %+v", invalid)
	}
	if _, err := runRead(t, "conflicts", "--db", dbPath, "--plan", planPath, "--json"); err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "dropbox-pp-cli index") {
		t.Fatalf("incomplete root plan error = %v", err)
	}
	db, err = store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: "/cold", LastFullAt: time.Now().UTC().Format(time.RFC3339), Complete: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := runRead(t, "conflicts", "--db", dbPath, "--plan", planPath, "--json"); err != nil {
		t.Fatal(err)
	}
	plan, err := dropbox.ReadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	planned := make(map[string]bool)
	for _, op := range plan.Ops {
		if op.Op != "delete" || op.Rev != "" {
			t.Fatalf("folder operation = %+v", op)
		}
		if op.ExpectFiles == nil || op.ExpectBytes == nil || *op.ExpectFiles != 1 || *op.ExpectBytes <= 0 {
			t.Fatalf("unguarded folder operation = %+v", op)
		}
		if op.Keeper == "" || op.ExpectTreeHash == "" {
			t.Fatalf("missing keeper tree attestation = %+v", op)
		}
		planned[op.Path] = true
	}
	if len(planned) != 5 || !planned["/Projects (Selective Sync Conflict)"] || !planned["/Projects (Selective Sync Conflict) (Selective Sync Conflict)"] || !planned["/Share (Selective Sync Conflict)"] || !planned["/Zed (MacBook-Pro.local's conflicted copy 2024-02-19)"] || !planned["/Cold (Selective Sync Conflict)"] {
		t.Fatalf("planned paths = %+v", planned)
	}
	checkData, err := runRead(t, "plan", "check", planPath, "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("plan check: %v: %s", err, checkData)
	}
	var report dropbox.CheckReport
	if err := json.Unmarshal(checkData, &report); err != nil || !report.OK {
		t.Fatalf("plan check = %+v; parse err = %v", report, err)
	}
	db, err = store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE dbx_files SET content_hash='changed' WHERE path_lower='/share/x'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	checkData, err = runRead(t, "plan", "check", planPath, "--db", dbPath, "--json")
	if err == nil || !strings.Contains(string(checkData), "keeper_tree_mismatch") {
		t.Fatalf("keeper drift: err=%v data=%s", err, checkData)
	}
}

func TestCompareConflictTreesCapsReportedPaths(t *testing.T) {
	copyFiles := make([]conflictTreeFile, 25)
	for i := range copyFiles {
		copyFiles[i] = conflictTreeFile{relative: fmt.Sprintf("missing%02d", i), hash: "H", size: 1}
	}
	class, paths, count := compareConflictTrees(copyFiles, nil)
	if class != "diverged_tree" || count != 25 || len(paths) != 20 || paths[0] != "missing00" || paths[19] != "missing19" {
		t.Fatalf("class=%s paths=%v count=%d", class, paths, count)
	}
}

func TestConflictsRequiresCompleteIndexRoot(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t,
		fixtureRow("/Docs/a.txt", "file", "H", "", 1),
		fixtureRow("/Docs/a (Dana's conflicted copy).txt", "file", "H", "", 1),
	)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDropboxIndexState(context.Background(), "/docs"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "conflicts", "--db", dbPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got conflictsResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Counts.IndexIncomplete != 1 || got.Counts.Identical != 0 {
		t.Fatalf("counts=%+v", got.Counts)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if _, err := runRead(t, "conflicts", "--db", dbPath, "--plan", planPath, "--json"); err == nil || !strings.Contains(err.Error(), "dropbox-pp-cli index") {
		t.Fatalf("plan error=%v", err)
	}
}

func TestConflictsEmptyCopyIsReviewOnly(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t,
		fixtureRow("/Folder", "folder", "", "", 0),
		fixtureRow("/Folder/a.txt", "file", "H", "", 1),
		fixtureRow("/Folder (Selective Sync Conflict)", "folder", "", "", 0),
	)
	planPath := filepath.Join(t.TempDir(), "plan.json")
	data, err := runRead(t, "conflicts", "--db", dbPath, "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got conflictsResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Counts.EmptyCopy != 1 || got.PlanOps != 0 || got.Pairs[0].Class != "empty_copy" {
		t.Fatalf("result=%+v", got)
	}
	p, err := dropbox.ReadPlan(planPath)
	if err != nil || len(p.Ops) != 0 {
		t.Fatalf("plan=%+v err=%v", p, err)
	}
}

func TestConflictsExcludesDevCopiesByDefault(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/repo/node_modules/a.txt", "file", "H", "", 4),
		fixtureRow("/repo/node_modules/a (Dana's conflicted copy).txt", "file", "H", "", 4),
		fixtureRow("/Docs/b.txt", "file", "N", "", 8),
		fixtureRow("/Docs/b (Dana's conflicted copy).txt", "file", "N", "", 8),
	)
	for _, tc := range []struct {
		include         bool
		pairs, excluded int
	}{{false, 1, 1}, {true, 2, 0}} {
		args := []string{"conflicts", "--db", db, "--json"}
		if tc.include {
			args = append(args, "--include-dev-dirs")
		}
		data, err := runRead(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		var got conflictsResult
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Pairs) != tc.pairs || got.ExcludedDevDirs.Files != tc.excluded {
			t.Fatalf("include=%t conflicts=%+v", tc.include, got)
		}
	}
}

func TestConflictsDoesNotPlanFolderContainingDevFiles(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Project", "folder", "", "", 0),
		fixtureRow("/Project/node_modules", "folder", "", "", 0),
		fixtureRow("/Project/node_modules/a.js", "file", "H", "", 5),
		fixtureRow("/Project (Selective Sync Conflict)", "folder", "", "", 0),
		fixtureRow("/Project (Selective Sync Conflict)/node_modules", "folder", "", "", 0),
		fixtureRow("/Project (Selective Sync Conflict)/node_modules/a.js", "file", "H", "", 5),
	)
	planPath := filepath.Join(t.TempDir(), "conflicts.json")
	data, err := runRead(t, "conflicts", "--db", db, "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got conflictsResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pairs) != 0 || got.ExcludedDevDirs.Files != 1 {
		t.Fatalf("default conflicts: %+v", got)
	}
	if p, err := dropbox.ReadPlan(planPath); err != nil || len(p.Ops) != 0 {
		t.Fatalf("expected empty default plan, got %+v, %v", p, err)
	}
	data, err = runRead(t, "conflicts", "--db", db, "--include-dev-dirs", "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pairs) != 1 || got.Counts.IdenticalTree != 1 {
		t.Fatalf("included conflicts: %+v", got)
	}
}
