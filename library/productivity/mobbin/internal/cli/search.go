// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/store"
)

// PATCH: Add top-level offline FTS search over the local Mobbin store.
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var entity string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Search the local Mobbin SQLite store across apps, screens, and flows.",
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
			var rows []map[string]any
			switch entity {
			case "apps":
				rows, err = db.SearchApps(cmd.Context(), args[0], limit)
			case "screens":
				rows, err = db.SearchScreens(cmd.Context(), args[0], limit)
			case "flows":
				rows, err = db.SearchFlows(cmd.Context(), args[0], limit)
			case "all":
				rows, err = db.SearchAll(cmd.Context(), args[0], limit)
			default:
				return usageErr(fmt.Errorf("--entity must be apps, screens, flows, or all"))
			}
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
	cmd.Flags().StringVar(&entity, "entity", "all", "Entity to search: apps, screens, flows, all")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	return cmd
}
