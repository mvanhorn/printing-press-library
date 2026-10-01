// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCategorizeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "categorize",
		Short:       "Suggest, then apply, categories for uncategorized transactions from your history",
		Example:     "  actual-budget-pp-cli categorize suggest --min-confidence 0.7 --agent",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelCategorizeSuggestCmd(flags), newNovelCategorizeApplyCmd(flags))
	return cmd
}
