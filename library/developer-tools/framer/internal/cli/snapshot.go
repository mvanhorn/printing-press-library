// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	var label string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Save a labeled snapshot of all synced project data",
		Long: `Takes a snapshot of all synced project data from the local SQLite store
and saves it with a label and timestamp. Snapshots can later be compared
with the 'diff' command to see what changed between two points in time.`,
		Example: strings.Trim(`
  # Take a snapshot with auto-generated label
  framer-pp-cli snapshot

  # Take a snapshot with a custom label
  framer-pp-cli snapshot --label "before-cms-migration"

  # JSON output
  framer-pp-cli snapshot --label "v2-launch" --json`, "\n"),
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

			// Create snapshots table if it doesn't exist.
			_, err = rawDB.ExecContext(cmd.Context(),
				`CREATE TABLE IF NOT EXISTS snapshots (
					id INTEGER PRIMARY KEY,
					label TEXT,
					created_at TEXT,
					data TEXT
				)`)
			if err != nil {
				return fmt.Errorf("creating snapshots table: %w", err)
			}

			// Query all rows from resources table.
			rows, err := rawDB.QueryContext(cmd.Context(),
				`SELECT id, resource_type, data, synced_at, updated_at FROM resources ORDER BY resource_type, id`)
			if err != nil {
				return fmt.Errorf("querying resources: %w", err)
			}
			defer rows.Close()

			var resources []map[string]interface{}
			for rows.Next() {
				var id, resourceType, data string
				var syncedAt, updatedAt string
				if err := rows.Scan(&id, &resourceType, &data, &syncedAt, &updatedAt); err != nil {
					return fmt.Errorf("scanning resource row: %w", err)
				}

				var parsed interface{}
				if json.Unmarshal([]byte(data), &parsed) != nil {
					parsed = data
				}

				resources = append(resources, map[string]interface{}{
					"id":            id,
					"resource_type": resourceType,
					"data":          parsed,
					"synced_at":     syncedAt,
					"updated_at":    updatedAt,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating resources: %w", err)
			}

			serialized, err := json.Marshal(resources)
			if err != nil {
				return fmt.Errorf("serializing resources: %w", err)
			}

			now := time.Now().UTC().Format(time.RFC3339)
			if label == "" {
				label = "snapshot-" + time.Now().UTC().Format("20060102-150405")
			}

			result, err := rawDB.ExecContext(cmd.Context(),
				`INSERT INTO snapshots (label, created_at, data) VALUES (?, ?, ?)`,
				label, now, string(serialized))
			if err != nil {
				return fmt.Errorf("inserting snapshot: %w", err)
			}

			snapshotID, _ := result.LastInsertId()

			output := map[string]interface{}{
				"snapshot_id": snapshotID,
				"label":       label,
				"created_at":  now,
				"row_count":   len(resources),
			}

			if flags.asJSON {
				return flags.printJSON(cmd, output)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Snapshot #%d saved: %q (%d resources)\n", snapshotID, label, len(resources))
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Label for the snapshot (default: auto-generated timestamp)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}
