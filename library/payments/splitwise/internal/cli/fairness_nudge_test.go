// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelFairnessNudgeHelpWires smoke-tests that the fairness nudge command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelFairnessNudgeHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"fairness", "nudge", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fairness nudge --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "nudge"} {
		if !strings.Contains(help, want) {
			t.Fatalf("fairness nudge --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestFairnessNudgeRejoinsNameAndDryRunPrecedesHarness(t *testing.T) {
	p := nudgeFixture(t, false)
	out, _, err := runRootArgs(t, "fairness", "nudge", "Friend", "One", "--json", "--db", p)
	if err != nil {
		t.Fatal(err)
	}
	var resolved struct {
		Friend string `json:"friend"`
	}
	if json.Unmarshal([]byte(out), &resolved) != nil || resolved.Friend != "Friend One" {
		t.Fatalf("out=%s", out)
	}
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	out, _, err = runRootArgs(t, "fairness", "nudge", "Friend", "One", "--send", "--dry-run", "--json", "--db", p)
	if err != nil || !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("err=%v out=%s", err, out)
	}
}
