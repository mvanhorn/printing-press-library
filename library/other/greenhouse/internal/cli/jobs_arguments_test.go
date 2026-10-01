// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestJobsCommandsRejectSurplusArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		cmd  func(*rootFlags) *cobra.Command
	}{
		{name: "list", args: []string{"board", "extra"}, cmd: newJobsListCmd},
		{name: "get", args: []string{"board", "42", "extra"}, cmd: newJobsGetCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd(&rootFlags{})
			err := cmd.Args(cmd, tc.args)
			if err == nil || !strings.Contains(err.Error(), "arg(s)") {
				t.Fatalf("Args(%q) error = %v, want surplus-argument rejection", tc.args, err)
			}
		})
	}
}
