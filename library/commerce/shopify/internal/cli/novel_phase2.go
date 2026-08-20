package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	jsonShippingAddressCountry  = "$.shippingAddress.countryCode"
	jsonShippingAddressProvince = "$.shippingAddress.provinceCode"
	jsonShippingAddressCity     = "$.shippingAddress.city"
	jsonShippingLines           = "$.shippingLines.nodes"
	jsonShippingLineTitle       = "$.title"
	jsonShippingLineAmount      = "$.originalPriceSet.shopMoney.amount"
)

func addDBOverrideFlag(cmd *cobra.Command, flags *rootFlags) {
	cmd.PersistentFlags().StringVar(&flags.reportDBPath, "db", "", "SQLite store path override for local analytics commands")
}

func newGrowthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "growth", Short: "Novel Shopify growth workflows from the local store.", RunE: parentNoSubcommandRunE(flags)}
	addDBOverrideFlag(cmd, flags)
	cmd.AddCommand(newGrowthWinbackCandidatesCmd(flags))
	cmd.AddCommand(newGrowthVIPSegmentsCmd(flags))
	cmd.AddCommand(newGrowthCampaignBriefCmd(flags))
	return cmd
}

func newOpsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "ops", Short: "Novel Shopify operations risk workflows from the local store.", RunE: parentNoSubcommandRunE(flags)}
	addDBOverrideFlag(cmd, flags)
	cmd.AddCommand(newOpsFulfillmentRiskCmd(flags))
	cmd.AddCommand(newOpsShippingAnomaliesCmd(flags))
	return cmd
}

func newMerchandisingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "merchandising", Short: "Novel Shopify merchandising workflows from the local store.", RunE: parentNoSubcommandRunE(flags)}
	addDBOverrideFlag(cmd, flags)
	cmd.AddCommand(newMerchandisingBundleOpportunitiesCmd(flags))
	cmd.AddCommand(newMerchandisingDeadStockActionsCmd(flags))
	cmd.AddCommand(newMerchandisingLaunchBriefCmd(flags))
	return cmd
}

func newStoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "store", Short: "Novel Shopify store health briefs from the local store.", RunE: parentNoSubcommandRunE(flags)}
	addDBOverrideFlag(cmd, flags)
	cmd.AddCommand(newStoreDailyBriefCmd(flags))
	cmd.AddCommand(newStoreAuditCmd(flags))
	return cmd
}

func newGrowthWinbackCandidatesCmd(flags *rootFlags) *cobra.Command {
	var idleDays, limit int
	cmd := &cobra.Command{Use: "winback-candidates", Short: "Rank customers whose last order is older than --idle-days for winback outreach.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		idleDays = normalizeDays(idleDays)
		limit = clampLimit(limit)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		cutoff := time.Now().AddDate(0, 0, -idleDays).UTC().Format(time.RFC3339)
		type row struct {
			CustomerID     string  `json:"customer_id"`
			Email          string  `json:"email"`
			LastOrderAt    string  `json:"last_order_at"`
			IdleDays       float64 `json:"idle_days"`
			Orders         int     `json:"orders"`
			LifetimeValue  float64 `json:"lifetime_value"`
			SuggestedAngle string  `json:"suggested_angle"`
		}
		rows, err := queryRows(db.DB(), fmt.Sprintf(`
			WITH customers AS (
				SELECT json_extract(data,'%s') AS customer_id,
				       MAX(json_extract(data,'%s')) AS email,
				       MAX(created_at) AS last_order_at,
				       COUNT(*) AS orders,
				       ROUND(SUM(CAST(json_extract(data,'%s') AS REAL)),2) AS ltv
				FROM orders
				WHERE json_extract(data,'%s') IS NOT NULL
				GROUP BY customer_id
			)
			SELECT customer_id, COALESCE(email,''), last_order_at,
			       ROUND(julianday('now') - julianday(last_order_at), 1) AS idle_days,
			       orders, ltv
			FROM customers
			WHERE last_order_at <= ?
			ORDER BY ltv DESC, idle_days DESC
			LIMIT ?`, jsonCustomerID, jsonCustomerEmail, jsonTotalAmount, jsonCustomerID), func(rows *sql.Rows) (row, error) {
			var r row
			if err := rows.Scan(&r.CustomerID, &r.Email, &r.LastOrderAt, &r.IdleDays, &r.Orders, &r.LifetimeValue); err != nil {
				return r, err
			}
			switch {
			case r.LifetimeValue >= 200:
				r.SuggestedAngle = "VIP winback: early access or bundle credit"
			case r.Orders > 1:
				r.SuggestedAngle = "Repeat buyer winback: replenish or new-arrivals nudge"
			default:
				r.SuggestedAngle = "First-repeat prompt: bestseller or education sequence"
			}
			return r, nil
		}, cutoff, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(rows), flags)
	}}
	cmd.Flags().IntVar(&idleDays, "idle-days", 60, "Minimum days since last order")
	addLimitFlag(cmd, &limit, 100)
	return cmd
}

func newGrowthVIPSegmentsCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "vip-segments", Short: "Segment high-value customers by spend, frequency, and recency.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		days = normalizeDays(days)
		limit = clampLimit(limit)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		type row struct {
			CustomerID  string  `json:"customer_id"`
			Email       string  `json:"email"`
			Orders      int     `json:"orders"`
			Revenue     float64 `json:"revenue"`
			RecencyDays float64 `json:"recency_days"`
			Segment     string  `json:"segment"`
			Recommended string  `json:"recommended_action"`
		}
		q := fmt.Sprintf(`
			WITH c AS (
				SELECT json_extract(data,'%s') AS customer_id,
				       MAX(json_extract(data,'%s')) AS email,
				       COUNT(*) AS orders,
				       ROUND(SUM(CAST(json_extract(data,'%s') AS REAL)),2) AS revenue,
				       ROUND(julianday('now') - julianday(MAX(created_at)),1) AS recency_days
				FROM orders
				WHERE %s AND json_extract(data,'%s') IS NOT NULL
				GROUP BY customer_id
			)
			SELECT customer_id, COALESCE(email,''), orders, revenue, recency_days,
			       CASE WHEN revenue >= 200 OR orders >= 3 THEN 'vip'
			            WHEN orders >= 2 THEN 'loyal'
			            WHEN recency_days <= 30 THEN 'new_recent'
			            ELSE 'standard' END AS segment
			FROM c
			ORDER BY revenue DESC, orders DESC
			LIMIT ?`, jsonCustomerID, jsonCustomerEmail, jsonTotalAmount, windowClause(days), jsonCustomerID)
		rows, err := queryRows(db.DB(), q, func(rows *sql.Rows) (row, error) {
			var r row
			if err := rows.Scan(&r.CustomerID, &r.Email, &r.Orders, &r.Revenue, &r.RecencyDays, &r.Segment); err != nil {
				return r, err
			}
			switch r.Segment {
			case "vip":
				r.Recommended = "Invite to VIP early access, bundle testing, or concierge offer"
			case "loyal":
				r.Recommended = "Cross-sell best bundle based on past product category"
			case "new_recent":
				r.Recommended = "Send first-repeat education and bestseller sequence"
			default:
				r.Recommended = "Keep in general campaign pool"
			}
			return r, nil
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(rows), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	addLimitFlag(cmd, &limit, 100)
	return cmd
}

func newGrowthCampaignBriefCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "campaign-brief", Short: "Generate a data-backed growth campaign brief from local Shopify behavior.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		days = normalizeDays(days)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		var revenue sql.NullFloat64
		var orders int
		if err := db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2) FROM orders WHERE %s`, jsonTotalAmount, windowClause(days))).Scan(&orders, &revenue); err != nil {
			return err
		}
		topProducts, err := topProductRows(db.DB(), days, 3)
		if err != nil {
			return err
		}
		plays := []map[string]any{
			{"play": "VIP / high-LTV early access", "why": "Protect and expand revenue from best customers", "command": "shopify-pp-cli growth vip-segments --json"},
			{"play": "Winback idle buyers", "why": "Recover customers whose last order is aging out", "command": "shopify-pp-cli growth winback-candidates --idle-days 60 --json"},
		}
		if len(topProducts) > 0 {
			plays = append(plays, map[string]any{"play": "Hero-product bundle", "why": fmt.Sprintf("%s is a current top product", topProducts[0]["product"]), "command": "shopify-pp-cli merchandising bundle-opportunities --json"})
		}
		out := map[string]any{"days": days, "orders": orders, "revenue": round2(revenue.Float64), "top_products": topProducts, "plays": plays}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	return cmd
}

func newOpsFulfillmentRiskCmd(flags *rootFlags) *cobra.Command {
	var hours, limit int
	cmd := &cobra.Command{Use: "fulfillment-risk", Short: "Find open fulfillment orders older than --hours.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if hours <= 0 {
			hours = 24
		}
		limit = clampLimit(limit)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339)
		type row struct {
			FulfillmentOrderID string  `json:"fulfillment_order_id"`
			Status             string  `json:"status"`
			RequestStatus      string  `json:"request_status"`
			CreatedAt          string  `json:"created_at"`
			AgeHours           float64 `json:"age_hours"`
			Risk               string  `json:"risk"`
		}
		rows, err := queryRows(db.DB(), `
			SELECT id, COALESCE(status,''), COALESCE(request_status,''), created_at,
			       ROUND((julianday('now') - julianday(created_at)) * 24, 1) AS age_hours
			FROM fulfillment_orders
			WHERE created_at <= ?
			  AND UPPER(COALESCE(status,'')) NOT IN ('CLOSED','CANCELLED','CANCELED')
			  AND UPPER(COALESCE(request_status,'')) NOT IN ('FULFILLED','CLOSED','CANCELLED','CANCELED')
			ORDER BY age_hours DESC
			LIMIT ?`, func(rows *sql.Rows) (row, error) {
			var r row
			if err := rows.Scan(&r.FulfillmentOrderID, &r.Status, &r.RequestStatus, &r.CreatedAt, &r.AgeHours); err != nil {
				return r, err
			}
			if r.AgeHours >= 72 {
				r.Risk = "critical"
			} else {
				r.Risk = "watch"
			}
			return r, nil
		}, cutoff, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(rows), flags)
	}}
	cmd.Flags().IntVar(&hours, "hours", 24, "Minimum fulfillment age in hours")
	addLimitFlag(cmd, &limit, 100)
	return cmd
}

func newOpsShippingAnomaliesCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "shipping-anomalies", Short: "Find orders with unusual shipping charges or missing shipping revenue.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		days = normalizeDays(days)
		limit = clampLimit(limit)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		type row struct {
			OrderID        string  `json:"order_id"`
			OrderName      string  `json:"order_name"`
			Country        string  `json:"country"`
			Province       string  `json:"province"`
			OrderTotal     float64 `json:"order_total"`
			ShippingAmount float64 `json:"shipping_amount"`
			ShippingPct    float64 `json:"shipping_pct"`
			Reason         string  `json:"reason"`
		}
		q := fmt.Sprintf(`
			WITH shipping AS (
				SELECT orders.id, orders.name,
				       COALESCE(json_extract(orders.data,'%s'),'') AS country,
				       COALESCE(json_extract(orders.data,'%s'),'') AS province,
				       CAST(json_extract(orders.data,'%s') AS REAL) AS order_total,
				       ROUND(COALESCE(SUM(CAST(json_extract(sl.value,'%s') AS REAL)),0),2) AS shipping_amount
				FROM orders
				LEFT JOIN json_each(json_extract(orders.data,'%s')) AS sl
				WHERE %s
				GROUP BY orders.id
			)
			SELECT id, name, country, province, order_total, shipping_amount,
			       ROUND(CASE WHEN order_total > 0 THEN shipping_amount / order_total * 100 ELSE 0 END,2) AS shipping_pct
			FROM shipping
			WHERE shipping_amount = 0 OR (order_total > 0 AND shipping_amount / order_total >= 0.25) OR shipping_amount >= 50
			ORDER BY shipping_pct DESC, shipping_amount DESC
			LIMIT ?`, jsonShippingAddressCountry, jsonShippingAddressProvince, jsonTotalAmount, jsonShippingLineAmount, jsonShippingLines, windowClause(days))
		rows, err := queryRows(db.DB(), q, func(rows *sql.Rows) (row, error) {
			var r row
			if err := rows.Scan(&r.OrderID, &r.OrderName, &r.Country, &r.Province, &r.OrderTotal, &r.ShippingAmount, &r.ShippingPct); err != nil {
				return r, err
			}
			if r.ShippingAmount == 0 {
				r.Reason = "free_or_missing_shipping"
			} else {
				r.Reason = "high_shipping_ratio"
			}
			return r, nil
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(rows), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	addLimitFlag(cmd, &limit, 100)
	return cmd
}

func newStoreDailyBriefCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "daily-brief", Short: "Executive store brief with metrics, top products, and suggested next actions.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		days = normalizeDays(days)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		var orders int
		var revenue, refunds sql.NullFloat64
		if err := db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2) FROM orders WHERE %s`, jsonTotalAmount, jsonRefundAmount, windowClause(days))).Scan(&orders, &revenue, &refunds); err != nil {
			return err
		}
		topProducts, err := topProductRows(db.DB(), days, 5)
		if err != nil {
			return err
		}
		actions := []string{"Review fulfillment risk queue", "Build campaign around top product", "Check high shipping-ratio orders"}
		out := map[string]any{"days": days, "summary": map[string]any{"orders": orders, "revenue": round2(revenue.Float64), "refunds": round2(refunds.Float64)}, "top_products": topProducts, "recommended_actions": actions, "note": lineItemCapNote}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	return cmd
}

func newStoreAuditCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "audit", Short: "Score store health from refunds, fulfillment risk, shipping anomalies, and dead stock.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		days = normalizeDays(days)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		var orders, riskyFulfillments, deadStock, shippingAnomalies int
		var revenue, refunds sql.NullFloat64
		if err := db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2) FROM orders WHERE %s`, jsonTotalAmount, jsonRefundAmount, windowClause(days))).Scan(&orders, &revenue, &refunds); err != nil {
			return err
		}
		if err := db.DB().QueryRow(`SELECT COUNT(*) FROM fulfillment_orders WHERE UPPER(COALESCE(status,'')) NOT IN ('CLOSED','CANCELLED','CANCELED') AND UPPER(COALESCE(request_status,'')) NOT IN ('FULFILLED','CLOSED','CANCELLED','CANCELED') AND created_at <= ?`, time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339)).Scan(&riskyFulfillments); err != nil {
			return err
		}
		if err := db.DB().QueryRow(fmt.Sprintf(`
			WITH sold AS (SELECT DISTINCT json_extract(li.value,'%s') AS sku FROM orders, json_each(json_extract(orders.data,'%s')) li WHERE %s)
			SELECT COUNT(*) FROM inventory_items inv WHERE COALESCE((SELECT SUM(CAST(json_extract(level.value,'%s') AS REAL)) FROM json_each(json_extract(inv.data,'%s')) level),0) > 0 AND inv.sku NOT IN (SELECT sku FROM sold WHERE sku IS NOT NULL)`, jsonLineItemSKU, jsonLineItems, windowClause(days), jsonInventoryQty, jsonInventoryLevels)).Scan(&deadStock); err != nil {
			return err
		}
		if err := db.DB().QueryRow(fmt.Sprintf(`
			WITH shipping AS (
				SELECT orders.id,
				       CAST(json_extract(orders.data,'%s') AS REAL) AS order_total,
				       ROUND(COALESCE(SUM(CAST(json_extract(sl.value,'%s') AS REAL)),0),2) AS shipping_amount
				FROM orders
				LEFT JOIN json_each(json_extract(orders.data,'%s')) AS sl
				WHERE %s
				GROUP BY orders.id
			)
			SELECT COUNT(*) FROM shipping
			WHERE shipping_amount = 0 OR (order_total > 0 AND shipping_amount / order_total >= 0.25) OR shipping_amount >= 50`, jsonTotalAmount, jsonShippingLineAmount, jsonShippingLines, windowClause(days))).Scan(&shippingAnomalies); err != nil {
			return err
		}
		refundRate := 0.0
		if revenue.Float64 > 0 {
			refundRate = round2(refunds.Float64 / revenue.Float64 * 100)
		}
		score := 100.0
		if refundRate > 5 {
			score -= 15
		}
		if riskyFulfillments > 0 {
			score -= 20
		}
		if deadStock > 0 {
			score -= 10
		}
		if shippingAnomalies > 0 {
			score -= 10
		}
		checks := []map[string]any{
			{"check": "refund_rate", "value": refundRate, "status": statusFor(refundRate <= 5)},
			{"check": "fulfillment_risk", "value": riskyFulfillments, "status": statusFor(riskyFulfillments == 0)},
			{"check": "shipping_anomalies", "value": shippingAnomalies, "status": statusFor(shippingAnomalies == 0)},
			{"check": "dead_stock", "value": deadStock, "status": statusFor(deadStock == 0)},
		}
		out := map[string]any{"days": days, "score": round2(score), "orders": orders, "revenue": round2(revenue.Float64), "checks": checks}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	return cmd
}

func newMerchandisingBundleOpportunitiesCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "bundle-opportunities", Short: "Suggest product bundles from co-purchase lift and confidence.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		days = normalizeDays(days)
		limit = clampLimit(limit)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		type row struct {
			ProductA   string  `json:"product_a"`
			ProductB   string  `json:"product_b"`
			Orders     int     `json:"orders"`
			Confidence float64 `json:"confidence_pct"`
			Lift       float64 `json:"lift"`
			Action     string  `json:"suggested_action"`
		}
		nameExpr := productNameExpr("li.value")
		idExpr := productIDExpr("li.value")
		q := fmt.Sprintf(`
			WITH order_products AS (
				SELECT orders.id AS order_id, %s AS product_id, %s AS product
				FROM orders, json_each(json_extract(orders.data,'%s')) li
				WHERE %s
				GROUP BY orders.id, product_id
			), product_counts AS (
				SELECT product_id, product, COUNT(*) AS product_orders FROM order_products GROUP BY product_id, product
			), total_orders AS (SELECT COUNT(DISTINCT order_id) AS total_orders FROM order_products), pairs AS (
				SELECT a.product_id AS a_id, a.product AS a_product, b.product_id AS b_id, b.product AS b_product, COUNT(*) AS support
				FROM order_products a JOIN order_products b ON a.order_id=b.order_id AND a.product_id < b.product_id
				GROUP BY a_id, b_id
			)
			SELECT a_product, b_product, support,
			       ROUND(100.0 * support / pc.product_orders, 2) AS confidence_pct,
			       ROUND(1.0 * support * total_orders.total_orders / (pc.product_orders * pc2.product_orders), 2) AS lift
			FROM pairs
			JOIN product_counts pc ON pc.product_id = a_id
			JOIN product_counts pc2 ON pc2.product_id = b_id
			CROSS JOIN total_orders
			ORDER BY lift DESC, support DESC
			LIMIT ?`, idExpr, nameExpr, jsonLineItems, windowClause(days))
		rows, err := queryRows(db.DB(), q, func(rows *sql.Rows) (row, error) {
			var r row
			if err := rows.Scan(&r.ProductA, &r.ProductB, &r.Orders, &r.Confidence, &r.Lift); err != nil {
				return r, err
			}
			r.Action = "Test bundle or cart upsell"
			return r, nil
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(rows), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 50)
	return cmd
}

func newMerchandisingDeadStockActionsCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "dead-stock-actions", Short: "Turn dead inventory into concrete markdown/bundle actions.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		days = normalizeDays(days)
		limit = clampLimit(limit)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		type row struct {
			SKU       string `json:"sku"`
			Available int    `json:"available"`
			Action    string `json:"suggested_action"`
		}
		q := fmt.Sprintf(`
			WITH sold AS (SELECT DISTINCT json_extract(li.value,'%s') AS sku FROM orders, json_each(json_extract(orders.data,'%s')) li WHERE %s),
			stock AS (
				SELECT inv.sku, COALESCE((SELECT SUM(CAST(json_extract(level.value,'%s') AS REAL)) FROM json_each(json_extract(inv.data,'%s')) level),0) AS available
				FROM inventory_items inv
			)
			SELECT sku, available
			FROM stock
			WHERE available > 0 AND sku NOT IN (SELECT sku FROM sold WHERE sku IS NOT NULL)
			ORDER BY available DESC
			LIMIT ?`, jsonLineItemSKU, jsonLineItems, windowClause(days), jsonInventoryQty, jsonInventoryLevels)
		rows, err := queryRows(db.DB(), q, func(rows *sql.Rows) (row, error) {
			var r row
			if err := rows.Scan(&r.SKU, &r.Available); err != nil {
				return r, err
			}
			r.Action = "Bundle with a top seller or mark down before replenishing"
			return r, nil
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(rows), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 50)
	return cmd
}

func newMerchandisingLaunchBriefCmd(flags *rootFlags) *cobra.Command {
	var days int
	var product string
	cmd := &cobra.Command{Use: "launch-brief", Short: "Build a product launch or relaunch brief from sales, inventory, and affinity data.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(product) == "" {
			return usageErr(fmt.Errorf("--product is required"))
		}
		days = normalizeDays(days)
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		nameExpr := productNameExpr("li.value")
		var units int
		var revenue sql.NullFloat64
		if err := db.DB().QueryRow(fmt.Sprintf(`
			SELECT COALESCE(SUM(CAST(json_extract(li.value,'%s') AS INTEGER)),0),
			       ROUND(COALESCE(SUM(CAST(json_extract(li.value,'%s') AS INTEGER) * CAST(json_extract(li.value,'%s') AS REAL)),0),2)
			FROM orders, json_each(json_extract(orders.data,'%s')) li
			WHERE %s AND LOWER(%s) LIKE LOWER(?)`, jsonLineItemQuantity, jsonLineItemQuantity, jsonLineItemUnitAmount, jsonLineItems, windowClause(days), nameExpr), "%"+product+"%").Scan(&units, &revenue); err != nil {
			return err
		}
		out := map[string]any{
			"product":           product,
			"days":              days,
			"units":             units,
			"revenue":           round2(revenue.Float64),
			"suggested_actions": []string{"Position against current top bundle partner", "Check inventory before launch", "Use winback audience if prior buyers are idle"},
			"note":              lineItemCapNote,
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	cmd.Flags().StringVar(&product, "product", "", "Product title substring")
	addDaysFlag(cmd, &days, 90)
	return cmd
}

func topProductRows(db *sql.DB, days, limit int) ([]map[string]any, error) {
	nameExpr := productNameExpr("li.value")
	q := fmt.Sprintf(`
		SELECT %s AS product,
		       SUM(CAST(json_extract(li.value,'%s') AS INTEGER)) AS units,
		       ROUND(SUM(CAST(json_extract(li.value,'%s') AS INTEGER) * CAST(json_extract(li.value,'%s') AS REAL)),2) AS revenue
		FROM orders, json_each(json_extract(orders.data,'%s')) li
		WHERE %s
		GROUP BY product
		ORDER BY revenue DESC
		LIMIT ?`, nameExpr, jsonLineItemQuantity, jsonLineItemQuantity, jsonLineItemUnitAmount, jsonLineItems, windowClause(days))
	return queryRows(db, q, func(rows *sql.Rows) (map[string]any, error) {
		var product string
		var units int
		var revenue sql.NullFloat64
		if err := rows.Scan(&product, &units, &revenue); err != nil {
			return nil, err
		}
		return map[string]any{"product": product, "units": units, "revenue": round2(revenue.Float64)}, nil
	}, limit)
}

func statusFor(ok bool) string {
	if ok {
		return "ok"
	}
	return "review"
}
