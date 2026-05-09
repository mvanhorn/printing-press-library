package compound

import (
	"fmt"
	"strings"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

// PipelineResult holds a cross-source result card for one grant opportunity.
type PipelineResult struct {
	Grant       *db.Grant          `json:"grant"`
	SAMNotices  []*db.SAMOpportunity `json:"sam_notices"`
	Awards      []*db.Award         `json:"related_awards"`
	AwardTotal  float64             `json:"award_total"`
	AwardCount  int                 `json:"award_count"`
}

func newPipelineCmd(opts *output.Options) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "pipeline <query>",
		Short: "Cross-source search: grants → SAM notices → USASpending award context",
		Args:  cobra.MinimumNArgs(1),
		Long: `Pipeline performs a three-stage cross-source search:
  1. Search grants_fts for the query → top results
  2. For each grant's agency, look up related SAM contract notices
  3. For each grant's CFDA number, look up historical award context from USASpending

Requires local data — run 'govprocure-pp-cli sync --all' first.`,
		Example: `  govprocure-pp-cli compound pipeline "AI consulting SDVOSB"
  govprocure-pp-cli compound pipeline "literacy education" --limit 3
  govprocure-pp-cli compound pipeline "workforce development" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("compound pipeline %q (limit=%d)", query, limit))
				return nil
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			// Stage 1: find matching grants
			grants, err := database.SearchGrants(query, limit)
			if err != nil || len(grants) == 0 {
				fmt.Printf("No local grants matched %q — run 'grants sync' first.\n", query)
				return nil
			}

			var results []PipelineResult

			for _, g := range grants {
				pr := PipelineResult{Grant: g}

				// Stage 2: related SAM notices by agency
				if g.Agency != "" {
					agencyKeyword := extractAgencyKeyword(g.Agency)
					sam, err := database.SearchSAM(agencyKeyword, 3)
					if err == nil {
						pr.SAMNotices = sam
					}
				}

				// Stage 3: award context by CFDA
				if g.CFDANumber != "" {
					cfda := strings.Split(g.CFDANumber, ",")[0]
					awards, err := database.SearchAwards(cfda, 5)
					if err == nil {
						pr.Awards = awards
						for _, a := range awards {
							pr.AwardTotal += a.Amount
							pr.AwardCount++
						}
					}
				}

				results = append(results, pr)
			}

			if output.EffectiveFormat(opts) == output.FormatJSON {
				output.PrintJSON(results)
				return nil
			}

			// Human-readable pipeline output
			fmt.Printf("Pipeline results for: %q\n", query)
			fmt.Printf("Found %d matching grants\n\n", len(results))
			fmt.Println(strings.Repeat("─", 80))

			for i, pr := range results {
				fmt.Printf("[%d] %s\n", i+1, pr.Grant.Title)
				fmt.Printf("    ID      : %s\n", pr.Grant.OpportunityID)
				fmt.Printf("    Agency  : %s\n", pr.Grant.Agency)
				fmt.Printf("    CFDA    : %s\n", pr.Grant.CFDANumber)
				fmt.Printf("    Closes  : %s\n", pr.Grant.CloseDate)
				fmt.Printf("    Ceiling : %s\n", output.FormatDollars(pr.Grant.AwardCeiling))

				if len(pr.SAMNotices) > 0 {
					fmt.Printf("\n    Related SAM Notices (%d):\n", len(pr.SAMNotices))
					for _, s := range pr.SAMNotices {
						fmt.Printf("      • %s [%s] — %s\n", output.Truncate(s.Title, 45), s.SetAside, s.ResponseDeadline)
					}
				}

				if pr.AwardCount > 0 {
					fmt.Printf("\n    Historical Awards (CFDA %s):\n", pr.Grant.CFDANumber)
					fmt.Printf("      %d awards totaling %s\n", pr.AwardCount, output.FormatDollars(pr.AwardTotal))
					for _, a := range pr.Awards {
						fmt.Printf("      • %s — %s (%s)\n", output.Truncate(a.RecipientName, 35), output.FormatDollars(a.Amount), a.EndDate)
					}
				} else {
					fmt.Printf("\n    Historical Awards: none found (potential low competition)\n")
				}

				fmt.Println(strings.Repeat("─", 80))
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 5, "max grants to include in pipeline")
	return cmd
}

// extractAgencyKeyword picks the most searchable part of an agency name for FTS.
func extractAgencyKeyword(agency string) string {
	// Use the first meaningful word after removing common stop prefixes
	words := strings.Fields(agency)
	stops := map[string]bool{"Department": true, "Office": true, "Bureau": true, "of": true, "the": true, "and": true}
	for _, w := range words {
		if !stops[w] && len(w) > 3 {
			return w
		}
	}
	if len(words) > 0 {
		return words[0]
	}
	return agency
}
