// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newSyncStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sync-status",
		Short: "Show last sync timestamp and row count for each synced resource",
		Long: `Read the local store and report freshness per resource: row count and the
most recent synced_at timestamp. Use this before any analytic command so you
know whether to call 'sync' first.

Local-only — does not call the API.`,
		Example: `  twilio-pp-cli sync-status --json
  twilio-pp-cli sync-status --select resource,row_count`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("twilio-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			tables, err := listResourceTables(cmd.Context(), db)
			if err != nil {
				return err
			}

			type row struct {
				Resource   string `json:"resource"`
				RowCount   int    `json:"row_count"`
				LastSynced string `json:"last_synced,omitempty"`
				StaleHours *int   `json:"stale_hours,omitempty"`
			}
			out := make([]row, 0, len(tables))
			for _, t := range tables {
				var count int
				var ts *string
				err := db.DB().QueryRowContext(cmd.Context(),
					fmt.Sprintf(`SELECT COUNT(*), MAX(synced_at) FROM "%s"`, t)).Scan(&count, &ts)
				if err != nil {
					continue
				}
				r := row{Resource: t, RowCount: count}
				if ts != nil && *ts != "" {
					r.LastSynced = *ts
					if parsed, err := time.Parse("2006-01-02 15:04:05", *ts); err == nil {
						hours := int(time.Since(parsed).Hours())
						r.StaleHours = &hours
					}
				}
				out = append(out, r)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Resource < out[j].Resource })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// listResourceTables returns the names of the synced-resource tables in the
// store, excluding internal scaffolding (resources, sync_state, FTS shadows).
func listResourceTables(ctx context.Context, db *store.Store) ([]string, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT name FROM sqlite_master
		 WHERE type='table'
		   AND name NOT LIKE 'sqlite_%'
		   AND name NOT IN ('resources','sync_state','is')
		   AND name NOT LIKE '%_fts'
		   AND name NOT LIKE '%_fts_data'
		   AND name NOT LIKE '%_fts_idx'
		   AND name NOT LIKE '%_fts_config'
		   AND name NOT LIKE '%_fts_docsize'
		   AND name NOT LIKE '%_fts_content'
		 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// jsonExtract is a small helper for reading a JSON path from a row's data column.
// Used by transcendence commands that need to surface specific fields without
// pulling the entire JSON blob across the SQL boundary.
func jsonExtract(data []byte, path string) (json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	v, ok := raw[path]
	if !ok {
		return nil, nil
	}
	return v, nil
}
