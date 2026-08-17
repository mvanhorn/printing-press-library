// Hand-authored transcendence command: chain-consistency. Population-level
// variance of one chain's ratings/prices — unavailable on the website.
// Not generated.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type consistencyView struct {
	Source     string  `json:"source"`
	Disclaimer string  `json:"disclaimer"`
	Chain      string  `json:"chain"`
	Scope      string  `json:"scope"`
	Metric     string  `json:"metric"`
	Hotels     int     `json:"hotels"`
	Mean       float64 `json:"mean"`
	Median     float64 `json:"median"`
	Stdev      float64 `json:"stdev"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Verdict    string  `json:"verdict"`
	Note       string  `json:"note,omitempty"`
}

func newNovelChainConsistencyCmd(flags *rootFlags) *cobra.Command {
	var chain, country, metric string

	cmd := &cobra.Command{
		Use:   "chain-consistency",
		Short: "Measure how consistent one chain's ratings (or prices) are across a scope",
		Long: "Compute the mean, median, spread, and range of a single chain's Hotelist ratings (or " +
			"prices), optionally within a country — to see whether a brand is reliably good or full of " +
			"outliers. A population-level stat the website cannot show. Use 'chain-compare' to pit chains " +
			"against each other. Data is scraped from hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli chain-consistency --chain marriott --country thailand
  hotelist-pp-cli chain-consistency --chain hilton --metric price --json`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if chain == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--chain is required (a name like 'Marriott' or code like 'EM')"))
			}
			if metric == "" {
				metric = "rating"
			}
			if metric != "rating" && metric != "price" {
				return usageErr(fmt.Errorf("--metric must be rating or price"))
			}
			code, display, ok := normalizeChain(chain)
			if !ok {
				return usageErr(fmt.Errorf("unknown chain %q", chain))
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

			scope := "worldwide"
			filters := []apiFilter{filterChain(code)}
			if country != "" {
				cloc, err := resolveLocation(cmd.Context(), c, db, country)
				if err != nil {
					return err
				}
				filters = append(filters, cloc.Filters...)
				scope = cloc.Label
			}

			hotels, err := fetchHotels(cmd.Context(), c, filters, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			hotels = dedupeHotels(hotels)
			storeHotels(db, hotels)

			view := consistencyView{
				Source: hotelistSource, Disclaimer: hotelistDisclaimer,
				Chain: display, Scope: scope, Metric: metric, Hotels: len(hotels),
			}
			if len(hotels) == 0 {
				view.Note = "no hotels found for this chain in scope"
				return printConsistency(cmd.OutOrStdout(), flags, view)
			}
			var xs []float64
			if metric == "rating" {
				xs = ratingsOf(hotels)
			} else {
				xs = pricesOf(hotels)
			}
			view.Mean = round2(meanF(xs))
			view.Median = round2(medianF(xs))
			view.Stdev = round2(stddevF(xs))
			mn, mx := minMaxF(xs)
			view.Min = round2(mn)
			view.Max = round2(mx)
			view.Verdict = consistencyVerdict(metric, view.Stdev)
			return printConsistency(cmd.OutOrStdout(), flags, view)
		},
	}
	cmd.Flags().StringVar(&chain, "chain", "", "Chain name (e.g. 'Marriott') or code (e.g. 'EM')")
	cmd.Flags().StringVar(&country, "country", "", "Restrict to a country")
	cmd.Flags().StringVar(&metric, "metric", "rating", "What to measure: rating or price")
	return cmd
}

func consistencyVerdict(metric string, stdev float64) string {
	if metric != "rating" {
		return ""
	}
	switch {
	case stdev < 0.5:
		return "very consistent — ratings cluster tightly"
	case stdev < 1.0:
		return "fairly consistent"
	case stdev < 1.5:
		return "variable — check individual properties"
	default:
		return "highly variable — significant outlier risk"
	}
}

func printConsistency(out io.Writer, flags *rootFlags, view consistencyView) error {
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, view, flags)
	}
	fmt.Fprintf(out, "%s — %s (%s)\n", view.Chain, view.Scope, view.Metric)
	fmt.Fprintln(out, strings.Repeat("-", 56))
	if view.Note != "" {
		fmt.Fprintf(out, "%s\n", view.Note)
		fmt.Fprintf(out, "%s\n", view.Disclaimer)
		return nil
	}
	fmt.Fprintf(out, "  hotels:  %d\n", view.Hotels)
	fmt.Fprintf(out, "  mean:    %.2f\n", view.Mean)
	fmt.Fprintf(out, "  median:  %.2f\n", view.Median)
	fmt.Fprintf(out, "  stdev:   %.2f\n", view.Stdev)
	fmt.Fprintf(out, "  range:   %.2f – %.2f\n", view.Min, view.Max)
	if view.Verdict != "" {
		fmt.Fprintf(out, "  verdict: %s\n", view.Verdict)
	}
	fmt.Fprintf(out, "%s\n", view.Disclaimer)
	return nil
}
