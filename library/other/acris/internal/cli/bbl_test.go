// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelBblHelpWires smoke-tests that the bbl command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBblHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"bbl", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bbl --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "bbl"} {
		if !strings.Contains(help, want) {
			t.Fatalf("bbl --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBBLResultNoteWarnsWhenLegalsCapPrecedesDocumentCap(t *testing.T) {
	note := bblResultNote(12, false, true, 100, 400)
	if !strings.Contains(note, "source query reached its 400-row legal-record limit") {
		t.Fatalf("missing upstream-cap warning: %q", note)
	}
	if strings.Contains(note, "results capped at 100 documents") {
		t.Fatalf("incorrect distinct-document cap warning: %q", note)
	}
}
