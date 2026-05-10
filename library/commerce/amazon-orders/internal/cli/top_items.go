package cli

import (
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/spf13/cobra"
)

// TopItem is one row in the top-items rollup.
type TopItem struct {
	ASIN       string  `json:"asin,omitempty"`
	Title      string  `json:"title"`
	OrderCount int     `json:"orderCount"`
	TotalSpend float64 `json:"totalSpend,omitempty"`
}

// newTopItemsCmd rolls up the most-ordered items across history.
func newTopItemsCmd(flags *rootFlags) *cobra.Command {
	var by string
	var limit int
	var window string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "top-items",
		Short: "Most-ordered items across recent history, grouped by ASIN or title.",
		Long: `Walks recent order history and rolls up items by ASIN (when known) or title.
Currently uses the per-order total as a stand-in for per-item spend, since
the listing page does not break out unit prices. For per-item spend run
'orders get <id>' on each order, or wait for the local-store sync path.`,
		Example:     "  amazon-orders-pp-cli top-items --by count --limit 10 --json",
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
			items := rollupTopItems(orders, by, limit)
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "count", "Sort by 'count' (most ordered) or 'total-spend' (highest total).")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of items to return.")
	cmd.Flags().StringVar(&window, "window", defaultYearWindow(), "Time window to scan: year-YYYY, last30days, months-3, months-6, archived (defaults to the current calendar year).")
	cmd.Flags().IntVar(&maxPages, "max-pages", 12, "Maximum order-history pages to walk (10 orders/page).")
	return cmd
}

func rollupTopItems(orders []parser.OrderSummary, by string, limit int) []TopItem {
	type bucket struct {
		ASIN  string
		Title string
		Count int
		Spend float64
	}
	buckets := map[string]*bucket{}
	for _, o := range orders {
		// Use ASIN when available, else use the first item title as the key.
		var key, asin, title string
		if len(o.ASINs) > 0 {
			asin = o.ASINs[0]
			key = "asin:" + asin
			if len(o.ItemTitles) > 0 {
				title = o.ItemTitles[0]
			} else {
				title = "(no title)"
			}
		} else if len(o.ItemTitles) > 0 {
			title = o.ItemTitles[0]
			key = "title:" + title
		} else {
			continue
		}
		b, ok := buckets[key]
		if !ok {
			b = &bucket{ASIN: asin, Title: title}
			buckets[key] = b
		}
		b.Count++
		b.Spend += o.Total
	}

	out := make([]TopItem, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, TopItem{ASIN: b.ASIN, Title: b.Title, OrderCount: b.Count, TotalSpend: b.Spend})
	}
	switch strings.ToLower(by) {
	case "total-spend", "totalspend", "spend":
		sort.SliceStable(out, func(i, j int) bool { return out[i].TotalSpend > out[j].TotalSpend })
	default:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].OrderCount != out[j].OrderCount {
				return out[i].OrderCount > out[j].OrderCount
			}
			return out[i].TotalSpend > out[j].TotalSpend
		})
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
