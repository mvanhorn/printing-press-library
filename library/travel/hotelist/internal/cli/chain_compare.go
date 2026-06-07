// Hand-authored transcendence command: chain-compare. Head-to-head chain value
// stats the single-chain map filter cannot produce. Not generated.
package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type chainStat struct {
	Code        string  `json:"code"`
	Chain       string  `json:"chain"`
	Hotels      int     `json:"hotels"`
	MeanRating  float64 `json:"mean_rating"`
	MedianPrice float64 `json:"median_price"`
	MeanValue   float64 `json:"mean_value_per_100usd"`
	RatingStdev float64 `json:"rating_stdev"`
}

type chainCompareView struct {
	Source     string      `json:"source"`
	Disclaimer string      `json:"disclaimer"`
	Scope      string      `json:"scope"`
	Metric     string      `json:"metric"`
	Chains     []chainStat `json:"chains"`
	Skipped    []string    `json:"skipped,omitempty"`
	Note       string      `json:"note,omitempty"`
}

func newNovelChainCompareCmd(flags *rootFlags) *cobra.Command {
	var chains, country, metric string
	var minHotels int

	cmd := &cobra.Command{
		Use:   "chain-compare",
		Short: "Compare hotel chains head-to-head on rating, price, and rating-per-dollar",
		Long: "Compare two or more hotel chains by mean Hotelist rating, median price, rating-per-dollar, " +
			"and rating spread, optionally scoped to a country. The website filters one chain at a time " +
			"with no aggregates; this fetches each chain and computes the comparison locally. Use " +
			"'chain-consistency' to measure variance within a single chain. Data is scraped from " +
			"hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli chain-compare --chains marriott,hilton,hyatt --country japan
  hotelist-pp-cli chain-compare --chains "four seasons,ritz-carlton" --metric rating --json`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			chainList := splitCSV(chains)
			if len(chainList) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--chains requires at least two chains, e.g. --chains marriott,hilton"))
			}
			if metric == "" {
				metric = "best-value"
			}
			if metric != "best-value" && metric != "rating" && metric != "price" {
				return usageErr(fmt.Errorf("--metric must be best-value, rating, or price"))
			}

			c, err := flags.politeClient()
			if err != nil {
				return err
			}
			db, err := openHotelStore(cmd.Context(), flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var countryFilter []apiFilter
			scope := "worldwide"
			if country != "" {
				cloc, err := resolveLocation(cmd.Context(), c, db, country)
				if err != nil {
					return err
				}
				countryFilter = cloc.Filters
				scope = cloc.Label
			}

			view := chainCompareView{Source: hotelistSource, Disclaimer: hotelistDisclaimer, Scope: scope, Metric: metric}
			for _, name := range chainList {
				code, display, ok := normalizeChain(name)
				if !ok {
					view.Skipped = append(view.Skipped, name+" (unknown chain)")
					continue
				}
				filters := append([]apiFilter{filterChain(code)}, countryFilter...)
				hotels, err := fetchHotels(cmd.Context(), c, filters, "")
				if err != nil {
					view.Skipped = append(view.Skipped, display+" (fetch error: "+err.Error()+")")
					continue
				}
				hotels = dedupeHotels(hotels)
				storeHotels(db, hotels)
				// Check empty first so it stays reachable even when --min-hotels
				// is 0 (where the count guard below would not fire).
				if len(hotels) == 0 {
					view.Skipped = append(view.Skipped, display+" (no hotels in scope)")
					continue
				}
				if len(hotels) < minHotels {
					view.Skipped = append(view.Skipped, fmt.Sprintf("%s (only %d hotels < --min-hotels %d)", display, len(hotels), minHotels))
					continue
				}
				ratings := ratingsOf(hotels)
				view.Chains = append(view.Chains, chainStat{
					Code:        code,
					Chain:       chainCodeToName[code],
					Hotels:      len(hotels),
					MeanRating:  round2(meanF(ratings)),
					MedianPrice: round2(medianF(pricesOf(hotels))),
					MeanValue:   round2(meanF(valuesOf(hotels))),
					RatingStdev: round2(stddevF(ratings)),
				})
			}

			sortChainStats(view.Chains, metric)
			return printChainCompare(cmd.OutOrStdout(), flags, view)
		},
	}
	cmd.Flags().StringVar(&chains, "chains", "", "Comma-separated chains (names like 'Marriott' or codes like 'EM')")
	cmd.Flags().StringVar(&country, "country", "", "Restrict comparison to a country")
	cmd.Flags().StringVar(&metric, "metric", "best-value", "Sort metric: best-value (default), rating, or price")
	cmd.Flags().IntVar(&minHotels, "min-hotels", 1, "Skip chains with fewer than this many hotels in scope")
	return cmd
}

func sortChainStats(cs []chainStat, metric string) {
	sort.SliceStable(cs, func(i, j int) bool {
		switch metric {
		case "rating":
			return cs[i].MeanRating > cs[j].MeanRating
		case "price":
			return cs[i].MedianPrice < cs[j].MedianPrice
		default: // best-value
			return cs[i].MeanValue > cs[j].MeanValue
		}
	})
}

func printChainCompare(out io.Writer, flags *rootFlags, view chainCompareView) error {
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, view, flags)
	}
	fmt.Fprintf(out, "Chain comparison — %s (by %s)\n", view.Scope, view.Metric)
	fmt.Fprintln(out, strings.Repeat("-", 72))
	fmt.Fprintf(out, "%-22s %6s %8s %10s %8s\n", "Chain", "hotels", "rating", "med $", "value")
	for _, cs := range view.Chains {
		fmt.Fprintf(out, "%-22s %6d %8.2f %10.0f %8.1f\n", truncate(cs.Chain, 22), cs.Hotels, cs.MeanRating, cs.MedianPrice, cs.MeanValue)
	}
	fmt.Fprintln(out, strings.Repeat("-", 72))
	for _, s := range view.Skipped {
		fmt.Fprintf(out, "skipped: %s\n", s)
	}
	fmt.Fprintf(out, "%s\n", view.Disclaimer)
	return nil
}
