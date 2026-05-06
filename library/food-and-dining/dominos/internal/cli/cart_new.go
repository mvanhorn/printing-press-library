package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cartstore"

	"github.com/spf13/cobra"
)

func newCartNewCmd(flags *rootFlags) *cobra.Command {
	var storeID, service, address string

	cmd := &cobra.Command{
		Use:         "new",
		Short:       "Start a fresh cart, overwriting any existing active cart",
		Example:     "  dominos-pp-cli cart new --store 7094 --service Delivery --address \"421 N 63rd St\"",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if storeID == "" {
				return usageErr(fmt.Errorf("--store is required"))
			}
			if service == "" {
				service = "Delivery"
			}
			if service != "Delivery" && service != "Carryout" {
				return usageErr(fmt.Errorf("--service must be Delivery or Carryout, got %q", service))
			}
			cart := &cartstore.Cart{
				StoreID:   storeID,
				Service:   service,
				Address:   address,
				Items:     []cartstore.CartItem{},
				CreatedAt: time.Now().UTC(),
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "cart_new",
					"dry_run": true,
					"cart":    cart,
				}, flags)
			}
			if err := cartstore.SaveActive(cart); err != nil {
				return fmt.Errorf("saving active cart: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action": "cart_new",
				"cart":   cart,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&storeID, "store", "", "Store ID (required)")
	cmd.Flags().StringVar(&service, "service", "Delivery", "Service method: Delivery or Carryout")
	cmd.Flags().StringVar(&address, "address", "", "Delivery address (free-form)")
	return cmd
}
