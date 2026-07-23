// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelTrainingBriefHelpWires smoke-tests that the training-brief command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTrainingBriefHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"training-brief", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("training-brief --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "training-brief"} {
		if !strings.Contains(help, want) {
			t.Fatalf("training-brief --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLowestPuzzleThemesReturnsThreeLowestPerformedThemes(t *testing.T) {
	data := []byte(`{"themes":{"fork":{"results":{"nb":3,"performance":800}},"mate":{"results":{"nb":2,"performance":900}},"pin":{"results":{"nb":1,"performance":700}},"unused":{"results":{"nb":0,"performance":1}}}}`)
	got, err := lowestPuzzleThemes(data)
	if err != nil {
		t.Fatalf("lowestPuzzleThemes() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Theme != "pin" || got[0].FollowUp != "lichess-pp-cli puzzle next --angle pin" {
		t.Fatalf("first weakness = %#v", got[0])
	}
	if got[1].FollowUp != "" || got[2].FollowUp != "" {
		t.Fatalf("only the lowest theme may have a follow-up: %#v", got)
	}
}
