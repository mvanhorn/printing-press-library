// Hand-authored novel feature. Body is hand-written; survives regen via regen-merge.
// pp:data-source live
package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type wbExportRow struct {
	CountryCode string   `json:"country_code"`
	Country     string   `json:"country"`
	Indicator   string   `json:"indicator"`
	Date        string   `json:"date"`
	Value       *float64 `json:"value"`
}

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var flagWide bool
	var flagDate string

	cmd := &cobra.Command{
		Use:         "export <country;...> <indicator;...>",
		Short:       "Bulk country x indicator pull as pipeline-ready wide or long CSV.",
		Long:        "Pull multiple country x indicator series and emit pipeline-ready CSV (long by default, --wide to pivot indicators into columns).\nUse this for data-pipeline extracts. Add --json for a structured array.",
		Example:     "  world-bank-pp-cli export USA;CHN NY.GDP.MKTP.CD;SP.POP.TOTL --wide --csv",
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
				return usageErr(fmt.Errorf("country list and indicator list are required"))
			}
			// Accept comma- or semicolon-separated lists; the API wants ';'.
			countries := strings.ReplaceAll(args[0], ",", ";")
			indicators := strings.Split(strings.ReplaceAll(args[1], ",", ";"), ";")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			extra := map[string]string{"per_page": "1000"}
			if flagDate != "" {
				extra["date"] = flagDate
			}
			var all []wbExportRow
			for _, ind := range indicators {
				ind = strings.TrimSpace(ind)
				if ind == "" {
					continue
				}
				obs, err := wbFetchObservations(ctx, c, countries, ind, copyMap(extra), 20)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, o := range obs {
					all = append(all, wbExportRow{CountryCode: o.CountryISO3Code, Country: o.Country.Value, Indicator: ind, Date: o.Date, Value: o.Value})
				}
			}
			sort.Slice(all, func(i, j int) bool {
				if all[i].CountryCode != all[j].CountryCode {
					return all[i].CountryCode < all[j].CountryCode
				}
				if all[i].Date != all[j].Date {
					return all[i].Date > all[j].Date
				}
				return all[i].Indicator < all[j].Indicator
			})

			if flags.asJSON {
				// Encode JSON directly so an accompanying --csv flag does not
				// convert the output back to CSV (json_fidelity must hold).
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(all)
			}
			w := csv.NewWriter(cmd.OutOrStdout())
			if flagWide {
				// Pivot: rows keyed by (country_code,country,date), columns per indicator.
				indOrder := []string{}
				for _, ind := range indicators {
					ind = strings.TrimSpace(ind)
					if ind != "" {
						indOrder = append(indOrder, ind)
					}
				}
				type key struct{ code, country, date string }
				cells := map[key]map[string]string{}
				order := []key{}
				for _, r := range all {
					k := key{r.CountryCode, r.Country, r.Date}
					if _, ok := cells[k]; !ok {
						cells[k] = map[string]string{}
						order = append(order, k)
					}
					cells[k][r.Indicator] = wbFloat(r.Value)
				}
				header := append([]string{"country_code", "country", "date"}, indOrder...)
				_ = w.Write(header)
				for _, k := range order {
					rec := []string{k.code, k.country, k.date}
					for _, ind := range indOrder {
						rec = append(rec, cells[k][ind])
					}
					_ = w.Write(rec)
				}
			} else {
				_ = w.Write([]string{"country_code", "country", "indicator", "date", "value"})
				for _, r := range all {
					_ = w.Write([]string{r.CountryCode, r.Country, r.Indicator, r.Date, wbFloat(r.Value)})
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return fmt.Errorf("writing csv: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagWide, "wide", false, "Pivot indicators into columns (wide format)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Year or range, e.g. 2010:2024")
	return cmd
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
