// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelStaleButValuableCmd(flags *rootFlags) *cobra.Command {
	var scoreMin float64
	var silentDays int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale-but-valuable",
		Short: "Find high-score, high-revenue contacts whose engagement has cooled — the VIPs going cold.",
		Long: strings.Trim(`
List contacts whose HubSpot score is at least --score-min and whose last
activity is older than --silent-days. Items are ranked by a composite
value_score = hubspotscore + (num_associated_deals * 10) + (total_revenue
/ 1000), so the most valuable cooled contacts come first.

Returns an empty list when hs_last_activity_date or hubspotscore is missing
from the sync — neither field is part of HubSpot's default property set, so
sync needs '--param properties=...' to populate them.
`, "\n"),
		Example: strings.Trim(`
  # VIPs (score >= 70) silent for at least 3 weeks, top 25
  hubspot-pp-cli stale-but-valuable --score-min 70 --silent-days 21 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if scoreMin < 0 {
				return usageErr(fmt.Errorf("--score-min must be >= 0"))
			}
			if silentDays < 0 {
				return usageErr(fmt.Errorf("--silent-days must be >= 0"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			items, scanned, parseErrors, err := computeStaleVIPs(cmd.Context(), db, now, scoreMin, silentDays)
			if err != nil {
				return err
			}
			matches := len(items)
			items = limitInt(items, limit)
			result := map[string]any{
				"score_min":   scoreMin,
				"silent_days": silentDays,
				"items":       items,
				"scanned":     scanned,
				"matches":     matches,
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().Float64Var(&scoreMin, "score-min", 70, "Minimum hubspotscore for a contact to qualify as 'valuable'")
	cmd.Flags().IntVar(&silentDays, "silent-days", 21, "Days since last activity that qualifies a contact as 'stale'")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of items to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}
