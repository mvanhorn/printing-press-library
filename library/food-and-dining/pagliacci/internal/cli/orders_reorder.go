// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci/internal/client"
)

// findMostRecentOrderID hits /OrderList/1/1 and returns the single order ID.
// Returns empty string with no error when the user has no orders.
func findMostRecentOrderID(c *client.Client) (string, error) {
	raw, err := c.Get("/OrderList/1/1", nil)
	if err != nil {
		return "", err
	}
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) != nil || len(arr) == 0 {
		return "", nil
	}
	for _, o := range arr {
		for _, k := range []string{"ID", "Id", "OrderID", "OrderId"} {
			if v, ok := o[k]; ok {
				switch n := v.(type) {
				case float64:
					return strconv.FormatInt(int64(n), 10), nil
				case string:
					return n, nil
				default:
					return fmt.Sprintf("%v", v), nil
				}
			}
		}
	}
	return "", nil
}

func newOrdersReorderCmd(flags *rootFlags) *cobra.Command {
	var useLast bool
	var cloneOnly bool // also bound to --dry-run alias
	var send bool

	cmd := &cobra.Command{
		Use:   "reorder [orderId]",
		Short: "Re-create a past order as a fresh, repriced cart",
		Long: `Clones a past order via /OrderClone, then re-prices it via /OrderPrice so
the displayed total reflects current menu prices. Pass --send to actually
submit the reorder; the default is preview-only.`,
		Example: `  pagliacci-pp-cli orders reorder --last
  pagliacci-pp-cli orders reorder --last --dry-run --json
  pagliacci-pp-cli orders reorder 12345`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !useLast && len(args) == 0 {
				return usageErr(fmt.Errorf("provide an orderId or use --last"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			orderID := ""
			if useLast {
				orderID, err = findMostRecentOrderID(c)
				if err != nil {
					return classifyAPIError(err)
				}
				if orderID == "" {
					return notFoundErr(fmt.Errorf("no past orders found. Place an order first or pass --order-id"))
				}
			} else {
				orderID = args[0]
			}

			cloneRaw, err := c.Get(fmt.Sprintf("/OrderClone/%s", orderID), nil)
			if err != nil {
				return classifyAPIError(err)
			}

			if cloneOnly {
				return printOutputWithFlags(cmd.OutOrStdout(), cloneRaw, flags)
			}

			// Re-price: feed the cloned cart back through /OrderPrice.
			var body any
			if json.Unmarshal(cloneRaw, &body) != nil {
				return apiErr(fmt.Errorf("OrderClone response was not JSON"))
			}
			pricedRaw, _, err := c.Post("/OrderPrice", body)
			if err != nil {
				return classifyAPIError(err)
			}

			if send {
				sentRaw, _, sendErr := c.Post("/OrderSend", body)
				if sendErr != nil {
					return classifyAPIError(sendErr)
				}
				return printOutputWithFlags(cmd.OutOrStdout(), sentRaw, flags)
			}

			return printOutputWithFlags(cmd.OutOrStdout(), pricedRaw, flags)
		},
	}
	cmd.Flags().BoolVar(&useLast, "last", false, "Use the most recent past order")
	cmd.Flags().BoolVar(&cloneOnly, "clone-only", false, "Skip the OrderPrice revalidation; show only the OrderClone result (use this instead of --dry-run, which suppresses ALL API calls at the root level)")
	cmd.Flags().BoolVar(&send, "send", false, "Submit the reordered cart via /OrderSend")
	return cmd
}
