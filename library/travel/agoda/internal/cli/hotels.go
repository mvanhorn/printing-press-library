// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHotelsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "hotels",
		Short:       "Search and price hotels with the true all-in cost",
		Example:     "  agoda-pp-cli hotels fees Tokyo --checkin 2026-10-15 --nights 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelHotelsFeesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHotelsRankCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHotelsSearchCmd(flags))
	return cmd
}
