// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/tabiwa"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newCatalogSaved(flags *rootFlags) *cobra.Command {
	o := &catalogOptions{}
	cmd := &cobra.Command{Use: "saved", Short: "Read previously selected catalog evidence offline with original observation times", Example: "  tabiwa-pp-cli catalog saved --region 10 --limit 3 --agent"}
	addRegion(cmd, o)
	cmd.Flags().IntVar(&o.limit, "limit", 10, "Maximum saved observations to return, from 1 to 50")
	catalogAnnotate(cmd, "--region=10;--limit=3", true)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read saved catalog evidence")
		}
		if len(args) > 0 {
			return catalogUsage("catalog saved does not accept positional arguments")
		}
		if _, err := tabiwa.Region(o.region); err != nil {
			return catalogUsage(err.Error())
		}
		if err := validateDataSourceStrategy(flags, "local"); err != nil {
			return catalogUsage(err.Error())
		}
		if o.limit < 1 || o.limit > 50 {
			return catalogUsage("--limit must be 1..50")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		path, err := savedPath()
		if err != nil {
			return err
		}
		rows, err := tabiwa.Saved(ctx, path, o.region, o.limit)
		if err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"products": rows, "region_id": o.region, "count": len(rows), "observation_state": "saved_historical_evidence", "note": "These are saved observations with original source clocks; they are not a current stock or terms check. " + catalogNote})
	}
	return cmd
}
