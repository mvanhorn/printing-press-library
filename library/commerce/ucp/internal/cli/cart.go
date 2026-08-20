package cli

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
	"github.com/spf13/cobra"
)

func newCartCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Manage local UCP carts",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCartAddCmd(flags))
	cmd.AddCommand(newCartListCmd(flags))
	cmd.AddCommand(newCartShowCmd(flags))
	cmd.AddCommand(newCartRemoveCmd(flags))
	cmd.AddCommand(newCartDeleteCmd(flags))
	return cmd
}

func newCartAddCmd(flags *rootFlags) *cobra.Command {
	var merchant, sku, gtin, title, cartID string
	var qty, price int
	var variantID int64

	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a line item to a local cart",
		Example: `  ucp-pp-cli cart add --merchant 127.0.0.1:8080 --sku coffee_001 --title "Coffee" --price 1500 --qty 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if merchant == "" {
				return usageErr(fmt.Errorf("--merchant is required"))
			}
			if sku == "" && gtin == "" && title == "" {
				return usageErr(fmt.Errorf("one of --sku, --gtin, or --title is required"))
			}
			if price <= 0 {
				return usageErr(fmt.Errorf("--price (in cents) is required"))
			}

			var cart *ucp.Cart
			var err error
			if cartID != "" {
				cart, err = store.Load(cartID)
				if err != nil {
					return fmt.Errorf("load cart: %w", err)
				}
				if merchant != "" && merchant != cart.Merchant {
					return usageErr(fmt.Errorf("--merchant %q does not match cart merchant %q; omit --merchant to add to this cart", merchant, cart.Merchant))
				}
			} else {
				cart = store.New(merchant)
			}

			li := ucp.LineItem{
				ID:       uuid.New().String(),
				Quantity: qty,
				Item: ucp.Item{
					ID:        uuid.New().String(),
					Title:     title,
					Price:     price,
					SKU:       sku,
					GTIN:      gtin,
					VariantID: variantID,
				},
			}
			cart.LineItems = append(cart.LineItems, li)

			if err := store.Save(cart); err != nil {
				return fmt.Errorf("save cart: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(cart)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added item to cart %s\n", cart.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&merchant, "merchant", "", "Merchant domain (required)")
	cmd.Flags().StringVar(&sku, "sku", "", "Product SKU")
	cmd.Flags().StringVar(&gtin, "gtin", "", "Product GTIN")
	cmd.Flags().StringVar(&title, "title", "", "Product title")
	cmd.Flags().IntVar(&qty, "qty", 1, "Quantity")
	cmd.Flags().IntVar(&price, "price", 0, "Price in cents (required)")
	cmd.Flags().StringVar(&cartID, "cart", "", "Cart ID (creates new cart if not set)")
	cmd.Flags().Int64Var(&variantID, "variant-id", 0, "Shopify numeric variant ID (required for real Shopify cart-add via checkout finalize)")
	return cmd
}

func newCartListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all local carts with merchant, line-item count, and total",
		Example: `  ucp-pp-cli cart list --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			carts, err := store.List()
			if err != nil {
				return fmt.Errorf("list carts: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(carts)
			}

			if len(carts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No carts.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tMERCHANT\tITEMS\tTOTAL\tUPDATED")
			for _, c := range carts {
				total := cartTotal(c)
				fmt.Fprintf(tw, "%s\t%s\t%d\t$%.2f\t%s\n",
					c.ID, c.Merchant, len(c.LineItems),
					float64(total)/100, c.UpdatedAt)
			}
			return tw.Flush()
		},
	}
}

func newCartShowCmd(flags *rootFlags) *cobra.Command {
	var cartID string
	cmd := &cobra.Command{
		Use:     "show",
		Short:   "Show a cart's line items, status, and totals by ID",
		Example: `  ucp-pp-cli cart show --cart <cart-id> --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cartID == "" {
				return usageErr(fmt.Errorf("--cart is required"))
			}
			cart, err := store.Load(cartID)
			if err != nil {
				return fmt.Errorf("load cart: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(cart)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "Cart ID:\t%s\n", cart.ID)
			fmt.Fprintf(tw, "Merchant:\t%s\n", cart.Merchant)
			fmt.Fprintf(tw, "Status:\t%s\n", cart.Status)
			fmt.Fprintln(tw)
			fmt.Fprintln(tw, "LINE ID\tTITLE\tQTY\tPRICE\tSKU")
			for _, li := range cart.LineItems {
				fmt.Fprintf(tw, "%s\t%s\t%d\t$%.2f\t%s\n",
					li.ID, li.Item.Title, li.Quantity,
					float64(li.Item.Price)/100, li.Item.SKU)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&cartID, "cart", "", "Cart ID (required)")
	return cmd
}

func newCartRemoveCmd(flags *rootFlags) *cobra.Command {
	var cartID, lineID string
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove a specific line item from a cart",
		Example: `  ucp-pp-cli cart remove --cart <cart-id> --line <line-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cartID == "" {
				return usageErr(fmt.Errorf("--cart is required"))
			}
			if lineID == "" {
				return usageErr(fmt.Errorf("--line is required"))
			}
			cart, err := store.Load(cartID)
			if err != nil {
				return fmt.Errorf("load cart: %w", err)
			}
			newItems := cart.LineItems[:0]
			removed := false
			for _, li := range cart.LineItems {
				if li.ID == lineID {
					removed = true
					continue
				}
				newItems = append(newItems, li)
			}
			if !removed {
				return fmt.Errorf("line item %q not found in cart %s", lineID, cartID)
			}
			cart.LineItems = newItems
			if err := store.Save(cart); err != nil {
				return fmt.Errorf("save cart: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed line %s from cart %s\n", lineID, cartID)
			return nil
		},
	}
	cmd.Flags().StringVar(&cartID, "cart", "", "Cart ID (required)")
	cmd.Flags().StringVar(&lineID, "line", "", "Line item ID (required)")
	return cmd
}

func newCartDeleteCmd(flags *rootFlags) *cobra.Command {
	var cartID string
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a local cart and all its line items by ID",
		Example: `  ucp-pp-cli cart delete --cart <cart-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cartID == "" {
				return usageErr(fmt.Errorf("--cart is required"))
			}
			if err := store.Delete(cartID); err != nil {
				return fmt.Errorf("delete cart: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted cart %s\n", cartID)
			return nil
		},
	}
	cmd.Flags().StringVar(&cartID, "cart", "", "Cart ID (required)")
	return cmd
}

func cartTotal(c *ucp.Cart) int {
	total := 0
	for _, li := range c.LineItems {
		total += li.Item.Price * li.Quantity
	}
	return total
}
