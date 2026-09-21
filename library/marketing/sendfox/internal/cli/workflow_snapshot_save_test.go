// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil/testenv"
)

func TestWorkflowSnapshotSaveRequiresOutputDirectory(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"workflow", "snapshot-save", "--input", "../../examples/snapshot.json", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--out is required") {
		t.Fatalf("error = %v, output = %s", err, out.String())
	}
}

func TestWorkflowSnapshotSaveWritesPrivateHistoryArtifact(t *testing.T) {
	testenv.Isolate(t)
	history := filepath.Join(t.TempDir(), "history")
	cmd := RootCmd()
	cmd.SetArgs([]string{"workflow", "snapshot-save", "--input", "../../examples/snapshot.json", "--out", history, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow snapshot-save error = %v, output = %s", err, out.String())
	}
	if !strings.Contains(out.String(), `"workflow": "snapshot-save"`) || !strings.Contains(out.String(), `"history_artifact"`) {
		t.Fatalf("snapshot-save output lacks truthful workflow/artifact metadata: %s", out.String())
	}
	entries, err := os.ReadDir(history)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("history entries = %v", entries)
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("history artifact mode = %o", info.Mode().Perm())
	}
	resolved, _, err := RootCmd().Find([]string{"workflow", "snapshot-save"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Annotations["mcp:local-write"] != "true" || resolved.Annotations["mcp:read-only"] == "true" {
		t.Fatalf("snapshot-save annotations = %#v", resolved.Annotations)
	}
}
