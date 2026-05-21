package cli

import (
	"github.com/spf13/cobra"
)

// newArrivingSoonCmd lists shipments whose ETA falls within the next N days.
func newArrivingSoonCmd(flags *rootFlags) *cobra.Command {
	var days int
	var window string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "arriving-soon",
		Short: "Shipments arriving in the next N days, sorted by ETA.",
		Long: `Cross-order ETA window query. Walks recent order history, filters by ETA,
and emits a list of orders coming in the next N days.`,
		Example:     "  amazon-orders-pp-cli arriving-soon --days 7 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			orders, err := fetchOrderListPages(cmd.Context(), c, window, maxPages)
			if err != nil {
				return err
			}
			arriving := arrivingByDay(orders, days)
			return printJSONFiltered(cmd.OutOrStdout(), arriving, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Window of days from today to consider (inclusive).")
	cmd.Flags().StringVar(&window, "window", "months-3", "Time window to scan: year-YYYY, last30days, months-3, months-6, archived.")
	cmd.Flags().IntVar(&maxPages, "max-pages", 4, "Maximum order-history pages to walk (10 orders/page).")
	return cmd
}
