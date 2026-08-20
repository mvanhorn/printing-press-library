package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	jsonTotalAmount        = "$.totalPriceSet.shopMoney.amount"
	jsonTotalCurrency      = "$.totalPriceSet.shopMoney.currencyCode"
	jsonRefundAmount       = "$.totalRefundedSet.shopMoney.amount"
	jsonCustomerID         = "$.customer.id"
	jsonCustomerEmail      = "$.customer.email"
	jsonLineItems          = "$.lineItems.nodes"
	jsonLineItemID         = "$.id"
	jsonLineItemTitle      = "$.title"
	jsonLineItemQuantity   = "$.quantity"
	jsonLineItemSKU        = "$.variant.sku"
	jsonLineItemProductID  = "$.variant.product.id"
	jsonLineItemProduct    = "$.variant.product.title"
	jsonLineItemUnitAmount = "$.originalUnitPriceSet.shopMoney.amount"
	jsonDiscountApps       = "$.discountApplications.nodes"
	jsonTags               = "$.tags"
	jsonInventoryLevels    = "$.inventoryLevels.nodes"
	jsonInventoryQty       = "$.quantities[0].quantity"
)

const lineItemCapNote = "Orders with more than 50 synced line items may be undercounted because Shopify GraphQL lineItems is capped at first:50."

func addDaysFlag(cmd *cobra.Command, ptr *int, def int) {
	cmd.Flags().IntVar(ptr, "days", def, "Window in days")
}
func addLimitFlag(cmd *cobra.Command, ptr *int, def int) {
	cmd.Flags().IntVar(ptr, "limit", def, "Maximum rows to return")
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeDays(days int) int {
	if days <= 0 {
		return 30
	}
	return days
}

func scanRows[T any](rows *sql.Rows, scan func(*sql.Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func queryRows[T any](db *sql.DB, q string, scan func(*sql.Rows) (T, error), args ...any) ([]T, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return scanRows(rows, scan)
}

func containsTagExpr(alias string) string {
	switch strings.TrimSpace(alias) {
	case "", "orders":
		alias = "orders"
	case "j.value":
		alias = "j.value"
	default:
		// Only hardcoded SQL identifiers used by this package are accepted.
		// User input must never choose a table/value alias.
		alias = "orders"
	}
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM json_each(json_extract(%s.data, '%s')) WHERE LOWER(value) LIKE LOWER(?))`, alias, jsonTags)
}

func productNameExpr(valueAlias string) string {
	return fmt.Sprintf(`COALESCE(NULLIF(json_extract(%s, '%s'), ''), NULLIF(json_extract(%s, '%s'), ''), NULLIF(json_extract(%s, '%s'), ''), '(unknown)')`, valueAlias, jsonLineItemProduct, valueAlias, jsonLineItemTitle, valueAlias, jsonLineItemSKU)
}

func productIDExpr(valueAlias string) string {
	return fmt.Sprintf(`COALESCE(NULLIF(json_extract(%s, '%s'), ''), NULLIF(json_extract(%s, '%s'), ''), NULLIF(json_extract(%s, '%s'), ''), json_extract(%s, '%s'))`, valueAlias, jsonLineItemProductID, valueAlias, jsonLineItemSKU, valueAlias, jsonLineItemTitle, valueAlias, jsonLineItemID)
}
