// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/store"
)

func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, _ := strings.CutSuffix(s, "d")
		h, _ := time.ParseDuration(days + "h")
		return h * 24
	}
	d, _ := time.ParseDuration(s)
	return d
}

func newStalePromotedCmd(flags *rootFlags) *cobra.Command {
	var staleAfter string
	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "List synced resources older than a threshold",
		Long:        "Checks the local store for resources that haven't been synced within the threshold. Useful for deciding when to re-sync.",
		Example:     "  synology-router-pp-cli stale --older-than 1h",
		Annotations: map[string]string{"pp:endpoint": "stale", "pp:method": "GET", "pp:path": "/stale", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("synology-router-pp-cli")
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			rows, err := s.DB().Query(`SELECT resource_type, total_count, last_synced_at FROM sync_state ORDER BY resource_type`)
			if err != nil {
				return fmt.Errorf("querying sync_state: %w", err)
			}
			defer rows.Close()

			threshold := parseDuration(staleAfter)
			var stale []map[string]any
			for rows.Next() {
				var rtype string
				var count int
				var lastSynced string
				if rows.Scan(&rtype, &count, &lastSynced) != nil {
					continue
				}
				syncTime, timeErr := time.Parse(time.RFC3339, lastSynced)
				if timeErr == nil && time.Since(syncTime) < threshold {
					continue
				}
				stale = append(stale, map[string]any{
					"resource":    rtype,
					"total_count": count,
					"last_synced": lastSynced,
					"threshold":   staleAfter,
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"threshold": staleAfter, "resources": stale}, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && len(stale) > 0 {
				return printAutoTable(cmd.OutOrStdout(), stale)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(map[string]any{"threshold": staleAfter, "resources": stale}), flags)
		},
	}
	cmd.Flags().StringVar(&staleAfter, "older-than", "1h", "Age threshold for staleness (e.g. 1h, 24h, 7d)")
	return cmd
}
