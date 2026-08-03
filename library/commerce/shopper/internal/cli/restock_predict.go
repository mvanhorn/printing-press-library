// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command: restock predictor.
// Predicts when basket staples will run out from historical cart snapshot cadence.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/store"
	"github.com/spf13/cobra"
)

type restockPrediction struct {
	ProductID      string  `json:"product_id"`
	Name           string  `json:"name"`
	AvgQtyPerCycle float64 `json:"avg_qty_per_cycle"`
	CycleDays      float64 `json:"cycle_days"`
	RunOutInDays   float64 `json:"run_out_in_days"`
	SuggestAdd     bool    `json:"suggest_add"`
}

type restockResult struct {
	Status      string              `json:"status"`
	Note        string              `json:"note,omitempty"`
	HorizonDays int                 `json:"horizon_days"`
	Snapshots   int                 `json:"snapshots"`
	Predictions []restockPrediction `json:"predictions"`
}

func newNovelRestockPredictCmd(flags *rootFlags) *cobra.Command {
	var flagHorizon string
	var flagSuggestAdds bool

	cmd := &cobra.Command{
		Use:     "predict",
		Short:   "Predicts when you'll run out of each staple from your historical buying cadence",
		Example: "  shopper-pp-cli restock predict --horizon 14d --suggest-adds --json",
		Long: `Analyzes your accumulated cart_snapshots to estimate per-SKU consumption cadence
and predict which items might run out within --horizon days.

Needs at least 2 cart snapshots taken on different dates. Snapshots accumulate
as you run 'basket diff' over time.`,
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"would":"analyze cart_snapshots and predict restock needs"}`)
				return nil
			}

			horizonStr := flagHorizon
			if horizonStr == "" {
				horizonStr = "14d"
			}
			horizonDur, err := cliutil.ParseDurationLoose(horizonStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --horizon %q: %w", flagHorizon, err))
			}
			horizonDays := int(horizonDur.Hours() / 24)

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("shopper-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			snaps, err := store.LatestCartSnapshots(db.DB(), 50)
			if err != nil {
				return fmt.Errorf("reading cart snapshots: %w", err)
			}

			result := restockResult{
				HorizonDays: horizonDays,
				Snapshots:   len(snaps),
				Predictions: make([]restockPrediction, 0),
			}

			if len(snaps) < 2 {
				result.Status = "insufficient_history"
				result.Note = fmt.Sprintf(
					"Need at least 2 cart snapshots to compute consumption cadence; found %d. "+
						"Run 'shopper-pp-cli basket diff' on different days to accumulate history.",
					len(snaps),
				)
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			type skuHistory struct {
				name   string
				points []struct {
					takenAt time.Time
					qty     float64
				}
			}

			skuMap := make(map[string]*skuHistory)
			for i := len(snaps) - 1; i >= 0; i-- {
				snap := snaps[i]
				for _, item := range snap.Items {
					h := skuMap[item.ID]
					if h == nil {
						h = &skuHistory{name: item.Name}
						skuMap[item.ID] = h
					}
					h.points = append(h.points, struct {
						takenAt time.Time
						qty     float64
					}{takenAt: snap.TakenAt, qty: item.Qty})
				}
			}

			type pred struct {
				id   string
				pred restockPrediction
			}
			var preds []pred

			for id, h := range skuMap {
				if len(h.points) < 2 {
					continue
				}
				totalQty := 0.0
				for _, pt := range h.points {
					totalQty += pt.qty
				}
				avgQty := totalQty / float64(len(h.points))
				productSpan := h.points[len(h.points)-1].takenAt.Sub(h.points[0].takenAt).Hours() / 24
				cycleDays := productSpan / float64(len(h.points)-1)
				if cycleDays < 1 {
					cycleDays = 7
				}
				dailyConsumption := avgQty / cycleDays
				if dailyConsumption <= 0 {
					continue
				}
				lastOrder := h.points[len(h.points)-1].takenAt
				daysSinceLast := time.Since(lastOrder).Hours() / 24
				if daysSinceLast < 0 {
					daysSinceLast = 0
				}
				runOutDays := cycleDays - daysSinceLast
				if runOutDays < 0 {
					runOutDays = 0
				}
				suggestAdd := flagSuggestAdds && runOutDays <= float64(horizonDays)

				preds = append(preds, pred{
					id: id,
					pred: restockPrediction{
						ProductID:      id,
						Name:           h.name,
						AvgQtyPerCycle: math.Round(avgQty*100) / 100,
						CycleDays:      math.Round(cycleDays*100) / 100,
						RunOutInDays:   math.Round(runOutDays*100) / 100,
						SuggestAdd:     suggestAdd,
					},
				})
			}

			sort.Slice(preds, func(i, j int) bool {
				return preds[i].pred.RunOutInDays < preds[j].pred.RunOutInDays
			})

			for _, p := range preds {
				if p.pred.RunOutInDays <= float64(horizonDays) {
					result.Predictions = append(result.Predictions, p.pred)
				}
			}

			if len(result.Predictions) == 0 {
				result.Status = "no_runouts"
				result.Note = fmt.Sprintf("No items predicted to run out within %d days based on %d snapshots.", horizonDays, len(snaps))
			} else {
				result.Status = "predictions"
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagHorizon, "horizon", "14d", "Prediction horizon (e.g. 14d, 4w)")
	cmd.Flags().BoolVar(&flagSuggestAdds, "suggest-adds", false, "Mark items predicted to run out as suggest_add=true")
	return cmd
}
