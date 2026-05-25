package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/commerce/shopify/internal/store"
)

// Compound analytics commands. All read from the local SQLite store populated
// by `sync`. None hit the Shopify API. Run `sync --since <window>` first.
//
// The store's orders table has scalar columns for name, created_at,
// processed_at, display_financial_status, display_fulfillment_status,
// currency_code, source_name, note. Everything else (tags, lineItems,
// discountApplications, totalPriceSet) lives in the data JSON blob and is
// reached via SQLite's json_extract.

func newReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Compound analytics over the local store: revenue, channel mix, tag impact, attach rate, customer lifecycle.",
	}
	cmd.AddCommand(newReportRevenueDailyCmd(flags))
	cmd.AddCommand(newReportChannelMixCmd(flags))
	cmd.AddCommand(newReportShowImpactCmd(flags))
	cmd.AddCommand(newReportAttachRateCmd(flags))
	cmd.AddCommand(newReportCustomerLifecycleCmd(flags))
	return cmd
}

func openReportDB(ctx context.Context, flags *rootFlags) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("shopify-pp-cli"))
}

func windowClause(days int) string {
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
	return fmt.Sprintf("created_at >= '%s'", cutoff)
}

// --- revenue-daily ---

func newReportRevenueDailyCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:     "revenue-daily",
		Short:   "Daily revenue breakdown over the last N days from local orders.",
		Example: "  shopify-pp-cli report revenue-daily --days 30 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openReportDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer db.Close()
			q := fmt.Sprintf(`
				SELECT date(created_at) AS day,
				       COUNT(*) AS order_count,
				       ROUND(SUM(CAST(json_extract(data, '$.totalPriceSet.shopMoney.amount') AS REAL)), 2) AS gross,
				       ROUND(SUM(CAST(COALESCE(json_extract(data, '$.totalRefundedSet.shopMoney.amount'), '0') AS REAL)), 2) AS refunded,
				       MAX(json_extract(data, '$.totalPriceSet.shopMoney.currencyCode')) AS currency
				FROM orders
				WHERE %s
				GROUP BY day
				ORDER BY day DESC`, windowClause(days))
			rows, err := db.DB().Query(q)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				Day        string  `json:"day"`
				OrderCount int     `json:"order_count"`
				Gross      float64 `json:"gross"`
				Refunded   float64 `json:"refunded"`
				Net        float64 `json:"net"`
				Currency   string  `json:"currency"`
			}
			var out []row
			for rows.Next() {
				var r row
				var gross, refunded sql.NullFloat64
				var currency sql.NullString
				if err := rows.Scan(&r.Day, &r.OrderCount, &gross, &refunded, &currency); err != nil {
					return err
				}
				r.Gross = gross.Float64
				r.Refunded = refunded.Float64
				r.Net = r.Gross - r.Refunded
				r.Currency = currency.String
				out = append(out, r)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	return cmd
}

// --- channel-mix ---

func newReportChannelMixCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:     "channel-mix",
		Short:   "Revenue by sourceName (web, draft_order, pos, channel id) over the last N days.",
		Example: "  shopify-pp-cli report channel-mix --days 90 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openReportDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer db.Close()
			q := fmt.Sprintf(`
				SELECT COALESCE(NULLIF(source_name, ''), '(unknown)') AS source,
				       COUNT(*) AS order_count,
				       ROUND(SUM(CAST(json_extract(data, '$.totalPriceSet.shopMoney.amount') AS REAL)), 2) AS gross,
				       MAX(json_extract(data, '$.totalPriceSet.shopMoney.currencyCode')) AS currency
				FROM orders
				WHERE %s
				GROUP BY source
				ORDER BY gross DESC`, windowClause(days))
			rows, err := db.DB().Query(q)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				Source     string  `json:"source"`
				OrderCount int     `json:"order_count"`
				Gross      float64 `json:"gross"`
				SharePct   float64 `json:"share_pct"`
				Currency   string  `json:"currency"`
			}
			var out []row
			var total float64
			for rows.Next() {
				var r row
				var gross sql.NullFloat64
				var currency sql.NullString
				if err := rows.Scan(&r.Source, &r.OrderCount, &gross, &currency); err != nil {
					return err
				}
				r.Gross = gross.Float64
				r.Currency = currency.String
				total += r.Gross
				out = append(out, r)
			}
			for i := range out {
				if total > 0 {
					out[i].SharePct = round2(out[i].Gross / total * 100)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	return cmd
}

// --- show-impact ---

func newReportShowImpactCmd(flags *rootFlags) *cobra.Command {
	var days int
	var tag string
	cmd := &cobra.Command{
		Use:     "show-impact",
		Short:   "Compare revenue for tag-matched orders against the rest over the last N days.",
		Long:    `Splits orders into two buckets — those whose tags include --tag and the rest — and reports order count, gross, AOV, and share for each bucket.`,
		Example: "  shopify-pp-cli report show-impact --tag roadshow --days 30 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(tag) == "" {
				return usageErr(fmt.Errorf("--tag is required"))
			}
			db, err := openReportDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer db.Close()
			// Shopify tags arrive as a JSON array; json_each iterates it. Compare lowercased.
			matchExpr := `EXISTS (SELECT 1 FROM json_each(json_extract(orders.data, '$.tags')) WHERE LOWER(value) = LOWER(?))`
			runBucket := func(label string, args ...any) (map[string]any, error) {
				where := windowClause(days)
				clause := fmt.Sprintf("%s AND %s", where, matchExpr)
				if label == "other" {
					clause = fmt.Sprintf("%s AND NOT %s", where, matchExpr)
				}
				q := fmt.Sprintf(`
					SELECT COUNT(*) AS order_count,
					       ROUND(COALESCE(SUM(CAST(json_extract(data, '$.totalPriceSet.shopMoney.amount') AS REAL)), 0), 2) AS gross,
					       MAX(json_extract(data, '$.totalPriceSet.shopMoney.currencyCode')) AS currency
					FROM orders WHERE %s`, clause)
				row := db.DB().QueryRow(q, args...)
				var oc int
				var gross sql.NullFloat64
				var currency sql.NullString
				if err := row.Scan(&oc, &gross, &currency); err != nil {
					return nil, err
				}
				aov := 0.0
				if oc > 0 {
					aov = round2(gross.Float64 / float64(oc))
				}
				return map[string]any{
					"bucket":      label,
					"order_count": oc,
					"gross":       round2(gross.Float64),
					"aov":         aov,
					"currency":    currency.String,
				}, nil
			}
			tagged, err := runBucket("tag:"+tag, tag)
			if err != nil {
				return err
			}
			other, err := runBucket("other", tag)
			if err != nil {
				return err
			}
			totalGross := tagged["gross"].(float64) + other["gross"].(float64)
			sharePct := 0.0
			if totalGross > 0 {
				sharePct = round2(tagged["gross"].(float64) / totalGross * 100)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{
				"tag":            tag,
				"days":           days,
				"tagged_bucket":  tagged,
				"other_bucket":   other,
				"tagged_share_pct": sharePct,
			}), flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	cmd.Flags().StringVar(&tag, "tag", "", "Tag to filter on (case-insensitive exact match)")
	return cmd
}

// --- attach-rate ---

func newReportAttachRateCmd(flags *rootFlags) *cobra.Command {
	var days int
	var anchor, attached string
	cmd := &cobra.Command{
		Use:   "attach-rate",
		Short: "Percent of orders containing --anchor that also contain --attached (case-insensitive substring on line-item title).",
		Long: `Computes: of orders in the window with --anchor in their line items,
what fraction also contained --attached. Useful for "do desk buyers also buy
a cable kit?"-style questions. Matching is case-insensitive substring against
line-item titles.`,
		Example: "  shopify-pp-cli report attach-rate --anchor desk --attached cable --days 90 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(anchor) == "" || strings.TrimSpace(attached) == "" {
				return usageErr(fmt.Errorf("--anchor and --attached are both required"))
			}
			db, err := openReportDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer db.Close()
			where := windowClause(days)
			// Single CTE wrap: re-evaluating the json_each EXISTS per row is
			// fine; SQLite can't share the predicate across CASE branches
			// without an inner alias, and the cardinality here (one row per
			// order in window) is bounded.
			q := fmt.Sprintf(`
				WITH base AS (
				  SELECT id, data FROM orders WHERE %s
				)
				SELECT
				  SUM(CASE WHEN EXISTS (SELECT 1 FROM json_each(json_extract(base.data, '$.lineItems.nodes')) WHERE LOWER(json_extract(value, '$.title')) LIKE LOWER(?)) THEN 1 ELSE 0 END) AS anchor_orders,
				  SUM(CASE WHEN
				       EXISTS (SELECT 1 FROM json_each(json_extract(base.data, '$.lineItems.nodes')) WHERE LOWER(json_extract(value, '$.title')) LIKE LOWER(?))
				       AND
				       EXISTS (SELECT 1 FROM json_each(json_extract(base.data, '$.lineItems.nodes')) WHERE LOWER(json_extract(value, '$.title')) LIKE LOWER(?))
				    THEN 1 ELSE 0 END) AS attached_orders
				FROM base`, where)
			row := db.DB().QueryRow(q, "%"+anchor+"%", "%"+anchor+"%", "%"+attached+"%")
			var anchorOrders, attachedOrders sql.NullInt64
			if err := row.Scan(&anchorOrders, &attachedOrders); err != nil {
				return fmt.Errorf("query: %w", err)
			}
			rate := 0.0
			if anchorOrders.Int64 > 0 {
				rate = round2(float64(attachedOrders.Int64) / float64(anchorOrders.Int64) * 100)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{
				"anchor":           anchor,
				"attached":         attached,
				"days":             days,
				"anchor_orders":    anchorOrders.Int64,
				"attached_orders":  attachedOrders.Int64,
				"attach_rate_pct":  rate,
			}), flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 90, "Window in days")
	cmd.Flags().StringVar(&anchor, "anchor", "", "Anchor product title substring (case-insensitive)")
	cmd.Flags().StringVar(&attached, "attached", "", "Attached product title substring (case-insensitive)")
	return cmd
}

// --- customer-lifecycle ---

func newReportCustomerLifecycleCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:     "customer-lifecycle",
		Short:   "Repeat-purchase distribution and mean time-between-orders over the last N days.",
		Example: "  shopify-pp-cli report customer-lifecycle --days 365 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openReportDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer db.Close()
			// Cohort distribution: orders per customer in the window.
			q1 := fmt.Sprintf(`
				WITH per_customer AS (
				  SELECT json_extract(data, '$.customer.id') AS customer_id, COUNT(*) AS order_count
				  FROM orders
				  WHERE %s AND json_extract(data, '$.customer.id') IS NOT NULL
				  GROUP BY customer_id
				)
				SELECT order_count, COUNT(*) AS customers
				FROM per_customer
				GROUP BY order_count
				ORDER BY order_count`, windowClause(days))
			rows, err := db.DB().Query(q1)
			if err != nil {
				return fmt.Errorf("cohort query: %w", err)
			}
			defer rows.Close()
			type bucket struct {
				OrderCount int `json:"order_count"`
				Customers  int `json:"customers"`
			}
			var dist []bucket
			for rows.Next() {
				var b bucket
				if err := rows.Scan(&b.OrderCount, &b.Customers); err != nil {
					return err
				}
				dist = append(dist, b)
			}
			// Mean time-between-orders for repeat customers (AVG).
			// Note: gap distributions are typically right-skewed (a few very
			// loyal customers ordering daily pull the mean above the typical
			// inter-order gap). Use min/max plus the order_count_distribution
			// to interpret context; a future median field would require
			// SQLite percentile_cont logic or window-function nth-row tricks.
			q2 := fmt.Sprintf(`
				WITH ordered AS (
				  SELECT json_extract(data, '$.customer.id') AS customer_id,
				         created_at,
				         LAG(created_at) OVER (PARTITION BY json_extract(data, '$.customer.id') ORDER BY created_at) AS prev
				  FROM orders
				  WHERE %s AND json_extract(data, '$.customer.id') IS NOT NULL
				),
				gaps AS (
				  SELECT (julianday(created_at) - julianday(prev)) AS days_between
				  FROM ordered
				  WHERE prev IS NOT NULL
				)
				SELECT COUNT(*), AVG(days_between), MIN(days_between), MAX(days_between)
				FROM gaps`, windowClause(days))
			var gapCount sql.NullInt64
			var avgGap, minGap, maxGap sql.NullFloat64
			if err := db.DB().QueryRow(q2).Scan(&gapCount, &avgGap, &minGap, &maxGap); err != nil {
				return fmt.Errorf("gap query: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(map[string]any{
				"days":                  days,
				"order_count_distribution": dist,
				"gap_count":             gapCount.Int64,
				"avg_days_between":      round2(avgGap.Float64),
				"min_days_between":      round2(minGap.Float64),
				"max_days_between":      round2(maxGap.Float64),
			}), flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 365, "Window in days")
	return cmd
}

// helpers

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
