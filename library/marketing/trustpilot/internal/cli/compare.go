package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPCompareCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:   "compare <domainA> <domainB> [<domainC>...]",
		Short: "Side-by-side TrustScore and review-mix across multiple companies (local store)",
		Example: `  trustpilot-pp-cli compare www.thriftbooks.com bookshop.org --window 90d --json
  trustpilot-pp-cli compare www.thriftbooks.com bookshop.org biblio.com`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 || len(args) > 6 {
				return fmt.Errorf("compare requires 2-6 domains")
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			minDate := windowMinDate(window)
			companies := make([]map[string]any, 0, len(args))
			for _, a := range args {
				domain := normalizeDomain(a)
				bu, err := tpkg.LoadCompany(ctx, db, domain)
				if err != nil {
					return fmt.Errorf("no cached company for %s; run `trustpilot-pp-cli sync-trustpilot %s` first", domain, domain)
				}
				reviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain, MinDate: minDate})
				if err != nil {
					return err
				}
				if len(reviews) == 0 {
					return fmt.Errorf("no synced reviews for %s in window %s; run `trustpilot-pp-cli sync-trustpilot %s` first", domain, window, domain)
				}
				one, five := ratingCounts(reviews)
				companies = append(companies, map[string]any{
					"domain":          domain,
					"displayName":     bu.DisplayName,
					"trustScore":      bu.TrustScore,
					"totalReviews":    bu.NumberOfReviews,
					"windowedCount":   len(reviews),
					"pct1Star":        100 * float64(one) / float64(len(reviews)),
					"pct5Star":        100 * float64(five) / float64(len(reviews)),
					"ratingHistogram": bu.RatingHistogram,
				})
			}
			payload := map[string]any{"window": window, "companies": companies}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, c := range companies {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\tTrustScore %.1f\t%d recent\t1* %.1f%%\t5* %.1f%%\n",
					c["domain"], c["trustScore"], c["windowedCount"], c["pct1Star"], c["pct5Star"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "90d", "Time window: 30d, 90d, 12w, 6m, 1y")
	return cmd
}

func ratingCounts(reviews []tpkg.Review) (one, five int) {
	for _, r := range reviews {
		if r.Rating == 1 {
			one++
		}
		if r.Rating == 5 {
			five++
		}
	}
	return one, five
}
