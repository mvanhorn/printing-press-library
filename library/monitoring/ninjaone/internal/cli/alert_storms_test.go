// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestAlertStormsDryRun(t *testing.T) {
	out, err := runNovelDryRun(t, newNovelAlertStormsCmd, "--window", "30m")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("dry-run output missing 'would': %q", out)
	}
}

func TestAlertStormsBadWindow(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelAlertStormsCmd(flags)
	cmd.SetArgs([]string{"--window", "not-a-duration"})
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected usage error (code 2) for bad --window, got %v", err)
	}
}
