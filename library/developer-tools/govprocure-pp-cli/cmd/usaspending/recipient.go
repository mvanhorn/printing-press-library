package usaspending

import (
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newRecipientCmd(opts *output.Options) *cobra.Command {
	var limit int
	var localOnly bool

	cmd := &cobra.Command{
		Use:   "recipient <name>",
		Short: "Look up award history for a recipient",
		Args:  cobra.MinimumNArgs(1),
		Example: `  govprocure-pp-cli usaspending recipient "Booz Allen Hamilton"
  govprocure-pp-cli usaspending recipient "Silent Northwest" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("usaspending recipient %q", name))
				return nil
			}

			// Check local DB first
			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			localAwards, err := database.AwardsByRecipient(name, limit)
			if err != nil {
				return fmt.Errorf("db lookup: %w", err)
			}

			if len(localAwards) > 0 {
				renderLocalAwards(localAwards, opts)
				return nil
			}

			if localOnly {
				fmt.Printf("No awards found for %q in local DB. Run 'sync --all' first.\n", name)
				return nil
			}

			// Fall back to live
			fmt.Printf("Searching USASpending.gov for %q...\n", name)
			client := api.NewUSASpendingClient(cfg.USASpendURL)
			resp, err := client.SearchRecipient(name, limit)
			if err != nil {
				return fmt.Errorf("recipient lookup: %w", err)
			}

			if len(resp.Results) == 0 {
				fmt.Printf("No recipients found matching %q\n", name)
				return nil
			}

			switch output.EffectiveFormat(opts) {
			case output.FormatJSON:
				var out []map[string]interface{}
				for _, r := range resp.Results {
					m := recipientToMap(r)
					out = append(out, output.FilterFields(m, opts.Select))
				}
				output.PrintJSON(out)

			default:
				headers := []string{"RECIPIENT ID", "NAME", "STATE", "TOTAL AWARDS", "AWARD COUNT"}
				var rows [][]string
				for _, r := range resp.Results {
					rows = append(rows, []string{
						r.RecipientID,
						output.Truncate(r.Name, 40),
						r.State,
						output.FormatDollars(r.TotalAmount),
						fmt.Sprintf("%d", r.AwardCount),
					})
				}
				output.PrintTable(headers, rows)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "max results")
	cmd.Flags().BoolVar(&localOnly, "local", false, "only search local DB, no API call")
	return cmd
}

func renderLocalAwards(awards []*db.Award, opts *output.Options) {
	switch output.EffectiveFormat(opts) {
	case output.FormatJSON:
		var out []map[string]interface{}
		for _, a := range awards {
			m := localAwardToMap(a)
			out = append(out, output.FilterFields(m, opts.Select))
		}
		output.PrintJSON(out)

	default:
		headers := []string{"AWARD ID", "AGENCY", "CFDA", "AMOUNT", "START", "END"}
		var rows [][]string
		for _, a := range awards {
			rows = append(rows, []string{
				output.Truncate(a.AwardID, 20),
				output.Truncate(a.Agency, 25),
				a.CFDANumber,
				output.FormatDollars(a.Amount),
				a.StartDate,
				a.EndDate,
			})
		}
		output.PrintTable(headers, rows)
		fmt.Printf("\n%d awards found\n", len(awards))
	}
}

func localAwardToMap(a *db.Award) map[string]interface{} {
	return map[string]interface{}{
		"award_id":       a.AwardID,
		"recipient_name": a.RecipientName,
		"agency":         a.Agency,
		"cfda_number":    a.CFDANumber,
		"amount":         a.Amount,
		"start_date":     a.StartDate,
		"end_date":       a.EndDate,
		"description":    a.Description,
	}
}

func recipientToMap(r api.RecipientResult) map[string]interface{} {
	return map[string]interface{}{
		"recipient_id":  r.RecipientID,
		"name":          r.Name,
		"state":         r.State,
		"total_amount":  r.TotalAmount,
		"award_count":   r.AwardCount,
		"entity_type":   r.EntityType,
	}
}
