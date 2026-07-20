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

// TestNovelProvisionHelpWires smoke-tests that the provision command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelProvisionHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"provision", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("provision --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "provision"} {
		if !strings.Contains(help, want) {
			t.Fatalf("provision --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReadProvisionCSVPreservesStructuredTypes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.csv")
	data := "Number,Groups,Enabled\n214,\"[\"\"sales\"\",\"\"support\"\"]\",true\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := readProvisionCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	groups, ok := rows[0].Body["Groups"].([]any)
	if !ok || len(groups) != 2 || rows[0].Body["Enabled"] != true {
		t.Fatalf("typed fields were not preserved: %#v", rows[0].Body)
	}
}
