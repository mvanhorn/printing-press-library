package compound

import (
	"fmt"
	"strings"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newProfileCmd(opts *output.Options) *cobra.Command {
	var agency string

	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Full agency intelligence card — open opportunities + award history + top recipients",
		Long: `Profile assembles a complete agency intelligence card by querying all three
local data sources:
  - grants.gov: open grant opportunities
  - SAM.gov: active contract notices
  - USASpending.gov: historical award totals + top recipients

Requires synced local data — run 'govprocure-pp-cli sync --all' first.`,
		Example: `  govprocure-pp-cli compound profile --agency "Department of Education"
  govprocure-pp-cli compound profile --agency "HHS" --agent
  govprocure-pp-cli compound profile --agency "Defense"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if agency == "" {
				return fmt.Errorf("--agency is required")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("compound profile --agency %q", agency))
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			profile, err := database.GetAgencyProfile(agency)
			if err != nil {
				return fmt.Errorf("build agency profile: %w", err)
			}

			if output.EffectiveFormat(opts) == output.FormatJSON {
				output.PrintJSON(profileToMap(profile))
				return nil
			}

			printProfile(profile)
			return nil
		},
	}

	cmd.Flags().StringVar(&agency, "agency", "", "agency name to profile (required)")
	return cmd
}

func printProfile(p *db.AgencyProfile) {
	border := strings.Repeat("═", 78)
	thin := strings.Repeat("─", 78)

	fmt.Printf("\n%s\n", border)
	fmt.Printf("  AGENCY INTELLIGENCE CARD: %s\n", strings.ToUpper(p.Agency))
	fmt.Printf("%s\n\n", border)

	// Award summary
	fmt.Printf("  Historical Awards (USASpending.gov)\n")
	fmt.Printf("  %s\n", thin)
	fmt.Printf("  Total Obligated : %s\n", output.FormatDollars(p.TotalAwarded))
	fmt.Printf("  Award Count     : %d\n", p.AwardCount)
	if len(p.TopRecipients) > 0 {
		fmt.Printf("  Top Recipients  :\n")
		for _, r := range p.TopRecipients {
			fmt.Printf("    • %s\n", r)
		}
	}
	fmt.Println()

	// Open grants
	fmt.Printf("  Open Grant Opportunities (grants.gov)\n")
	fmt.Printf("  %s\n", thin)
	if len(p.OpenGrants) == 0 {
		fmt.Println("  No open grants in local DB.")
	} else {
		for _, g := range p.OpenGrants {
			fmt.Printf("  • [%s] %s\n", g.CloseDate, output.Truncate(g.Title, 55))
			fmt.Printf("    ID: %s | CFDA: %s | Ceiling: %s\n",
				g.OpportunityID, g.CFDANumber, output.FormatDollars(g.AwardCeiling))
		}
	}
	fmt.Println()

	// SAM notices
	fmt.Printf("  Active SAM Notices (SAM.gov)\n")
	fmt.Printf("  %s\n", thin)
	if len(p.OpenSAM) == 0 {
		fmt.Println("  No SAM notices in local DB.")
	} else {
		for _, s := range p.OpenSAM {
			fmt.Printf("  • [%s] %s\n", s.ResponseDeadline, output.Truncate(s.Title, 55))
			fmt.Printf("    Notice: %s | Set-Aside: %s | NAICS: %s\n",
				s.NoticeID, s.SetAside, s.NAICSCode)
		}
	}

	fmt.Printf("\n%s\n", border)
	fmt.Printf("  Tip: Run 'compound pipeline' for deeper cross-source analysis.\n")
	fmt.Printf("%s\n\n", border)
}

func profileToMap(p *db.AgencyProfile) map[string]interface{} {
	grantList := make([]map[string]interface{}, 0, len(p.OpenGrants))
	for _, g := range p.OpenGrants {
		grantList = append(grantList, map[string]interface{}{
			"opportunity_id": g.OpportunityID,
			"title":          g.Title,
			"close_date":     g.CloseDate,
			"cfda_number":    g.CFDANumber,
			"award_ceiling":  g.AwardCeiling,
		})
	}

	samList := make([]map[string]interface{}, 0, len(p.OpenSAM))
	for _, s := range p.OpenSAM {
		samList = append(samList, map[string]interface{}{
			"notice_id":         s.NoticeID,
			"title":             s.Title,
			"set_aside":         s.SetAside,
			"response_deadline": s.ResponseDeadline,
			"naics_code":        s.NAICSCode,
		})
	}

	return map[string]interface{}{
		"agency":          p.Agency,
		"total_awarded":   p.TotalAwarded,
		"award_count":     p.AwardCount,
		"top_recipients":  p.TopRecipients,
		"open_grants":     grantList,
		"open_sam":        samList,
	}
}
