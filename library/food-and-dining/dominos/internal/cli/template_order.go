package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newTemplateOrderCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	var etaWatch bool
	cmd := &cobra.Command{
		Use:   "order <name>",
		Short: "Load a template into the active cart and place the order",
		Long: `Load a template into the active cart and place the order.

Without --confirm, prints a dry-run preview of what would be placed.
With --confirm, places the order. With --eta-watch, then polls the
tracker until the order is delivered.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("template name is required"))
			}
			name := args[0]
			tpl, err := cartstore.LoadTemplate(name)
			if err != nil {
				if errors.Is(err, cartstore.ErrNotFound) {
					return usageErr(fmt.Errorf("template %q not found; run 'dominos-pp-cli template list' to see saved templates", name))
				}
				return err
			}
			cart := &cartstore.Cart{
				StoreID: tpl.StoreID, Service: tpl.Service, Address: tpl.Address,
				Items: tpl.Items, CreatedAt: tpl.CreatedAt,
			}
			if dryRunOK(flags) || !confirm {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":   "template_order",
					"dry_run":  true,
					"template": name,
					"cart":     cart,
					"hint":     "rerun with --confirm to place the order",
				}, flags)
			}
			// Confirm path: defer to the order_quick composition so
			// we don't re-implement validate -> price -> place here.
			if err := cartstore.SaveActive(cart); err != nil {
				return fmt.Errorf("activating template: %w", err)
			}
			return placeQuickOrder(cmd, flags, cart, confirm, etaWatch && confirm)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually place the order (without this flag, dry-run preview)")
	cmd.Flags().BoolVar(&etaWatch, "eta-watch", false, "After placing, poll the tracker until delivered")
	return cmd
}
