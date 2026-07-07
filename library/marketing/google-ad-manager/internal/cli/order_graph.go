// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/client"
)

// orderGraphLineItemNode is one line item under the order, with the fields the
// REST v1 LineItem actually exposes (goal, type, flight dates). GAM REST v1
// does not expose line-item targeting, so the graph stops at line items — the
// targeting → ad-unit linkage is SOAP-only.
type orderGraphLineItemNode struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	LineItemType string          `json:"line_item_type,omitempty"`
	Goal         json.RawMessage `json:"goal,omitempty"`
	StartTime    json.RawMessage `json:"start_time,omitempty"`
	EndTime      json.RawMessage `json:"end_time,omitempty"`
}

type orderGraphOrderNode struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	LineItems []orderGraphLineItemNode `json:"lineItems"`
}

type orderGraphView struct {
	Order orderGraphOrderNode `json:"order"`
}

// pp:data-source live -- fetches the order and its line items directly from the
// GAM API to build the expansion graph; results are not mirrored locally.
func newNovelOrderGraphCmd(flags *rootFlags) *cobra.Command {
	var flagNetwork string
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "graph <order-id>",
		Short: "Expand one order into its line items (goal, type, flight dates) in one structured object",
		Long: strings.Trim(`
Expand a single order into a nested object: the order plus each of its line
items with goal, line-item type, and flight dates.

This is a LIVE command — it reads the Ad Manager REST API directly (one GET for
the order, one filtered list for its line items).

Note: GAM REST v1 line items do not expose targeting, so this does not resolve
the ad units a line item targets — that linkage is SOAP-only.`, "\n"),
		Example:     "  google-ad-manager-pp-cli order graph 554433 --network 123456 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would expand order graph (order -> line items)")
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("order id required: graph <order-id> --network <code>"))
			}
			orderID := strings.TrimSpace(args[0])

			code, err := resolveNetworkCode(flagNetwork)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view, err := buildOrderGraph(ctx, c, code, orderID, flagLimit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}

	cmd.Flags().StringVar(&flagNetwork, "network", "", "Ad Manager network code (or set GOOGLE_AD_MANAGER_NETWORK_CODE)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "max line items to list under the order")
	cmd.Flags().StringVar(&flagDB, "db", "", "unused for this live command; accepted for flag parity with mirror commands")
	return cmd
}

// buildOrderGraph performs the live read:
//  1. GET /v1/networks/{code}/orders/{orderId}
//  2. GET /v1/networks/{code}/lineItems?filter=order = "networks/{code}/orders/{orderId}"
//
// The lineItems filter uses the `order` field (the order RESOURCE NAME) — its
// filterable fields are displayName/endTime/goal.units/lineItemType/name/order/
// startTime; there is no `orderId` field.
func buildOrderGraph(ctx context.Context, c *client.Client, code, orderID string, limit int) (orderGraphView, error) {
	parent := networkParent(code)

	orderRaw, err := c.Get(ctx, "/v1/"+parent+"/orders/"+orderID, nil)
	if err != nil {
		return orderGraphView{}, classifyAPIError(err, nil)
	}
	var order struct {
		OrderID     string `json:"orderId"`
		DisplayName string `json:"displayName"`
	}
	_ = json.Unmarshal(orderRaw, &order)

	orderNode := orderGraphOrderNode{
		ID:        firstNonEmpty(order.OrderID, orderID),
		Name:      order.DisplayName,
		LineItems: make([]orderGraphLineItemNode, 0),
	}

	orderResource := parent + "/orders/" + orderID
	liParams := map[string]string{"filter": fmt.Sprintf("order = %q", orderResource)}
	if limit > 0 {
		liParams["pageSize"] = fmt.Sprintf("%d", limit)
	}
	liRaw, err := c.Get(ctx, "/v1/"+parent+"/lineItems", liParams)
	if err != nil {
		return orderGraphView{}, classifyAPIError(err, nil)
	}
	var liResp struct {
		LineItems []json.RawMessage `json:"lineItems"`
	}
	_ = json.Unmarshal(liRaw, &liResp)

	lineItems := liResp.LineItems
	if limit > 0 && len(lineItems) > limit {
		lineItems = lineItems[:limit]
	}
	for _, raw := range lineItems {
		orderNode.LineItems = append(orderNode.LineItems, parseOrderLineItem(raw))
	}
	return orderGraphView{Order: orderNode}, nil
}

// parseOrderLineItem projects a raw GAM line item into the graph node shape.
// Pure, side-effect-free helper covered by tests. Time fields are kept as raw
// JSON so a string or object shape both pass through unchanged.
func parseOrderLineItem(raw []byte) orderGraphLineItemNode {
	var li struct {
		Name         string          `json:"name"`
		DisplayName  string          `json:"displayName"`
		LineItemType string          `json:"lineItemType"`
		Goal         json.RawMessage `json:"goal"`
		StartTime    json.RawMessage `json:"startTime"`
		EndTime      json.RawMessage `json:"endTime"`
	}
	_ = json.Unmarshal(raw, &li)
	node := orderGraphLineItemNode{
		ID:           tailSegment(li.Name),
		Name:         li.DisplayName,
		LineItemType: li.LineItemType,
	}
	if len(li.Goal) > 0 && string(li.Goal) != "null" {
		node.Goal = li.Goal
	}
	if len(li.StartTime) > 0 && string(li.StartTime) != "null" {
		node.StartTime = li.StartTime
	}
	if len(li.EndTime) > 0 && string(li.EndTime) != "null" {
		node.EndTime = li.EndTime
	}
	return node
}

// tailSegment returns the final "/"-separated segment, e.g.
// "networks/123/orders/456" -> "456". Empty input yields "".
func tailSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
