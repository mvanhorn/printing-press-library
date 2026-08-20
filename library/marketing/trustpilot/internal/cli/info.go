package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPInfoCmd(flags *rootFlags) *cobra.Command {
	var fromLocal bool
	var includeSimilar bool
	cmd := &cobra.Command{
		Use:   "info <domain>",
		Short: "Show TrustScore, totalReviews, rating histogram, and Trustpilot's AI summary",
		Long:  "Shows the business-unit details Trustpilot publishes about a company: TrustScore, total review count, star distribution, claimed status, similarBusinessUnits, and Trustpilot's own AI summary. Reads from the local store with --local; otherwise hits the live API (and caches the result).",
		Example: `  trustpilot-pp-cli info www.thriftbooks.com --json
  trustpilot-pp-cli info www.thriftbooks.com --similar
  trustpilot-pp-cli info www.thriftbooks.com --local --json`,
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
				bu, lerr := tpkg.LoadCompany(ctx, db, domain)
				if lerr != nil {
					return fmt.Errorf("no cached info for %s; run `trustpilot-pp-cli info %s` (without --local) or `sync %s` first", domain, domain, domain)
				}
				return renderInfo(cmd, flags, bu, includeSimilar, "local")
			}

			sess, _, err := loadOrHarvestSession(ctx, db, true)
			if err != nil {
				return err
			}
			page, err := fetchPageWithRetry(ctx, db, &sess, domain, tpkg.PageFilters{Page: 1, Sort: "recency"})
			if err != nil {
				return err
			}
			_ = tpkg.UpsertCompany(ctx, db, page.BusinessUnit)
			return renderInfo(cmd, flags, page.BusinessUnit, includeSimilar, "live")
		},
	}
	cmd.Flags().BoolVar(&fromLocal, "local", false, "Read cached info from the local store instead of calling the API")
	cmd.Flags().BoolVar(&includeSimilar, "similar", false, "Include the similarBusinessUnits Trustpilot suggests")
	return cmd
}

func renderInfo(cmd *cobra.Command, flags *rootFlags, bu tpkg.BusinessUnit, includeSimilar bool, source string) error {
	// PATCH: Include a standard meta envelope on info JSON output.
	meta := NewMeta(source)
	addBusinessUnitMeta(&meta, bu)
	// PATCH: Explain empty Trustpilot-provided enrichment fields in meta notices.
	if len(bu.RatingHistogram) == 0 {
		meta.AddNotice(NoticeHistogramEmptyFromAPI)
	}
	if bu.AISummary == "" {
		meta.AddNotice(NoticeAISummaryEmpty)
	}
	payload := map[string]any{
		"domain":               bu.IdentifyingName,
		"displayName":          bu.DisplayName,
		"trustScore":           bu.TrustScore,
		"stars":                bu.Stars,
		"numberOfReviews":      bu.NumberOfReviews,
		"totalFilteredReviews": bu.TotalFilteredReviews,
		"websiteUrl":           bu.WebsiteURL,
		"isClaimed":            bu.IsClaimed,
		"isCollectingReviews":  bu.IsCollectingReviews,
		"categories":           bu.Categories,
		"ratingHistogram":      bu.RatingHistogram,
		"aiSummary":            bu.AISummary,
		"aiSummaryModel":       bu.AISummaryModelVersion,
	}
	if includeSimilar {
		payload["similar"] = bu.SimilarBusinessUnits
	}
	attachMeta(payload, meta)
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, payload)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\nTrustScore: %.1f (%.1f stars over %d reviews)\nClaimed: %v  CollectingReviews: %v\n",
		bu.DisplayName, bu.IdentifyingName, bu.TrustScore, bu.Stars, bu.NumberOfReviews, bu.IsClaimed, bu.IsCollectingReviews)
	if len(bu.RatingHistogram) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Histogram:")
		for i := 5; i >= 1; i-- {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d*=%d", i, bu.RatingHistogram[i])
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	if bu.AISummary != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nAI Summary (Trustpilot, %s):\n%s\n", bu.AISummaryModelVersion, bu.AISummary)
	}
	if includeSimilar && len(bu.SimilarBusinessUnits) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nSimilar companies:")
		for _, s := range bu.SimilarBusinessUnits {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s (TrustScore=%.1f, n=%d)\n", s.IdentifyingName, s.TrustScore, s.NumberOfReviews)
		}
	}
	return nil
}
