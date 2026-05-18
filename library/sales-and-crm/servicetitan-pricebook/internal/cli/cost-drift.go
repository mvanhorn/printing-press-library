package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newCostDriftCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "cost-drift",
		Short: "Show SKUs whose cost moved and whether the price followed",
		Long: "Snapshots the current pricebook into the local sku_cost_history change\n" +
			"log, then diffs each SKU's cost between a baseline snapshot and the\n" +
			"latest one. price_followed reports whether the price moved with the\n" +
			"cost — the margin-discipline question the ServiceTitan UI cannot answer.\n" +
			"Needs at least two snapshots to show drift; run it after each sync.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli cost-drift
  servicetitan-pricebook-pp-cli cost-drift --since 2026-04-01 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openPricebookStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			// Capture the current state before diffing so the latest
			// snapshot always reflects the just-synced pricebook.
			if _, _, err := pricebook.Snapshot(db); err != nil {
				return err
			}
			rows, err := pricebook.CostDrift(db, since)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []pricebook.DriftRow{}
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					string(r.Kind), r.Code, r.DisplayName,
					f2(r.OldCost), f2(r.NewCost), f2(r.CostDelta),
					f2(r.OldPrice), f2(r.NewPrice), f2(r.PriceDelta),
					strconv.FormatBool(r.PriceFollowed),
				})
			}
			return pbOutput(cmd, flags, rows,
				[]string{"KIND", "CODE", "NAME", "OLD COST", "NEW COST", "COST D", "OLD PRICE", "NEW PRICE", "PRICE D", "PRICE FOLLOWED"},
				table)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Baseline date (RFC3339 or YYYY-MM-DD); drift is measured from the newest snapshot at or before this")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
