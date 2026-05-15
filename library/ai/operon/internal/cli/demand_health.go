// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

// knownCategories is the closed list demand health checks for coverage gaps.
// Sourced from the operon brand context (defi, fintech, travel, insurance,
// ecommerce, saas, health, education, gambling, general).
var knownCategories = []string{
	"defi", "fintech", "travel", "insurance", "ecommerce",
	"saas", "health", "education", "gambling", "general",
}

type categoryStats struct {
	Category string   `json:"category"`
	Count    int      `json:"count"`
	AvgTrust *float64 `json:"avg_trust_score,omitempty"`
}

type demandHealthReport struct {
	TotalActive       int             `json:"total_active"`
	FreshCount        int             `json:"fresh_count"`
	StaleCount        int             `json:"stale_count"`
	ByCategory        []categoryStats `json:"by_category"`
	MissingCategories []string        `json:"missing_categories"`
	LastSyncMS        int64           `json:"last_sync_ms"`
}

func newDemandHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Composite freshness + coverage report for the locally synced demand index.",
		Long: `Compute a quick health summary over the locally synced demand entries:

  - total active entries
  - count per category
  - average trust score per category (from scoutscore_history)
  - fresh entries (last_seen_at within the last hour)
  - stale entries (more than 24 hours old)
  - known categories that have no entries

Reads from the local store. Run 'operon-pp-cli sync' to refresh.`,
		Example: strings.Trim(`
  operon-pp-cli demand health
  operon-pp-cli demand health --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would summarize demand_entries + scoutscore_history\n")
				return nil
			}

			ctx := context.Background()
			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			lastSync, rowsSynced, err := st.LastSync(ctx, "demand_entries")
			if err != nil {
				return apiErr(fmt.Errorf("reading sync state: %w", err))
			}
			if lastSync == 0 && rowsSynced == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(),
						demandHealthReport{MissingCategories: append([]string(nil), knownCategories...)}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No demand entries synced. Run `operon-pp-cli sync` first.")
				return nil
			}

			entries, err := st.ListDemandEntries(ctx)
			if err != nil {
				return apiErr(err)
			}

			now := time.Now().UnixMilli()
			oneHourAgo := now - (1 * time.Hour).Milliseconds()
			twentyFourHoursAgo := now - (24 * time.Hour).Milliseconds()

			byCat := map[string]int{}
			fresh, stale := 0, 0
			present := map[string]bool{}
			for _, e := range entries {
				byCat[e.Category]++
				present[strings.ToLower(e.Category)] = true
				if e.LastSeenAt >= oneHourAgo {
					fresh++
				}
				if e.LastSeenAt < twentyFourHoursAgo {
					stale++
				}
			}

			// Compute per-category average trust via latest score per advertiser.
			// Cheap approach: pull the latest score for each advertiser in the
			// store, then aggregate by category.
			latestScore := map[string]float64{}
			for _, e := range entries {
				history, err := st.GetTrustHistory(ctx, e.ID)
				if err != nil {
					continue
				}
				for i := len(history) - 1; i >= 0; i-- {
					if history[i].TrustScore != nil {
						latestScore[e.ID] = *history[i].TrustScore
						break
					}
				}
			}
			sumByCat := map[string]float64{}
			countByCat := map[string]int{}
			for _, e := range entries {
				if score, ok := latestScore[e.ID]; ok {
					sumByCat[e.Category] += score
					countByCat[e.Category]++
				}
			}

			stats := make([]categoryStats, 0, len(byCat))
			for cat, count := range byCat {
				s := categoryStats{Category: cat, Count: count}
				if countByCat[cat] > 0 {
					avg := sumByCat[cat] / float64(countByCat[cat])
					s.AvgTrust = &avg
				}
				stats = append(stats, s)
			}
			sort.Slice(stats, func(i, j int) bool {
				if stats[i].Count != stats[j].Count {
					return stats[i].Count > stats[j].Count
				}
				return stats[i].Category < stats[j].Category
			})

			var missing []string
			for _, k := range knownCategories {
				if !present[k] {
					missing = append(missing, k)
				}
			}

			report := demandHealthReport{
				TotalActive:       len(entries),
				FreshCount:        fresh,
				StaleCount:        stale,
				ByCategory:        stats,
				MissingCategories: missing,
				LastSyncMS:        lastSync,
			}

			if flags.asJSON || flags.csv || flags.compact || flags.selectFields != "" {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "total_active : %d\n", report.TotalActive)
			fmt.Fprintf(w, "fresh (<1h)  : %d\n", report.FreshCount)
			fmt.Fprintf(w, "stale (>24h) : %d\n", report.StaleCount)
			fmt.Fprintln(w)
			fmt.Fprintln(w, "category    count  avg_trust")
			for _, s := range report.ByCategory {
				avg := "-"
				if s.AvgTrust != nil {
					avg = fmt.Sprintf("%.1f", *s.AvgTrust)
				}
				fmt.Fprintf(w, "%-10s  %-5d  %s\n", s.Category, s.Count, avg)
			}
			if len(report.MissingCategories) > 0 {
				fmt.Fprintf(w, "\nmissing categories: %s\n", strings.Join(report.MissingCategories, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}
