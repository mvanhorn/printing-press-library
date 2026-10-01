package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/internal/cliutil/testenv"
	"testing"
)

func TestTABParseErrorsAreStructuredAndActionable(t *testing.T) {
	for _, args := range [][]string{{"events", "search", "--limit", "abc", "--agent"}, {"events", "search", "--unknown-flag", "--agent"}, {"does-not-exist", "--agent"}, {"events", "search", "--limit"}} {
		t.Run(args[0]+args[len(args)-1], func(t *testing.T) {
			testenv.Isolate(t)
			var f rootFlags
			root := newRootCmd(&f)
			var out, diagnostics bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&diagnostics)
			root.SetArgs(args)
			e := root.Execute()
			if e == nil {
				t.Fatal("invalid args accepted")
			}
			e = tabUsageError(root, e)
			if ExitCode(e) != 2 {
				t.Fatal(e)
			}
			var payload map[string]any
			if json.Unmarshal(out.Bytes(), &payload) != nil || payload["error"] == nil || diagnostics.Len() == 0 {
				t.Fatal(out.String(), diagnostics.String())
			}
		})
	}
}

func TestTABDryRunValidatesLocalInputs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		valid bool
	}{
		{"search limit", []string{"events", "search", "--limit", "0", "--dry-run"}, false},
		{"date", []string{"events", "search", "--from", "2026-99-01", "--dry-run"}, false},
		{"cache conflict", []string{"events", "search", "--fresh", "--offline", "--dry-run"}, false},
		{"detail identity", []string{"events", "detail", "--dry-run"}, false},
		{"detail date", []string{"events", "detail", "e", "--on", "2026-99-01", "--dry-run"}, false},
		{"venue pagination", []string{"venues", "search", "--limit", "0", "--dry-run"}, false},
		{"nearby coordinates", []string{"nearby", "--dry-run"}, false},
		{"compare identity", []string{"compare", "e", "https://evil.example/events/-/x", "--dry-run"}, false},
		{"search preview", []string{"events", "search", "--area", "Roppongi", "--dry-run"}, true},
		{"compare preview", []string{"compare", "e", "f", "--dry-run"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testenv.Isolate(t)
			var f rootFlags
			root := newRootCmd(&f)
			var out, diagnostics bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&diagnostics)
			root.SetArgs(tt.args)
			err := root.Execute()
			if tt.valid {
				if err != nil {
					t.Fatal(err, out.String())
				}
				return
			}
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("invalid preview accepted: %v %s", err, out.String())
			}
			var payload map[string]any
			if json.Unmarshal(out.Bytes(), &payload) != nil || payload["error"] == nil {
				t.Fatal(out.String())
			}
		})
	}
}
