package grants

import (
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDeadlinesCmd(opts *output.Options) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "deadlines",
		Short: "List grants closing within N days (default 14)",
		Example: `  govprocure-pp-cli grants deadlines
  govprocure-pp-cli grants deadlines --days 30
  govprocure-pp-cli grants deadlines --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("grants deadlines --days %d", days))
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			grants, err := database.GrantsClosingWithin(days)
			if err != nil {
				return fmt.Errorf("query deadlines: %w", err)
			}

			if len(grants) == 0 {
				if output.EffectiveFormat(opts) == output.FormatJSON {
					output.PrintJSON([]interface{}{})
				} else {
					fmt.Printf("No grants closing within %d days in local DB. Run 'grants sync' first.\n", days)
				}
				return nil
			}

			switch output.EffectiveFormat(opts) {
			case output.FormatJSON:
				var out []map[string]interface{}
				for _, g := range grants {
					m := grantToMap(g)
					out = append(out, output.FilterFields(m, opts.Select))
				}
				output.PrintJSON(out)

			case output.FormatCSV:
				headers := []string{"opportunity_id", "title", "agency", "close_date", "award_ceiling"}
				var rows [][]string
				for _, g := range grants {
					rows = append(rows, []string{g.OpportunityID, g.Title, g.Agency, g.CloseDate, output.FormatDollars(g.AwardCeiling)})
				}
				output.PrintCSV(headers, rows)

			case output.FormatCompact:
				for _, g := range grants {
					fmt.Printf("%s | %s | %s | %s\n", g.OpportunityID, output.Truncate(g.Title, 45), g.CloseDate, output.FormatDollars(g.AwardCeiling))
				}

			default:
				fmt.Printf("Grants closing within %d days:\n\n", days)
				headers := []string{"ID", "TITLE", "AGENCY", "CLOSE DATE", "CEILING"}
				var rows [][]string
				for _, g := range grants {
					rows = append(rows, []string{
						g.OpportunityID,
						output.Truncate(g.Title, 45),
						output.Truncate(g.Agency, 28),
						g.CloseDate,
						output.FormatDollars(g.AwardCeiling),
					})
				}
				output.PrintTable(headers, rows)
				fmt.Printf("\n%d grants closing within %d days\n", len(grants), days)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "number of days to look ahead")
	return cmd
}
