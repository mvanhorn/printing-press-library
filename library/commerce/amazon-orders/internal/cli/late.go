package cli

import (
	"github.com/spf13/cobra"
)

// newLateCmd lists in-flight shipments past their original ETA.
func newLateCmd(flags *rootFlags) *cobra.Command {
	var window string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "late",
		Short: "In-flight shipments past their estimated delivery date.",
		Long: `Carrier or seller delays surface immediately: walks recent order history,
filters to in-flight orders whose ETA has already passed, sorted by ETA.`,
		Example:     "  amazon-orders-pp-cli late --json",
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
			late := lateOrders(orders)
			return printJSONFiltered(cmd.OutOrStdout(), late, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "months-3", "Time window to scan: year-YYYY, last30days, months-3, months-6, archived.")
	cmd.Flags().IntVar(&maxPages, "max-pages", 6, "Maximum order-history pages to walk (10 orders/page).")
	return cmd
}
