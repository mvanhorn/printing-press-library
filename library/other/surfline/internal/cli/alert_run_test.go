// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNovelAlertRunCommand(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelAlertRunCmd(flags)
	if cmd.Use == "" || cmd.Short == "" {
		t.Fatalf("command missing Use/Short: %q / %q", cmd.Use, cmd.Short)
	}
	// Dry-run must short-circuit before any network/store access and return nil.
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("dry-run RunE returned error: %v", err)
	}
}
