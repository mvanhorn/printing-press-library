package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPRepliesCmd(flags *rootFlags) *cobra.Command {
	var unreplied bool
	var stars, limit int
	cmd := &cobra.Command{
		Use:   "replies <domain>",
		Short: "Reply rate by star bucket; lists unreplied 1-stars with --unreplied",
		Example: `  trustpilot-pp-cli replies www.thriftbooks.com --json
  trustpilot-pp-cli replies bookshop.org --unreplied --stars 1 --limit 25`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if unreplied && !cmd.Flags().Changed("stars") {
				stars = 1
			}
			domain := normalizeDomain(args[0])
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			if unreplied {
				filterStars := []int(nil)
				if stars > 0 {
					filterStars = []int{stars}
				}
				reviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain, Stars: filterStars, Limit: limit})
				if err != nil {
					return err
				}
				out := make([]tpkg.Review, 0, len(reviews))
				for _, r := range reviews {
					if r.ReplyMessage == "" {
						out = append(out, r)
					}
				}
				payload := map[string]any{"domain": domain, "unreplied": out}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return flags.printJSON(cmd, payload)
				}
				for _, r := range out {
					fmt.Fprintf(cmd.OutOrStdout(), "%d* %s %s\n", r.Rating, r.PublishedDate.Format("2006-01-02"), r.Title)
				}
				return nil
			}
			reviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain})
			if err != nil {
				return err
			}
			byRating := make([]map[string]any, 0, 5)
			for rating := 1; rating <= 5; rating++ {
				count, replied := 0, 0
				for _, r := range reviews {
					if r.Rating == rating {
						count++
						if r.ReplyMessage != "" {
							replied++
						}
					}
				}
				rate := 0.0
				if count > 0 {
					rate = float64(replied) / float64(count)
				}
				byRating = append(byRating, map[string]any{"rating": rating, "count": count, "replyRate": rate})
			}
			payload := map[string]any{"domain": domain, "byRating": byRating}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, row := range byRating {
				fmt.Fprintf(cmd.OutOrStdout(), "%d*\t%d reviews\treply rate %.1f%%\n", row["rating"], row["count"], 100*row["replyRate"].(float64))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&unreplied, "unreplied", false, "List unreplied reviews instead of aggregate reply rates")
	cmd.Flags().IntVar(&stars, "stars", 0, "Star rating for --unreplied (0 = all, default 1 when --unreplied)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum unreplied reviews to inspect")
	return cmd
}
