// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelDocsHelpWires smoke-tests that the docs command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDocsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"docs", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "docs"} {
		if !strings.Contains(help, want) {
			t.Fatalf("docs --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNormalizeDocTypeResolvesAdvertisedSpellings guards the friendly doc-type
// words the command's own Short/Long text promises. Before normalization,
// "spec sheet" and "install guide" both failed with "unknown doc type" even
// though the help advertised them.
func TestNormalizeDocTypeResolvesAdvertisedSpellings(t *testing.T) {
	cases := map[string]string{
		"spec sheet":    "webcontent=productspecs",
		"Spec Sheet":    "webcontent=productspecs",
		"spec_sheet":    "webcontent=productspecs",
		"install guide": "webcontent=productgettingstarted",
		"  manual  ":    "webcontent=productdocument",
		"spec":          "webcontent=productspecs",
	}
	for in, want := range cases {
		got, ok := docTypeLabels[normalizeDocType(in)]
		if !ok {
			t.Fatalf("normalizeDocType(%q) = %q, not a known doc type", in, normalizeDocType(in))
		}
		if got != want {
			t.Fatalf("doc type %q mapped to %q, want %q", in, got, want)
		}
	}
	if _, ok := docTypeLabels[normalizeDocType("not a real type")]; ok {
		t.Fatal("normalizeDocType let an unknown doc type through")
	}
}
