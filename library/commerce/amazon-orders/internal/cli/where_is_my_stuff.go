package cli

import (
	"sort"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/spf13/cobra"
)

// newWhereIsMyStuffCmd returns the radar command — the unified view of every
// in-flight Amazon shipment with current status and ETA.
func newWhereIsMyStuffCmd(flags *rootFlags) *cobra.Command {
	var window string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "where-is-my-stuff",
		Short: "All in-flight Amazon shipments with current status and ETA in one view.",
		Long: `Walks the most recent N pages of order history, filters to in-flight orders
(not delivered, not cancelled), and emits a single list sorted by ETA.

Use this when you need to answer 'is something coming today' across many
orders without clicking into each one.`,
		Example:     "  amazon-orders-pp-cli where-is-my-stuff --json",
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
			inflight := inflightOrders(orders)
			sortByETAAsc(inflight)
			return printJSONFiltered(cmd.OutOrStdout(), inflight, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "months-3", "Time window to scan: year-YYYY, last30days, months-3, months-6, archived.")
	cmd.Flags().IntVar(&maxPages, "max-pages", 6, "Maximum order-history pages to walk (10 orders/page).")
	return cmd
}

// sortByETAAsc sorts orders so that orders with an ETA come first (ascending
// chronological), then orders without ETA.
func sortByETAAsc(orders []parser.OrderSummary) {
	sort.SliceStable(orders, func(i, j int) bool {
		a, b := orders[i].ETADate, orders[j].ETADate
		if a == "" && b == "" {
			return false
		}
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a < b
	})
}
