// Hand-authored novel feature. Body is hand-written; survives regen via regen-merge.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type wbCompareRow struct {
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	Date        string   `json:"date"`
	Value       *float64 `json:"value"`
	DeltaVsBase *float64 `json:"delta_vs_base,omitempty"`
	PctVsBase   *float64 `json:"pct_vs_base,omitempty"`
}

type wbCompareView struct {
	Indicator string         `json:"indicator"`
	Baseline  string         `json:"baseline"`
	Date      string         `json:"date,omitempty"`
	Rows      []wbCompareRow `json:"rows"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagDate string

	cmd := &cobra.Command{
		Use:         "compare <indicator> <country;country;...>",
		Short:       "Line up one indicator across countries with deltas vs a baseline.",
		Long:        "Line up one indicator across countries in a single aligned table with deltas vs the first (baseline) country.\nUse this to compare economies on one indicator. Do NOT use it for one country's history; use 'trend'.",
		Example:     "  world-bank-pp-cli compare NY.GDP.MKTP.CD USA;CHN;IND --date 2024",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indicator and country list are required"))
			}
			indicator := args[0]
			// Accept comma- or semicolon-separated country lists; the API wants ';'.
			countries := strings.ReplaceAll(args[1], ",", ";")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			extra := map[string]string{}
			if flagDate != "" {
				extra["date"] = flagDate
			} else {
				extra["mrv"] = "1"
			}
			obs, err := wbFetchObservations(ctx, c, countries, indicator, extra, 10)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Keep the most recent non-null observation per country.
			best := map[string]wbObservation{}
			order := []string{}
			for _, o := range obs {
				if o.Value == nil {
					continue
				}
				key := o.CountryISO3Code
				if _, seen := best[key]; !seen {
					order = append(order, key)
				}
				cur, ok := best[key]
				if !ok || o.Date > cur.Date {
					best[key] = o
				}
			}
			// Baseline = first country in the user's ;-list that has data.
			wantOrder := strings.Split(countries, ";")
			var baseVal *float64
			var baseName string
			view := wbCompareView{Indicator: indicator, Date: flagDate}
			emit := func(code string) {
				o, ok := best[code]
				if !ok {
					return
				}
				row := wbCompareRow{Country: o.Country.Value, CountryCode: o.CountryISO3Code, Date: o.Date, Value: o.Value}
				if baseVal == nil {
					baseVal = o.Value
					baseName = o.Country.Value
				} else if o.Value != nil && baseVal != nil {
					d := *o.Value - *baseVal
					row.DeltaVsBase = &d
					if *baseVal != 0 {
						p := d / *baseVal * 100
						row.PctVsBase = &p
					}
				}
				view.Rows = append(view.Rows, row)
			}
			seen := map[string]bool{}
			for _, code := range wantOrder {
				code = strings.ToUpper(strings.TrimSpace(code))
				// match by ISO3 directly or via observed code
				for _, k := range order {
					if seen[k] {
						continue
					}
					if strings.EqualFold(k, code) || strings.EqualFold(best[k].Country.ID, code) {
						emit(k)
						seen[k] = true
					}
				}
			}
			// Append any remaining (e.g. when input was an aggregate code).
			for _, k := range order {
				if !seen[k] {
					emit(k)
					seen[k] = true
				}
			}
			view.Baseline = baseName
			// view.Rows is already in baseline-first insertion order.
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Year or range (e.g. 2024). Defaults to most recent value.")
	return cmd
}
