// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source computed
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
)

func newWheelogCategoriesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "categories", Short: "Show exact source category values and their question-ID ranges.",
		Example: "  wheelog-pp-cli categories --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "show recorded source categories")
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("categories accepts no positional arguments"))
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			if mode == "live" {
				return usageErr(fmt.Errorf("categories is a recorded local source catalog; no live equivalent is supported"))
			}
			return emitWheelog(cmd, flags, map[string]any{"results": wheelog.Categories(), "observed_at": "2026-10-03", "source_url": "https://docs.wheelog.app/en/info/", "note": "Category membership is a source label. Inspect a spot for question labels and actual report counts."}, "computed")
		}}
}
