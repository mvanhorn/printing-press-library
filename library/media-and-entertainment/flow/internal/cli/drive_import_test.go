// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
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

// TestNovelDriveImportHelpWires smoke-tests that the drive import command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDriveImportHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"drive", "import", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drive import --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "import"} {
		if !strings.Contains(help, want) {
			t.Fatalf("drive import --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelDriveImportCopiesImagesFromLocalPath(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "fade.png"), []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "notes.txt"), []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dest")

	cmd := RootCmd()
	cmd.SetArgs([]string{"drive", "import", "--folder-id", source, "--dest", dest, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drive import execute error = %v, output:\n%s", err, out.String())
	}

	if _, err := os.Stat(filepath.Join(dest, "fade.png")); err != nil {
		t.Fatalf("expected fade.png copied into %s: %v", dest, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "notes.txt")); err == nil {
		t.Fatalf("expected notes.txt (non-image) NOT to be copied")
	}
	var result struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if result.Count != 1 {
		t.Fatalf("expected count=1, got %+v", result)
	}
}

func TestNovelDriveImportRejectsBareDriveID(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"drive", "import", "--folder-id", "1a2b3c4d-not-a-real-local-path"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected an error for a bare Drive ID that doesn't resolve to a local path")
	}
	if !strings.Contains(err.Error(), "Drive API") {
		t.Fatalf("expected error to explain the Drive API limitation, got: %v", err)
	}
}
