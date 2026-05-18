package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newRepriceCmd(flags *rootFlags) *cobra.Command {
	var tolerance float64
	var apply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "reprice",
		Short: "Compute tier-correct prices for markup-drifted SKUs (preview unless --apply)",
		Long: "Takes the markup-audit drift findings, computes the tier-correct price\n" +
			"for each from the materials-markup ladder, and shows the proposed price\n" +
			"changes. This is the hold-markup workflow: when a vendor cost moves,\n" +
			"the price should move with it.\n\n" +
			"Without --apply this only previews the plan. With --apply it pushes the\n" +
			"changes through one pricebook bulk-update call.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli reprice --tolerance 5
  servicetitan-pricebook-pp-cli reprice --tolerance 5 --apply
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openPricebookStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := pricebook.Reprice(db, tolerance)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []pricebook.RepriceRow{}
			}

			if !apply {
				table := make([][]string, 0, len(rows))
				for _, r := range rows {
					table = append(table, []string{
						string(r.Kind), r.Code, r.DisplayName,
						f2(r.Cost), f2(r.OldPrice), f2(r.NewPrice), f2(r.TierPercent),
					})
				}
				return pbOutput(cmd, flags, rows,
					[]string{"KIND", "CODE", "NAME", "COST", "OLD PRICE", "NEW PRICE", "TIER%"}, table)
			}

			// --apply: build one bulk-update payload from the proposed prices.
			changes := make([]pricebook.BulkChange, 0, len(rows))
			for _, r := range rows {
				newPrice := r.NewPrice // stable per-iteration address
				changes = append(changes, pricebook.BulkChange{Kind: r.Kind, ID: r.ID, Price: &newPrice})
			}
			if len(changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no markup-drifted SKUs to reprice")
				return nil
			}
			payload := pricebook.BulkPlan(changes)

			// Defense-in-depth: never write during a verify pass even if the
			// dryRunOK guard above is ever loosened.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would apply %d price changes to the pricebook\n", len(changes))
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			tenant := cfg.TenantID
			if tenant == "" {
				return fmt.Errorf("ST_TENANT_ID is not set — cannot apply changes")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, status, err := c.Patch("/tenant/"+tenant+"/pricebook", payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "repriced %d SKUs (HTTP %d)\n", len(changes), status)
			return nil
		},
	}
	cmd.Flags().Float64Var(&tolerance, "tolerance", 5.0, "Markup-drift tolerance (percentage points) — only SKUs past this band are repriced")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the proposed prices via a pricebook bulk-update (default: preview only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
