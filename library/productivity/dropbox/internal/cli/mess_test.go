// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

// TestNovelMessHelpWires smoke-tests that the mess command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMessHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"mess", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mess --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "mess"} {
		if !strings.Contains(help, want) {
			t.Fatalf("mess --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMessCategoriesAndMaximalEmptyPlan(t *testing.T) {
	testenv.Isolate(t)
	shared := fixtureRow("/Shared", "folder", "", "", 0)
	shared.SharedFolderID = "shared-id"
	db := seedIndex(t,
		fixtureRow("/Old", "folder", "", "", 0),
		fixtureRow("/Old/Empty1", "folder", "", "", 0),
		fixtureRow("/Old/Empty1/Empty2", "folder", "", "", 0),
		fixtureRow("/Taxes", "folder", "", "", 0),
		fixtureRow("/Taxes 2", "folder", "", "", 0),
		fixtureRow("/Travel", "folder", "", "", 0),
		fixtureRow("/Docs", "folder", "", "", 0),
		shared,
		fixtureRow("/Taxes/return.pdf", "file", "T1", "2020-01-01T00:00:00Z", 10),
		fixtureRow("/Taxes 2/return.pdf", "file", "T2", "2020-01-01T00:00:00Z", 10),
		fixtureRow("/Travel/ticket.pdf", "file", "T3", "2020-01-01T00:00:00Z", 10),
		fixtureRow("/loose.pdf", "file", "H", "2020-01-01T00:00:00Z", 10),
		fixtureRow("/Docs/Copy of budget.xlsx", "file", "H2", "2020-01-01T00:00:00Z", 20),
	)
	planPath := filepath.Join(t.TempDir(), "mess.json")
	data, err := runRead(t, "mess", "--db", db, "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got messResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/Old", "/Old/Empty1", "/Old/Empty1/Empty2"} {
		if !slices.Contains(got.EmptyFolders.Items, want) {
			t.Errorf("missing empty %s: %+v", want, got.EmptyFolders)
		}
	}
	if slices.Contains(got.EmptyFolders.Items, "/Shared") || !slices.Contains(got.RootFiles.Items, "/loose.pdf") {
		t.Fatalf("categories %+v", got)
	}
	if got.JunkNames.Count == 0 || got.NearDuplicateFolders.Count != 1 || len(got.NearDuplicateFolders.Items[0].Folders) != 2 {
		t.Fatalf("categories %+v", got)
	}
	plan, err := dropbox.ReadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Path != "/Old" {
		t.Fatalf("plan ops %+v", plan.Ops)
	}
	if plan.Ops[0].ExpectFiles == nil || *plan.Ops[0].ExpectFiles != 0 || plan.Ops[0].ExpectBytes == nil || *plan.Ops[0].ExpectBytes != 0 {
		t.Fatalf("empty delete guard=%+v", plan.Ops[0])
	}
}

func TestMessSkipsEmptyDevFoldersAndAncestors(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/repo", "folder", "", "", 0),
		fixtureRow("/repo/.git", "folder", "", "", 0),
		fixtureRow("/repo/.git/refs", "folder", "", "", 0),
		fixtureRow("/repo/.git/refs/tags", "folder", "", "", 0),
	)
	planPath := filepath.Join(t.TempDir(), "mess.json")
	data, err := runRead(t, "mess", "--db", db, "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got messResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.EmptyFolders.Count != 0 {
		t.Fatalf("default empty folders: %+v", got.EmptyFolders)
	}
	if p, err := dropbox.ReadPlan(planPath); err != nil || len(p.Ops) != 0 {
		t.Fatalf("expected empty default plan, got %+v, %v", p, err)
	}
	data, err = runRead(t, "mess", "--db", db, "--include-dev-dirs", "--plan", planPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got.EmptyFolders.Items, "/repo/.git/refs/tags") {
		t.Fatalf("include empty folders: %+v", got.EmptyFolders)
	}
	plan, err := dropbox.ReadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Path != "/repo" {
		t.Fatalf("include plan: %+v", plan.Ops)
	}
}

func TestMessExcludesIncompleteEmptyRootAndRefusesPlan(t *testing.T) {
	testenv.Isolate(t)
	dbPath := seedIndex(t, fixtureRow("/Empty", "folder", "", "", 0))
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDropboxIndexState(context.Background(), "/empty"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "mess", "--db", dbPath, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got messResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.EmptyFolders.Count != 0 {
		t.Fatalf("empty folders=%+v", got.EmptyFolders)
	}
	if _, err := runRead(t, "mess", "--db", dbPath, "--plan", filepath.Join(t.TempDir(), "plan.json"), "--json"); err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "dropbox-pp-cli index") {
		t.Fatalf("plan error=%v", err)
	}
}
