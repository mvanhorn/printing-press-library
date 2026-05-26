// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newRestaurantsDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		city     string
		category string
		since    string
		noSnap   bool
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "See newcomers and closures in a city + cuisine since the last snapshot",
		Long: `Compare the current restaurant listing for a (city, category) against
the most recent snapshot stored locally that's older than the --since
threshold. Emits added (newcomers) and removed (closures) rows. The
first time you run this command for a given selector it just snapshots
without diff; the second and later runs answer "what's new this week".`,
		Example:     "  rappi-pp-cli restaurants diff --city ciudad-de-mexico --category sushi --since 2026-04-01 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if city == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			// PATCH: Reject malformed --since values instead of silently using the default baseline.
			var sinceT time.Time
			if since != "" {
				var parseErr error
				sinceT, parseErr = time.Parse("2006-01-02", since)
				if parseErr != nil {
					return fmt.Errorf("--since must be YYYY-MM-DD: %w", parseErr)
				}
			}
			db, _, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			// PATCH: Use the root request settings for live Rappi fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			current, err := fetchRestaurantListPage(cmd.Context(), rappiClient, city, category)
			if err != nil {
				return err
			}
			// Build "current" lookup keyed by restaurant URL (the stable ID slug).
			currMap := map[string]rappi.RestaurantListItem{}
			for _, r := range current {
				if r.ID == "" {
					continue
				}
				currMap[r.URL] = r
			}

			// Find the oldest snapshot before --since (if any).
			snaps, err := listRestaurantSnapshots(db, city, category)
			if err != nil {
				return err
			}
			var baseline []rappi.RestaurantListItem
			var baselineAt time.Time
			for _, id := range snaps {
				items, takenAt, err := loadRestaurantSnapshot(db, id)
				if err != nil {
					continue
				}
				if sinceT.IsZero() || !takenAt.After(sinceT) {
					baseline = items
					baselineAt = takenAt
				}
			}

			result := map[string]any{
				"city":     city,
				"category": category,
				"taken_at": time.Now().UTC().Format(time.RFC3339),
			}
			if baseline == nil {
				result["baseline"] = nil
				result["note"] = "No prior snapshot before --since; recording current state. Re-run later to see a diff."
				result["added"] = []any{}
				result["removed"] = []any{}
			} else {
				result["baseline_taken_at"] = baselineAt.Format(time.RFC3339)
				baseMap := map[string]rappi.RestaurantListItem{}
				for _, r := range baseline {
					if r.ID == "" {
						continue
					}
					baseMap[r.URL] = r
				}
				var added, removed []rappi.RestaurantListItem
				for url, r := range currMap {
					if _, found := baseMap[url]; !found {
						added = append(added, r)
					}
				}
				for url, r := range baseMap {
					if _, found := currMap[url]; !found {
						removed = append(removed, r)
					}
				}
				result["added"] = added
				result["removed"] = removed
			}

			if !noSnap {
				if err := snapshotRestaurants(db, city, category, current); err != nil {
					stderrf("warning: snapshot write failed: %v\n", err)
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), result, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Diff %s/%s vs baseline taken %s:\n",
				city, category, formatTime(baselineAt))
			added, _ := result["added"].([]rappi.RestaurantListItem)
			removed, _ := result["removed"].([]rappi.RestaurantListItem)
			fmt.Fprintf(w, "  Added (%d):\n", len(added))
			for _, r := range added {
				fmt.Fprintf(w, "    + %s (%s)\n", r.Name, r.URL)
			}
			fmt.Fprintf(w, "  Removed (%d):\n", len(removed))
			for _, r := range removed {
				fmt.Fprintf(w, "    - %s (%s)\n", r.Name, r.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&city, "city", "", "City slug (required)")
	cmd.Flags().StringVar(&category, "category", "", "Cuisine category slug")
	cmd.Flags().StringVar(&since, "since", "", "Compare against the latest snapshot taken before this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&noSnap, "no-snapshot", false, "Don't write a new snapshot after diffing")
	return cmd
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "(no baseline)"
	}
	return t.Format("2006-01-02 15:04 MST")
}
