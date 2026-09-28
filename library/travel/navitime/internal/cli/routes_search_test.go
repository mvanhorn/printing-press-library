package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cliutil/testenv"
)

// TestNovelRoutesSearchHelpWires smoke-tests that the routes search command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRoutesSearchHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"routes", "search", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("routes search --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "search"} {
		if !strings.Contains(help, want) {
			t.Fatalf("routes search --help missing %q in output:\n%s", want, help)
		}
	}
}
