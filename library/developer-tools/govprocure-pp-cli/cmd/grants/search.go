package grants

import (
	"encoding/json"
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSearchCmd(opts *output.Options) *cobra.Command {
	var limit int
	var live bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search grants.gov opportunities (local FTS5 or live API)",
		Args:  cobra.MinimumNArgs(1),
		Example: `  govprocure-pp-cli grants search "AI technology SDVOSB"
  govprocure-pp-cli grants search "literacy education" --live
  govprocure-pp-cli grants search "workforce" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("grants search %q (live=%v, limit=%d)", query, live, limit))
				return nil
			}

			// Prefer local FTS unless --live or --data-source live
			if !live {
				return searchLocal(query, limit, cfg, opts)
			}
			return searchLive(query, limit, cfg, opts)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "max results to return")
	cmd.Flags().BoolVar(&live, "live", false, "query grants.gov API directly instead of local DB")
	return cmd
}

func searchLocal(query string, limit int, cfg *config.Config, opts *output.Options) error {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	grants, err := database.SearchGrants(query, limit)
	if err != nil {
		// FTS query error — fall back to live
		output.Warn("local FTS search failed (%v), falling back to live API", err)
		return searchLive(query, limit, cfg, opts)
	}

	if len(grants) == 0 {
		output.Warn("no local results for %q — run 'govprocure-pp-cli grants sync' or use --live", query)
		return nil
	}

	renderGrants(grants, opts)
	return nil
}

func searchLive(query string, limit int, cfg *config.Config, opts *output.Options) error {
	client := api.NewGrantsClient(cfg.GrantsURL)
	resp, err := client.Search(query, limit, 0)
	if err != nil {
		return fmt.Errorf("grants.gov search: %w", err)
	}

	// Convert to db.Grant for unified rendering
	var grants []*db.Grant
	for _, opp := range resp.OppHits {
		raw, _ := json.Marshal(opp)
		grants = append(grants, &db.Grant{
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
			RawJSON:       string(raw),
		})
	}

	if len(grants) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	renderGrants(grants, opts)
	return nil
}

func renderGrants(grants []*db.Grant, opts *output.Options) {
	switch output.EffectiveFormat(opts) {
	case output.FormatJSON:
		var out []map[string]interface{}
		for _, g := range grants {
			m := grantToMap(g)
			out = append(out, output.FilterFields(m, opts.Select))
		}
		output.PrintJSON(out)

	case output.FormatCSV:
		headers := []string{"opportunity_id", "title", "agency", "cfda", "close_date", "award_floor", "award_ceiling"}
		var rows [][]string
		for _, g := range grants {
			rows = append(rows, []string{
				g.OpportunityID, g.Title, g.Agency, g.CFDANumber, g.CloseDate,
				output.FormatDollars(g.AwardFloor), output.FormatDollars(g.AwardCeiling),
			})
		}
		output.PrintCSV(headers, rows)

	case output.FormatCompact:
		for _, g := range grants {
			fmt.Printf("%s | %s | %s | %s\n",
				g.OpportunityID,
				output.Truncate(g.Title, 50),
				g.CloseDate,
				output.FormatDollars(g.AwardCeiling),
			)
		}

	default: // table
		headers := []string{"ID", "TITLE", "AGENCY", "CLOSE DATE", "CEILING"}
		var rows [][]string
		for _, g := range grants {
			rows = append(rows, []string{
				g.OpportunityID,
				output.Truncate(g.Title, 45),
				output.Truncate(g.Agency, 30),
				g.CloseDate,
				output.FormatDollars(g.AwardCeiling),
			})
		}
		output.PrintTable(headers, rows)
		fmt.Printf("\n%d results\n", len(grants))
	}
}

func grantToMap(g *db.Grant) map[string]interface{} {
	return map[string]interface{}{
		"opportunity_id": g.OpportunityID,
		"title":          g.Title,
		"agency":         g.Agency,
		"cfda_number":    g.CFDANumber,
		"close_date":     g.CloseDate,
		"post_date":      g.PostDate,
		"eligibility":    g.Eligibility,
		"synopsis":       g.Synopsis,
		"award_floor":    g.AwardFloor,
		"award_ceiling":  g.AwardCeiling,
	}
}
