// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelLayoutCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "layout",
		Short:       "Lay out boards from Markdown or CSV, re-space them on an even grid, and check tidiness",
		Example:     "  milanote-pp-cli layout check 1Ab2Cd3EfGh4iJ --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelLayoutCheckCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelLayoutCsvCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelLayoutGridCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelLayoutMdCmd(flags))
	return cmd
}
