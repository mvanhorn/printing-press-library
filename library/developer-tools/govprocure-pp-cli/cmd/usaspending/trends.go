package usaspending

import (
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/spf13/cobra"
)

func newTrendsCmd(opts *output.Options) *cobra.Command {
	var agency string
	var years int

	cmd := &cobra.Command{
		Use:   "spending-trends",
		Short: "Year-over-year spending by agency or program",
		Example: `  govprocure-pp-cli usaspending spending-trends --agency "Department of Education"
  govprocure-pp-cli usaspending spending-trends --agency "HHS" --years 3
  govprocure-pp-cli usaspending spending-trends --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("usaspending spending-trends agency=%q years=%d", agency, years))
				return nil
			}

			client := api.NewUSASpendingClient(cfg.USASpendURL)
			resp, err := client.SpendingTrends(agency, years)
			if err != nil {
				return fmt.Errorf("spending trends: %w", err)
			}

			if len(resp.Results) == 0 {
				fmt.Println("No spending data found.")
				return nil
			}

			switch output.EffectiveFormat(opts) {
			case output.FormatJSON:
				var out []map[string]interface{}
				for _, r := range resp.Results {
					out = append(out, map[string]interface{}{
						"period": r.Period,
						"amount": r.AggregatedAmount,
					})
				}
				output.PrintJSON(map[string]interface{}{
					"agency":  agency,
					"results": out,
				})

			case output.FormatCSV:
				headers := []string{"period", "amount"}
				var rows [][]string
				for _, r := range resp.Results {
					rows = append(rows, []string{fmt.Sprintf("%v", r.Period), output.FormatDollars(r.AggregatedAmount)})
				}
				output.PrintCSV(headers, rows)

			case output.FormatCompact:
				for _, r := range resp.Results {
					fmt.Printf("%v | %s\n", r.Period, output.FormatDollars(r.AggregatedAmount))
				}

			default:
				title := "U.S. Government"
				if agency != "" {
					title = agency
				}
				fmt.Printf("Spending trends for: %s (last %d fiscal years)\n\n", title, years)
				headers := []string{"PERIOD", "TOTAL OBLIGATED"}
				var rows [][]string
				for _, r := range resp.Results {
					rows = append(rows, []string{fmt.Sprintf("%v", r.Period), output.FormatDollars(r.AggregatedAmount)})
				}
				output.PrintTable(headers, rows)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agency, "agency", "", "agency name to filter (empty = all federal)")
	cmd.Flags().IntVar(&years, "years", 5, "number of fiscal years to include")
	return cmd
}
