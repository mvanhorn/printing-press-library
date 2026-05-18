package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPGeoCmd(flags *rootFlags) *cobra.Command {
	var window string
	var limit int
	cmd := &cobra.Command{
		Use:   "geo <domain>",
		Short: "Reviewer-country distribution: counts, avg rating, 1-star rate",
		Example: `  trustpilot-pp-cli geo www.thriftbooks.com --window 90d --limit 20 --json
  trustpilot-pp-cli geo bookshop.org`,
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
			reviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain, MinDate: windowMinDate(window)})
			if err != nil {
				return err
			}
			rows := buildGeoRows(reviews)
			sort.Slice(rows, func(i, j int) bool { return rows[i]["count"].(int) > rows[j]["count"].(int) })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			payload := map[string]any{"domain": domain, "window": window, "countries": rows}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, row := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d reviews\tavg %.2f\t1* %.1f%%\n", row["country"], row["count"], row["meanRating"], row["pct1Star"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "90d", "Time window: 30d, 90d, 12w, 6m, 1y")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum countries to return")
	return cmd
}

func buildGeoRows(reviews []tpkg.Review) []map[string]any {
	type agg struct{ count, sum, one int }
	m := map[string]*agg{}
	for _, r := range reviews {
		country := r.ConsumerCountry
		if country == "" {
			country = "unknown"
		}
		a := m[country]
		if a == nil {
			a = &agg{}
			m[country] = a
		}
		a.count++
		a.sum += r.Rating
		if r.Rating == 1 {
			a.one++
		}
	}
	rows := make([]map[string]any, 0, len(m))
	for country, a := range m {
		rows = append(rows, map[string]any{
			"country":    country,
			"count":      a.count,
			"meanRating": float64(a.sum) / float64(a.count),
			"pct1Star":   100 * float64(a.one) / float64(a.count),
		})
	}
	return rows
}
