// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNovelInventoryHelpWires smoke-tests that the inventory command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelInventoryHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"inventory", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inventory --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "inventory"} {
		if !strings.Contains(help, want) {
			t.Fatalf("inventory --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelInventoryScansDeterministicallyAndSkipsExcludedPaths(t *testing.T) {
	db := seedComponentDB(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "view.tsx"), []byte("const x = Combobox;\nconst y = Selectable;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "node_modules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "skip.ts"), []byte("Combobox"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "view.tsx"), filepath.Join(root, "linked.tsx")); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "--json", "--no-learn", "inventory", root, "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	var got inventoryResponse
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "view.tsx" || len(got.Files[0].Matches) != 1 || got.Files[0].Matches[0].Symbol != "Combobox" {
		t.Fatalf("inventory = %#v", got)
	}
	if got.Totals.FilesScanned != 1 || got.Totals.Matches != 1 {
		t.Fatalf("totals = %#v", got.Totals)
	}
	stdout, _, err = runRootArgs(t, "--json", "--no-learn", "--select", "files.matches.symbol,files.matches.component,files.matches.source_url", "inventory", root, "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	var selected map[string]any
	if err := json.Unmarshal([]byte(stdout), &selected); err != nil {
		t.Fatal(err)
	}
	files := selected["files"].([]any)
	matches := files[0].(map[string]any)["matches"].([]any)
	match := matches[0].(map[string]any)
	for _, key := range []string{"symbol", "component", "source_url"} {
		if match[key] == "" {
			t.Fatalf("documented nested selector dropped %s: %s", key, stdout)
		}
	}
}

func TestNovelInventoryDryRunOversizeAndMissingMirror(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "view.swift")
	if err := os.WriteFile(path, []byte("Picker"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "--json", "--no-learn", "inventory", root, "--db", filepath.Join(root, "missing.db"), "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"walked": false`) || !strings.Contains(stdout, `"sqlite_opened": false`) {
		t.Fatalf("dry run = %s", stdout)
	}
	if _, _, err := runRootArgs(t, "--no-learn", "inventory", root, "--db", filepath.Join(root, "missing.db")); err == nil || !strings.Contains(err.Error(), "sync --resources catalog") {
		t.Fatalf("missing mirror error = %v", err)
	}
	large := filepath.Join(root, "large.go")
	if err := os.WriteFile(large, make([]byte, novelMaxFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runRootArgs(t, "--no-learn", "inventory", root, "--db", seedComponentDB(t)); err == nil || !strings.Contains(err.Error(), "2 MiB") {
		t.Fatalf("oversize error = %v", err)
	}
}
