// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Preserved across regen.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/internal/store"
)

type driftView struct {
	LocationID        string                `json:"location_id"`
	Name              string                `json:"name"`
	Threshold         float64               `json:"threshold"`
	HasBaseline       bool                  `json:"has_baseline"`
	Baseline          *store.RatingSnapshot `json:"baseline,omitempty"`
	CurrentRating     float64               `json:"current_rating"`
	CurrentNumReviews int                   `json:"current_num_reviews"`
	CurrentRanking    int                   `json:"current_ranking"`
	RatingDelta       float64               `json:"rating_delta"`
	ReviewsDelta      int                   `json:"reviews_delta"`
	RankingDelta      int                   `json:"ranking_delta"`
	RatingDropped     bool                  `json:"rating_dropped"`
	Note              string                `json:"note,omitempty"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		threshold float64
		since     string
		language  string
		noRecord  bool
	)

	cmd := &cobra.Command{
		Use:   "drift <locationId>",
		Short: "Compare a location's stored rating/review snapshot against a fresh fetch and flag drops",
		Long: "Fetch a location's current rating/review-count/ranking, compare against the most recent snapshot " +
			"saved locally on a previous run, and flag a meaningful drop. Each run also records a new snapshot, " +
			"so drift compounds over time. The first run for a location establishes the baseline.",
		Example: "  tripadvisor-pp-cli drift 93450 --threshold 0.2 --agent",
		// No mcp:read-only hint: drift appends a row to the local
		// rating_snapshots table on every call (unless --no-record), so it
		// modifies local state and must not advertise readOnlyHint=true.
		Annotations: map[string]string{
			"pp:happy-args": "<locationId>=89575",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("locationId is required"))
			}
			id := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cur, err := taFetchDetail(cmd.Context(), c, id, language, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("tripadvisor-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			baseline, hasBaseline, err := db.LastRatingSnapshot(cmd.Context(), id, since)
			if err != nil {
				return fmt.Errorf("reading baseline snapshot: %w", err)
			}

			view := driftView{
				LocationID:        id,
				Name:              cur.Name,
				Threshold:         threshold,
				HasBaseline:       hasBaseline,
				CurrentRating:     cur.Rating,
				CurrentNumReviews: cur.NumReviews,
				CurrentRanking:    cur.Ranking,
			}
			if hasBaseline {
				b := baseline
				view.Baseline = &b
				view.RatingDelta = round2(cur.Rating - baseline.Rating)
				view.ReviewsDelta = cur.NumReviews - baseline.NumReviews
				if baseline.Ranking != 0 && cur.Ranking != 0 {
					view.RankingDelta = cur.Ranking - baseline.Ranking
				}
				view.RatingDropped = (baseline.Rating - cur.Rating) >= threshold
				if view.RatingDropped {
					view.Note = fmt.Sprintf("rating dropped %.2f since %s (threshold %.2f)", baseline.Rating-cur.Rating, baseline.CapturedAt, threshold)
				}
			} else {
				view.Note = "no prior snapshot; recorded a baseline. Run drift again later to detect movement."
			}

			if !noRecord {
				if rErr := db.RecordRatingSnapshot(cmd.Context(), store.RatingSnapshot{
					LocationID: id,
					Name:       cur.Name,
					Rating:     cur.Rating,
					NumReviews: cur.NumReviews,
					Ranking:    cur.Ranking,
				}); rErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not record snapshot: %v\n", rErr)
				}
			}
			return emitTANovel(cmd, flags, view, []taDetail{cur})
		},
	}

	cmd.Flags().Float64Var(&threshold, "threshold", 0.2, "Flag a rating drop of at least this much")
	cmd.Flags().StringVar(&since, "since", "", "Compare against the last snapshot before this RFC3339 timestamp")
	cmd.Flags().StringVar(&language, "language", "en", "Language code")
	cmd.Flags().BoolVar(&noRecord, "no-record", false, "Do not save a new snapshot this run")
	return cmd
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5*sign(f))) / 100
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}
