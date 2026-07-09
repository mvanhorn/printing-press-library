// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "sql <select-statement>",
		Short:       "Run a read-only SELECT, WITH, or EXPLAIN query against the local Mobbin store.",
		Example:     "  mobbin-pp-cli sql \"SELECT app_name, COUNT(*) FROM screens GROUP BY app_name LIMIT 10\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			db, err := openStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local store yet; run `mobbin-pp-cli sync` first")
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
			}
			defer db.Close()
			rows, err := db.RawQuery(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAutoTable(cmd.OutOrStdout(), rows)
			}
			data, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	return cmd
}
