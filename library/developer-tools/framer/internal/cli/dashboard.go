// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

func newDashboardCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Show status across all synced data in the local store",
		Long: `Displays a summary dashboard of all synced data including resource
counts by type, snapshot count, last sync time, and collection details.`,
		Example: strings.Trim(`
  # Show dashboard
  framer-pp-cli dashboard

  # JSON output
  framer-pp-cli dashboard --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("framer-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'framer-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			rawDB := db.DB()

			// Count resources by type.
			resourceCounts, err := db.Status()
			if err != nil {
				return fmt.Errorf("querying resource counts: %w", err)
			}

			// Total resources.
			totalResources := 0
			for _, c := range resourceCounts {
				totalResources += c
			}

			// Count snapshots (table may not exist yet).
			snapshotCount := 0
			var lastSnapshotLabel sql.NullString
			var lastSnapshotTime sql.NullString
			_ = rawDB.QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM snapshots`).Scan(&snapshotCount)
			if snapshotCount > 0 {
				_ = rawDB.QueryRowContext(cmd.Context(),
					`SELECT label, created_at FROM snapshots ORDER BY id DESC LIMIT 1`,
				).Scan(&lastSnapshotLabel, &lastSnapshotTime)
			}

			// Last sync time: newest updated_at in resources.
			var lastSync sql.NullString
			_ = rawDB.QueryRowContext(cmd.Context(),
				`SELECT MAX(updated_at) FROM resources`).Scan(&lastSync)

			// Collection summary: items per collection (cms-items with collection info).
			collectionSummary := map[string]int{}
			collRows, err := rawDB.QueryContext(cmd.Context(),
				`SELECT json_extract(data, '$.collectionId') AS cid, COUNT(*)
				 FROM resources
				 WHERE resource_type = 'cms-items' AND json_extract(data, '$.collectionId') IS NOT NULL
				 GROUP BY cid`)
			if err == nil {
				defer collRows.Close()
				for collRows.Next() {
					var cid string
					var count int
					if collRows.Scan(&cid, &count) == nil && cid != "" {
						collectionSummary[cid] = count
					}
				}
			}

			if flags.asJSON {
				output := map[string]interface{}{
					"total_resources":   totalResources,
					"resources_by_type": resourceCounts,
					"snapshot_count":    snapshotCount,
					"last_sync":         nullStringValue(lastSync),
					"collections":       collectionSummary,
				}
				if lastSnapshotLabel.Valid {
					output["last_snapshot_label"] = lastSnapshotLabel.String
				}
				if lastSnapshotTime.Valid {
					output["last_snapshot_time"] = lastSnapshotTime.String
				}
				return flags.printJSON(cmd, output)
			}

			// Human-readable output.
			w := cmd.OutOrStdout()

			fmt.Fprintf(w, "Dashboard\n")
			fmt.Fprintf(w, "=========\n\n")

			fmt.Fprintf(w, "Total resources: %d\n", totalResources)
			if lastSync.Valid {
				fmt.Fprintf(w, "Last sync:       %s\n", lastSync.String)
			} else {
				fmt.Fprintf(w, "Last sync:       never\n")
			}
			fmt.Fprintf(w, "Snapshots:       %d\n", snapshotCount)
			if lastSnapshotLabel.Valid {
				fmt.Fprintf(w, "Last snapshot:   %s (%s)\n", lastSnapshotLabel.String, lastSnapshotTime.String)
			}

			// Resources by type table.
			if len(resourceCounts) > 0 {
				fmt.Fprintf(w, "\nResources by Type\n")
				fmt.Fprintf(w, "-----------------\n")

				types := make([]string, 0, len(resourceCounts))
				for t := range resourceCounts {
					types = append(types, t)
				}
				sort.Strings(types)

				headers := []string{"TYPE", "COUNT"}
				rows := make([][]string, 0, len(types))
				for _, t := range types {
					rows = append(rows, []string{t, fmt.Sprintf("%d", resourceCounts[t])})
				}
				if err := flags.printTable(cmd, headers, rows); err != nil {
					return err
				}
			}

			// Collection summary.
			if len(collectionSummary) > 0 {
				fmt.Fprintf(w, "\nCMS Collections\n")
				fmt.Fprintf(w, "---------------\n")

				collIDs := make([]string, 0, len(collectionSummary))
				for id := range collectionSummary {
					collIDs = append(collIDs, id)
				}
				sort.Strings(collIDs)

				headers := []string{"COLLECTION ID", "ITEMS"}
				rows := make([][]string, 0, len(collIDs))
				for _, id := range collIDs {
					rows = append(rows, []string{id, fmt.Sprintf("%d", collectionSummary[id])})
				}
				if err := flags.printTable(cmd, headers, rows); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}

func nullStringValue(ns sql.NullString) interface{} {
	if ns.Valid {
		return ns.String
	}
	return nil
}
