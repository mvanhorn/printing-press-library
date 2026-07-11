// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source live

// ordersBriefQuery joins order + fulfillment status + line-item inventory
// availability in one GraphQL call so a support agent never has to stitch
// three separate tool calls together to answer "what's the status of order
// #1234".
const ordersBriefQuery = `query($id: ID!) {
  order(id: $id) {
    id
    name
    displayFinancialStatus
    displayFulfillmentStatus
    createdAt
    processedAt
    totalPriceSet { shopMoney { amount currencyCode } }
    customer { id email displayName }
    fulfillmentOrders(first: 10) {
      nodes {
        id
        status
        requestStatus
        assignedLocation { name }
      }
    }
    lineItems(first: 50) {
      nodes {
        id
        title
        quantity
        variant {
          id
          sku
          inventoryItem {
            id
            tracked
            inventoryLevels(first: 5) {
              nodes {
                location { name }
                quantities(names: ["available"]) { name quantity }
              }
            }
          }
        }
      }
    }
  }
}`

func newNovelOrdersBriefCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "brief <id>",
		Short:       "See a single support-ready summary of an order with fulfillment status and inventory availability",
		Long:        "Use this command when the operator/agent wants a single support-ready summary of an order including fulfillment and inventory-availability context. Do NOT use this command for the raw order object; use 'orders get' instead.",
		Example:     "  shopify-pp-cli orders brief gid://shopify/Order/1234 --agent --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch order brief")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Query(ctx, ordersBriefQuery, map[string]any{"id": args[0]})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope struct {
				Order json.RawMessage `json:"order"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing order brief response: %w", err)
			}
			if envelope.Order == nil || string(envelope.Order) == "null" {
				return notFoundErr(fmt.Errorf("order %q not found", args[0]))
			}

			if flags.asJSON || wantsMachineOutput(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), envelope.Order, flags)
			}

			var pretty map[string]any
			if err := json.Unmarshal(envelope.Order, &pretty); err != nil {
				// Fall back to raw JSON if the order shape didn't parse as a
				// plain object for some reason.
				return printJSONFiltered(cmd.OutOrStdout(), envelope.Order, flags)
			}
			name, _ := pretty["name"].(string)
			finStatus, _ := pretty["displayFinancialStatus"].(string)
			fulfStatus, _ := pretty["displayFulfillmentStatus"].(string)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\tfinancial=%s\tfulfillment=%s\n", name, finStatus, fulfStatus)
			return nil
		},
	}
	return cmd
}
