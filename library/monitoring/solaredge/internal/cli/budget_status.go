// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-written: reads the local call-log table that 'site
// health', 'site underperformance', 'site changes', and 'equipment faults'
// write to after each live call. The SolarEdge API exposes no header or
// endpoint for the documented 300-requests-per-day-per-site limit, so this
// is the only quota visibility this CLI can offer, and it only sees calls
// routed through the commands above.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/internal/store"
	"github.com/spf13/cobra"
)

type budgetSiteStatus struct {
	SiteID       string `json:"site_id"`
	CallsToday   int    `json:"calls_tracked_today"`
	DailyLimit   int    `json:"daily_limit"`
	RemainingEst int    `json:"remaining_estimate"`
}

type budgetStatusView struct {
	Sites []budgetSiteStatus `json:"sites"`
	Note  string             `json:"note"`
}

func newNovelBudgetStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status [siteId]",
		Short:       "See how much of today's 300-request quota this CLI has used, per site.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  solaredge-pp-cli budget status --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("solaredge-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			view := budgetStatusView{
				Sites: []budgetSiteStatus{},
				Note:  "Tracks calls made by this CLI's site health/underperformance/changes and equipment faults commands only. The SolarEdge API does not expose the real server-side remaining quota for the documented 300-requests-per-day-per-site limit.",
			}

			if len(args) > 0 && args[0] != "" {
				siteID := args[0]
				n, err := store.SolarEdgeCallsToday(db.DB(), siteID)
				if err != nil {
					return fmt.Errorf("reading call log: %w", err)
				}
				view.Sites = append(view.Sites, newBudgetSiteStatus(siteID, n))
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			counts, err := store.SolarEdgeCallsTodayAllSites(db.DB())
			if err != nil {
				return fmt.Errorf("reading call log: %w", err)
			}
			siteIDs := make([]string, 0, len(counts))
			for id := range counts {
				siteIDs = append(siteIDs, id)
			}
			sort.Strings(siteIDs)
			for _, id := range siteIDs {
				view.Sites = append(view.Sites, newBudgetSiteStatus(id, counts[id]))
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func newBudgetSiteStatus(siteID string, calls int) budgetSiteStatus {
	remaining := solarEdgeDailyRequestLimit - calls
	if remaining < 0 {
		remaining = 0
	}
	return budgetSiteStatus{
		SiteID:       siteID,
		CallsToday:   calls,
		DailyLimit:   solarEdgeDailyRequestLimit,
		RemainingEst: remaining,
	}
}
