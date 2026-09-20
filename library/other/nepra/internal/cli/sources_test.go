// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// The behaviour tests live in nepra_sources_test.go; this file keeps the
// scaffold's wiring smoke test, whose comment asks for exactly that.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelSourcesHelpWires smoke-tests that the sources command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
//
// It is carried over verbatim from the scaffold, and it is the guard that a
// future rewiring of root.go cannot silently unwire the command: the
// implementation kept the newNovelSourcesCmd name so root.go's existing
// addNovelCommandIfAbsent registration keeps working untouched.
func TestNovelSourcesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"sources", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sources --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "sources"} {
		if !strings.Contains(help, want) {
			t.Fatalf("sources --help missing %q in output:\n%s", want, help)
		}
	}
	// The four flags the README and SKILL.md already document must all be on
	// the help page, because those docs ship as the command's contract.
	for _, want := range []string{"--kind", "--diff", "--fetch", "--strict"} {
		if !strings.Contains(help, want) {
			t.Errorf("sources --help does not document %s", want)
		}
	}
	// The help text must not advertise a value placeholder cobra inferred from
	// a backquoted word: "--kind events" once appeared here because the usage
	// string wrapped `events` in backticks.
	if strings.Contains(help, "--kind events") {
		t.Error("cobra inferred 'events' as the --kind placeholder from a backquoted word in the usage string")
	}
}
