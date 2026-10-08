// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
)

// TestNovelPlanCheckHelpWires smoke-tests that the plan check command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPlanCheckHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"plan", "check", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan check --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "check"} {
		if !strings.Contains(help, want) {
			t.Fatalf("plan check --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestPlanCheckPrintsReportOnRevisionError(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/B", "folder", "", "", 0), fixtureRow("/A/file.txt", "file", "", "", 1))
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "move", From: "/A/file.txt", To: "/B/file.txt", Rev: "stale"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "plan", "check", planPath, "--db", db, "--json")
	if err == nil {
		t.Fatal("expected exit 2")
	}
	var report dropbox.CheckReport
	if jerr := json.Unmarshal(data, &report); jerr != nil {
		t.Fatalf("JSON %q: %v", data, jerr)
	}
	if report.OK || report.Errors != 1 || report.Results[0].Code != "rev_mismatch" {
		t.Fatalf("report: %+v", report)
	}
}

func TestPlanCheckMissingIndexReportsError(t *testing.T) {
	testenv.Isolate(t)
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "mkdir", Path: "/A"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "plan", "check", planPath, "--db", filepath.Join(t.TempDir(), "missing.db"), "--json")
	if err == nil {
		t.Fatal("missing index accepted")
	}
	var report dropbox.CheckReport
	if jsonErr := json.Unmarshal(data, &report); jsonErr != nil || report.OK || report.Errors != 1 || report.Results[0].Code != "index_missing" {
		t.Fatalf("report=%s err=%v", data, jsonErr)
	}
}

func TestPlanCheckFindsDevDescendant(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t, fixtureRow("/repo", "folder", "", "", 0), fixtureRow("/repo/node_modules", "folder", "", "", 0), fixtureRow("/repo/node_modules/a.js", "file", "", "", 1))
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/repo"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "plan", "check", planPath, "--db", db, "--allow-nonempty-delete", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var report dropbox.CheckReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Warnings != 2 || report.Results[0].Code != "dev_dir" || report.Results[1].Code != "delete_nonempty_allowed" {
		t.Fatalf("report=%+v", report)
	}
}
