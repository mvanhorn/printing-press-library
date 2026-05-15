// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/store"
)

// PATCH: Add read-only SQL access to the local Mobbin SQLite store.
func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "sql <select-statement>",
		Short:       "Run a read-only SELECT, WITH, or EXPLAIN query against the local Mobbin store.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dbPath == "" {
				dbPath = defaultStorePath()
			}
			db, err := store.Open(context.Background(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
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
