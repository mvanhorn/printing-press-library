// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNewDashboardCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newDashboardCmd(flags)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}
