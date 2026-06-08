// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPatchSweepDryRun(t *testing.T) {
	out, err := runNovelDryRun(t, newNovelPatchSweepCmd, "--df", "org = 5")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("dry-run output missing 'would': %q", out)
	}
}

func TestPatchSweepRequiresDf(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelPatchSweepCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// A flag is set (so we don't hit help) but --df is absent.
	cmd.SetArgs([]string{"--type", "os"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected usage error when --df missing")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
	if !strings.Contains(err.Error(), "--df is required") {
		t.Fatalf("error message = %q, want mention of --df", err.Error())
	}
}

func TestPatchSweepRejectsBadType(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelPatchSweepCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--df", "x", "--type", "bogus"})
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected usage error (code 2) for bad --type, got %v", err)
	}
}
