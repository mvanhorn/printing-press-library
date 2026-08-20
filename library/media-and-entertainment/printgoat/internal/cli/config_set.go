// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

func newConfigSetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a local download preference",
		Example: `  printgoat-pp-cli config set download_dir ~/3d-prints
  printgoat-pp-cli config set default_formats stl,3mf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("expected <key> <value>\nUsage: %s <key> <value>", cmd.CommandPath()))
			}
			key, value := args[0], args[1]
			if err := validatePrintgoatConfigKey(key); err != nil {
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("printgoat-pp-cli")
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := store.EnsurePrintgoatSchema(db.DB()); err != nil {
				return fmt.Errorf("preparing local config store: %w", err)
			}
			if _, err := db.DB().ExecContext(ctx, `
INSERT INTO printgoat_config_kv (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
`, key, value); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"key": key, "value": value}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "set %s = %s\n", key, value)
			return nil
		},
	}
	return cmd
}
