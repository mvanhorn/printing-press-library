// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCfHygieneDryRun(t *testing.T) {
	out, err := runNovelDryRun(t, newNovelCfHygieneCmd, "--require", "assetTag")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("dry-run output missing 'would': %q", out)
	}
}

func TestCfHygieneRequiresRequire(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelCfHygieneCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--scope", "device"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected usage error when --require missing")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
	if !strings.Contains(err.Error(), "--require is required") {
		t.Fatalf("error = %q, want mention of --require", err.Error())
	}
}

func TestCfHygieneRejectsBadScope(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelCfHygieneCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--require", "x", "--scope", "bogus"})
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected usage error (code 2) for bad --scope, got %v", err)
	}
}
