// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
)

type pickRow struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Score     float64            `json:"score"`
	Breakdown map[string]float64 `json:"breakdown"`
	Fee       string             `json:"delivery_fee"`
	ETA       int                `json:"eta_minutes"`
	Rating    float64            `json:"rating"`
	Deals     int                `json:"deals"`
}

func newNovelPickCmd(flags *rootFlags) *cobra.Command {
	var wFee, wEta, wRating, wDeal float64
	var limit int
	var cuisine string
	var pickup bool

	cmd := &cobra.Command{
		Use:   "pick <address>",
		Short: "Recommend one restaurant from a transparent score over fee, rating, deals, and ETA",
		Long: "Score every nearby restaurant on a transparent, weighted blend of low delivery fee, high rating, active deals, and fast ETA, and return the top pick with its score breakdown. The scoring is deterministic arithmetic — no LLM judgment.\n\n" +
			"Use this when you want a single 'just pick one' answer. For the full ranked table use 'compare'.",
		Example:     "  grubhub-pp-cli pick \"350 5th Ave, New York, NY\" --weight-deal 2",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would score nearby restaurants and pick the best value")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an address is required, e.g. pick \"350 5th Ave, New York, NY\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := grubhubClient(ctx, flags)
			if err != nil {
				return err
			}
			coord, err := geocodeAddress(ctx, c, args[0])
			if err != nil {
				return err
			}
			method := "delivery"
			if pickup {
				method = "pickup"
			}
			cards, _, err := searchCards(ctx, c, coord, searchOptions{orderMethod: method, pageSize: 50})
			if err != nil {
				return err
			}
			rows := cardsToRows(cards, cuisine, true) // open-now only: don't recommend a closed place
			if len(rows) == 0 {
				if wantsJSON(cmd, flags) {
					return emitJSON(cmd, flags, map[string]any{"address": args[0], "count": 0, "picks": []pickRow{}})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No open restaurants found near %s, %s.\n", coord.Locality, coord.Region)
				return nil
			}
			weights := scoreWeights{fee: wFee, eta: wEta, rating: wRating, deal: wDeal}
			picks := scorePicks(rows, weights)
			if limit < 1 {
				limit = 1
			}
			if len(picks) > limit {
				picks = picks[:limit]
			}

			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"address": args[0],
					"weights": map[string]float64{"fee": weights.fee, "eta": weights.eta, "rating": weights.rating, "deal": weights.deal},
					"count":   len(picks),
					"picks":   picks,
				})
			}
			return renderPicks(cmd, picks)
		},
	}
	cmd.Flags().Float64Var(&wFee, "weight-fee", 1, "Weight for low delivery fee")
	cmd.Flags().Float64Var(&wEta, "weight-eta", 1, "Weight for fast ETA")
	cmd.Flags().Float64Var(&wRating, "weight-rating", 1, "Weight for high rating")
	cmd.Flags().Float64Var(&wDeal, "weight-deal", 1, "Weight for active deals")
	cmd.Flags().StringVar(&cuisine, "cuisine", "", "Restrict the pick to a cuisine (e.g. pizza)")
	cmd.Flags().IntVar(&limit, "limit", 1, "How many top picks to return")
	cmd.Flags().BoolVar(&pickup, "pickup", false, "Use pickup instead of delivery")
	return cmd
}

type scoreWeights struct{ fee, eta, rating, deal float64 }

// scorePicks computes a normalized 0-100 score per restaurant and returns rows
// sorted best-first. Each component is min-max normalized across the candidate
// set so the weights are comparable.
func scorePicks(rows []restaurantRow, w scoreWeights) []pickRow {
	if len(rows) == 0 {
		return nil
	}
	minFee, maxFee := rows[0].DeliveryFeeCents, rows[0].DeliveryFeeCents
	minETA, maxETA := rows[0].ETAMinutes, rows[0].ETAMinutes
	minRating, maxRating := rows[0].Rating, rows[0].Rating
	maxDeals := 0
	for _, r := range rows {
		minFee = min(minFee, r.DeliveryFeeCents)
		maxFee = max(maxFee, r.DeliveryFeeCents)
		minETA = min(minETA, r.ETAMinutes)
		maxETA = max(maxETA, r.ETAMinutes)
		minRating = min(minRating, r.Rating)
		maxRating = max(maxRating, r.Rating)
		maxDeals = max(maxDeals, r.Deals)
	}
	totalWeight := w.fee + w.eta + w.rating + w.deal
	if totalWeight <= 0 {
		totalWeight = 1
	}
	picks := make([]pickRow, 0, len(rows))
	for _, r := range rows {
		feeScore := normLowerBetter(float64(r.DeliveryFeeCents), float64(minFee), float64(maxFee))
		etaScore := normLowerBetter(float64(r.ETAMinutes), float64(minETA), float64(maxETA))
		ratingScore := normHigherBetter(r.Rating, minRating, maxRating)
		dealScore := 0.0
		if maxDeals > 0 {
			dealScore = float64(r.Deals) / float64(maxDeals)
		}
		raw := w.fee*feeScore + w.eta*etaScore + w.rating*ratingScore + w.deal*dealScore
		score := raw / totalWeight * 100
		picks = append(picks, pickRow{
			ID:    r.ID,
			Name:  r.Name,
			Score: round1(score),
			Breakdown: map[string]float64{
				"fee":    round1(feeScore * 100),
				"eta":    round1(etaScore * 100),
				"rating": round1(ratingScore * 100),
				"deal":   round1(dealScore * 100),
			},
			Fee:    grubhub.Dollars(r.DeliveryFeeCents),
			ETA:    r.ETAMinutes,
			Rating: r.Rating,
			Deals:  r.Deals,
		})
	}
	sort.SliceStable(picks, func(i, j int) bool { return picks[i].Score > picks[j].Score })
	return picks
}

func normLowerBetter(v, lo, hi float64) float64 {
	if hi <= lo {
		return 1
	}
	return 1 - (v-lo)/(hi-lo)
}

// normHigherBetter min-max normalizes a value where higher is better, so the
// rating component is candidate-relative and comparable to the fee/eta weights.
func normHigherBetter(v, lo, hi float64) float64 {
	if hi <= lo {
		return 1
	}
	return (v - lo) / (hi - lo)
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func renderPicks(cmd *cobra.Command, picks []pickRow) error {
	out := cmd.OutOrStdout()
	for i, p := range picks {
		fmt.Fprintf(out, "#%d  %s  (score %.1f)\n", i+1, p.Name, p.Score)
		rating := "-"
		if p.Rating > 0 {
			rating = fmt.Sprintf("%.1f", p.Rating)
		}
		fmt.Fprintf(out, "    fee %s · eta %dm · rating %s · deals %d\n", p.Fee, p.ETA, rating, p.Deals)
		fmt.Fprintf(out, "    breakdown: fee %.0f, eta %.0f, rating %.0f, deal %.0f (id %s)\n",
			p.Breakdown["fee"], p.Breakdown["eta"], p.Breakdown["rating"], p.Breakdown["deal"], p.ID)
	}
	return nil
}
