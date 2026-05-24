package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Read tools must advertise readOnlyHint; sync mutates local state (writes the
// snapshot DB + FTS index) and must not. A false readOnlyHint on a mutating
// tool is a real bug, so guard the count and the per-tool annotation.
func TestToolReadOnlyHints(t *testing.T) {
	ts := tools()
	if len(ts) != 19 {
		t.Fatalf("expected 19 tools, got %d", len(ts))
	}
	writeTools := map[string]bool{"sync": true}
	for _, spec := range ts {
		hint := spec.tool.Annotations.ReadOnlyHint
		if hint == nil {
			t.Fatalf("tool %q has no read-only annotation", spec.tool.Name)
		}
		wantReadOnly := !writeTools[spec.tool.Name]
		if *hint != wantReadOnly {
			t.Fatalf("tool %q readOnlyHint = %v, want %v", spec.tool.Name, *hint, wantReadOnly)
		}
	}
}

func TestRunSelfReturnsStdoutOnlyWhenStderrHasHint(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-cli.sh")
	content := "#!/usr/bin/env bash\nprintf '[]\\n'\nprintf 'no activity hint\\n' >&2\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	prev := osExecutable
	osExecutable = func() (string, error) { return script, nil }
	t.Cleanup(func() { osExecutable = prev })

	out, err := runSelf("list", "--json", "--since", "2099-01-01")
	if err != nil {
		t.Fatalf("runSelf err: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("stdout polluted: %q", out)
	}
}
