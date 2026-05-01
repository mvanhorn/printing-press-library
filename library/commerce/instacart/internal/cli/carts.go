package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

func newCartsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "carts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List every active cart across your retailers",
		Long: `Instacart users typically have one cart per retailer. This command lists
every active cart on your account -- Costco, Sprouts, CVS, whatever you've
been shopping at -- with item counts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}
			client := gql.NewClient(app.Session, app.Cfg, app.Store)
			resp, err := client.Query(app.Ctx, "PersonalActiveCarts", map[string]any{})
			if err != nil {
				return err
			}
			var parsed struct {
				Data instacart.PersonalActiveCartsResponse `json:"data"`
			}
			if err := json.Unmarshal(resp.RawBody, &parsed); err != nil {
				return err
			}
			carts := parsed.Data.UserCarts.Carts
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(carts)
			}
			if len(carts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no active carts")
				return nil
			}
			// Side effect: cache retailers we see in carts so `retailers list`
			// has something to show on next invocation.
			for _, c := range carts {
				if c.Retailer.Slug != "" {
					_ = app.Store.UpsertRetailer(store.Retailer{
						Slug:       c.Retailer.Slug,
						RetailerID: c.Retailer.ID,
						Name:       c.Retailer.Name,
					})
				}
			}
			for _, c := range carts {
				name := c.Retailer.Name
				if name == "" {
					name = c.Retailer.Slug
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s slug=%s cart=%s items=%d\n",
					name, c.Retailer.Slug, c.ID, c.ItemCount)
			}
			return nil
		},
	}
}

func newCartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cart",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Inspect or modify a specific cart",
	}
	cmd.AddCommand(newCartShowCmd(), newCartRemoveCmd())
	return cmd
}

func newCartShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <retailer-slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show the contents of your cart at a retailer with real item names",
		Long: `Lists every item in your active cart at <retailer> with its real product
name, quantity, and item id. Chains CartData -> ShopCollectionScoped ->
Items under the hood, caching resolved names to the local products table
so subsequent calls for the same items skip the Items round trip.`,
		Example: `  instacart cart show pcc-community-markets
  instacart cart show costco --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			retailer := args[0]
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}
			cartID, _ := resolveActiveCartID(app, retailer)
			if cartID == "" {
				return coded(ExitNotFound, "no active cart at %s -- run `instacart carts` to see your active carts", retailer)
			}

			items, err := gql.FetchCartItems(app.Ctx, app.Session, app.Cfg, app.Store, cartID, retailer)
			if err != nil {
				return coded(ExitTransient, "%v", err)
			}

			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"retailer":   retailer,
					"cart_id":    cartID,
					"item_count": len(items),
					"items":      items,
				})
			}
			if len(items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s cart (id=%s): empty\n", retailer, cartID)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s cart (id=%s): %d item(s)\n", retailer, cartID, len(items))
			for i, it := range items {
				name := it.Name
				if name == "" {
					name = "(name unresolved)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %2d. %s  qty=%g\n      item_id=%s\n",
					i+1, name, it.Quantity, it.ItemID)
			}
			return nil
		},
	}
}

func newCartRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <item-id> <retailer-slug>",
		Short: "Remove an item from your cart at a retailer",
		Long: `Removes a specific item from your active cart by item id. Use
'instacart cart show' to discover item ids, or 'instacart search' to find a
product's item id before removing.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			retailer := args[1]
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}
			cartID, _ := resolveActiveCartID(app, retailer)
			if cartID == "" {
				return coded(ExitNotFound, "no active cart at %s -- run `instacart carts` to see your active carts", retailer)
			}
			if app.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would remove %s from cart %s at %s\n", itemID, cartID, retailer)
				return nil
			}
			client := gql.NewClient(app.Session, app.Cfg, app.Store)
			vars := instacart.UpdateCartItemsVars{
				CartItemUpdates: []instacart.CartItemUpdate{{
					ItemID:         itemID,
					Quantity:       0,
					QuantityType:   "each",
					TrackingParams: json.RawMessage(`{}`),
				}},
				CartType: "grocery",
				CartID:   cartID,
			}
			_, err = client.Mutation(app.Ctx, "UpdateCartItemsMutation", vars, "")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s from %s cart\n", itemID, retailer)
			return nil
		},
	}
}
