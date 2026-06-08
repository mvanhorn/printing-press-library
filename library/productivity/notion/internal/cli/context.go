// Copyright 2026 Vincent Lauriat and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/store"
)

type workspaceContext struct {
	GeneratedAt  string             `json:"generated_at"`
	SyncStates   []syncStateEntry   `json:"sync_states"`
	Databases    []json.RawMessage  `json:"databases,omitempty"`
	RecentEdits  []json.RawMessage  `json:"recent_edits,omitempty"`
	TotalRecords int                `json:"total_records"`
}

type syncStateEntry struct {
	ResourceType string `json:"resource_type"`
	TotalCount   int    `json:"total_count"`
	LastSyncedAt string `json:"last_synced_at,omitempty"`
}

func newNovelContextCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "context",
		Short: "Dump the full workspace context as structured JSON — database schemas, property types, recent changes",
		Long: `Produce a structured JSON snapshot of the local Notion workspace — useful for
passing to an LLM agent. Output includes:
  - Per-resource sync state (counts, last-synced timestamps)
  - Database and data source schemas (property names and types)
  - Recently edited pages (when --since is set)

Run 'notion-pp-cli sync' first to populate the local database.

Examples:
  notion-pp-cli context --json
  notion-pp-cli context --since 7d --json
  notion-pp-cli context | pbcopy`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			rawDB := db.DB()

			// 1. Sync states — close rows before opening the next query.
			syncRows, err := rawDB.QueryContext(cmd.Context(),
				`SELECT resource_type, total_count, last_synced_at FROM sync_state ORDER BY resource_type`)
			if err != nil {
				return fmt.Errorf("querying sync_state: %w", err)
			}
			var syncStates []syncStateEntry
			var totalRecords int
			for syncRows.Next() {
				var rt string
				var count int
				var syncedAt sql.NullString
				if err := syncRows.Scan(&rt, &count, &syncedAt); err != nil {
					continue
				}
				e := syncStateEntry{ResourceType: rt, TotalCount: count}
				if syncedAt.Valid {
					e.LastSyncedAt = syncedAt.String
				}
				syncStates = append(syncStates, e)
				totalRecords += count
			}
			syncRows.Close()
			if err := syncRows.Err(); err != nil {
				return fmt.Errorf("iterating sync_state: %w", err)
			}

			// 2. Database schemas — compact to schema-relevant fields only.
			schemaRows, err := rawDB.QueryContext(cmd.Context(),
				`SELECT data FROM resources
				 WHERE resource_type IN ('data_sources', 'databases')
				 ORDER BY json_extract(data, '$.last_edited_time') DESC
				 LIMIT 50`)
			if err != nil {
				return fmt.Errorf("querying database schemas: %w", err)
			}
			var databases []json.RawMessage
			for schemaRows.Next() {
				var data string
				if err := schemaRows.Scan(&data); err != nil {
					continue
				}
				var obj map[string]json.RawMessage
				if err := json.Unmarshal([]byte(data), &obj); err != nil {
					continue
				}
				compact := make(map[string]json.RawMessage, 7)
				for _, k := range []string{"id", "object", "title", "url", "properties", "last_edited_time", "created_time"} {
					if v, ok := obj[k]; ok {
						compact[k] = v
					}
				}
				if b, err := json.Marshal(compact); err == nil {
					databases = append(databases, b)
				}
			}
			schemaRows.Close()
			if err := schemaRows.Err(); err != nil {
				return fmt.Errorf("iterating database schemas: %w", err)
			}

			// 3. Recent edits (only when --since is set).
			var recentEdits []json.RawMessage
			if flagSince != "" {
				since, err := parseSinceArg(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", flagSince, err))
				}
				sinceStr := since.UTC().Format(time.RFC3339)
				editRows, err := rawDB.QueryContext(cmd.Context(),
					`SELECT data FROM resources
					 WHERE resource_type IN ('pages', 'data_sources')
					 AND json_extract(data, '$.last_edited_time') >= ?
					 ORDER BY json_extract(data, '$.last_edited_time') DESC
					 LIMIT ?`,
					sinceStr, flagLimit,
				)
				if err != nil {
					return fmt.Errorf("querying recent edits: %w", err)
				}
				for editRows.Next() {
					var data string
					if err := editRows.Scan(&data); err != nil {
						continue
					}
					recentEdits = append(recentEdits, json.RawMessage(data))
				}
				editRows.Close()
				if err := editRows.Err(); err != nil {
					return fmt.Errorf("iterating recent edits: %w", err)
				}
			}

			if syncStates == nil {
				syncStates = []syncStateEntry{}
			}
			out := workspaceContext{
				GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
				SyncStates:   syncStates,
				Databases:    databases,
				RecentEdits:  recentEdits,
				TotalRecords: totalRecords,
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Include recent edits since this duration or timestamp (e.g. 7d, 2024-01-01)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum recent edits when --since is set")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/github.com/mvanhorn/printing-press-library/library/productivity/notion/data.db)")
	return cmd
}
