// Hand-authored novel command. Niche go/no-go verdict.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
)

const nicheKeywordSortColumn = "searchVolume"

type nicheView struct {
	Term                  string   `json:"term"`
	Verdict               string   `json:"verdict"`
	SearchVolume          float64  `json:"search_volume"`
	CompetingListings     float64  `json:"competing_listings"`
	CompetingShops        float64  `json:"competing_shops"`
	OpportunityRatio      float64  `json:"opportunity_ratio"`
	AvgPrice              float64  `json:"avg_price"`
	TopSellerAvgAgeMonths float64  `json:"top_seller_avg_age_months"`
	WinnablePct           float64  `json:"winnable_pct"`
	TopSellerSamples      int      `json:"top_seller_samples"`
	Signals               []string `json:"signals"`
}

func newNovelNicheCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "niche <term>",
		Short: "Get a single go/no-go verdict on an Etsy niche, combining keyword demand, competition, best-seller density, price band, and winnability.",
		Long: "Get a single go/no-go verdict on an Etsy niche.\n\n" +
			"Combines keyword demand and competition with the top-selling listings' age and price band to judge not just whether a niche is in demand, but whether its current winners are beatable.\n\n" +
			"Use this for a single-term go/no-go before committing designs. To rank across many already-researched terms use 'opportunities'; for whether a term is moving over time use 'drift'.",
		Example: "  listingview-pp-cli niche \"retro cat mom sweatshirt\" --agent",
		// Any free-text term is valid; an unknown niche returns INSUFFICIENT-DATA, not an error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would score niche demand vs competition and best-seller winnability")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			term := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Keyword demand/competition.
			kwData, err := callProxyPOST(ctx, c, "getFilteredKeywords", map[string]any{
				"search": term, "sort_column": nicheKeywordSortColumn, "sort_order": "desc", "page": 1, "limit": 1, "filters": map[string]any{},
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := nicheView{Term: term, Signals: []string{}}
			if kws := listOf(kwData, "keywords"); len(kws) > 0 {
				k := kws[0]
				applyNicheKeywordMetrics(&view, k)
				if view.CompetingListings > 0 {
					view.OpportunityRatio = round2(view.SearchVolume / view.CompetingListings)
				}
				if db := tryOpenStore(dbPath); db != nil {
					saveSnapshot(db.DB(), "keyword", term, map[string]float64{
						"volume": view.SearchVolume, "competingListings": view.CompetingListings,
						"competingShops": view.CompetingShops, "averagePrice": view.AvgPrice,
					})
					_ = db.Close()
				}
			} else {
				view.Signals = append(view.Signals, "no keyword data found for this term")
			}

			// Top-seller winnability.
			lData, err := callProxyPOST(ctx, c, "getFilteredListings", map[string]any{
				"search": term, "sort_column": "sales", "sort_order": "desc", "page": 1, "limit": 20,
				"filter": "top", "search_after": nil, "timeframe": "", "filters": map[string]any{}, "salesInterval": 30,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			listings := listOf(lData, "listings")
			if len(listings) > 0 {
				var ageSum float64
				var ageCount, recent int
				now := time.Now()
				for _, l := range listings {
					age := listingAgeMonths(l, now)
					if age > 0 {
						ageSum += age
						ageCount++
						if age < 12 {
							recent++
						}
					}
				}
				view.TopSellerSamples = ageCount
				if ageCount > 0 {
					view.TopSellerAvgAgeMonths = round2(ageSum / float64(ageCount))
					view.WinnablePct = round2(float64(recent) / float64(ageCount) * 100)
				} else {
					view.Signals = append(view.Signals, "top listings did not include usable listing dates")
				}
			} else {
				view.Signals = append(view.Signals, "no top listings found for this term")
			}

			view.Verdict, view.Signals = nicheVerdict(view)
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path for snapshot history (defaults to the standard location).")
	return cmd
}

func applyNicheKeywordMetrics(view *nicheView, keyword map[string]json.RawMessage) {
	view.SearchVolume = numOf(keyword, "searchVolume")
	view.CompetingListings = numOf(keyword, "competition")
	view.CompetingShops = numOf(keyword, "competitionShops")
	view.AvgPrice = numOf(keyword, "avgPrice")
}

func listingAgeMonths(listing map[string]json.RawMessage, now time.Time) float64 {
	if age := numOf(listing, "ageInMonths"); age > 0 {
		return age
	}
	listedText := strOf(listing, "dateListed")
	if listedText == "" {
		return 0
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		listed, err := time.Parse(layout, listedText)
		if err != nil {
			continue
		}
		if !listed.Before(now) {
			return 0
		}
		return now.Sub(listed).Hours() / 24 / 30.4375
	}
	return 0
}

func nicheVerdict(v nicheView) (string, []string) {
	signals := v.Signals
	if v.CompetingListings == 0 || v.TopSellerSamples == 0 {
		return "INSUFFICIENT-DATA", append(signals, "not enough demand/competition data to judge")
	}
	signals = append(signals, fmt.Sprintf("demand/competition ratio %.1f (volume %.0f vs %.0f competing listings)", v.OpportunityRatio, v.SearchVolume, v.CompetingListings))
	signals = append(signals, fmt.Sprintf("%.0f%% of top sellers are under 12 months old (avg age %.1f mo)", v.WinnablePct, v.TopSellerAvgAgeMonths))
	switch {
	case v.OpportunityRatio >= 10 && v.WinnablePct >= 30:
		return "GO", append(signals, "strong demand relative to competition and recent winners prove the niche is still beatable")
	case v.OpportunityRatio >= 3 && v.WinnablePct >= 20:
		return "CAUTION", append(signals, "workable demand but the field is competitive — differentiate strongly")
	case v.OpportunityRatio < 3:
		return "AVOID", append(signals, "demand is low relative to competition")
	case v.WinnablePct < 15:
		return "AVOID", append(signals, "top sellers are entrenched/old; hard to break in without a unique angle")
	default:
		// Solid demand (ratio >= 3) but too few recent winners to clear GO/CAUTION.
		return "CAUTION", append(signals, "demand is solid but few top sellers are recent — a strong, differentiated listing can still break in")
	}
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
