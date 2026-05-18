package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPDriftCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	cmd := &cobra.Command{
		Use:   "drift <domain>",
		Short: "Week-over-week trustScore and rating mix from locally synced reviews",
		Example: `  trustpilot-pp-cli drift www.thriftbooks.com --weeks 12 --json
  trustpilot-pp-cli drift bookshop.org`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			domain := normalizeDomain(args[0])
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			minDate := time.Now().UTC().AddDate(0, 0, -7*weeks)
			reviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain, MinDate: minDate})
			if err != nil {
				return err
			}
			if len(reviews) == 0 {
				return fmt.Errorf("no synced reviews for %s in the last %d weeks; run `trustpilot-pp-cli sync-trustpilot %s` first", domain, weeks, domain)
			}
			buckets := map[time.Time][]tpkg.Review{}
			for _, r := range reviews {
				t := r.PublishedDate.UTC()
				if t.IsZero() {
					continue
				}
				y, w := t.ISOWeek()
				start := isoWeekStart(y, w)
				buckets[start] = append(buckets[start], r)
			}
			keys := make([]time.Time, 0, len(buckets))
			for k := range buckets {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
			out := make([]map[string]any, 0, len(keys))
			for _, k := range keys {
				out = append(out, driftWeek(k, buckets[k]))
			}
			payload := map[string]any{"domain": domain, "weeks": weeks, "drift": out}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, row := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d reviews\tmean %.2f\t1* %.1f%%\t5* %.1f%%\n",
					row["weekStart"], row["count"], row["meanRating"], row["pct1Star"], row["pct5Star"])
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of trailing weeks to analyze")
	return cmd
}

func isoWeekStart(year, week int) time.Time {
	t := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, -1)
	}
	return t.AddDate(0, 0, (week-1)*7)
}

func driftWeek(start time.Time, reviews []tpkg.Review) map[string]any {
	var sum, one, five int
	for _, r := range reviews {
		sum += r.Rating
		if r.Rating == 1 {
			one++
		}
		if r.Rating == 5 {
			five++
		}
	}
	n := len(reviews)
	return map[string]any{
		"weekStart":  start.Format("2006-01-02"),
		"count":      n,
		"meanRating": float64(sum) / float64(n),
		"pct1Star":   100 * float64(one) / float64(n),
		"pct5Star":   100 * float64(five) / float64(n),
	}
}
