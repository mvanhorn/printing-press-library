package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPSearchReviewsCmd(flags *rootFlags) *cobra.Command {
	var stars int
	var window, lang string
	var limit int
	cmd := &cobra.Command{
		Use:   "search-reviews <domain> <term>",
		Short: "Full-text search over locally synced reviews (FTS5)",
		Long:  "Searches locally synced reviews. A non-empty term uses FTS5 over titles, text, and replies; an empty quoted term lists synced reviews using only the domain, star, language, window, and limit filters.",
		Example: `  trustpilot-pp-cli search-reviews www.thriftbooks.com shipping --stars 1 --window 90d --json
  trustpilot-pp-cli search-reviews bookshop.org "customer service" --limit 20
  trustpilot-pp-cli search-reviews www.thriftbooks.com "" --stars 1 --limit 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return fmt.Errorf("term required")
			}
			if dryRunOK(flags) {
				return nil
			}
			domain := normalizeDomain(args[0])
			term := strings.Join(args[1:], " ")
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			filters := tpkg.QueryFilters{Domain: domain, MinDate: windowMinDate(window), Language: lang, Limit: limit}
			if stars > 0 {
				filters.Stars = []int{stars}
			}
			mode := "fts"
			var reviews []tpkg.Review
			// PATCH: Empty terms are list-mode filters, not invalid FTS5 queries.
			if strings.TrimSpace(term) == "" {
				mode = "list"
				reviews, err = tpkg.QueryReviews(ctx, db, filters)
			} else {
				reviews, err = tpkg.FullTextSearchReviews(ctx, db, term, filters)
			}
			if err != nil {
				return err
			}
			// PATCH: Include local count and newest-review metadata on search results.
			meta := NewMeta("local")
			meta.Mode = mode
			meta.LocalCount = intPtr(len(reviews))
			meta.NewestReviewAt = newestReviewAt(reviews)
			payload := map[string]any{"domain": domain, "term": term, "reviews": reviews, "count": len(reviews)}
			attachMeta(payload, meta)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, r := range reviews {
				fmt.Fprintf(cmd.OutOrStdout(), "%d* %s %s\n%s\n\n", r.Rating, r.PublishedDate.Format("2006-01-02"), r.Title, truncateText(r.Text, 180))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&stars, "stars", 0, "Filter to a specific star rating (1-5)")
	cmd.Flags().StringVar(&window, "window", "", "Time window: 30d, 90d, 12w, 6m, 1y")
	cmd.Flags().StringVar(&lang, "lang", "", "Filter to a single language (ISO 639-1, e.g. en)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum reviews to return")
	return cmd
}
