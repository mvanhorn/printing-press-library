// PATCH: hand-authored novel-feature file. See .printing-press-patches.json patch id "novel-inflation-adjust".
package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// InflationResult is the structured output of `inflation adjust`.
type InflationResult struct {
	Amount     float64 `json:"amount"`
	FromYear   int     `json:"from_year"`
	ToYear     int     `json:"to_year"`
	IndexFrom  float64 `json:"cpi_from"`
	IndexTo    float64 `json:"cpi_to"`
	Adjusted   float64 `json:"adjusted_amount"`
	Series     string  `json:"series_id"`
	Index      string  `json:"index"` // "CPI-U" or "CPI-W"
	Annualized bool    `json:"annualized"`
}

func newInflationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inflation",
		Short: "CPI-based inflation calculations.",
	}
	cmd.AddCommand(newInflationAdjustCmd(flags))
	return cmd
}

func newInflationAdjustCmd(flags *rootFlags) *cobra.Command {
	var amount float64
	var fromYear, toYear int
	var index string
	cmd := &cobra.Command{
		Use:   "adjust",
		Short: "Convert a dollar amount from one year's purchasing power to another via CPI-U (or CPI-W).",
		Long: `Fetches annual averages of CPI-U (default) or CPI-W from the BLS API and
deflates/inflates the input amount: adjusted = amount * cpi_to / cpi_from.

Returns annual-average index values so the answer is stable regardless of
which month you happen to run it. BLS caps each request at 20 years with
a registered key (10 years unauthenticated); for wider windows, chain two
calls and multiply the ratios.`,
		Example: `  bls-pp-cli inflation adjust --amount 100 --from-year 2010 --to-year 2024
  bls-pp-cli inflation adjust --amount 50000 --from-year 2010 --to-year 2024 --index CPI-W --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if fromYear == 0 || toYear == 0 || amount == 0 {
				return cmd.Help()
			}
			seriesID := "CUUR0000SA0" // CPI-U all items, NSA
			displayIndex := "CPI-U"
			switch strings.ToUpper(strings.TrimSpace(index)) {
			case "", "CPI-U", "CPIU", "CU":
				// keep default
			case "CPI-W", "CPIW", "CW":
				seriesID = "CWUR0000SA0"
				displayIndex = "CPI-W"
			default:
				return usageErr(fmt.Errorf("unknown --index %q; valid values are CPI-U and CPI-W", index))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"seriesid":      []string{seriesID},
				"startyear":     strconv.Itoa(fromYear),
				"endyear":       strconv.Itoa(toYear),
				"annualaverage": true,
			}
			body = injectRegistrationKey(body)
			raw, _, err := c.Post("/timeseries/data/", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cpiFrom, cpiTo, ok := pickAnnualIndex(raw, seriesID, fromYear, toYear)
			if !ok {
				if toYear-fromYear > 20 {
					return apiErr(fmt.Errorf("window %d-%d exceeds the BLS 20-year-per-request cap; pick a window <=20 years or chain two calls", fromYear, toYear))
				}
				return apiErr(fmt.Errorf("BLS returned no annual-average row for %d or %d on %s; the annualaverage flag requires a registered BLS_API_KEY (set BLS_API_KEY=<key> in your environment)", fromYear, toYear, seriesID))
			}
			adjusted := amount * cpiTo / cpiFrom
			res := InflationResult{
				Amount:    amount,
				FromYear:  fromYear,
				ToYear:    toYear,
				IndexFrom: cpiFrom,
				IndexTo:   cpiTo,
				Adjusted:  adjusted,
				Series:    seriesID,
				Index:     displayIndex,
			}
			out, _ := json.Marshal(res)
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "$%.2f in %d ≈ $%.2f in %d (using %s annual averages: %g → %g)\n",
				amount, fromYear, adjusted, toYear, displayIndex, cpiFrom, cpiTo)
			return nil
		},
	}
	cmd.Flags().Float64Var(&amount, "amount", 0, "Dollar amount to convert.")
	cmd.Flags().IntVar(&fromYear, "from-year", 0, "Source year (the year the dollars were earned/spent).")
	cmd.Flags().IntVar(&toYear, "to-year", 0, "Target year (the year to express the amount in).")
	cmd.Flags().StringVar(&index, "index", "CPI-U", "Which index to use: CPI-U (default, all urban consumers) or CPI-W (urban wage earners).")
	return cmd
}

// pickAnnualIndex finds the annual-average row (period == "M13") for the
// requested years from a BLS response.
func pickAnnualIndex(raw json.RawMessage, seriesID string, fromYear, toYear int) (cpiFrom, cpiTo float64, ok bool) {
	var env struct {
		Results struct {
			Series []struct {
				SeriesID string `json:"seriesID"`
				Data     []struct {
					Year       string `json:"year"`
					Period     string `json:"period"`
					PeriodName string `json:"periodName"`
					Value      string `json:"value"`
				} `json:"data"`
			} `json:"series"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, 0, false
	}
	for _, s := range env.Results.Series {
		if s.SeriesID != seriesID {
			continue
		}
		for _, d := range s.Data {
			if d.Period != "M13" {
				continue
			}
			yr, err := strconv.Atoi(strings.TrimSpace(d.Year))
			if err != nil {
				continue
			}
			val, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64)
			if err != nil {
				continue
			}
			if yr == fromYear {
				cpiFrom = val
			}
			if yr == toYear {
				cpiTo = val
			}
		}
	}
	return cpiFrom, cpiTo, cpiFrom != 0 && cpiTo != 0
}
