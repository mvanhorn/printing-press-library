// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/store"
)

func newStatsPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Aggregated network statistics from local store",
		Long:        "Queries the local SQLite store for aggregated statistics: total devices, bandwidth totals, and resource counts by type.",
		Example:     "  synology-router-pp-cli stats",
		Annotations: map[string]string{"pp:endpoint": "stats", "pp:method": "GET", "pp:path": "/stats", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("synology-router-pp-cli")
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			stats := map[string]any{}

			rows, err := s.DB().Query(`SELECT resource_type, COUNT(*) as cnt FROM resources GROUP BY resource_type ORDER BY resource_type`)
			if err == nil {
				defer rows.Close()
				resourceCounts := map[string]int{}
				total := 0
				for rows.Next() {
					var rtype string
					var cnt int
					if rows.Scan(&rtype, &cnt) == nil {
						resourceCounts[rtype] = cnt
						total += cnt
					}
				}
				stats["total_resources"] = total
				stats["by_type"] = resourceCounts
			}

			syncRows, syncErr := s.DB().Query(`SELECT resource_type, total_count, last_synced_at FROM sync_state ORDER BY resource_type`)
			if syncErr == nil {
				defer syncRows.Close()
				var syncInfo []map[string]any
				for syncRows.Next() {
					var rtype string
					var count int
					var lastSynced string
					if syncRows.Scan(&rtype, &count, &lastSynced) == nil {
						syncInfo = append(syncInfo, map[string]any{
							"resource":    rtype,
							"row_count":   count,
							"last_synced": lastSynced,
						})
					}
				}
				stats["sync_state"] = syncInfo
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(stats), flags)
		},
	}
	return cmd
}
