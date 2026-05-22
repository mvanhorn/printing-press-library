package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCostEstimateCmd(flags *rootFlags) *cobra.Command {
	var rows int
	var withDescriptions, withEmail, withPhone bool

	cmd := &cobra.Command{
		Use:   "cost-estimate [endpoint]",
		Short: "Estimate the credit cost of a billed Sumble call before running it",
		Long: strings.Trim(`
Estimate the worst-case credit cost of a billed Sumble endpoint BEFORE you run
it, so an agent can decide whether the call is worth the spend. Pass the
endpoint key and the number of rows you expect (usually your --limit).

If a budget ceiling is set ('budget set <n>'), the estimate is checked against
it: an estimate over the ceiling exits with code 2 so a script can stop before
spending. Run without an endpoint to list the cost of every endpoint.

Endpoint keys: organizations.find, organizations.enrich, organizations.match,
organizations.intelligence-brief, people.find, people.find-related-people,
people.enrich, postings.find, postings.get, postings.find-related-people,
technologies.find, organization-lists.list, organization-lists.get,
contact-lists.list, contact-lists.get.
`, "\n"),
		Example: strings.Trim(`
  sumble-pp-cli cost-estimate organizations.find --rows 25
  sumble-pp-cli cost-estimate people.enrich --rows 5 --include-email
  sumble-pp-cli cost-estimate postings.find --rows 50 --include-descriptions
  sumble-pp-cli cost-estimate
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return printCostTable(cmd, flags)
			}
			key := args[0]
			est, err := estimateCost(key, rows, withDescriptions, withEmail, withPhone)
			if err != nil {
				return usageErr(err)
			}

			// Read the budget ceiling. Surface a store/read failure rather than
			// silently treating the call as within-budget — a silent bypass
			// would defeat the guard exactly when the local state is broken.
			db, derr := openCreditStore()
			if derr != nil {
				return configErr(fmt.Errorf("cannot read budget ceiling: %w", derr))
			}
			ceiling, hasBudget, berr := getBudget(db.DB())
			_ = db.Close()
			if berr != nil {
				return apiErr(fmt.Errorf("reading budget ceiling: %w", berr))
			}
			withinBudget := !hasBudget || est.EstimatedCredits <= ceiling

			if flags.asJSON {
				out := struct {
					costEstimate
					Budget       *int `json:"budget_ceiling,omitempty"`
					WithinBudget bool `json:"within_budget"`
				}{costEstimate: est, WithinBudget: withinBudget}
				if hasBudget {
					out.Budget = &ceiling
				}
				if err := flags.printJSON(cmd, out); err != nil {
					return err
				}
			} else {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "%s: ~%d credits (%d rows)\n", est.Endpoint, est.EstimatedCredits, est.Rows)
				fmt.Fprintf(w, "  %s\n", est.Note)
				if hasBudget {
					status := "within budget"
					if !withinBudget {
						status = "OVER budget"
					}
					fmt.Fprintf(w, "  budget ceiling: %d credits (%s)\n", ceiling, status)
				}
			}

			if !withinBudget {
				return usageErr(fmt.Errorf("estimated %d credits exceeds budget ceiling %d", est.EstimatedCredits, ceiling))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&rows, "rows", 1, "Number of billable rows you expect (your --limit; for organizations.enrich this is the number of technologies found, not organizations, so a single enrich can cost 5x many)")
	cmd.Flags().BoolVar(&withDescriptions, "include-descriptions", false, "postings.find: include descriptions (3 credits/job instead of 2)")
	cmd.Flags().BoolVar(&withEmail, "include-email", false, "people.enrich: reveal email (10 credits/person)")
	cmd.Flags().BoolVar(&withPhone, "include-phone", false, "people.enrich: reveal phone (80 credits/person)")
	return cmd
}

func printCostTable(cmd *cobra.Command, flags *rootFlags) error {
	if flags.asJSON {
		type row struct {
			Endpoint string `json:"endpoint"`
			Note     string `json:"note"`
		}
		rows := make([]row, 0, len(creditCosts))
		for _, k := range sortedCostKeys() {
			rows = append(rows, row{Endpoint: k, Note: creditCosts[k].note})
		}
		return flags.printJSON(cmd, rows)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "Sumble v6 credit costs:")
	for _, k := range sortedCostKeys() {
		fmt.Fprintf(w, "  %-34s %s\n", k, creditCosts[k].note)
	}
	return nil
}
