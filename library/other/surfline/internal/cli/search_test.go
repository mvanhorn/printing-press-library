// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNovelSearchCommand(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelSearchCmd(flags)
	if cmd.Use == "" || cmd.Short == "" {
		t.Fatalf("command missing Use/Short: %q / %q", cmd.Use, cmd.Short)
	}
	if err := cmd.RunE(cmd, []string{"Trestles"}); err != nil {
		t.Fatalf("dry-run RunE returned error: %v", err)
	}
}
