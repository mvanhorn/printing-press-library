// Hand-authored novel feature. Body is hand-written; survives regen via regen-merge.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/world-bank/internal/client"

	"github.com/spf13/cobra"
)

type wbRankRow struct {
	Rank        int      `json:"rank"`
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	Date        string   `json:"date"`
	Value       *float64 `json:"value"`
}

type wbRankView struct {
	Indicator string      `json:"indicator"`
	Year      string      `json:"year"`
	Filter    string      `json:"filter,omitempty"`
	Count     int         `json:"count"`
	Rows      []wbRankRow `json:"rows"`
}

// wbCountrySet fetches real-country ISO3 codes, optionally filtered by region/income.
func wbCountrySet(ctx context.Context, c *client.Client, region, income string) (map[string]bool, error) {
	params := map[string]string{"per_page": "400"}
	if region != "" {
		params["region"] = region
	}
	if income != "" {
		params["incomeLevel"] = income
	}
	rows, err := wbGetAllRows(ctx, c, "/country", params, 5)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, r := range rows {
		var cm struct {
			ID     string      `json:"id"`
			Region wbCodeValue `json:"region"`
		}
		if json.Unmarshal(r, &cm) != nil {
			continue
		}
		// Exclude aggregates: their region.value is "Aggregates".
		if strings.EqualFold(cm.Region.Value, "Aggregates") {
			continue
		}
		set[strings.ToUpper(cm.ID)] = true
	}
	return set, nil
}

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	var flagYear string
	var flagTop int
	var flagIncome string
	var flagRegion string
	var flagAsc bool

	cmd := &cobra.Command{
		Use:         "rank <indicator>",
		Short:       "Rank economies by an indicator for a year, filtered by region or income.",
		Long:        "Rank all economies by an indicator for a year, optionally filtered by region or income level.\nUse this to find leaders/laggards on an indicator.",
		Example:     "  world-bank-pp-cli rank NY.GDP.PCAP.CD --year 2024 --top 10 --income HIC",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indicator is required"))
			}
			indicator := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			set, err := wbCountrySet(ctx, c, flagRegion, flagIncome)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			extra := map[string]string{"per_page": "400"}
			if flagYear != "" {
				extra["date"] = flagYear
			} else {
				extra["mrv"] = "1"
			}
			obs, err := wbFetchObservations(ctx, c, "all", indicator, extra, 20)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := make([]wbRankRow, 0)
			for _, o := range obs {
				if o.Value == nil || !set[strings.ToUpper(o.CountryISO3Code)] {
					continue
				}
				rows = append(rows, wbRankRow{Country: o.Country.Value, CountryCode: o.CountryISO3Code, Date: o.Date, Value: o.Value})
			}
			sort.Slice(rows, func(i, j int) bool {
				if flagAsc {
					return *rows[i].Value < *rows[j].Value
				}
				return *rows[i].Value > *rows[j].Value
			})
			if flagTop > 0 && len(rows) > flagTop {
				rows = rows[:flagTop]
			}
			for i := range rows {
				rows[i].Rank = i + 1
			}
			filter := strings.TrimSpace(strings.Join([]string{flagRegion, flagIncome}, " "))
			return flags.printJSON(cmd, wbRankView{Indicator: indicator, Year: flagYear, Filter: filter, Count: len(rows), Rows: rows})
		},
	}
	cmd.Flags().StringVar(&flagYear, "year", "", "Year to rank (e.g. 2024). Defaults to most recent value.")
	cmd.Flags().IntVar(&flagTop, "top", 0, "Keep only the top N (0 = all)")
	cmd.Flags().StringVar(&flagIncome, "income", "", "Filter by income level code (e.g. HIC, LIC)")
	cmd.Flags().StringVar(&flagRegion, "region", "", "Filter by region code (e.g. EAS, ECS)")
	cmd.Flags().BoolVar(&flagAsc, "asc", false, "Rank ascending (lowest first)")
	return cmd
}
