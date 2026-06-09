package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type reorderResult struct {
	OrderID    string              `json:"order_id"`
	PlacedAt   string              `json:"placed_at"`
	DryRun     bool                `json:"dry_run"`
	Items      []reorderResultItem `json:"items"`
	AddedCount int                 `json:"added_count"`
}

type reorderResultItem struct {
	ASIN     string `json:"asin"`
	Title    string `json:"title"`
	Quantity int    `json:"quantity"`
	Added    bool   `json:"added"`
	Error    string `json:"error,omitempty"`
}

func newReorderLastCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reorder-last",
		Short: "Re-add every item from your most recent order in one call",
		Long: "Looks up the most recent order in local history and adds each of its " +
			"line items back to the cart. Use --dry-run to preview the plan.",
		Example: "  amazon-pp-cli reorder-last --dry-run\n  amazon-pp-cli reorder-last --profile personal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			order, err := app.Store.MostRecentOrder()
			if err != nil {
				return coded(ExitConflict, "looking up most recent order: %v", err)
			}
			if order.OrderID == "" {
				return coded(ExitNotFound, "no orders in local history (run `history import <path>` first)")
			}
			items, err := app.Store.OrderItems(order.OrderID)
			if err != nil {
				return coded(ExitConflict, "loading items for %s: %v", order.OrderID, err)
			}
			res := reorderResult{
				OrderID:  order.OrderID,
				PlacedAt: order.PlacedAt.Format(time.RFC3339),
				DryRun:   app.DryRun,
			}
			if app.DryRun {
				for _, it := range items {
					res.Items = append(res.Items, reorderResultItem{
						ASIN: it.ASIN, Title: it.Title, Quantity: it.Quantity,
					})
				}
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would re-add %d items from order %s (placed %s):\n",
					len(items), order.OrderID, order.PlacedAt.Format("2006-01-02"))
				for _, it := range items {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d × %s  %s\n", it.Quantity, it.ASIN, truncate(it.Title, 60))
				}
				return nil
			}
			if err := app.RequireSession(); err != nil {
				return err
			}
			// Route through the browser helper, one ASIN at a time. The static
			// /gp/aws/cart/add.html POST silently routes items to items-of-interest;
			// see add.go for the documented failure mode. Slow (each browser
			// launch is ~5-10s) but correct. An add-many helper action could
			// batch this in one browser, but for now correctness > speed.
			cookiesPath := app.Cfg.CookiesPath(app.Profile.Name)
			for _, it := range items {
				ri := reorderResultItem{ASIN: it.ASIN, Title: it.Title, Quantity: it.Quantity}
				addCtx, addCancel := contextWithTimeout(app.Ctx, 120*time.Second)
				br, addErr := runBrowserHelperAdd(addCtx, cookiesPath, it.ASIN, it.Quantity)
				addCancel()
				switch {
				case addErr != nil:
					ri.Error = addErr.Error()
				case br != nil && br.Status == "add_failed":
					ri.Error = br.Reason
				default:
					ri.Added = true
					if br != nil && br.Title != "" {
						ri.Title = br.Title
					}
					res.AddedCount++
				}
				res.Items = append(res.Items, ri)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "re-added %d/%d items from order %s\n", res.AddedCount, len(items), order.OrderID)
			for _, ri := range res.Items {
				status := "OK"
				if !ri.Added {
					status = "FAIL: " + ri.Error
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %d × %s  %s\n", status, ri.Quantity, ri.ASIN, truncate(ri.Title, 50))
			}
			return nil
		},
	}
	return cmd
}
