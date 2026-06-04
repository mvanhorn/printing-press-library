package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newReportProductDashboardCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "product-dashboard", Short: "Per-product revenue, units, orders, and AOV from synced line items.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH li AS (SELECT %s product_id, %s product, o.id order_id, CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL) qty, CAST(COALESCE(json_extract(j.value,'%s'), json_extract(o.data,'%s'), '0') AS REAL) price FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s) SELECT product_id,product,COUNT(DISTINCT order_id),ROUND(SUM(qty),2),ROUND(SUM(qty*price),2),ROUND(SUM(qty*price)/NULLIF(COUNT(DISTINCT order_id),0),2) FROM li GROUP BY product_id,product ORDER BY 5 DESC LIMIT ?`, productIDExpr("j.value"), productNameExpr("j.value"), jsonLineItemQuantity, jsonLineItemUnitAmount, jsonTotalAmount, jsonLineItems, windowClause(days))
		type row struct {
			ProductID string  `json:"product_id"`
			Product   string  `json:"product"`
			Orders    int     `json:"orders"`
			Units     float64 `json:"units"`
			Revenue   float64 `json:"revenue"`
			AOV       float64 `json:"aov"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.ProductID, &x.Product, &x.Orders, &x.Units, &x.Revenue, &x.AOV)
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "note": lineItemCapNote, "products": out}), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 25)
	return cmd
}

func newReportProductVelocityCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "product-velocity", Short: "Weekly units per product and week-over-week growth.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH weekly AS (SELECT %s product_id,%s product,strftime('%%Y-%%W',o.created_at) week,SUM(CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL)) units FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s GROUP BY product_id,product,week), ranked AS (SELECT *, LAG(units) OVER (PARTITION BY product_id ORDER BY week) prev_units, ROW_NUMBER() OVER (PARTITION BY product_id ORDER BY week DESC) rn FROM weekly) SELECT product_id,product,week,ROUND(units,2),ROUND(COALESCE(prev_units,0),2),CASE WHEN prev_units>0 THEN ROUND((units-prev_units)/prev_units*100,2) ELSE NULL END FROM ranked WHERE rn=1 ORDER BY units DESC LIMIT ?`, productIDExpr("j.value"), productNameExpr("j.value"), jsonLineItemQuantity, jsonLineItems, windowClause(days))
		type row struct {
			ProductID     string   `json:"product_id"`
			Product       string   `json:"product"`
			Week          string   `json:"week"`
			Units         float64  `json:"units"`
			PreviousUnits float64  `json:"previous_units"`
			GrowthPct     *float64 `json:"growth_pct,omitempty"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			var g sql.NullFloat64
			err := r.Scan(&x.ProductID, &x.Product, &x.Week, &x.Units, &x.PreviousUnits, &g)
			if g.Valid {
				x.GrowthPct = &g.Float64
			}
			return x, err
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "note": lineItemCapNote, "products": out}), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 25)
	return cmd
}

func newReportProductAffinityCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "product-affinity", Short: "Co-purchase product pairs with support, confidence, and lift.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH order_products AS (SELECT DISTINCT o.id order_id,%s product_id,%s product FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s), totals AS (SELECT COUNT(DISTINCT order_id) total_orders FROM order_products), counts AS (SELECT product_id,COUNT(*) orders FROM order_products GROUP BY product_id), pairs AS (SELECT a.product_id a_id,a.product a_product,b.product_id b_id,b.product b_product,COUNT(*) pair_orders FROM order_products a JOIN order_products b ON a.order_id=b.order_id AND a.product_id<b.product_id GROUP BY a_id,b_id) SELECT a_product,b_product,pair_orders,ROUND(pair_orders*100.0/t.total_orders,2),ROUND(pair_orders*100.0/ca.orders,2),ROUND((pair_orders*1.0/t.total_orders)/((ca.orders*1.0/t.total_orders)*(cb.orders*1.0/t.total_orders)),2) FROM pairs JOIN counts ca ON ca.product_id=a_id JOIN counts cb ON cb.product_id=b_id CROSS JOIN totals t ORDER BY pair_orders DESC, 6 DESC LIMIT ?`, productIDExpr("j.value"), productNameExpr("j.value"), jsonLineItems, windowClause(days))
		type row struct {
			ProductA       string  `json:"product_a"`
			ProductB       string  `json:"product_b"`
			PairOrders     int     `json:"pair_orders"`
			SupportPct     float64 `json:"support_pct"`
			ConfidenceAPct float64 `json:"confidence_a_pct"`
			Lift           float64 `json:"lift"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.ProductA, &x.ProductB, &x.PairOrders, &x.SupportPct, &x.ConfidenceAPct, &x.Lift)
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "note": lineItemCapNote, "pairs": out}), flags)
	}}
	addDaysFlag(cmd, &days, 90)
	addLimitFlag(cmd, &limit, 20)
	return cmd
}

func newReportProductCannibalizationCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	cmd := &cobra.Command{Use: "product-cannibalization", Short: "Product pairs with negative weekly revenue correlation.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		limit = clampLimit(limit)
		q := fmt.Sprintf(`WITH weekly AS (SELECT %s product_id,%s product,strftime('%%Y-%%W',o.created_at) week,SUM(CAST(COALESCE(json_extract(j.value,'%s'),json_extract(o.data,'%s'),'0') AS REAL)*CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL)) revenue FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s GROUP BY product_id,product,week), pairs AS (SELECT a.product a_product,b.product b_product,COUNT(*) n,SUM(a.revenue) sx,SUM(b.revenue) sy,SUM(a.revenue*b.revenue) sxy,SUM(a.revenue*a.revenue) sx2,SUM(b.revenue*b.revenue) sy2 FROM weekly a JOIN weekly b ON a.week=b.week AND a.product_id<b.product_id GROUP BY a.product_id,b.product_id HAVING n>1), corr AS (SELECT a_product,b_product,ROUND((n*sxy-sx*sy)/NULLIF(sqrt((n*sx2-sx*sx)*(n*sy2-sy*sy)),0),3) correlation,n weeks FROM pairs) SELECT a_product,b_product,correlation,weeks FROM corr WHERE correlation < 0 ORDER BY correlation ASC LIMIT ?`, productIDExpr("j.value"), productNameExpr("j.value"), jsonLineItemUnitAmount, jsonTotalAmount, jsonLineItemQuantity, jsonLineItems, windowClause(days))
		type row struct {
			ProductA    string  `json:"product_a"`
			ProductB    string  `json:"product_b"`
			Correlation float64 `json:"correlation"`
			Weeks       int     `json:"weeks"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.ProductA, &x.ProductB, &x.Correlation, &x.Weeks)
		}, limit)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
	}}
	addDaysFlag(cmd, &days, 180)
	addLimitFlag(cmd, &limit, 20)
	return cmd
}

func newReportProductSeasonalityCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "product-seasonality", Short: "Monthly product units and seasonality index versus product monthly average.", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openReportDB(flags)
		if err != nil {
			return err
		}
		defer db.Close()
		days = normalizeDays(days)
		q := fmt.Sprintf(`WITH monthly AS (SELECT %s product_id,%s product,strftime('%%m',o.created_at) month,SUM(CAST(COALESCE(json_extract(j.value,'%s'),'1') AS REAL)) units FROM orders o, json_each(json_extract(o.data,'%s')) j WHERE %s GROUP BY product_id,product,month), avgm AS (SELECT product_id,AVG(units) avg_units FROM monthly GROUP BY product_id) SELECT product,month,ROUND(units,2),ROUND(units/NULLIF(avg_units,0),2) seasonality_index FROM monthly JOIN avgm USING(product_id) ORDER BY product,month`, productIDExpr("j.value"), productNameExpr("j.value"), jsonLineItemQuantity, jsonLineItems, windowClause(days))
		type row struct {
			Product          string  `json:"product"`
			Month            string  `json:"month"`
			Units            float64 `json:"units"`
			SeasonalityIndex float64 `json:"seasonality_index"`
		}
		out, err := queryRows(db.DB(), q, func(r *sql.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.Product, &x.Month, &x.Units, &x.SeasonalityIndex)
		})
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{"days": days, "note": lineItemCapNote, "seasonality": out}), flags)
	}}
	addDaysFlag(cmd, &days, 365)
	return cmd
}
