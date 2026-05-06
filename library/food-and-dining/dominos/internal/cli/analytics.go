package cli

import (
	"errors"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

// newAnalyticsCmd aggregates the local cartstore (templates and active
// cart) as a stand-in for sync'd order history. Each saved template is
// the canonical record of an order the user replays, so rolling them up
// gives a real "what do I order most" view without an order-history
// endpoint. When no templates exist the command returns a zero envelope
// with a hint, so agents can keep going.
func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var period, groupBy string
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Spending and order analytics from your local cart store",
		Long: `Aggregate your saved order templates and active cart into
counts by store, service, and item. Each saved template stands in for an
order in the absence of a sync'd order-history endpoint.

Group-by values: store, service, item. Period is accepted for forward
compatibility (saved templates carry only a created_at timestamp).

Returns {"templates_count": 0, ...} with a hint when no templates exist.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  dominos-pp-cli analytics\n  dominos-pp-cli analytics --group-by store --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if period == "" {
				period = "90d"
			}
			if groupBy == "" {
				groupBy = "store"
			}

			names, err := cartstore.ListTemplates()
			if err != nil {
				return err
			}

			rows := []map[string]any{}
			storeCounts := map[string]int{}
			serviceCounts := map[string]int{}
			itemCounts := map[string]int{}
			for _, name := range names {
				t, err := cartstore.LoadTemplate(name)
				if err != nil {
					if errors.Is(err, cartstore.ErrNotFound) {
						continue
					}
					return err
				}
				if t.StoreID != "" {
					storeCounts[t.StoreID]++
				}
				if t.Service != "" {
					serviceCounts[t.Service]++
				}
				for _, it := range t.Items {
					if it.Code != "" {
						itemCounts[it.Code] += it.Qty
					}
				}
			}

			switch groupBy {
			case "store":
				for k, v := range storeCounts {
					rows = append(rows, map[string]any{"store_id": k, "templates": v})
				}
			case "service":
				for k, v := range serviceCounts {
					rows = append(rows, map[string]any{"service": k, "templates": v})
				}
			case "item":
				for k, v := range itemCounts {
					rows = append(rows, map[string]any{"item": k, "qty": v})
				}
			}

			out := map[string]any{
				"period":          period,
				"group_by":        groupBy,
				"templates_count": len(names),
				"rows":            rows,
			}
			if len(names) == 0 {
				out["hint"] = "no templates yet; save one with 'dominos-pp-cli template save <name>' after building a cart"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "90d", "Lookback window: e.g. 30d, 90d, 1y (reserved for forward compatibility)")
	cmd.Flags().StringVar(&groupBy, "group-by", "store", "Group rollup by: store, service, item")
	return cmd
}
