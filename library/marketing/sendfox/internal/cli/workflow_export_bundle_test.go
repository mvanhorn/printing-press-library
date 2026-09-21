// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil/testenv"
)

// TestNovelWorkflowExportBundleHelpWires smoke-tests that the workflow export-bundle command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWorkflowExportBundleHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"workflow", "export-bundle", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow export-bundle --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "export-bundle"} {
		if !strings.Contains(help, want) {
			t.Fatalf("workflow export-bundle --help missing %q in output:\n%s", want, help)
		}
	}
	if strings.Contains(help, "--out") {
		t.Fatalf("read-only export-bundle unexpectedly advertises --out:\n%s", help)
	}
	resolved, _, err := RootCmd().Find([]string{"workflow", "export-bundle"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Annotations["mcp:read-only"] != "true" || resolved.Annotations["mcp:local-write"] == "true" {
		t.Fatalf("export-bundle annotations = %#v", resolved.Annotations)
	}
}
