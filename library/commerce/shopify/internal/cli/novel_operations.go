package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newReportFulfillmentSpeedCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "fulfillment-speed", Short: "Fulfillment order speed from created_at to fulfill_at.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`SELECT COALESCE(NULLIF(status,''),'(unknown)'), COUNT(*), ROUND(AVG((julianday(fulfill_at)-julianday(created_at))*24),2), ROUND(MIN((julianday(fulfill_at)-julianday(created_at))*24),2), ROUND(MAX((julianday(fulfill_at)-julianday(created_at))*24),2) FROM fulfillment_orders WHERE %s AND fulfill_at IS NOT NULL AND fulfill_at != '' AND created_at IS NOT NULL GROUP BY 1 ORDER BY 2 DESC`, windowClause(days))
		type row struct {
			Status   string  `json:"status"`
			Count    int     `json:"count"`
			AvgHours float64 `json:"avg_hours"`
			MinHours float64 `json:"min_hours"`
			MaxHours float64 `json:"max_hours"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.Status, &x.Count, &x.AvgHours, &x.MinHours, &x.MaxHours)
		})
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	return cmd
}

func newReportAbandonedCheckoutAnalysisCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "abandoned-checkout-analysis", Short: "Abandoned checkout totals, completion rate, and value over the selected window.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`SELECT COUNT(*), SUM(CASE WHEN completed_at IS NOT NULL AND completed_at!='' THEN 1 ELSE 0 END), ROUND(COALESCE(SUM(CAST(json_extract(data,'$.totalPriceSet.presentmentMoney.amount') AS REAL)),0),2), ROUND(COALESCE(AVG(CAST(json_extract(data,'$.totalPriceSet.presentmentMoney.amount') AS REAL)),0),2) FROM abandoned_checkouts WHERE %s`, windowClause(days))
		var total, completed sql.NullInt64
		var value, avg sql.NullFloat64
		if err := db.DB().QueryRow(q).Scan(&total, &completed, &value, &avg); err != nil {
			return err
		}
		rate := 0.0
		if total.Int64 > 0 {
			rate = round2(float64(completed.Int64) / float64(total.Int64) * 100)
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "checkouts": total.Int64, "completed": completed.Int64, "completion_rate_pct": rate, "abandoned": total.Int64 - completed.Int64, "total_value": round2(value.Float64), "avg_value": round2(avg.Float64)}), flags)
	}}
	addDaysFlag(cmd, &days, 30)
	return cmd
}

func newReportCartValueDistributionCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "cart-value-distribution", Short: "Order/cart value distribution buckets for synced orders.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`WITH vals AS (SELECT CAST(json_extract(data,'%s') AS REAL) total FROM orders WHERE %s), buckets AS (SELECT CASE WHEN total<25 THEN 'under_25' WHEN total<50 THEN '25_49' WHEN total<100 THEN '50_99' WHEN total<200 THEN '100_199' ELSE '200_plus' END bucket,total FROM vals) SELECT bucket,COUNT(*),ROUND(SUM(total),2),ROUND(AVG(total),2) FROM buckets GROUP BY bucket ORDER BY MIN(total)`, jsonTotalAmount, windowClause(days))
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
	addDaysFlag(cmd, &days, 30)
	return cmd
}
