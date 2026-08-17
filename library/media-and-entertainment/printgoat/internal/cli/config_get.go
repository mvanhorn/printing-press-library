// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

func newConfigGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get [key]",
		Short:       "Get a local download preference, or all of them with no argument",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  printgoat-pp-cli config get download_dir
  printgoat-pp-cli config get --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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

			if len(args) > 0 {
				key := args[0]
				if err := validatePrintgoatConfigKey(key); err != nil {
					return usageErr(err)
				}
				var value string
				switch err := db.DB().QueryRowContext(ctx, `SELECT value FROM printgoat_config_kv WHERE key = ?`, key).Scan(&value); {
				case err == sql.ErrNoRows:
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"key": key, "value": nil, "set": false}, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s is not set\n", key)
					return nil
				case err != nil:
					return fmt.Errorf("reading config: %w", err)
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"key": key, "value": value, "set": true}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), value)
				return nil
			}

			rows, err := db.DB().QueryContext(ctx, `SELECT key, value FROM printgoat_config_kv ORDER BY key`)
			if err != nil {
				return fmt.Errorf("reading config: %w", err)
			}
			defer rows.Close()
			out := map[string]string{}
			for rows.Next() {
				var k, v string
				if err := rows.Scan(&k, &v); err != nil {
					return err
				}
				out[k] = v
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No config values set.")
				return nil
			}
			for _, k := range validPrintgoatConfigKeys() {
				if v, ok := out[k]; ok {
					fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", k, v)
				}
			}
			return nil
		},
	}
	return cmd
}
