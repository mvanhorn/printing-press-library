// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0. See LICENSE.
// Command group; its children implement the saved-evidence workflows.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelShortlistCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "shortlist",
		Short:       "Work with shortlist",
		Example:     "  wheelog-pp-cli shortlist changes --data-source local --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
	}
	cmd.AddCommand(newNovelShortlistChangesCmd(flags))
	cmd.AddCommand(newNovelShortlistListCmd(flags))
	return cmd
}
