// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelTemplatesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "templates",
		Short:       "Check envelope funding against #template/#goal targets from category notes",
		Example:     "  actual-budget-pp-cli templates status --month 2026-09 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelTemplatesStatusCmd(flags))
	return cmd
}
