// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelTrendsTrendingAtCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var flagGeo string

	cmd := &cobra.Command{
		Use:         "at",
		Short:       "See what was trending on a specific past date and geo — a question Google Trends' own live UI cannot answer.",
		Example:     "  google-trends-pp-cli trends trending at --date 2026-06-15 --geo US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagDate == "" {
				return usageErr(fmt.Errorf("--date is required (format YYYY-MM-DD)"))
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "google-trends-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return noLocalMirrorHint(cmd, flags, "google-trends-pp-cli trends trending", make([]gtTrendingTopicRecord, 0))
			}
			defer db.Close()

			query := `SELECT data FROM resources WHERE resource_type = 'gt_trending_topic' AND json_extract(data, '$.date') = ?`
			args2 := []any{flagDate}
			if flagGeo != "" {
				query += ` AND json_extract(data, '$.geo') = ?`
				args2 = append(args2, flagGeo)
			}
			query += ` ORDER BY json_extract(data, '$.rank')`

			rows, err := db.Query(query, args2...)
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}
			out := make([]gtTrendingTopicRecord, 0)
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					rows.Close()
					return fmt.Errorf("scanning row: %w", err)
				}
				var rec gtTrendingTopicRecord
				if err := json.Unmarshal([]byte(data), &rec); err == nil {
					out = append(out, rec)
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("reading rows: %w", err)
			}
			rows.Close()

			return printLocalResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date to look up (YYYY-MM-DD), required")
	cmd.Flags().StringVar(&flagGeo, "geo", "", "Filter by geo code (e.g. US)")
	return cmd
}
