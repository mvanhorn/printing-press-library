// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
)

// feeRow annotates a property with how its fee load compares to the destination.
type feeRow struct {
	agoda.Property
	FeeRatioPct    float64 `json:"fee_ratio_pct"`
	MedianRatioPct float64 `json:"destination_median_ratio_pct"`
	ExcessPct      float64 `json:"excess_over_median_pct"`
	Outlier        bool    `json:"outlier"`
}

type feesResult struct {
	Destination         string   `json:"destination"`
	CityID              int      `json:"city_id"`
	CheckIn             string   `json:"checkin"`
	Nights              int      `json:"nights"`
	Currency            string   `json:"currency"`
	MedianFeeRatioPct   float64  `json:"median_fee_ratio_pct"`
	OutlierThresholdPct float64  `json:"outlier_threshold_pct"`
	ScannedProperties   int      `json:"scanned_properties"`
	ReturnedProperties  int      `json:"returned_properties"`
	Outliers            int      `json:"outliers"`
	Results             []feeRow `json:"results"`
	Note                string   `json:"note,omitempty"`
}

func newNovelHotelsFeesCmd(flags *rootFlags) *cobra.Command {
	sf := &searchFlags{}
	var threshold float64
	var outliersOnly bool

	cmd := &cobra.Command{
		Use:   "fees [destination]",
		Short: "Expose properties whose tax-and-fee load is an outlier for the destination",
		Long: `Rank a destination's hotels by how much of the real price is tax and fees.

Resort and facility fees are the most-reported Agoda surprise: a nightly rate
looks competitive, then a per-night fee lands at the property. Because the fee
ratio varies widely between hotels in the same city, comparing each property
against the destination median exposes the bait pattern mechanically.

A property flagged as an outlier is not necessarily a bad deal - it means an
unusually large share of what you pay is invisible in the headline price.`,
		Example: "  agoda-pp-cli hotels fees Tokyo --checkin 2026-10-15 --nights 2 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "destination=Tokyo;--nights=2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hotels fees")
			}
			dest := ""
			if len(args) > 0 {
				dest = args[0]
			}
			if dest == "" && sf.cityID <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a destination argument or --city-id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newAgodaClient(flags)
			base, priced, err := runSearch(ctx, c, cmd.ErrOrStderr(), dest, sf, hasAgodaSession())
			if err != nil {
				return err
			}

			out := feesResult{
				Destination:         base.Destination,
				CityID:              base.CityID,
				CheckIn:             base.CheckIn,
				Nights:              base.Nights,
				Currency:            base.Currency,
				ScannedProperties:   base.ScannedProperties,
				OutlierThresholdPct: threshold,
				Results:             make([]feeRow, 0, len(priced)),
				Note:                base.Note,
			}
			if len(priced) == 0 {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No priced hotels found for %s on %s.\n", out.Destination, out.CheckIn)
				return nil
			}

			median := medianHiddenPct(priced)
			out.MedianFeeRatioPct = median

			rows := make([]feeRow, 0, len(priced))
			for _, p := range priced {
				excess := round2Pct(p.HiddenPct - median)
				r := feeRow{
					Property:       p,
					FeeRatioPct:    p.HiddenPct,
					MedianRatioPct: median,
					ExcessPct:      excess,
					Outlier:        excess >= threshold,
				}
				if r.Outlier {
					out.Outliers++
				}
				rows = append(rows, r)
			}
			// Worst offenders first: this command exists to surface them.
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].ExcessPct > rows[j].ExcessPct })
			if outliersOnly {
				filtered := make([]feeRow, 0, len(rows))
				for _, r := range rows {
					if r.Outlier {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
				if len(rows) == 0 {
					out.Note = appendNote(out.Note, fmt.Sprintf(
						"no property exceeded the destination median (%.1f%%) by %.1f points or more",
						median, threshold))
				}
			}
			if sf.limit > 0 && len(rows) > sf.limit {
				rows = rows[:sf.limit]
			}
			out.Results = rows
			out.ReturnedProperties = len(rows)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAgodaJSON(cmd.OutOrStdout(), out, flags, "live")
			}
			return renderFeeTable(cmd, out)
		},
	}
	bindSearchFlags(cmd, sf)
	cmd.Flags().Float64Var(&threshold, "outlier-threshold", 5.0,
		"Percentage points above the destination median at which a property is flagged")
	cmd.Flags().BoolVar(&outliersOnly, "outliers-only", false, "Return only flagged properties")
	return cmd
}

// medianHiddenPct returns the median fee ratio across the result set. Median is
// used rather than mean so one extreme property cannot drag the baseline.
func medianHiddenPct(props []agoda.Property) float64 {
	if len(props) == 0 {
		return 0
	}
	vals := make([]float64, 0, len(props))
	for _, p := range props {
		vals = append(vals, p.HiddenPct)
	}
	sort.Float64s(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return round2Pct(vals[mid])
	}
	return round2Pct((vals[mid-1] + vals[mid]) / 2)
}

func round2Pct(v float64) float64 {
	return float64(int(v*100+copySign(0.5, v))) / 100
}

func copySign(mag, sign float64) float64 {
	if sign < 0 {
		return -mag
	}
	return mag
}

func renderFeeTable(cmd *cobra.Command, res feesResult) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s (city %d) - check-in %s, %d night(s), prices in %s\n",
		res.Destination, res.CityID, res.CheckIn, res.Nights, res.Currency)
	fmt.Fprintf(out, "Destination median fee load: %.1f%% of the advertised price\n\n", res.MedianFeeRatioPct)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROPERTY\tADVERTISED\tALL-IN\tFEE LOAD\tVS MEDIAN\tFLAG")
	for _, r := range res.Results {
		name := r.Name
		if len(name) > 38 {
			name = name[:35] + "..."
		}
		flag := ""
		if r.Outlier {
			flag = "OUTLIER"
		}
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.1f%%\t%+.1f\t%s\n",
			name, r.PriceAdvertised, r.PriceAllIn, r.FeeRatioPct, r.ExcessPct, flag)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%d properties shown, %d flagged at >= %.1f points over median.\n",
		res.ReturnedProperties, res.Outliers, res.OutlierThresholdPct)
	if res.Note != "" {
		fmt.Fprintf(out, "%s\n", res.Note)
	}
	return nil
}
