// Copyright 2026 Vincent Lauriat and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/store"
)

func newNovelSqlCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run raw SQL across your entire synced Notion workspace — join Tasks with Projects, filter by any property",
		Long: `Run arbitrary read-only SQL against the local SQLite database populated by 'notion-pp-cli sync'.

Tables available:
  resources       All synced resources (id, resource_type, data JSON, synced_at, updated_at)
  sync_state      Per-resource sync state (resource_type, last_cursor, last_synced_at, total_count)
  children        Block children (id, blocks_id, data JSON, synced_at)
  query           Data source query results (id, data_sources_id, data JSON, synced_at)
  users           Users (id, data JSON, synced_at, avatar_url, name, type)
  resources_fts   Full-text search index (virtual, FTS5)

Use json_extract(data, '$.field') to access JSON properties.

Examples:
  notion-pp-cli sql "SELECT resource_type, COUNT(*) n FROM resources GROUP BY resource_type"
  notion-pp-cli sql "SELECT json_extract(data,'$.title[0].plain_text') title, json_extract(data,'$.last_edited_time') edited FROM resources WHERE resource_type='pages' LIMIT 10"
  notion-pp-cli sql "SELECT * FROM sync_state"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("notion-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'notion-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("getting columns: %w", err)
			}

			var results []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				row := make(map[string]any, len(cols))
				for i, col := range cols {
					v := vals[i]
					if b, ok := v.([]byte); ok {
						v = string(b)
					}
					row[col] = v
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}
			if results == nil {
				results = []map[string]any{}
			}
			return flags.printJSON(cmd, results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/github.com/mvanhorn/printing-press-library/library/productivity/notion/data.db)")
	return cmd
}
