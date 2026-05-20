package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/spf13/cobra"
)

// newFindCmd searches order history for a free-text query across order ID,
// item titles, ship-to recipient, and ASINs.
func newFindCmd(flags *rootFlags) *cobra.Command {
	var window string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Search orders, items, and ASINs for a free-text query.",
		Long: `Walks recent order history, then filters orders whose order ID, ship-to
recipient, item titles, or ASINs contain the query (case-insensitive
substring match).

Live-mode only in v1; future versions will add FTS5 search over the local
SQLite store after a 'sync' run.`,
		Example:     "  amazon-orders-pp-cli find 'usb-c cable' --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			if query == "" {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			orders, err := fetchOrderListPages(cmd.Context(), c, window, maxPages)
			if err != nil {
				return err
			}
			out := []parser.OrderSummary{}
			for _, o := range orders {
				if matchesQuery(o, query) {
					out = append(out, o)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", defaultYearWindow(), "Time window to scan: year-YYYY, last30days, months-3, months-6, archived (defaults to the current calendar year).")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum order-history pages to walk (10 orders/page).")
	return cmd
}

// matchesQuery returns true if any text field of an order contains q
// (case-insensitive substring).
func matchesQuery(o parser.OrderSummary, q string) bool {
	if q == "" {
		return false
	}
	if strings.Contains(strings.ToLower(o.OrderID), q) {
		return true
	}
	if strings.Contains(strings.ToLower(o.ShipTo), q) {
		return true
	}
	for _, a := range o.ASINs {
		if strings.Contains(strings.ToLower(a), q) {
			return true
		}
	}
	for _, t := range o.ItemTitles {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}
