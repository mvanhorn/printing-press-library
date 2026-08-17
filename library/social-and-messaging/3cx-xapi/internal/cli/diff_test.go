// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
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

// TestNovelDiffHelpWires smoke-tests that the diff command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDiffHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"diff", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "diff"} {
		if !strings.Contains(help, want) {
			t.Fatalf("diff --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSnapshotPathRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../outside", "a/b", `a\\b`, ".", ".."} {
		if _, err := snapshotPath(name); err == nil {
			t.Fatalf("snapshotPath(%q) accepted traversal", name)
		}
	}
}

func TestWriteSnapshotUsesPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := writeSnapshot(path, []byte(`{"name":"safe"}`)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("snapshot mode = %o, want 600", got)
	}
}
