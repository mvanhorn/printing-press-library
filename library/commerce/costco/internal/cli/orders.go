package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// onlineOrdersQuery mirrors the live getOnlineOrders request captured from
// costco.com. warehouseNumber is required by the schema; it is the member's
// home/context warehouse.
const onlineOrdersQuery = `query getOnlineOrders($startDate: String!, $endDate: String!, $pageNumber: Int, $pageSize: Int, $warehouseNumber: String!) {
  getOnlineOrders(startDate: $startDate, endDate: $endDate, pageNumber: $pageNumber, pageSize: $pageSize, warehouseNumber: $warehouseNumber) {
    pageNumber
    pageSize
    totalNumberOfRecords
    bcOrders {
      orderHeaderId
      orderedDate
      sourceOrderNumber
      orderTotal
      warehouseNumber
      status
      emailAddress
      orderLineItems {
        itemId
        itemNumber
        itemDescription
        status
        deliveryDate
      }
    }
  }
}`

type onlineLineItem struct {
	ItemID          string `json:"itemId"`
	ItemNumber      string `json:"itemNumber"`
	ItemDescription string `json:"itemDescription"`
	Status          string `json:"status"`
	DeliveryDate    string `json:"deliveryDate"`
}

type onlineOrder struct {
	OrderHeaderID     string           `json:"orderHeaderId"`
	OrderedDate       string           `json:"orderedDate"`
	SourceOrderNumber string           `json:"sourceOrderNumber"`
	OrderTotal        num              `json:"orderTotal"`
	WarehouseNumber   string           `json:"warehouseNumber"`
	Status            string           `json:"status"`
	OrderLineItems    []onlineLineItem `json:"orderLineItems"`
}

type onlineOrdersEnvelope struct {
	Data struct {
		GetOnlineOrders struct {
			PageNumber           int           `json:"pageNumber"`
			PageSize             int           `json:"pageSize"`
			TotalNumberOfRecords int           `json:"totalNumberOfRecords"`
			BCOrders             []onlineOrder `json:"bcOrders"`
		} `json:"getOnlineOrders"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// orderSummary is the flat row emitted by `orders`.
type orderSummary struct {
	OrderNumber string  `json:"orderNumber"`
	OrderedDate string  `json:"orderedDate"`
	Status      string  `json:"status"`
	ItemCount   int     `json:"itemCount"`
	OrderTotal  float64 `json:"orderTotal"`
}

func fetchOnlineOrdersPage(ctx context.Context, flags *rootFlags, start, end, warehouse string, page, pageSize int) (onlineOrdersEnvelope, error) {
	c, err := flags.newClient()
	var env onlineOrdersEnvelope
	if err != nil {
		return env, err
	}
	body := map[string]any{
		"query": onlineOrdersQuery,
		"variables": map[string]any{
			"startDate":       start,
			"endDate":         end,
			"pageNumber":      page,
			"pageSize":        pageSize,
			"warehouseNumber": warehouse,
		},
	}
	data, _, err := c.PostQueryWithParams(ctx, costcoGraphQLPath, nil, body)
	if err != nil {
		return env, err
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return env, fmt.Errorf("decoding online orders response: %w", err)
	}
	if len(env.Errors) > 0 {
		return env, fmt.Errorf("costco API error: %s", env.Errors[0].Message)
	}
	return env, nil
}

func newOrdersCmd(flags *rootFlags) *cobra.Command {
	var since, until, warehouse string
	var years, pageSize, maxPages int
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "List online costco.com orders for a date range",
		Long: strings.Trim(`
List online costco.com orders (not in-warehouse receipts) for a date range.

The online-orders API requires a home warehouse number (--warehouse); it scopes
delivery context, not which orders are returned. Find yours on a receipt
(warehouseNumber) or in 'receipts' output. Use 'receipts' for in-warehouse
purchases.`, "\n"),
		Example:     "  costco-pp-cli orders --warehouse 847 --years 2 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch online orders for the requested range")
				return nil
			}
			if strings.TrimSpace(warehouse) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--warehouse <number> is required (your home warehouse; see warehouseNumber on any receipt)"))
			}
			start, end, err := resolveRange(since, until, years)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if pageSize <= 0 {
				pageSize = 10
			}
			if maxPages <= 0 {
				maxPages = 10
			}
			rows := []orderSummary{}
			truncated := true
			for page := 1; page <= maxPages; page++ {
				env, err := fetchOnlineOrdersPage(ctx, flags, start, end, warehouse, page, pageSize)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				got := env.Data.GetOnlineOrders.BCOrders
				for _, o := range got {
					rows = append(rows, orderSummary{
						OrderNumber: o.SourceOrderNumber,
						OrderedDate: o.OrderedDate,
						Status:      o.Status,
						ItemCount:   len(o.OrderLineItems),
						OrderTotal:  o.OrderTotal.float(),
					})
				}
				total := env.Data.GetOnlineOrders.TotalNumberOfRecords
				if len(got) == 0 || len(got) < pageSize || (total > 0 && page*pageSize >= total) {
					truncated = false
					break
				}
			}
			if truncated && len(rows) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetched %d pages (%d orders); more may exist — increase --max-pages to fetch all\n", maxPages, len(rows))
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].OrderedDate > rows[j].OrderedDate })
			b, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start of range (YYYY-MM-DD or duration)")
	cmd.Flags().StringVar(&until, "until", "", "End of range (YYYY-MM-DD; default today)")
	cmd.Flags().IntVar(&years, "years", 2, "Lookback in years when --since is not set")
	cmd.Flags().StringVar(&warehouse, "warehouse", "", "Home warehouse number (required; see warehouseNumber on a receipt)")
	cmd.Flags().IntVar(&pageSize, "page-size", 10, "Online orders per page")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum pages to fetch")
	return cmd
}
