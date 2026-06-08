// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestAlertFlappersDryRun(t *testing.T) {
	out, err := runNovelDryRun(t, newNovelAlertFlappersCmd, "--window", "30d")
	if err != nil {
		t.Fatalf("dry-run err: %v", err)
	}
	if !strings.Contains(out, "would") {
		t.Fatalf("dry-run output missing 'would': %q", out)
	}
}

func TestAlertFlappersBadWindow(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelAlertFlappersCmd(flags)
	cmd.SetArgs([]string{"--window", "xyz"})
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected usage error (code 2) for bad --window, got %v", err)
	}
}
