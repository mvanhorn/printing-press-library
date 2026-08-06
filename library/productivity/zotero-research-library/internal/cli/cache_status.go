// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: offline cache introspection.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newNovelCacheStatusCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Sync cursors, row and FTS counts, and last-sync age from the local cache",
		Long:        "Reports cache health without touching the network: per-resource sync cursors and ages, row counts, how many items carry indexed full text, FTS index size, and the database file itself. Use before trusting offline 'ground' results.",
		Example:     "  zotero-research-library-pp-cli cache status --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			dbPath := defaultDBPath("zotero-research-library-pp-cli")
			db, err := requireLocalCache(ctx, "zotero-research-library-pp-cli")
			if err != nil {
				return err
			}
			defer db.Close()

			type resourceState struct {
				Resource   string `json:"resource"`
				Rows       int    `json:"rows"`
				LastSynced string `json:"last_synced,omitempty"`
				AgeHours   *int   `json:"age_hours,omitempty"`
				Cursor     string `json:"cursor,omitempty"`
			}
			var states []resourceState
			srows, err := db.DB().QueryContext(ctx,
				`SELECT resource_type, COALESCE(last_cursor, ''), COALESCE(last_synced_at, ''), total_count
				 FROM sync_state ORDER BY resource_type`)
			if err != nil {
				return fmt.Errorf("reading sync state: %w", err)
			}
			defer srows.Close()
			for srows.Next() {
				var rs resourceState
				var syncedAt string
				if err := srows.Scan(&rs.Resource, &rs.Cursor, &syncedAt, &rs.Rows); err != nil {
					return err
				}
				// Zero-value or pre-epoch timestamps mean "never completed" —
				// rendering them as an age would show hundreds of years.
				if t, terr := time.Parse(time.RFC3339, syncedAt); terr == nil && t.Year() > 1971 {
					rs.LastSynced = syncedAt
					age := int(time.Since(t).Hours())
					rs.AgeHours = &age
				}
				states = append(states, rs)
			}
			if err := srows.Err(); err != nil {
				return err
			}

			countOf := func(query string) int {
				var n int
				if err := db.DB().QueryRowContext(ctx, query).Scan(&n); err != nil {
					return -1
				}
				return n
			}
			items := countOf(`SELECT COUNT(*) FROM "items"`)
			withFulltext := countOf(`SELECT COUNT(*) FROM "items" WHERE COALESCE("content", '') != ''`)
			collections := countOf(`SELECT COUNT(*) FROM "collections"`)
			ftsRows := countOf(`SELECT COUNT(*) FROM "items_fts"`)

			var dbBytes int64
			if fi, err := os.Stat(dbPath); err == nil {
				dbBytes = fi.Size()
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"db_path":             dbPath,
					"db_bytes":            dbBytes,
					"items":               items,
					"items_with_fulltext": withFulltext,
					"collections":         collections,
					"items_fts_rows":      ftsRows,
					"resources":           states,
				}, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Cache: %s (%.1f MB)\n", dbPath, float64(dbBytes)/1024/1024)
			fmt.Fprintf(out, "Items: %d (%d with indexed full text)   Collections: %d   FTS rows: %d\n",
				items, withFulltext, collections, ftsRows)
			if len(states) == 0 {
				fmt.Fprintln(out, "No sync has recorded state yet — run 'sync --full'.")
				return nil
			}
			fmt.Fprintln(out, "Synced resources:")
			for _, rs := range states {
				fmt.Fprintf(out, "  %-14s %6d rows", rs.Resource, rs.Rows)
				if rs.AgeHours != nil {
					fmt.Fprintf(out, "   last sync %dh ago", *rs.AgeHours)
				}
				if rs.Cursor != "" {
					fmt.Fprintf(out, "   cursor=%s", rs.Cursor)
				}
				fmt.Fprintln(out)
			}
			return nil
		},
	}
	return cmd
}
