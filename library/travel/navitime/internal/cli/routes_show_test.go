package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cliutil/testenv"
)

// TestNovelRoutesShowHelpWires smoke-tests that the routes show command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRoutesShowHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"routes", "show", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("routes show --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "show"} {
		if !strings.Contains(help, want) {
			t.Fatalf("routes show --help missing %q in output:\n%s", want, help)
		}
	}
}
