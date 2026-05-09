package compound

import (
	"fmt"
	"strings"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newStaleCmd(opts *output.Options) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Grants expiring soon with no matching award history (zombie grants)",
		Long: `Stale identifies "zombie grants" — opportunities closing within N days that have
no matching award records in USASpending.gov by CFDA number.

These are potentially low-competition opportunities worth pursuing.

Requires synced local data — run 'govprocure-pp-cli sync --all' first.`,
		Example: `  govprocure-pp-cli compound stale
  govprocure-pp-cli compound stale --days 30
  govprocure-pp-cli compound stale --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("compound stale --days %d", days))
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			zombies, err := database.ZombieGrants(days)
			if err != nil {
				return fmt.Errorf("zombie grant query: %w", err)
			}

			if len(zombies) == 0 {
				if output.EffectiveFormat(opts) == output.FormatJSON {
					output.PrintJSON([]interface{}{})
				} else {
					fmt.Printf("No zombie grants found closing within %d days.\n", days)
					fmt.Println("(Run 'sync --all' to populate local data)")
				}
				return nil
			}

			switch output.EffectiveFormat(opts) {
			case output.FormatJSON:
				var out []map[string]interface{}
				for _, z := range zombies {
					out = append(out, map[string]interface{}{
						"opportunity_id":  z.OpportunityID,
						"title":           z.Title,
						"agency":          z.Agency,
						"cfda_number":     z.CFDANumber,
						"close_date":      z.CloseDate,
						"award_ceiling":   z.AwardCeiling,
						"days_until_close": z.DaysUntilClose,
						"award_history":   "none",
					})
				}
				output.PrintJSON(out)

			case output.FormatCompact:
				for _, z := range zombies {
					fmt.Printf("%s | %s | closes in %d days | %s\n",
						z.OpportunityID, output.Truncate(z.Title, 40), z.DaysUntilClose, output.FormatDollars(z.AwardCeiling))
				}

			default:
				fmt.Printf("Zombie grants closing within %d days (no award history — low competition):\n\n", days)
				headers := []string{"ID", "TITLE", "AGENCY", "CLOSE DATE", "DAYS LEFT", "CEILING"}
				var rows [][]string
				for _, z := range zombies {
					rows = append(rows, []string{
						z.OpportunityID,
						output.Truncate(z.Title, 38),
						output.Truncate(z.Agency, 25),
						z.CloseDate,
						fmt.Sprintf("%d", z.DaysUntilClose),
						output.FormatDollars(z.AwardCeiling),
					})
				}
				output.PrintTable(headers, rows)
				fmt.Printf("\n%d zombie grants found\n", len(zombies))
				fmt.Println(strings.Repeat("─", 40))
				fmt.Println("These grants have no matching award records — potential low-competition opportunities.")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "closing window in days")
	return cmd
}
