// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// Some framework commands ignore --dry-run under --json (doctor probes the
// API, sync and workflow archive walk resources, feedback/profile list print
// a bare array). Wrap them so --dry-run always ends in the standard dry-run
// envelope before any network or store work. Kept in a hand-authored file so
// regeneration preserves it.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, path := range [][]string{{"doctor"}, {"sync"}, {"workflow", "archive"}, {"feedback", "list"}, {"profile", "list"}} {
			c, _, err := root.Find(path)
			if err != nil || c == nil || c == root || c.Name() != path[len(path)-1] || c.RunE == nil {
				continue
			}
			inner := c.RunE
			action := strings.Join(path, " ")
			c.RunE = func(cmd *cobra.Command, args []string) error {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, action)
				}
				return inner(cmd, args)
			}
		}
	})
}
