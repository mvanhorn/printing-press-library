// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

// newNovelBomCmd is the single `bom` parent.
//
// It lives here rather than beside either child on purpose: `verify` and
// `risks` are authored in separate files, and when each file built its own
// parent the first one registered claimed the name and addNovelCommandIfAbsent
// silently dropped the other's subcommand. One parent, both children, one place
// to add the third.
func newNovelBomCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bom",
		Short:       "Equipment-list checks: Designer-version support, EOL, spec availability, and known risks",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelBomVerifyCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBomRisksCmd(flags))
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelBomCmd(flags))
	})
}
