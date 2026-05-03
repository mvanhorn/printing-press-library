// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// Order commands are read-only because TikTok Shop order data contains buyer PII.

package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

func newOrdersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "orders", Short: "TikTok Shop order operations"}
	cmd.AddCommand(newOrdersListCmd(flags))
	cmd.AddCommand(newOrdersGetCmd(flags))
	return cmd
}

func newOrdersGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <order-id>",
		Short: "Get one order with the confirmed 202309 Order API",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			q.Set("ids", args[0])
			return runOpenAPI(cmd, flags, "GET", "/order/202309/orders", q, nil)
		},
	}
}

func newOrdersListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var pageToken, sortOrder, sortField string
	var orderStatus, shippingType, buyerUserID string
	var createTimeGE, createTimeLT, updateTimeGE, updateTimeLT int64
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Search orders with the confirmed 202309 Order API",
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > 100 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 100"))
			}
			q := url.Values{}
			q.Set("page_size", strconv.Itoa(limit))
			setIf(q, "page_token", pageToken)
			setIf(q, "sort_order", sortOrder)
			setIf(q, "sort_field", sortField)

			body := map[string]any{}
			setBodyString(body, "order_status", orderStatus)
			setBodyString(body, "shipping_type", shippingType)
			setBodyString(body, "buyer_user_id", buyerUserID)
			setBodyInt(body, "create_time_ge", createTimeGE)
			setBodyInt(body, "create_time_lt", createTimeLT)
			setBodyInt(body, "update_time_ge", updateTimeGE)
			setBodyInt(body, "update_time_lt", updateTimeLT)

			return runOpenAPI(cmd, flags, "POST", "/order/202309/orders/search", q, body)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Orders per page, official range 1-100")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Opaque next page token")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "", "Optional sort order: ASC or DESC")
	cmd.Flags().StringVar(&sortField, "sort-field", "", "Optional sort field: create_time or update_time")
	cmd.Flags().StringVar(&orderStatus, "status", "", "Optional order status filter")
	cmd.Flags().StringVar(&shippingType, "shipping-type", "", "Optional shipping type filter: TIKTOK, SELLER, or TIKTOK_DIGITAL")
	cmd.Flags().StringVar(&buyerUserID, "buyer-user-id", "", "Optional buyer user ID filter; treat as PII")
	cmd.Flags().Int64Var(&createTimeGE, "create-time-ge", 0, "Filter created at or after Unix timestamp")
	cmd.Flags().Int64Var(&createTimeLT, "create-time-lt", 0, "Filter created before Unix timestamp")
	cmd.Flags().Int64Var(&updateTimeGE, "update-time-ge", 0, "Filter updated at or after Unix timestamp")
	cmd.Flags().Int64Var(&updateTimeLT, "update-time-lt", 0, "Filter updated before Unix timestamp")
	return cmd
}

func setBodyString(body map[string]any, key, value string) {
	if value != "" {
		body[key] = value
	}
}

func setBodyInt(body map[string]any, key string, value int64) {
	if value > 0 {
		body[key] = value
	}
}
