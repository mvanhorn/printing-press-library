package grants

import (
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newGetCmd(opts *output.Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <opportunity-id>",
		Short: "Fetch a single grants.gov opportunity",
		Args:  cobra.ExactArgs(1),
		Example: `  govprocure-pp-cli grants get 12345
  govprocure-pp-cli grants get GRANTS-12345 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("grants get %q", id))
				return nil
			}

			// Try local DB first
			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			g, err := database.GetGrant(id)
			if err != nil {
				return fmt.Errorf("db lookup: %w", err)
			}

			if g == nil {
				// Fall back to live API
				client := api.NewGrantsClient(cfg.GrantsURL)
				opp, err := client.GetOpportunity(id)
				if err != nil {
					return fmt.Errorf("grants.gov API: %w", err)
				}
				if opp == nil {
					output.Error("opportunity %q not found", id)
					return &exitCodeErr{output.ExitNotFound}
				}
				g = &db.Grant{
					OpportunityID: opp.Number,
					Title:         opp.Title,
					Agency:        opp.Agency,
					CFDANumber:    api.CFDAString(opp.CFDAList),
					CloseDate:     opp.CloseDate,
					PostDate:      opp.OpenDate,
					Eligibility:   opp.EligApplicants,
					Synopsis:      opp.Synopsis,
					AwardFloor:    opp.AwardFloor,
					AwardCeiling:  opp.AwardCeiling,
				}
			}

			if output.EffectiveFormat(opts) == output.FormatJSON {
				output.PrintJSON(output.FilterFields(grantToMap(g), opts.Select))
				return nil
			}

			// Human readable detail
			printGrantDetail(g)
			return nil
		},
	}
}

func printGrantDetail(g *db.Grant) {
	fmt.Printf("Opportunity ID : %s\n", g.OpportunityID)
	fmt.Printf("Title          : %s\n", g.Title)
	fmt.Printf("Agency         : %s\n", g.Agency)
	fmt.Printf("CFDA Number    : %s\n", g.CFDANumber)
	fmt.Printf("Post Date      : %s\n", g.PostDate)
	fmt.Printf("Close Date     : %s\n", g.CloseDate)
	fmt.Printf("Award Floor    : %s\n", output.FormatDollars(g.AwardFloor))
	fmt.Printf("Award Ceiling  : %s\n", output.FormatDollars(g.AwardCeiling))
	fmt.Printf("Eligibility    : %s\n", g.Eligibility)
	fmt.Println()
	if g.Synopsis != "" {
		fmt.Printf("Synopsis:\n%s\n", g.Synopsis)
	}
}

type exitCodeErr struct{ code int }

func (e *exitCodeErr) Error() string { return fmt.Sprintf("exit code %d", e.code) }
