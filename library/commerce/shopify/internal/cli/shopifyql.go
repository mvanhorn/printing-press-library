package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const shopifyqlQueryMutation = `query($query: String!) {
  shopifyqlQuery(query: $query) {
    tableData {
      columns { name dataType displayName }
      rows
    }
    parseErrors
  }
}`

func newShopifyqlCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shopifyql",
		Short: "Run ShopifyQL analytics queries (sessions, conversion, funnel, raw query). Requires read_analytics scope.",
	}
	cmd.AddCommand(newShopifyqlQueryCmd(flags))
	cmd.AddCommand(newShopifyqlSessionsCmd(flags))
	cmd.AddCommand(newShopifyqlConversionCmd(flags))
	cmd.AddCommand(newShopifyqlFunnelCmd(flags))
	return cmd
}

func runShopifyql(cmd *cobra.Command, flags *rootFlags, query string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, err := c.Query(cmd.Context(), shopifyqlQueryMutation, map[string]any{"query": query})
	if err == nil && !flags.dryRun {
		// Surface ShopifyQL parseErrors before extracting so missing scopes
		// or syntax errors fail loudly instead of printing an empty rows
		// payload with exit 0. Same defense applied in extractScalarSessions
		// for the funnel command.
		if perr := checkShopifyqlParseErrors(data); perr != nil {
			return classifyAPIError(perr, flags)
		}
		data, err = extractGraphQLObject(data, "shopifyqlQuery")
	}
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

// checkShopifyqlParseErrors decodes the shopifyqlQuery.parseErrors array from
// a raw GraphQL response and returns a non-nil error when ShopifyQL rejected
// the query. ShopifyQL returns HTTP 200 with structurally valid JSON in this
// case, so without an explicit check callers print "successful" empty rows.
func checkShopifyqlParseErrors(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var env struct {
		ShopifyqlQuery struct {
			ParseErrors []struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"parseErrors"`
		} `json:"shopifyqlQuery"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		// Not a shopifyqlQuery shape; not our concern.
		return nil
	}
	if len(env.ShopifyqlQuery.ParseErrors) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(env.ShopifyqlQuery.ParseErrors))
	for _, pe := range env.ShopifyqlQuery.ParseErrors {
		if pe.Code != "" {
			msgs = append(msgs, pe.Code+": "+pe.Message)
		} else {
			msgs = append(msgs, pe.Message)
		}
	}
	return fmt.Errorf("shopifyql parse error: %s", strings.Join(msgs, "; "))
}

func newShopifyqlQueryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query [query-text]",
		Short:   "Run a raw ShopifyQL query string.",
		Example: `  shopify-pp-cli shopifyql query "FROM sales SINCE -7d UNTIL today SHOW total_sales BY day" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return runShopifyql(cmd, flags, "FROM sales SINCE -7d UNTIL today SHOW total_sales BY day")
				}
				return cmd.Help()
			}
			return runShopifyql(cmd, flags, args[0])
		},
	}
	return cmd
}

func daysClause(days int) string {
	if days <= 0 {
		days = 30
	}
	return fmt.Sprintf("SINCE -%dd UNTIL today", days)
}

func newShopifyqlSessionsCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:     "sessions",
		Short:   "Sessions per day for the Online Store over the last N days (default 30).",
		Example: "  shopify-pp-cli shopifyql sessions --days 30 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := fmt.Sprintf("SHOW sessions FROM sessions %s TIMESERIES day", daysClause(days))
			return runShopifyql(cmd, flags, q)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	return cmd
}

func newShopifyqlConversionCmd(flags *rootFlags) *cobra.Command {
	var days int
	var byDay bool
	cmd := &cobra.Command{
		Use:     "conversion",
		Short:   "Online Store conversion rate over the last N days, optionally per-day.",
		Example: "  shopify-pp-cli shopifyql conversion --days 30 --by-day --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			metrics := "sessions, conversion_rate"
			q := fmt.Sprintf("SHOW %s FROM sessions %s", metrics, daysClause(days))
			if byDay {
				q = fmt.Sprintf("SHOW %s FROM sessions %s TIMESERIES day", metrics, daysClause(days))
			}
			return runShopifyql(cmd, flags, q)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	cmd.Flags().BoolVar(&byDay, "by-day", false, "Return per-day rows instead of the aggregate")
	return cmd
}

func newShopifyqlFunnelCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "funnel",
		Short: "Three-stage funnel over N days: sessions -> checkouts -> orders.",
		Long: `Returns a three-stage funnel computed from Shopify's available data:

  sessions          - visitors (ShopifyQL sessions dataset, live API call)
  checkouts_started - abandoned + completed checkouts in window
  orders            - completed orders in window

Cart-addition and checkout-start columns do not exist in the ShopifyQL
sessions dataset that merchants can access. checkouts_started is computed
locally as (abandoned_checkouts rows + orders rows) in the window since
completed checkouts become orders.

Requires:
  - read_analytics scope on the access token (for sessions)
  - 'sync orders' and 'sync abandoned-checkouts' to have populated the
    local store within the window.`,
		Example: "  shopify-pp-cli shopifyql funnel --days 30 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShopifyqlFunnel(cmd, flags, days)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	return cmd
}

func runShopifyqlFunnel(cmd *cobra.Command, flags *rootFlags, days int) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}

	// 1. Sessions - live ShopifyQL call (no BY clause = single aggregate row)
	sessQuery := fmt.Sprintf("SHOW sessions FROM sessions %s", daysClause(days))
	sessRaw, err := c.Query(cmd.Context(), shopifyqlQueryMutation, map[string]any{"query": sessQuery})
	if err != nil {
		return classifyAPIError(err, flags)
	}
	sessions, err := extractScalarSessions(sessRaw)
	if err != nil {
		return fmt.Errorf("parsing sessions response: %w", err)
	}

	// 2 + 3. Local-store counts in window
	db, err := openReportDB(cmd.Context(), flags)
	if err != nil {
		return err
	}
	defer db.Close()

	wc := windowClause(days)
	var abandonedCount, ordersCount int
	if err := db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "abandoned_checkouts" WHERE %s`, wc)).Scan(&abandonedCount); err != nil {
		return fmt.Errorf("counting abandoned_checkouts: %w", err)
	}
	if err := db.DB().QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "orders" WHERE %s`, wc)).Scan(&ordersCount); err != nil {
		return fmt.Errorf("counting orders: %w", err)
	}
	checkoutsStarted := abandonedCount + ordersCount

	rate := func(num, denom int) float64 {
		if denom == 0 {
			return 0
		}
		return round2(float64(num) / float64(denom) * 100)
	}

	type stageOut struct {
		Sessions             int     `json:"sessions"`
		CheckoutsStarted     int     `json:"checkouts_started"`
		AbandonedCheckouts   int     `json:"abandoned_checkouts"`
		Orders               int     `json:"orders"`
		SessionToCheckoutPct float64 `json:"session_to_checkout_pct"`
		CheckoutToOrderPct   float64 `json:"checkout_to_order_pct"`
		SessionToOrderPct    float64 `json:"session_to_order_pct"`
		Days                 int     `json:"days"`
	}
	out := stageOut{
		Sessions:             sessions,
		CheckoutsStarted:     checkoutsStarted,
		AbandonedCheckouts:   abandonedCount,
		Orders:               ordersCount,
		SessionToCheckoutPct: rate(checkoutsStarted, sessions),
		CheckoutToOrderPct:   rate(ordersCount, checkoutsStarted),
		SessionToOrderPct:    rate(ordersCount, sessions),
		Days:                 days,
	}
	return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(out), flags)
}

// extractScalarSessions pulls the first numeric value out of a TableResponse
// payload. Shopify's tableData.rows is an array of {column_name: value}
// objects (values arrive as strings even for INTEGER columns). For a no-BY
// aggregate it's a single-row, single-column object.
func extractScalarSessions(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("empty response")
	}
	// Surface ShopifyQL parseErrors first so a rejected query (missing scope,
	// schema change, syntax error) fails loudly instead of returning 0
	// sessions and silently producing 0% conversion rates downstream.
	if err := checkShopifyqlParseErrors(raw); err != nil {
		return 0, err
	}
	var env struct {
		ShopifyqlQuery struct {
			TableData struct {
				Rows []map[string]json.RawMessage `json:"rows"`
			} `json:"tableData"`
		} `json:"shopifyqlQuery"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, err
	}
	rows := env.ShopifyqlQuery.TableData.Rows
	if len(rows) == 0 {
		return 0, nil
	}
	for _, cell := range rows[0] {
		var asStr string
		if err := json.Unmarshal(cell, &asStr); err == nil {
			if n, err := strconv.Atoi(asStr); err == nil {
				return n, nil
			}
		}
		var asNum json.Number
		if err := json.Unmarshal(cell, &asNum); err == nil {
			if n, err := asNum.Int64(); err == nil {
				return int(n), nil
			}
		}
		return 0, fmt.Errorf("unexpected sessions cell shape: %s", string(cell))
	}
	return 0, nil
}
