// Copyright 2026 h179922. Licensed under Apache-2.0. See LICENSE.
// Novel command: search history — view and manage past searches.
// The search_history table is written to by the fan-out search command
// (novel_fanout_search.go). This file provides read/display/clear commands.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "List past searches",
		Long:  "Show recent search history. The search_history table is populated by the fan-out search command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT id, query, categories, sources_queried, result_count, searched_at
				FROM search_history
				ORDER BY searched_at DESC
				LIMIT ?`, limit)
			if err != nil {
				return fmt.Errorf("listing search history: %w", err)
			}
			defer rows.Close()

			var history []map[string]any
			for rows.Next() {
				var (
					id          int64
					query       string
					categories  *string
					sources     *string
					resultCount *int
					searchedAt  string
				)
				if err := rows.Scan(&id, &query, &categories, &sources, &resultCount, &searchedAt); err != nil {
					return fmt.Errorf("scanning history row: %w", err)
				}
				entry := map[string]any{
					"id":          id,
					"query":       query,
					"searched_at": searchedAt,
				}
				if categories != nil {
					entry["categories"] = *categories
				}
				if sources != nil {
					entry["sources_queried"] = *sources
				}
				if resultCount != nil {
					entry["result_count"] = *resultCount
				}
				history = append(history, entry)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating history: %w", err)
			}

			if len(history) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"history": []any{},
					"count":   0,
				}, flags)
			}

			return printJSONFiltered(cmd.OutOrStdout(), history, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of history entries to show")

	cmd.AddCommand(newHistoryClearCmd(flags))

	return cmd
}

func newHistoryClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear search history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			res, err := db.Exec(`DELETE FROM search_history`)
			if err != nil {
				return fmt.Errorf("clearing history: %w", err)
			}
			deleted, _ := res.RowsAffected()

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":  "cleared",
				"deleted": deleted,
			}, flags)
		},
	}
}
