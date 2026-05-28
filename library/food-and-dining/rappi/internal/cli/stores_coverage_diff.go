// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/store"

	"github.com/spf13/cobra"
)

func newStoresCoverageDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		types []string
	)
	cmd := &cobra.Command{
		Use:   "coverage-diff",
		Short: "Delta-vs-prior-snapshot of the store-type coverage matrix",
		Long: `Compare the current store-type counts to the latest snapshot taken
before --since. Emits per (store_type) delta. Re-runs of
'stores coverage' write snapshots automatically, so a weekly
cron of 'stores coverage' produces the timeline this command
walks.`,
		Example:     "  rappi-pp-cli stores coverage-diff --since 2026-04-01 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if len(types) == 0 {
				for _, t := range rappi.StoreTypes {
					types = append(types, t.Slug)
				}
			}
			db, _, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			type delta struct {
				StoreType  string `json:"store_type"`
				Before     int    `json:"before"`
				After      int    `json:"after"`
				Change     int    `json:"change"`
				BaselineAt string `json:"baseline_at,omitempty"`
				LatestAt   string `json:"latest_at,omitempty"`
			}
			out := []delta{}
			for _, t := range types {
				snaps, err := listStoreSnapshots(db, t)
				if err != nil {
					continue
				}
				if len(snaps) == 0 {
					out = append(out, delta{StoreType: t})
					continue
				}
				// PATCH: Keep the default baseline one snapshot behind latest.
				latestID := snaps[len(snaps)-1]
				latestItems, latestAt, _ := loadStoreSnapshot(db, latestID)
				baselineID := selectStoreCoverageBaselineSnapshot(db, snaps, sinceT)
				if baselineID == "" {
					out = append(out, delta{
						StoreType: t, After: len(latestItems),
						LatestAt: latestAt.Format(time.RFC3339),
					})
					continue
				}
				baselineItems, baselineAt, _ := loadStoreSnapshot(db, baselineID)
				out = append(out, delta{
					StoreType:  t,
					Before:     len(baselineItems),
					After:      len(latestItems),
					Change:     len(latestItems) - len(baselineItems),
					BaselineAt: baselineAt.Format(time.RFC3339),
					LatestAt:   latestAt.Format(time.RFC3339),
				})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].StoreType < out[j].StoreType })
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "STORE_TYPE         BEFORE  AFTER  CHANGE   BASELINE_AT")
			for _, d := range out {
				fmt.Fprintf(w, "%-18s %6d %6d %+7d   %s\n", d.StoreType, d.Before, d.After, d.Change, d.BaselineAt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Compare against the latest snapshot taken before this date (YYYY-MM-DD)")
	cmd.Flags().StringSliceVar(&types, "types", nil, "Store types to include")
	return cmd
}

func selectStoreCoverageBaselineSnapshot(db *store.Store, snaps []string, sinceT time.Time) string {
	if len(snaps) < 2 {
		return ""
	}
	candidates := snaps[:len(snaps)-1]
	if sinceT.IsZero() {
		return candidates[len(candidates)-1]
	}
	var baselineID string
	for _, id := range candidates {
		_, takenAt, err := loadStoreSnapshot(db, id)
		if err != nil {
			continue
		}
		if !takenAt.After(sinceT) {
			baselineID = id
		}
	}
	return baselineID
}
