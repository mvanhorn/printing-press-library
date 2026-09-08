// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestNovelUniverseHelpWires smoke-tests that the universe command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUniverseHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"universe", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("universe --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "universe"} {
		if !strings.Contains(help, want) {
			t.Fatalf("universe --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestUniverseRejectsPositionalArgs guards a real footgun: every legal
// --resource value is also a plausible bare argument, so `universe daily-nav`
// used to run silently against the default resource and report a different
// universe's width.
func TestUniverseRejectsPositionalArgs(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"universe", "daily-nav", "--home", t.TempDir(), "--no-learn"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("universe daily-nav returned no error; output:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "positional") {
		t.Fatalf("universe daily-nav error = %v, want a positional-argument rejection", err)
	}
}

// TestUniverseEmptyMirrorKeepsObjectSchema pins the one-top-level-type rule:
// the missing-mirror path must emit the same {"summary":...,"rows":[]} object
// as the success path, not a bare [], or an agent reading
// .summary.min_fund_count hits a type error exactly when the mirror is empty.
func TestUniverseEmptyMirrorKeepsObjectSchema(t *testing.T) {
	cmd := RootCmd()
	missingDB := filepath.Join(t.TempDir(), "no-such-mirror.db")
	cmd.SetArgs([]string{
		"universe", "--json", "--no-learn",
		"--home", t.TempDir(),
		"--db", missingDB,
		"--from", "2026-09-01", "--to", "2026-09-04",
	})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("universe against a missing mirror error = %v (stderr: %s)", err, errOut.String())
	}
	var view struct {
		Summary map[string]any `json:"summary"`
		Rows    []any          `json:"rows"`
	}
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("missing-mirror output is not the universeView object: %v\noutput:\n%s", err, out.String())
	}
	if view.Summary == nil {
		t.Fatalf("missing-mirror output dropped .summary:\n%s", out.String())
	}
	if view.Rows == nil {
		t.Fatalf("missing-mirror output dropped .rows:\n%s", out.String())
	}
	for _, key := range []string{"resource", "from", "to", "dates", "min_fund_count", "max_fund_count"} {
		if _, ok := view.Summary[key]; !ok {
			t.Fatalf("missing-mirror summary omits %q:\n%s", key, out.String())
		}
	}
}
