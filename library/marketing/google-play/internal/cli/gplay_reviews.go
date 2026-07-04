// Hand-authored Google Play reviews command (live; persists to local store so
// review-digest can aggregate offline).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/gplay"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

// pp:data-source live
func newReviewsCmd(flags *rootFlags) *cobra.Command {
	var sort string
	var limit, score, device int
	cmd := &cobra.Command{
		Use:   "reviews <appId>",
		Short: "Fetch reviews for an app",
		Long: "Fetch reviews for an app with optional sort (NEWEST, RATING, HELPFULNESS), star filter (--score 1-5), " +
			"and device filter. Reviews are also stored locally so 'review-digest' can aggregate them offline.",
		Example: "  google-play-pp-cli reviews com.dreamgames.royalkingdom --sort NEWEST --limit 100 --agent --select userName,score,text",
		Args:    cobra.ArbitraryArgs,
		// An unknown appId returns an empty review set (HTTP 200), which is
		// indistinguishable from an app that has no reviews; no not-found error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch reviews")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required"))
			}
			sortMode, ok := gplay.NormalizeSort(sort)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort must be NEWEST, RATING, or HELPFULNESS"))
			}
			if score < 0 || score > 5 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--score must be between 1 and 5 (0 for no filter)"))
			}
			// Under live dogfood, curtail to a single page to fit the time budget.
			if cliutil.IsDogfoodEnv() && limit > 40 {
				limit = 40
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			reviews, err := c.Reviews(ctx, args[0], sortMode, limit, score, device)
			if err != nil {
				return classifyGplayErr(err)
			}
			persistReviews(cmd, args[0], reviews)
			if reviews == nil {
				reviews = []gplay.Review{}
			}
			return emit(cmd, flags, reviews)
		},
	}
	cmd.Flags().StringVar(&sort, "sort", "NEWEST", "Sort order: NEWEST, RATING, or HELPFULNESS")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum reviews to fetch")
	cmd.Flags().IntVar(&score, "score", 0, "Filter by star rating 1-5 (0 = all)")
	cmd.Flags().IntVar(&device, "device", 0, "Filter by device: 2 mobile, 3 tablet, 5 chromebook, 6 tv (0 = all)")
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}

func persistReviews(cmd *cobra.Command, appID string, reviews []gplay.Review) {
	if len(reviews) == 0 {
		return
	}
	rows := make([]store.ReviewRow, 0, len(reviews))
	for _, r := range reviews {
		rows = append(rows, store.ReviewRow{
			ReviewID: r.ID,
			Score:    r.Score,
			At:       r.At,
			Version:  r.Version,
			Reply:    r.ReplyText != "",
			Text:     r.Text,
		})
	}
	s, err := openStoreFor(cmd.Context(), resolveDBFlag(cmd))
	if err != nil {
		return
	}
	defer s.Close()
	_ = s.UpsertReviews(cmd.Context(), appID, rows)
}
