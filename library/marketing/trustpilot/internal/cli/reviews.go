package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPReviewsCmd(flags *rootFlags) *cobra.Command {
	var (
		stars     int
		language  string
		sort      string
		dateWin   string
		verified  bool
		page      int
		maxPages  int
		fromLocal bool
		limit     int
	)
	cmd := &cobra.Command{
		Use:     "reviews-fetch <domain>",
		Aliases: []string{"rfetch"},
		Short:   "Fetch reviews for a company by domain (via the JSON-API, paginated)",
		Long:    "Fetches one or more pages of reviews from Trustpilot's Next.js JSON API for the given company domain. Uses the persisted aws-waf-token cookie; auto-harvests on first use or expiry. With --local, reads from the synced SQLite store instead.",
		Example: `  trustpilot-pp-cli reviews-fetch www.thriftbooks.com --json
  trustpilot-pp-cli reviews-fetch www.thriftbooks.com --stars 1 --max-pages 3
  trustpilot-pp-cli reviews-fetch www.thriftbooks.com --local --limit 50`,
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

			if fromLocal {
				qf := tpkg.QueryFilters{Domain: domain, Limit: orDefaultInt(limit, 50)}
				if stars > 0 {
					qf.Stars = []int{stars}
				}
				if language != "" {
					qf.Language = language
				}
				reviews, err := tpkg.QueryReviews(ctx, db, qf)
				if err != nil {
					return err
				}
				// PATCH: Include local count and newest-review metadata on local reads.
				meta := NewMeta("local")
				meta.LocalCount = intPtr(len(reviews))
				meta.NewestReviewAt = newestReviewAt(reviews)
				payload := map[string]any{"domain": domain, "reviews": reviews, "count": len(reviews), "source": "local"}
				attachMeta(payload, meta)
				return flags.printJSON(cmd, payload)
			}

			sess, _, err := loadOrHarvestSession(ctx, db, true)
			if err != nil {
				return err
			}

			startPage := page
			if startPage <= 0 {
				startPage = 1
			}
			pagesWanted := orDefaultInt(maxPages, 1)
			all := make([]tpkg.Review, 0, 20*pagesWanted)
			var lastPage tpkg.ReviewsPage
			for i := 0; i < pagesWanted; i++ {
				pp, err := fetchPageWithRetry(ctx, db, &sess, domain, tpkg.PageFilters{
					Page: startPage + i, Stars: stars, Language: language, Sort: sort, DateWindow: dateWin, VerifiedOnly: verified,
				})
				if err != nil {
					if len(all) > 0 {
						// Return what we got with a stderr note.
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: stopped at page %d: %v\n", startPage+i, err)
						break
					}
					return err
				}
				lastPage = pp
				_ = tpkg.UpsertCompany(ctx, db, pp.BusinessUnit)
				if _, _, uerr := tpkg.UpsertReviews(ctx, db, pp.Reviews); uerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: persist failed: %v\n", uerr)
				}
				all = append(all, pp.Reviews...)
				if pp.Pagination.TotalPages > 0 && startPage+i >= pp.Pagination.TotalPages {
					break
				}
				if limit > 0 && len(all) >= limit {
					all = all[:limit]
					break
				}
			}
			payload := map[string]any{
				"domain":     domain,
				"reviews":    all,
				"count":      len(all),
				"pagination": lastPage.Pagination,
				"source":     "live",
			}
			// PATCH: Include newest-review metadata on live reads.
			meta := NewMeta("live")
			meta.NewestReviewAt = newestReviewAt(all)
			attachMeta(payload, meta)
			return flags.printJSON(cmd, payload)
		},
	}
	cmd.Flags().IntVar(&stars, "stars", 0, "Filter to a specific star rating (1-5)")
	cmd.Flags().StringVar(&language, "lang", "", "Filter to a single language (ISO 639-1, e.g. en)")
	cmd.Flags().StringVar(&sort, "sort", "recency", "Sort order: recency | relevance")
	cmd.Flags().StringVar(&dateWin, "date", "", "Date window: last30days | last3months | last6months | last12months")
	cmd.Flags().BoolVar(&verified, "verified", false, "Filter to verified reviews only")
	cmd.Flags().IntVar(&page, "page", 1, "Page to start from (1-indexed)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 1, "Maximum number of pages to fetch")
	cmd.Flags().BoolVar(&fromLocal, "local", false, "Read from the local synced store instead of calling the API")
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after N reviews (0 = no limit)")
	return cmd
}

func orDefaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
