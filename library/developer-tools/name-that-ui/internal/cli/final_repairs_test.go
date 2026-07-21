package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNovelCommandHelpUsesCompleteExecutableExamples(t *testing.T) {
	for _, command := range []string{"context-pack", "crosswalk", "lint", "inventory", "impact"} {
		t.Run(command, func(t *testing.T) {
			var flags rootFlags
			root := newRootCmd(&flags)
			out := &bytes.Buffer{}
			root.SetOut(out)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{command, "--help"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "name-that-ui-pp-cli "+command) {
				t.Fatalf("%s help lacks a complete executable example:\n%s", command, out.String())
			}
		})
	}
}
