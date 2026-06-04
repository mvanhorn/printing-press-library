package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newReportInventoryHealthCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "inventory-health", Short: "Inventory stock, sales velocity, and approximate days of supply by SKU.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH sales AS (SELECT json_extract(j.value,'%s') sku, SUM(CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL)) units FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s AND json_extract(j.value,'%s') IS NOT NULL GROUP BY sku), stock AS (SELECT id, sku, tracked, COALESCE((SELECT SUM(CAST(json_extract(level.value,'%s') AS REAL)) FROM json_each(json_extract(inventory_items.data,'%s')) level),0) available FROM inventory_items) SELECT stock.sku,stock.tracked,ROUND(stock.available,2),ROUND(COALESCE(sales.units,0),2),ROUND(COALESCE(sales.units,0)/?,4),CASE WHEN COALESCE(sales.units,0)>0 THEN ROUND(stock.available/(sales.units/?),1) ELSE NULL END FROM stock LEFT JOIN sales USING(sku) ORDER BY COALESCE(sales.units,0) DESC, stock.available DESC LIMIT ?`, jsonLineItemSKU, jsonLineItemQuantity, jsonLineItems, windowClause(days), jsonLineItemSKU, jsonInventoryQty, jsonInventoryLevels)
		type row struct {
			SKU          string   `json:"sku"`
			Tracked      bool     `json:"tracked"`
			Available    float64  `json:"available"`
			UnitsSold    float64  `json:"units_sold"`
			UnitsPerDay  float64  `json:"units_per_day"`
			DaysOfSupply *float64 `json:"days_of_supply,omitempty"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			var tracked sql.NullBool
			var dos sql.NullFloat64
			err := r.Scan(&x.SKU, &tracked, &x.Available, &x.UnitsSold, &x.UnitsPerDay, &dos)
			x.Tracked = tracked.Bool
			if dos.Valid {
				x.DaysOfSupply = &dos.Float64
			}
			return x, err
		}, float64(days), float64(days), limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 50)
	return cmd
}

func newReportDeadInventoryCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "dead-inventory", Short: "Inventory items with available stock and no matching sales in the selected window.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH sales AS (SELECT DISTINCT json_extract(j.value,'%s') sku FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s AND json_extract(j.value,'%s') IS NOT NULL), stock AS (SELECT id, sku, tracked, COALESCE((SELECT SUM(CAST(json_extract(level.value,'%s') AS REAL)) FROM json_each(json_extract(inventory_items.data,'%s')) level),0) available FROM inventory_items) SELECT id,sku,tracked,ROUND(available,2) FROM stock WHERE available>0 AND sku NOT IN (SELECT sku FROM sales) ORDER BY available DESC LIMIT ?`, jsonLineItemSKU, jsonLineItems, windowClause(days), jsonLineItemSKU, jsonInventoryQty, jsonInventoryLevels)
		type row struct {
			ID        string  `json:"id"`
			SKU       string  `json:"sku"`
			Tracked   bool    `json:"tracked"`
			Available float64 `json:"available"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			var tracked sql.NullBool
			err := r.Scan(&x.ID, &x.SKU, &tracked, &x.Available)
			x.Tracked = tracked.Bool
			return x, err
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 50)
	return cmd
}
