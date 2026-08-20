// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNovelBuoyCheckCommand(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelBuoyCheckCmd(flags)
	if cmd.Use == "" || cmd.Short == "" {
		t.Fatalf("command missing Use/Short: %q / %q", cmd.Use, cmd.Short)
	}
	// Dry-run must short-circuit before any network/store access and return nil.
	if err := cmd.RunE(cmd, []string{"5842041f4e65fad6a7708807"}); err != nil {
		t.Fatalf("dry-run RunE returned error: %v", err)
	}
}
