package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newReportDashboardCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "dashboard", Short: "Executive Shopify dashboard: revenue, orders, AOV, top products, and customers.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		var orders int
		var revenue, aov, refunds sql.NullFloat64
		var customers sql.NullInt64
		summary := fmt.Sprintf(`SELECT COUNT(*), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2), ROUND(COALESCE(AVG(CAST(json_extract(data,'%s') AS REAL)),0),2), ROUND(COALESCE(SUM(CAST(COALESCE(json_extract(data,'%s'),'0') AS REAL)),0),2), COUNT(DISTINCT json_extract(data,'%s')) FROM orders WHERE %s`, jsonTotalAmount, jsonTotalAmount, jsonRefundAmount, jsonCustomerID, windowClause(days))
		if err := db.DB().QueryRow(summary).Scan(&orders, &revenue, &aov, &refunds, &customers); err != nil {
			return err
		}
		topQ := fmt.Sprintf(`WITH li AS (SELECT %s product,SUM(CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL)) units,SUM(CAST(COALESCE(json_extract(j.value,'%s'),json_extract(o.data,'%s'),'0') AS REAL)*CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL)) revenue FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s GROUP BY product) SELECT product,ROUND(units,2),ROUND(revenue,2) FROM li ORDER BY revenue DESC LIMIT 5`, productNameExpr("j.value"), jsonLineItemQuantity, jsonLineItemUnitAmount, jsonTotalAmount, jsonLineItemQuantity, jsonLineItems, windowClause(days))
		type product struct {
			Product string  `json:"product"`
			Units   float64 `json:"units"`
			Revenue float64 `json:"revenue"`
		}
		top, err := queryRows(db.DB(), topQ, func(r *sql.Rows) (product, error) { var x product; return x, r.Scan(&x.Product, &x.Units, &x.Revenue) })
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "orders": orders, "revenue": round2(revenue.Float64), "aov": round2(aov.Float64), "refunds": round2(refunds.Float64), "customers": customers.Int64, "top_products": top, "note": lineItemCapNote}), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	return cmd
}

func newReportWeeklyDigestCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "weekly-digest", Short: "This window vs previous window comparison for revenue, orders, and AOV.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		now := time.Now().UTC()
		metric := func(offset int) (int, float64, float64, error) {
			start := now.AddDate(0, 0, -(offset + days)).Format(time.RFC3339)
			end := now.AddDate(0, 0, -offset).Format(time.RFC3339)
			q := fmt.Sprintf(`SELECT COUNT(*), ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2), ROUND(COALESCE(AVG(CAST(json_extract(data,'%s') AS REAL)),0),2) FROM orders WHERE created_at >= ? AND created_at < ?`, jsonTotalAmount, jsonTotalAmount)
			var o int
			var rev, aov sql.NullFloat64
			err := db.DB().QueryRow(q, start, end).Scan(&o, &rev, &aov)
			return o, round2(rev.Float64), round2(aov.Float64), err
		}
		co, cr, ca, err := metric(0)
		if err != nil {
			return err
		}
		po, pr, pa, err := metric(days)
		if err != nil {
			return err
		}
		chg := func(c, p float64) float64 {
			if p == 0 {
				return 0
			}
			return round2((c - p) / p * 100)
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "current": map[string]any{"orders": co, "revenue": cr, "aov": ca}, "previous": map[string]any{"orders": po, "revenue": pr, "aov": pa}, "change_pct": map[string]any{"orders": chg(float64(co), float64(po)), "revenue": chg(cr, pr), "aov": chg(ca, pa)}}), flags)
	}}
	addDaysFlag(cmd, &days, 7)
	return cmd
}

func newReportHealthScoreCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "health-score", Short: "Composite 0-100 ecommerce health score from revenue trend, repeat rate, refunds, and fulfillment.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		now := time.Now().UTC()
		currentStart := now.AddDate(0, 0, -days).Format(time.RFC3339)
		previousStart := now.AddDate(0, 0, -(days * 2)).Format(time.RFC3339)
		var revenue, prevRevenue, refunds sql.NullFloat64
		var customers, repeaters sql.NullInt64
		var riskyFulfillments int
		if err := db.DB().QueryRow(fmt.Sprintf(`SELECT ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2), ROUND(COALESCE(SUM(CAST(COALESCE(json_extract(data,'%s'),'0') AS REAL)),0),2) FROM orders WHERE created_at >= ?`, jsonTotalAmount, jsonRefundAmount), currentStart).Scan(&revenue, &refunds); err != nil {
			return err
		}
		if err := db.DB().QueryRow(fmt.Sprintf(`SELECT ROUND(COALESCE(SUM(CAST(json_extract(data,'%s') AS REAL)),0),2) FROM orders WHERE created_at >= ? AND created_at < ?`, jsonTotalAmount), previousStart, currentStart).Scan(&prevRevenue); err != nil {
			return err
		}
		if err := db.DB().QueryRow(fmt.Sprintf(`WITH per AS (SELECT json_extract(data,'%s') cid, COUNT(*) orders FROM orders WHERE created_at >= ? AND json_extract(data,'%s') IS NOT NULL GROUP BY cid) SELECT COUNT(*), SUM(CASE WHEN orders>1 THEN 1 ELSE 0 END) FROM per`, jsonCustomerID, jsonCustomerID), currentStart).Scan(&customers, &repeaters); err != nil {
			return err
		}
		fulfillmentCutoff := now.Add(-24 * time.Hour).Format(time.RFC3339)
		if err := db.DB().QueryRow(`SELECT COUNT(*) FROM fulfillment_orders WHERE UPPER(COALESCE(status,'')) NOT IN ('CLOSED','CANCELLED','CANCELED') AND UPPER(COALESCE(request_status,'')) NOT IN ('FULFILLED','CLOSED','CANCELLED','CANCELED') AND created_at <= ?`, fulfillmentCutoff).Scan(&riskyFulfillments); err != nil {
			return err
		}
		revenueTrend := 0.0
		if prevRevenue.Float64 > 0 {
			revenueTrend = (revenue.Float64 - prevRevenue.Float64) / prevRevenue.Float64 * 100
		}
		repeatRate := 0.0
		if customers.Int64 > 0 {
			repeatRate = float64(repeaters.Int64) / float64(customers.Int64) * 100
		}
		refundRate := 0.0
		if revenue.Float64 > 0 {
			refundRate = refunds.Float64 / revenue.Float64 * 100
		}
		score := 50.0 + revenueTrend*0.25 + repeatRate*0.3 - refundRate*1.5
		if riskyFulfillments > 0 {
			score -= 20
		}
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "score": round2(score), "components": map[string]any{"revenue": round2(revenue.Float64), "previous_revenue": round2(prevRevenue.Float64), "revenue_trend_pct": round2(revenueTrend), "repeat_rate_pct": round2(repeatRate), "refund_rate_pct": round2(refundRate), "fulfillment_risk": riskyFulfillments}}), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	return cmd
}
