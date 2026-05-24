// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/store"
)

func newCoveragePromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "coverage",
		Short:       "Show API endpoint coverage and sync completeness",
		Long:        "Queries the local sync_state table to compute coverage percentage per resource type, showing how many resources have been synced versus total available.",
		Example:     "  synology-router-pp-cli coverage",
		Annotations: map[string]string{"pp:endpoint": "coverage", "pp:method": "GET", "pp:path": "/coverage", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("synology-router-pp-cli")
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			rows, err := s.DB().Query(`SELECT ss.resource_type, ss.total_count, COUNT(r.rowid) AS synced_rows FROM sync_state ss LEFT JOIN resources r ON r.resource_type = ss.resource_type GROUP BY ss.resource_type ORDER BY ss.resource_type`)
			if err != nil {
				return fmt.Errorf("querying coverage: %w", err)
			}
			defer rows.Close()

			var coverage []map[string]any
			for rows.Next() {
				var rtype string
				var totalCount, syncedRows int
				if rows.Scan(&rtype, &totalCount, &syncedRows) != nil {
					continue
				}
				pct := 0.0
				if totalCount > 0 {
					pct = float64(syncedRows) * 100 / float64(totalCount)
				}
				coverage = append(coverage, map[string]any{
					"resource":      rtype,
					"total":         totalCount,
					"synced":        syncedRows,
					"coverage_pct":  fmt.Sprintf("%.1f%%", pct),
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), coverage, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && len(coverage) > 0 {
				return printAutoTable(cmd.OutOrStdout(), coverage)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(coverage), flags)
		},
	}
	return cmd
}
