// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source live
// pp:client-call
// runSeasonalShift reaches the generated client through the shared
// privacy-redacting novelGet helper.
func newNovelNearbySeasonalShiftCmd(flags *rootFlags) *cobra.Command {
	var placeID string
	var recentDays, baselineDays int
	cmd := &cobra.Command{
		Use:         "seasonal-shift",
		Short:       "Compare two field windows and surface taxa that changed materially.",
		Example:     "  inaturalist-pp-cli nearby seasonal-shift --place-id 97394 --recent-days 30 --baseline-days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSeasonalShift(cmd, flags, placeID, recentDays, baselineDays)
		},
	}
	cmd.Flags().StringVar(&placeID, "place-id", "", "iNaturalist place ID to compare")
	cmd.Flags().IntVar(&recentDays, "recent-days", 30, "Length of the most recent comparison window")
	cmd.Flags().IntVar(&baselineDays, "baseline-days", 30, "Length of the preceding baseline window")
	return cmd
}
