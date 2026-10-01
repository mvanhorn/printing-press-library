// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil/testenv"
)

// TestNovelTemplatesStatusHelpWires smoke-tests that the templates status command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTemplatesStatusHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"templates", "status", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("templates status --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "status"} {
		if !strings.Contains(help, want) {
			t.Fatalf("templates status --help missing %q in output:\n%s", want, help)
		}
	}
}

type s3TemplateRow struct {
	CategoryID   string `json:"category_id"`
	Category     string `json:"category"`
	TemplateType string `json:"template_type"`
	Target       int64  `json:"target"`
	Budgeted     int64  `json:"budgeted"`
	Status       string `json:"status"`
}

func TestTemplatesStatusSeptember(t *testing.T) {
	s3Fixture(t)
	var rows []s3TemplateRow
	s3RunJSON(t, &rows, "templates", "status", "--month", "2026-09")
	by := map[string]s3TemplateRow{}
	for _, r := range rows {
		by[r.Category] = r
	}
	cases := []struct {
		cat              string
		target, budgeted int64
		status           string
	}{
		{"Groceries", 40000, 40000, "funded"},
		{"Dining Out", 15000, 10000, "underfunded"},
		{"Rent", 150000, 150000, "funded"},
		{"Car Fund", 600000, 20000, "goal"},
	}
	for _, c := range cases {
		r, ok := by[c.cat]
		if !ok || r.Target != c.target || r.Budgeted != c.budgeted || r.Status != c.status {
			t.Errorf("%s = %+v, want target %d budgeted %d %s", c.cat, r, c.target, c.budgeted, c.status)
		}
	}
	if _, ok := by["Salary"]; ok {
		t.Error("Salary has no template and must not be listed")
	}
}

func TestTemplatesStatusFilterAndValidation(t *testing.T) {
	s3Fixture(t)
	var rows []s3TemplateRow
	s3RunJSON(t, &rows, "templates", "status", "--month", "2026-09", "--status", "underfunded")
	if len(rows) != 1 || rows[0].Category != "Dining Out" {
		t.Fatalf("underfunded filter = %+v", rows)
	}
	if _, _, err := s3Run(t, "templates", "status", "--month", "2026-13"); s3ExitCode(err) != 2 {
		t.Errorf("bad month exit %d", s3ExitCode(err))
	}
}

func TestTemplatesStatusNoMirrorEmpty(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "templates", "status", "--month", "2026-09")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("no mirror: out=%q err=%v", out, err)
	}
}
