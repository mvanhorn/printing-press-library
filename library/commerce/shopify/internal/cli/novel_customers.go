package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newReportCustomerCohortsCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "customer-cohorts", Short: "Customer retention cohorts by absolute first purchase month.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`WITH firsts AS (SELECT json_extract(data,'%s') cid, MIN(created_at) first_at FROM orders WHERE json_extract(data,'%s') IS NOT NULL GROUP BY 1), cohorts AS (SELECT cid, substr(first_at,1,7) cohort_month, first_at FROM firsts WHERE first_at >= datetime('now','-'||?||' days')), activity AS (SELECT c.cohort_month,c.cid, MAX(CASE WHEN julianday(o.created_at)-julianday(c.first_at) BETWEEN 1 AND 30 THEN 1 ELSE 0 END) r30, MAX(CASE WHEN julianday(o.created_at)-julianday(c.first_at) BETWEEN 31 AND 60 THEN 1 ELSE 0 END) r60, MAX(CASE WHEN julianday(o.created_at)-julianday(c.first_at) BETWEEN 61 AND 90 THEN 1 ELSE 0 END) r90 FROM cohorts c LEFT JOIN orders o ON json_extract(o.data,'%s')=c.cid GROUP BY c.cohort_month,c.cid) SELECT cohort_month, COUNT(*), SUM(r30), SUM(r60), SUM(r90) FROM activity GROUP BY cohort_month ORDER BY cohort_month DESC`, jsonCustomerID, jsonCustomerID, jsonCustomerID)
		type row struct {
			CohortMonth    string  `json:"cohort_month"`
			Customers      int     `json:"customers"`
			Retained30d    int     `json:"retained_30d"`
			Retained60d    int     `json:"retained_60d"`
			Retained90d    int     `json:"retained_90d"`
			Retention30Pct float64 `json:"retention_30_pct"`
			Retention60Pct float64 `json:"retention_60_pct"`
			Retention90Pct float64 `json:"retention_90_pct"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			err := r.Scan(&x.CohortMonth, &x.Customers, &x.Retained30d, &x.Retained60d, &x.Retained90d)
			if x.Customers > 0 {
				den := float64(x.Customers)
				x.Retention30Pct = round2(float64(x.Retained30d) / den * 100)
				x.Retention60Pct = round2(float64(x.Retained60d) / den * 100)
				x.Retention90Pct = round2(float64(x.Retained90d) / den * 100)
			}
			return x, err
		}, days)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	return cmd
}

func newReportCustomerRFMCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "customer-rfm", Short: "RFM customer segmentation; recency score is inverted so recent buyers score higher.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH per AS (SELECT json_extract(data,'%s') cid, MAX(json_extract(data,'%s')) email, julianday('now')-julianday(MAX(created_at)) recency_days, COUNT(*) frequency, SUM(CAST(json_extract(data,'%s') AS REAL)) monetary FROM orders WHERE %s AND json_extract(data,'%s') IS NOT NULL GROUP BY cid), scored AS (SELECT *, NTILE(5) OVER (ORDER BY recency_days DESC) r_score, NTILE(5) OVER (ORDER BY frequency ASC) f_score, NTILE(5) OVER (ORDER BY monetary ASC) m_score FROM per) SELECT cid,email,ROUND(recency_days,2),frequency,ROUND(monetary,2),r_score,f_score,m_score,(r_score+f_score+m_score) total_score FROM scored ORDER BY total_score DESC, monetary DESC LIMIT ?`, jsonCustomerID, jsonCustomerEmail, jsonTotalAmount, windowClause(days), jsonCustomerID)
		type row struct {
			CustomerID  string  `json:"customer_id"`
			Email       string  `json:"email"`
			RecencyDays float64 `json:"recency_days"`
			Monetary    float64 `json:"monetary"`
			Frequency   int     `json:"frequency"`
			RScore      int     `json:"r_score"`
			FScore      int     `json:"f_score"`
			MScore      int     `json:"m_score"`
			TotalScore  int     `json:"total_score"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			var email sql.NullString
			err := r.Scan(&x.CustomerID, &email, &x.RecencyDays, &x.Frequency, &x.Monetary, &x.RScore, &x.FScore, &x.MScore, &x.TotalScore)
			x.Email = email.String
			return x, err
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	addLimitFlag(cmd, &limit, 100)
	return cmd
}

func newReportCustomerLTVCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "customer-ltv", Short: "Top customers by lifetime value within the selected order window.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`SELECT json_extract(data,'%s') cid, MAX(json_extract(data,'%s')) email, COUNT(*), ROUND(SUM(CAST(json_extract(data,'%s') AS REAL)),2), MIN(created_at), MAX(created_at) FROM orders WHERE %s AND json_extract(data,'%s') IS NOT NULL GROUP BY cid ORDER BY 4 DESC LIMIT ?`, jsonCustomerID, jsonCustomerEmail, jsonTotalAmount, windowClause(days), jsonCustomerID)
		type row struct {
			CustomerID   string  `json:"customer_id"`
			Email        string  `json:"email"`
			Orders       int     `json:"orders"`
			LTV          float64 `json:"ltv"`
			FirstOrderAt string  `json:"first_order_at"`
			LastOrderAt  string  `json:"last_order_at"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			var email sql.NullString
			err := r.Scan(&x.CustomerID, &email, &x.Orders, &x.LTV, &x.FirstOrderAt, &x.LastOrderAt)
			x.Email = email.String
			return x, err
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	addLimitFlag(cmd, &limit, 50)
	return cmd
}

func newReportRepeatRateCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "repeat-rate", Short: "Repeat-purchase rate plus monthly trend.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		base := fmt.Sprintf(`WITH per AS (SELECT json_extract(data,'%s') cid, COUNT(*) orders FROM orders WHERE %s AND json_extract(data,'%s') IS NOT NULL GROUP BY cid) SELECT COUNT(*), SUM(CASE WHEN orders>1 THEN 1 ELSE 0 END) FROM per`, jsonCustomerID, windowClause(days), jsonCustomerID)
		var customers, repeaters sql.NullInt64
		if err := db.DB().QueryRow(base).Scan(&customers, &repeaters); err != nil {
			return err
		}
		trendQ := fmt.Sprintf(`WITH windowed AS (SELECT substr(created_at,1,7) month, created_at, json_extract(data,'%s') cid FROM orders WHERE %s AND json_extract(data,'%s') IS NOT NULL), months AS (SELECT month, date(month || '-01','+1 month') cutoff FROM windowed GROUP BY month), customer_month AS (SELECT months.month, windowed.cid, COUNT(*) orders_to_date FROM months JOIN windowed ON windowed.created_at < months.cutoff GROUP BY months.month, windowed.cid) SELECT month, COUNT(*), SUM(CASE WHEN orders_to_date>1 THEN 1 ELSE 0 END) FROM customer_month GROUP BY month ORDER BY month`, jsonCustomerID, windowClause(days), jsonCustomerID)
		type trend struct {
			Month         string  `json:"month"`
			Customers     int     `json:"customers"`
			Repeaters     int     `json:"repeaters"`
			RepeatRatePct float64 `json:"repeat_rate_pct"`
		}
		rows, err := queryRows(db.DB(), trendQ, func(r *sql.Rows) (trend, error) {
			var x trend
			err := r.Scan(&x.Month, &x.Customers, &x.Repeaters)
			if x.Customers > 0 {
				x.RepeatRatePct = round2(float64(x.Repeaters) / float64(x.Customers) * 100)
			}
			return x, err
		})
		if err != nil {
			return err
		}
		rate := 0.0
		if customers.Int64 > 0 {
			rate = round2(float64(repeaters.Int64) / float64(customers.Int64) * 100)
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "customers": customers.Int64, "repeaters": repeaters.Int64, "repeat_rate_pct": rate, "monthly": rows}), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	return cmd
}

func newReportCustomerChurnRiskCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "customer-churn-risk", Short: "Customers whose time since last order exceeds 1.5x their average order interval.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH ordered AS (SELECT json_extract(data,'%s') cid, json_extract(data,'%s') email, created_at, LAG(created_at) OVER (PARTITION BY json_extract(data,'%s') ORDER BY created_at) prev FROM orders WHERE %s AND json_extract(data,'%s') IS NOT NULL), gaps AS (SELECT cid, MAX(email) email, MAX(created_at) last_order_at, COUNT(*) orders, AVG(julianday(created_at)-julianday(prev)) avg_gap FROM ordered GROUP BY cid HAVING COUNT(*)>1 AND AVG(julianday(created_at)-julianday(prev)) IS NOT NULL) SELECT cid,email,last_order_at,orders,ROUND(avg_gap,2),ROUND(julianday('now')-julianday(last_order_at),2), CASE WHEN julianday('now')-julianday(last_order_at) > avg_gap*1.5 THEN 'high' ELSE 'normal' END risk FROM gaps ORDER BY (julianday('now')-julianday(last_order_at))/avg_gap DESC LIMIT ?`, jsonCustomerID, jsonCustomerEmail, jsonCustomerID, windowClause(days), jsonCustomerID)
		type row struct {
			CustomerID    string  `json:"customer_id"`
			Email         string  `json:"email"`
			LastOrderAt   string  `json:"last_order_at"`
			Risk          string  `json:"risk"`
			Orders        int     `json:"orders"`
			AvgGapDays    float64 `json:"avg_gap_days"`
			DaysSinceLast float64 `json:"days_since_last"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			var email sql.NullString
			err := r.Scan(&x.CustomerID, &email, &x.LastOrderAt, &x.Orders, &x.AvgGapDays, &x.DaysSinceLast, &x.Risk)
			x.Email = email.String
			return x, err
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	addLimitFlag(cmd, &limit, 100)
	return cmd
}
