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
