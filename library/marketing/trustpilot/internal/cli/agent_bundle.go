package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPAgentBundleCmd(flags *rootFlags) *cobra.Command {
	var window, lang string
	var good, bad int
	cmd := &cobra.Command{
		Use:   "agent-bundle <domain>",
		Short: "One-call agent payload: company metadata + AI summary + top recent good and bad + histogram",
		Example: `  trustpilot-pp-cli agent-bundle www.thriftbooks.com --window 30d --good 5 --bad 5 --json
  trustpilot-pp-cli agent-bundle bookshop.org --lang en`,
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
			minDate := windowMinDate(window)
			dateWindow := dateWindowFromCLI(window)
			hasLocalData := false
			if flags.dataSource != "live" {
				localProbe, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain, MinDate: minDate, Limit: 1})
				if err != nil {
					return err
				}
				hasLocalData = len(localProbe) > 0
			}
			source, fallback := resolveDataSource(flags, hasLocalData)
			// PATCH: Source agent-bundle from local data first and mark live fallback.
			meta := NewMeta(source)
			if !hasLocalData && flags.dataSource != "live" {
				meta.AddNotice(NoticeLocalStoreEmpty)
			}
			if fallback {
				meta.AddNotice(NoticeLiveFallback)
			}
			var goodReviews, badReviews []tpkg.Review
			var bu tpkg.BusinessUnit
			if source == "local" {
				goodReviews, err = fetchLocalBucket(ctx, db, domain, []int{5, 4}, lang, minDate, good)
				if err != nil {
					return err
				}
				badReviews, err = fetchLocalBucket(ctx, db, domain, []int{1, 2}, lang, minDate, bad)
				if err != nil {
					return err
				}
				meta.LocalCount = intPtr(len(goodReviews) + len(badReviews))
				if loaded, lerr := tpkg.LoadCompany(ctx, db, domain); lerr == nil {
					bu = loaded
					addBusinessUnitMeta(&meta, bu)
				}
			} else {
				sess, _, err := loadOrHarvestSession(ctx, db, true)
				if err != nil {
					return err
				}
				page, err := fetchPageWithRetry(ctx, db, &sess, domain, tpkg.PageFilters{Page: 1, Sort: "recency"})
				if err != nil {
					return err
				}
				bu = page.BusinessUnit
				addBusinessUnitMeta(&meta, bu)
				goodReviews, _ = fetchBucket(ctx, db, &sess, domain, []int{5, 4}, lang, dateWindow, minDate, good*2)
				badReviews, _ = fetchBucket(ctx, db, &sess, domain, []int{1, 2}, lang, dateWindow, minDate, bad*2)
				goodReviews = goodReviews[:minInt(len(goodReviews), good)]
				badReviews = badReviews[:minInt(len(badReviews), bad)]
			}
			newest := newestReviewAt(goodReviews, badReviews)
			addReviewFreshnessMeta(&meta, newest, minDate)
			// PATCH: Explain empty Trustpilot-provided enrichment fields in meta notices.
			if len(bu.RatingHistogram) == 0 {
				meta.AddNotice(NoticeHistogramEmptyFromAPI)
			}
			if bu.AISummary == "" {
				meta.AddNotice(NoticeAISummaryEmpty)
			}
			companyDomain := bu.IdentifyingName
			if companyDomain == "" {
				companyDomain = domain
			}
			payload := map[string]any{
				"company": map[string]any{
					"domain":          companyDomain,
					"displayName":     bu.DisplayName,
					"trustScore":      bu.TrustScore,
					"stars":           bu.Stars,
					"numberOfReviews": bu.NumberOfReviews,
					"isClaimed":       bu.IsClaimed,
					"categories":      bu.Categories,
				},
				"aiSummary":           bu.AISummary,
				"ratingHistogram":     bu.RatingHistogram,
				"topRecent":           map[string]any{"good": goodReviews, "bad": badReviews},
				"window":              window,
				"fetchedAt":           time.Now().UTC().Format(time.RFC3339),
				"isCollectingReviews": meta.IsCollectingReviews,
				"newestReviewAt":      newest,
			}
			attachMeta(payload, meta)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) TrustScore %.1f\n", bu.DisplayName, bu.IdentifyingName, bu.TrustScore)
			fmt.Fprintf(cmd.OutOrStdout(), "Good: %d  Bad: %d  Window: %s\n", len(goodReviews), len(badReviews), window)
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Time window: 30d, 12w, 6m, 1y")
	cmd.Flags().IntVar(&good, "good", 5, "Number of fresh 4-5 star reviews to include")
	cmd.Flags().IntVar(&bad, "bad", 5, "Number of fresh 1-2 star reviews to include")
	cmd.Flags().StringVar(&lang, "lang", "", "Filter to a single language (ISO 639-1, e.g. en)")
	return cmd
}
