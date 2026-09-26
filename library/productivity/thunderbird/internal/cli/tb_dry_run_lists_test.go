package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestTBListCommandsHonourDryRun(t *testing.T) {
	for _, args := range [][]string{{"feedback", "list"}, {"profile", "list"}} {
		t.Run(args[0], func(t *testing.T) {
			root := RootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(append(args, "--json", "--dry-run"))
			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &payload); err != nil {
				t.Fatalf("want JSON object, got %q: %v", out.String(), err)
			}
			if payload["dry_run"] != true || payload["action"] == "" || payload["action"] == nil {
				t.Fatalf("want dry_run envelope with action, got %v", payload)
			}
		})
	}
}
