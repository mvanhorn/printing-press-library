package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newReportOrderTrendsCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "order-trends", Short: "Daily order/revenue trend with 7-day rolling averages.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`WITH daily AS (
			SELECT date(created_at) day, COUNT(*) orders, ROUND(COALESCE(SUM(CAST(json_extract(data, '%s') AS REAL)),0),2) revenue
			FROM orders WHERE %s GROUP BY day)
			SELECT day, orders, revenue, ROUND(AVG(revenue) OVER (ORDER BY day ROWS BETWEEN 6 PRECEDING AND CURRENT ROW),2), ROUND(AVG(orders) OVER (ORDER BY day ROWS BETWEEN 6 PRECEDING AND CURRENT ROW),2)
			FROM daily ORDER BY day DESC`, jsonTotalAmount, windowClause(days))
		type row struct {
			Day              string  `json:"day"`
			Orders           int     `json:"orders"`
			Revenue          float64 `json:"revenue"`
			RollingRevenue7d float64 `json:"rolling_revenue_7d"`
			RollingOrders7d  float64 `json:"rolling_orders_7d"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.Day, &x.Orders, &x.Revenue, &x.RollingRevenue7d, &x.RollingOrders7d)
		})
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	return cmd
}

func newReportAOVAnalysisCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "aov-analysis", Short: "Average order value overall and by source_name.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`SELECT COALESCE(NULLIF(source_name,''),'(unknown)'), COUNT(*), ROUND(AVG(CAST(json_extract(data,'%s') AS REAL)),2), ROUND(SUM(CAST(json_extract(data,'%s') AS REAL)),2), MAX(json_extract(data,'%s')) FROM orders WHERE %s GROUP BY 1 ORDER BY 4 DESC`, jsonTotalAmount, jsonTotalAmount, jsonTotalCurrency, windowClause(days))
		type channel struct {
			Source   string  `json:"source"`
			Orders   int     `json:"orders"`
			AOV      float64 `json:"aov"`
			Revenue  float64 `json:"revenue"`
			Currency string  `json:"currency"`
		}
		channels, err := queryRows(db.DB(), q, func(r *sql.Rows) (channel, error) {
			var x channel
			var c sql.NullString
			err := r.Scan(&x.Source, &x.Orders, &x.AOV, &x.Revenue, &c)
			x.Currency = c.String
			return x, err
		})
		if err != nil {
			return err
		}
		var orders int
		var aov, revenue sql.NullFloat64
		var currency sql.NullString
		if err := db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*), ROUND(AVG(CAST(json_extract(data,'%s') AS REAL)),2), ROUND(SUM(CAST(json_extract(data,'%s') AS REAL)),2), MAX(json_extract(data,'%s')) FROM orders WHERE %s`, jsonTotalAmount, jsonTotalAmount, jsonTotalCurrency, windowClause(days))).Scan(&orders, &aov, &revenue, &currency); err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "orders": orders, "aov": round2(aov.Float64), "revenue": round2(revenue.Float64), "currency": currency.String, "by_source": channels}), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	return cmd
}

func newReportDiscountImpactCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "discount-impact", Short: "Compare orders with any discount application vs orders without discounts.", Long: "Shopify synced data includes discount application type/target/value, not codes; this reports any-discount impact, not per-code ROI.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`SELECT CASE WHEN json_array_length(json_extract(data,'%s'))>0 THEN 'discounted' ELSE 'full_price' END bucket, COUNT(*), ROUND(SUM(CAST(json_extract(data,'%s') AS REAL)),2), ROUND(AVG(CAST(json_extract(data,'%s') AS REAL)),2) FROM orders WHERE %s GROUP BY bucket ORDER BY bucket`, jsonDiscountApps, jsonTotalAmount, jsonTotalAmount, windowClause(days))
		type row struct {
			Bucket  string  `json:"bucket"`
			Orders  int     `json:"orders"`
			Revenue float64 `json:"revenue"`
			AOV     float64 `json:"aov"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) { var x row; return x, r.Scan(&x.Bucket, &x.Orders, &x.Revenue, &x.AOV) })
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	return cmd
}

func newReportRefundAnalysisCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "refund-analysis", Short: "Refunded order/product analysis from financial status and refund totals.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH refunded AS (SELECT id,data FROM orders WHERE %s AND (UPPER(COALESCE(display_financial_status,'')) LIKE '%%REFUND%%' OR CAST(COALESCE(json_extract(data,'%s'),'0') AS REAL)>0)), li AS (SELECT %s product, SUM(CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL)) units, COUNT(DISTINCT r.id) orders FROM refunded r, json_each(json_extract(r.data,'%s')) j GROUP BY product) SELECT product, orders, units FROM li ORDER BY orders DESC, units DESC LIMIT ?`, windowClause(days), jsonRefundAmount, productNameExpr("j.value"), jsonLineItemQuantity, jsonLineItems)
		type row struct {
			Product string  `json:"product"`
			Orders  int     `json:"orders"`
			Units   float64 `json:"units"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) { var x row; return x, r.Scan(&x.Product, &x.Orders, &x.Units) }, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "note": lineItemCapNote, "products": out}), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 50)
	return cmd
}

func newReportPeakHoursCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "peak-hours", Short: "UTC order volume and revenue by hour/day-of-week.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`SELECT CAST(strftime('%%w',created_at) AS INTEGER), CAST(strftime('%%H',created_at) AS INTEGER), COUNT(*), ROUND(SUM(CAST(json_extract(data,'%s') AS REAL)),2) FROM orders WHERE %s GROUP BY 1,2 ORDER BY 1,2`, jsonTotalAmount, windowClause(days))
		type row struct {
			WeekdayUTC int     `json:"weekday_utc"`
			HourUTC    int     `json:"hour_utc"`
			Orders     int     `json:"orders"`
			Revenue    float64 `json:"revenue"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.WeekdayUTC, &x.HourUTC, &x.Orders, &x.Revenue)
		})
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	return cmd
}

func newReportFirstPurchaseAnalysisCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "first-purchase-analysis", Short: "First-purchase cohort products and customer LTV for absolute first orders.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH firsts AS (SELECT json_extract(data,'%s') customer_id, MIN(created_at) first_at FROM orders WHERE json_extract(data,'%s') IS NOT NULL GROUP BY 1), first_orders AS (SELECT o.id,o.data,json_extract(o.data,'%s') customer_id FROM orders o JOIN firsts f ON f.customer_id=json_extract(o.data,'%s') AND f.first_at=o.created_at WHERE o.created_at >= datetime('now', '-' || ? || ' days')), ltv AS (SELECT json_extract(data,'%s') customer_id, SUM(CAST(json_extract(data,'%s') AS REAL)) ltv FROM orders GROUP BY 1), li AS (SELECT %s product, COUNT(DISTINCT fo.customer_id) customers, ROUND(AVG(ltv.ltv),2) avg_ltv FROM first_orders fo JOIN ltv ON ltv.customer_id=fo.customer_id, json_each(json_extract(fo.data,'%s')) j GROUP BY product) SELECT product, customers, avg_ltv FROM li ORDER BY customers DESC LIMIT ?`, jsonCustomerID, jsonCustomerID, jsonCustomerID, jsonCustomerID, jsonCustomerID, jsonTotalAmount, productNameExpr("j.value"), jsonLineItems)
		type row struct {
			Product   string  `json:"first_product"`
			Customers int     `json:"customers"`
			AvgLTV    float64 `json:"avg_ltv"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) { var x row; return x, r.Scan(&x.Product, &x.Customers, &x.AvgLTV) }, days, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "note": lineItemCapNote, "first_purchase_products": out}), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	addLimitFlag(cmd, &limit, 50)
	return cmd
}

func newReportKlaviyoAttributionCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "klaviyo-attribution", Short: "Revenue attributed to email/Klaviyo source names or tags.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`SELECT COUNT(*), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2), ROUND(COALESCE(AVG(CAST(json_extract(data,'%s') AS REAL)),0),2) FROM orders WHERE %s AND (LOWER(COALESCE(source_name,'')) LIKE '%%email%%' OR LOWER(COALESCE(source_name,'')) LIKE '%%klaviyo%%' OR %s)`, jsonTotalAmount, jsonTotalAmount, windowClause(days), containsTagExpr("orders"))
		var orders int
		var revenue, aov sql.NullFloat64
		if err := db.DB().QueryRow(q, "%klaviyo%").Scan(&orders, &revenue, &aov); err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "orders": orders, "revenue": round2(revenue.Float64), "aov": round2(aov.Float64), "match": "source_name contains email/klaviyo or tag contains klaviyo"}), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	return cmd
}
