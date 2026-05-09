package usaspending

import (
	"fmt"
	"strings"

	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAwardsCmd(opts *output.Options) *cobra.Command {
	var agency string
	var cfda string
	var limit int
	var awardType string

	cmd := &cobra.Command{
		Use:   "awards",
		Short: "Search award history by agency or CFDA number",
		Example: `  govprocure-pp-cli usaspending awards --agency "Department of Defense"
  govprocure-pp-cli usaspending awards --cfda 84.215
  govprocure-pp-cli usaspending awards --agency "HHS" --limit 50 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("usaspending awards agency=%q cfda=%q", agency, cfda))
				return nil
			}

			var agencies []string
			if agency != "" {
				agencies = []string{agency}
			}
			var cfdaList []string
			if cfda != "" {
				cfdaList = strings.Split(cfda, ",")
			}

			var awardTypes []string
			switch strings.ToLower(awardType) {
			case "grants":
				awardTypes = api.GrantAwardTypes
			case "contracts":
				awardTypes = api.ContractAwardTypes
			}

			client := api.NewUSASpendingClient(cfg.USASpendURL)
			resp, err := client.SearchAwards(agencies, cfdaList, awardTypes, limit)
			if err != nil {
				return fmt.Errorf("usaspending awards: %w", err)
			}

			if len(resp.Results) == 0 {
				fmt.Println("No awards found.")
				return nil
			}

			switch output.EffectiveFormat(opts) {
			case output.FormatJSON:
				var out []map[string]interface{}
				for _, a := range resp.Results {
					m := awardToMap(a)
					out = append(out, output.FilterFields(m, opts.Select))
				}
				output.PrintJSON(out)

			case output.FormatCSV:
				headers := []string{"award_id", "recipient", "agency", "amount", "start_date", "end_date"}
				var rows [][]string
				for _, a := range resp.Results {
					rows = append(rows, []string{a.AwardID, a.RecipientName, a.Agency, output.FormatDollars(a.AwardAmount), a.StartDate, a.EndDate})
				}
				output.PrintCSV(headers, rows)

			case output.FormatCompact:
				for _, a := range resp.Results {
					fmt.Printf("%s | %s | %s | %s\n",
						a.AwardID, output.Truncate(a.RecipientName, 35), output.FormatDollars(a.AwardAmount), a.EndDate)
				}

			default:
				headers := []string{"AWARD ID", "RECIPIENT", "AGENCY", "AMOUNT", "START", "END"}
				var rows [][]string
				for _, a := range resp.Results {
					rows = append(rows, []string{
						output.Truncate(a.AwardID, 20),
						output.Truncate(a.RecipientName, 30),
						output.Truncate(a.Agency, 25),
						output.FormatDollars(a.AwardAmount),
						a.StartDate,
						a.EndDate,
					})
				}
				output.PrintTable(headers, rows)
				fmt.Printf("\n%d awards (total shown)\n", len(resp.Results))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agency, "agency", "", "filter by awarding agency name")
	cmd.Flags().StringVar(&cfda, "cfda", "", "filter by CFDA number(s), comma-separated")
	cmd.Flags().IntVar(&limit, "limit", 25, "max results")
	cmd.Flags().StringVar(&awardType, "type", "", "award type: grants|contracts (default: both)")
	return cmd
}

func awardToMap(a api.AwardResult) map[string]interface{} {
	return map[string]interface{}{
		"award_id":       a.AwardID,
		"recipient_name": a.RecipientName,
		"agency":         a.Agency,
		"sub_agency":     a.SubAgency,
		"amount":         a.AwardAmount,
		"start_date":     a.StartDate,
		"end_date":       a.EndDate,
		"description":    a.Description,
		"cfda_number":    a.CFDANumber,
	}
}
