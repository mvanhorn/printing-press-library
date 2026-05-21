// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// newJobsCmd surfaces background-style job state for the multi-source sync
// flow. Coffee-goat's sync orchestrates Shopify (24 roasters), Coffee Review,
// and YouTube concurrently and records per-source state in coffee_sync_state.
// `jobs list` is read-only history; `jobs show` zooms one source.
func newJobsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "jobs",
		Short:       "Inspect recent multi-source sync runs (per-source status, item counts, last-success timestamp)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  coffee-goat-pp-cli jobs list
  coffee-goat-pp-cli jobs show shopify
  coffee-goat-pp-cli jobs list --json`,
	}
	cmd.AddCommand(newJobsListCmd(flags))
	cmd.AddCommand(newJobsShowCmd(flags))
	return cmd
}

func newJobsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List per-source sync history with last status, item count, and last-synced-at",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := loadJobs(ctx, db.DB(), "")
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"jobs": rows, "count": len(rows)}, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  No sync history. Run 'coffee-goat-pp-cli sync' to create one.")
				return nil
			}
			headers := []string{"source", "status", "items", "last_synced_at"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.Source, r.Status, fmt.Sprintf("%d", r.ItemCount), r.LastSyncedAt})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	return cmd
}

func newJobsShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <source>",
		Short:       "Show one source's last sync run (shopify, coffee-review, youtube)",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := loadJobs(ctx, db.DB(), args[0])
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("no sync history for source %q", args[0])
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows[0], flags)
			}
			r := rows[0]
			fmt.Fprintf(cmd.OutOrStdout(), "  source:           %s\n", r.Source)
			fmt.Fprintf(cmd.OutOrStdout(), "  last_status:      %s\n", r.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "  item_count:       %d\n", r.ItemCount)
			fmt.Fprintf(cmd.OutOrStdout(), "  last_synced_at:   %s\n", r.LastSyncedAt)
			return nil
		},
	}
	return cmd
}

type jobRow struct {
	Source       string `json:"source"`
	Status       string `json:"last_status"`
	ItemCount    int    `json:"item_count"`
	LastSyncedAt string `json:"last_synced_at,omitempty"`
}

func loadJobs(ctx context.Context, db *sql.DB, source string) ([]jobRow, error) {
	q := `SELECT source, COALESCE(last_status,''), COALESCE(item_count,0), last_synced_at
	      FROM coffee_sync_state`
	args := []any{}
	if source != "" {
		q += ` WHERE source = ?`
		args = append(args, source)
	}
	q += ` ORDER BY last_synced_at DESC`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []jobRow
	for rows.Next() {
		var r jobRow
		var ts sql.NullTime
		if err := rows.Scan(&r.Source, &r.Status, &r.ItemCount, &ts); err != nil {
			continue
		}
		if ts.Valid {
			r.LastSyncedAt = ts.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
