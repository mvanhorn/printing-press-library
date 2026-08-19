// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: run one query across every foodpanda market and compare.

package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// fpMarketAnchor is a representative metro coordinate per market, used when the
// caller does not pin one. Markets are geographically disjoint, so a single
// coordinate cannot serve them all.
var fpMarketAnchor = map[string]struct {
	City string
	Lat  float64
	Lng  float64
	Cur  string
}{
	"pk": {"Lahore", 31.5204, 74.3587, "PKR"},
	"bd": {"Dhaka", 23.8103, 90.4125, "BDT"},
	"sg": {"Singapore", 1.3521, 103.8198, "SGD"},
	"my": {"Kuala Lumpur", 3.1390, 101.6869, "MYR"},
	"hk": {"Hong Kong", 22.3193, 114.1694, "HKD"},
	"th": {"Bangkok", 13.7563, 100.5018, "THB"},
}

type fpMarketRow struct {
	Market         string   `json:"market"`
	City           string   `json:"city"`
	Currency       string   `json:"currency"`
	AvailableCount int      `json:"available_count"`
	Sampled        int      `json:"sampled"`
	AvgRating      float64  `json:"avg_rating,omitempty"`
	MedianFee      float64  `json:"median_delivery_fee,omitempty"`
	MinFee         float64  `json:"min_delivery_fee,omitempty"`
	FeePricedCount int      `json:"fee_priced_count"`
	AdBuyerPct     float64  `json:"ad_buyer_percentage"`
	TopVendors     []string `json:"top_vendors,omitempty"`
	FeeNote        string   `json:"fee_note,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type fpMarketView struct {
	Query         string        `json:"query"`
	Markets       []fpMarketRow `json:"markets"`
	FetchFailures []string      `json:"fetch_failures,omitempty"`
	Note          string        `json:"note,omitempty"`
}

func newNovelMarketCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		query   string
		markets string
		sample  int
	)

	cmd := &cobra.Command{
		Use:   "market-compare",
		Short: "Run the same query across every foodpanda market and compare vendor counts, ratings and fees side by side.",
		Long: "Fan one query across foodpanda markets and compare supply, ratings and fees.\n\n" +
			"Each market uses a representative metro coordinate because markets are\n" +
			"geographically disjoint. Fees are in local currency and are NOT converted.\n" +
			"Do NOT use this to compare vendors within one city; use 'fees' for that.",
		Example:     "  foodpanda-pp-cli market-compare --query pizza --markets pk,sg,my --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market-compare")
			}
			wanted := fpMarkets
			if strings.TrimSpace(markets) != "" {
				wanted = nil
				for _, m := range strings.Split(markets, ",") {
					m = strings.ToLower(strings.TrimSpace(m))
					if m == "" {
						continue
					}
					if _, ok := fpMarketAnchor[m]; !ok {
						_ = cmd.Usage()
						return usageErr(fmt.Errorf("unknown market %q (known: %s)", m, strings.Join(fpMarkets, ", ")))
					}
					wanted = append(wanted, m)
				}
			}
			if len(wanted) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--markets resolved to an empty list"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type result struct {
				idx int
				row fpMarketRow
				err error
			}
			ch := make(chan result, len(wanted))
			var wg sync.WaitGroup
			url := fpDiscoVendors
			if strings.TrimSpace(query) != "" {
				url = fpDiscoSearch
			}
			for i, m := range wanted {
				wg.Add(1)
				go func(i int, m string) {
					defer wg.Done()
					a := fpMarketAnchor[m]
					sweep, err := fpSweep(ctx, c, url,
						fpQuery{Lat: a.Lat, Lng: a.Lng, Country: m, Query: query}, 1)
					if err != nil {
						ch <- result{idx: i, row: fpMarketRow{Market: m, City: a.City, Currency: a.Cur}, err: err}
						return
					}
					ch <- result{idx: i, row: summarizeMarket(m, a.City, a.Cur, sweep, sample)}
				}(i, m)
			}
			go func() { wg.Wait(); close(ch) }()

			rows := make([]fpMarketRow, len(wanted))
			errs := make([]error, len(wanted))
			for r := range ch {
				rows[r.idx] = r.row
				errs[r.idx] = r.err
			}

			view := fpMarketView{Query: query}
			failures := make([]string, 0)
			ok := make([]fpMarketRow, 0, len(rows))
			for i, r := range rows {
				if errs[i] != nil {
					r.Error = truncate(errs[i].Error(), 140)
					failures = append(failures, fmt.Sprintf("%s: %s", r.Market, r.Error))
					ok = append(ok, r) // keep visible, excluded from aggregates
					continue
				}
				ok = append(ok, r)
			}
			view.Markets = ok
			if len(failures) > 0 {
				view.FetchFailures = failures
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d market fetches failed; comparison covers the remaining %d\n",
					len(failures), len(wanted), len(wanted)-len(failures))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			label := query
			if label == "" {
				label = "(all vendors)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Market comparison for %s — fees are local currency, not converted\n\n", label)
			out := make([][]string, 0, len(ok))
			unpriced := make([]string, 0, len(ok))
			for _, r := range ok {
				if r.Error != "" {
					out = append(out, []string{r.Market, r.City, "-", "-", "-", "-", "FAILED"})
					continue
				}
				// An unpriced market has no median to show. Print n/a rather
				// than a formatted 0, which reads as free delivery. Fees carry
				// two decimals because SGD/MYR/HKD fees are single-digit with
				// real cents — %.0f turned 4.30 SGD into "4 SGD".
				fee := "n/a"
				status := "ok"
				switch {
				case r.FeePricedCount == 0:
					status = "no fee data"
					unpriced = append(unpriced, r.Market)
				default:
					fee = fmt.Sprintf("%.2f %s", r.MedianFee, r.Currency)
					if r.FeeNote != "" {
						// Priced, but thinly. Qualify the number in the row so
						// the median is not read as market-wide.
						fee += fmt.Sprintf(" (%d)", r.FeePricedCount)
						status = "thin fee data"
					}
				}
				out = append(out, []string{
					r.Market, truncate(r.City, 14), fmt.Sprintf("%d", r.AvailableCount),
					fmt.Sprintf("%.2f", r.AvgRating), fee,
					fmt.Sprintf("%.0f%%", r.AdBuyerPct), status,
				})
			}
			if err := flags.printTable(cmd,
				[]string{"MARKET", "CITY", "VENDORS", "AVG RATING", "MEDIAN FEE", "ADS", "STATUS"}, out); err != nil {
				return err
			}
			if len(unpriced) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: no delivery fee returned for market(s) %s; those rows show n/a, not free delivery. Run 'foodpanda-pp-cli auth login --chrome' and retry for session-priced fees.\n",
					strings.Join(unpriced, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Search text to run in every market (empty compares total supply)")
	cmd.Flags().StringVar(&markets, "markets", "", "Comma-separated markets (default: all known)")
	cmd.Flags().IntVar(&sample, "sample", 5, "Top vendor names to include per market")
	return cmd
}

func summarizeMarket(market, city, cur string, sweep *fpSweepResult, sample int) fpMarketRow {
	row := fpMarketRow{
		Market: market, City: city, Currency: cur,
		AvailableCount: sweep.Available, Sampled: len(sweep.Vendors),
	}
	if len(sweep.Vendors) == 0 {
		return row
	}
	var ratingSum float64
	var rated int
	// Collect only priced fees. foodpanda's dynamic-pricing service stays
	// silent for some markets and returns MinDeliveryFee 0 for every vendor;
	// folding those zeros into the median made an unpriced market render as
	// "0 MYR", which reads as free delivery rather than missing data.
	fees := make([]float64, 0, len(sweep.Vendors))
	ads := 0
	for _, v := range sweep.Vendors {
		if v.Rating > 0 {
			ratingSum += v.Rating
			rated++
		}
		if v.MinDeliveryFee > 0 {
			fees = append(fees, v.MinDeliveryFee)
		}
		if v.AdBuyer() {
			ads++
		}
	}
	if rated > 0 {
		row.AvgRating = fpRound2(ratingSum / float64(rated))
	}
	row.FeePricedCount = len(fees)
	switch {
	case len(fees) == 0:
		row.FeeNote = "no vendor returned a delivery fee in this market; foodpanda prices delivery in a separate service that stays silent without a session, so treat the fee columns as unknown rather than zero"
	default:
		for i := 1; i < len(fees); i++ {
			for j := i; j > 0 && fees[j] < fees[j-1]; j-- {
				fees[j], fees[j-1] = fees[j-1], fees[j]
			}
		}
		row.MinFee = fpRound2(fees[0])
		row.MedianFee = fpRound2(fees[len(fees)/2])
		// A median over a handful of priced vendors is not comparable to one
		// over the whole sample. Say how thin the basis is rather than letting
		// a one-vendor median sit next to a 48-vendor median unqualified.
		if len(fees)*5 < len(sweep.Vendors) {
			row.FeeNote = fmt.Sprintf("delivery fee is based on only %d of %d sampled vendors; the rest returned no price, so this median is not comparable to a fully priced market",
				len(fees), len(sweep.Vendors))
		}
	}
	row.AdBuyerPct = fpRound2(float64(ads) * 100 / float64(len(sweep.Vendors)))
	for _, v := range fpTrim(sweep.Vendors, sample) {
		row.TopVendors = append(row.TopVendors, v.Name)
	}
	return row
}
