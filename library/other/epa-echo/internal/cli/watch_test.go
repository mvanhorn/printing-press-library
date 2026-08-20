// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNovelWatchHelpWires smoke-tests that the watch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"watch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("watch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "watch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("watch --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReadPortfolioIDsDeduplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portfolio.csv")
	if err := os.WriteFile(path, []byte("name,registry_id\nOne,110009441979\nDuplicate,110009441979\nTwo,110000000001\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ids, err := readPortfolioIDs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ids, ","), "110009441979,110000000001"; got != want {
		t.Fatalf("IDs = %q, want %q", got, want)
	}
}

func TestReadPortfolioIDsRejectsShortRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portfolio.csv")
	if err := os.WriteFile(path, []byte("name,registry_id\nmissing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPortfolioIDs(path); err == nil {
		t.Fatal("readPortfolioIDs() error = nil, want malformed-row error")
	}
}
