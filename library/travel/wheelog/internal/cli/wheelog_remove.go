// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newWheelogRemoveCmd(flags *rootFlags) *cobra.Command {
	var options wheelogReadOptions
	cmd := &cobra.Command{Use: "remove [spot-id]", Short: "Remove one selected public spot from the local shortlist.", Example: "  wheelog-pp-cli shortlist remove 166345 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:local-write": "true", "pp:data-source": "local", "pp:happy-args": "spot-id=166347;--data-source=local", "pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "remove selected public spot from local shortlist")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("shortlist remove needs exactly one public spot ID"))
			}
			id, err := wheelogID(args[0])
			if err != nil {
				return err
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			if mode == "live" {
				return usageErr(fmt.Errorf("shortlist remove has no live equivalent; source records are read-only"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openWheelogStore(ctx, options.DB)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.RemoveWheelog(ctx, id); err != nil {
				return configErr(err)
			}
			return emitWheelog(cmd, flags, map[string]any{"id": id, "status": "removed_from_selected_shortlist"}, "local")
		}}
	cmd.Flags().StringVar(&options.DB, "db", defaultDBPath("wheelog-pp-cli"), "Local SQLite shortlist path containing the selected public spot.")
	return cmd
}
