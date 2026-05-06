package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newCartAddCmd(flags *rootFlags) *cobra.Command {
	var size string
	var qty int
	var toppings []string

	cmd := &cobra.Command{
		Use:   "add <code>",
		Short: "Append an item to the active cart",
		Long: `Append an item to the active cart.

Toppings use the syntax "code:placement:weight", e.g. "P:left:1.5" for
extra pepperoni on the left half. Repeat --topping for multiple toppings.`,
		Example:     "  dominos-pp-cli cart add S_PIZPH --size large --qty 2 --topping P:full:1",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("item code is required (e.g. cart add S_PIZPH)"))
			}
			if qty <= 0 {
				qty = 1
			}
			item := cartstore.CartItem{
				Code:     args[0],
				Qty:      qty,
				Size:     size,
				Toppings: toppings,
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "cart_add",
					"dry_run": true,
					"item":    item,
				}, flags)
			}
			cart, err := cartstore.LoadActive()
			if err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					return usageErr(fmt.Errorf("no active cart; run 'dominos-pp-cli cart new --store <id> --service Delivery --address \"...\"' first"))
				}
				return err
			}
			cart.Items = append(cart.Items, item)
			if err := cartstore.SaveActive(cart); err != nil {
				return fmt.Errorf("saving active cart: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action":     "cart_add",
				"cart":       cart,
				"item_count": len(cart.Items),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&size, "size", "", "Item size (small, medium, large, x-large)")
	cmd.Flags().IntVar(&qty, "qty", 1, "Quantity")
	cmd.Flags().StringArrayVar(&toppings, "topping", nil, "Topping spec: code:placement:weight (repeatable)")
	return cmd
}
