package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/spf13/cobra"
)

// PATCH(greptile-year-default): replaced static "year-2026" default with current-year helper.
// defaultYearWindow returns "year-YYYY" for the current calendar year. Used as
// the default value for --window flags so the CLI does not silently query an
// empty result set after the year ends.
func defaultYearWindow() string {
	return fmt.Sprintf("year-%d", time.Now().Year())
}

// SpendBucket is one row of a spending rollup.
type SpendBucket struct {
	Key        string  `json:"key"`
	OrderCount int     `json:"orderCount"`
	Total      float64 `json:"total"`
	Currency   string  `json:"currency,omitempty"`
}

// newSpendCmd rolls up spending by a chosen dimension.
func newSpendCmd(flags *rootFlags) *cobra.Command {
	var by string
	var window string
	var year int
	var maxPages int

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Spending broken down by month, year, or recipient.",
		Long: `Walks recent order history and rolls up totals by the chosen dimension.

Supported --by values:
  month   — group by YYYY-MM of placed date
  year    — group by YYYY of placed date
  ship-to — group by recipient name
  status  — group by order status

Use --year YYYY as a shortcut for --window year-YYYY.
`,
		Example:     "  amazon-orders-pp-cli spend --by month --year 2026 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			effectiveWindow := window
			if year > 0 {
				effectiveWindow = fmt.Sprintf("year-%d", year)
			}
			orders, err := fetchOrderListPages(cmd.Context(), c, effectiveWindow, maxPages)
			if err != nil {
				return err
			}
			rollup := rollupSpend(orders, by)
			return printJSONFiltered(cmd.OutOrStdout(), rollup, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "month", "Dimension to roll up by: month, year, ship-to, status.")
	cmd.Flags().StringVar(&window, "window", defaultYearWindow(), "Time window to scan: year-YYYY, last30days, months-3, months-6, archived (defaults to the current calendar year).")
	cmd.Flags().IntVar(&year, "year", 0, "Convenience: roll up the given calendar year (overrides --window).")
	cmd.Flags().IntVar(&maxPages, "max-pages", 12, "Maximum order-history pages to walk (10 orders/page).")
	return cmd
}

func rollupSpend(orders []parser.OrderSummary, by string) []SpendBucket {
	buckets := map[string]*SpendBucket{}
	for _, o := range orders {
		key := bucketKey(o, by)
		if key == "" {
			key = "(unknown)"
		}
		b, ok := buckets[key]
		if !ok {
			// PATCH(greptile-rollup-currency): use the per-order Currency that
			// parser.parseOrderCard populated (e.g. "GBP" on amazon.co.uk)
			// instead of hardcoding "USD". Fall back to "USD" only if the
			// parser couldn't determine one. We freeze on the first order's
			// currency per bucket — mixed-currency aggregation isn't a real
			// case because a single Amazon TLD only emits one currency.
			currency := o.Currency
			if currency == "" {
				currency = "USD"
			}
			b = &SpendBucket{Key: key, Currency: currency}
			buckets[key] = b
		}
		b.OrderCount++
		b.Total += o.Total
	}
	out := make([]SpendBucket, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, *b)
	}
	// Sort: month/year keys ascending, others by total descending.
	if by == "month" || by == "year" {
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	}
	return out
}

func bucketKey(o parser.OrderSummary, by string) string {
	switch strings.ToLower(by) {
	case "year":
		if len(o.PlacedDate) >= 4 {
			return o.PlacedDate[:4]
		}
	case "month":
		if len(o.PlacedDate) >= 7 {
			return o.PlacedDate[:7]
		}
	case "ship-to", "shipto":
		return strings.TrimSpace(o.ShipTo)
	case "status":
		if o.Status == "" {
			return "(unknown)"
		}
		return o.Status
	default:
		return fmt.Sprintf("(unsupported --by=%q)", by)
	}
	return ""
}
