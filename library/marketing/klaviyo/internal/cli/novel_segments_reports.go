// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSegmentsBuildCmd(flags *rootFlags) *cobra.Command {
	var name, tenure, purchased, notPurchased, boughtTogether, category, viewed, clicked, addedToCart, noEngagementDays, tag, lastPurchase, lapsed, recentBuyer, openedRecent, months, excludePurchased, excludeInFlow string
	var minOrders, maxOrders int
	var minItems int
	var minSpend, singleOrderMin, highAOV, lowAOV float64
	var repeatBuyer, oneTimeBuyer, seasonalBuyer, neverOpened, smsSubscribed, emailOnly, smsOnly, vip, wasVIP, giftBuyer, selfBuyer, exactMatch bool

	cmd := &cobra.Command{
		Use:     "build",
		Short:   "Create a segment from common ecommerce conditions",
		Example: `  klaviyo-pp-cli segments build --name "Journal Buyers 90d+ Tenure" --tenure ">90d" --purchased "Self Journal" --min-orders 1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"name":    name,
					"conditions": map[string]any{
						"tenure":             tenure,
						"purchased":          purchased,
						"not_purchased":      notPurchased,
						"bought_together":    boughtTogether,
						"category":           category,
						"viewed":             viewed,
						"clicked":            clicked,
						"added_to_cart":      addedToCart,
						"exact_match":        exactMatch,
						"min_orders":         minOrders,
						"max_orders":         maxOrders,
						"repeat_buyer":       repeatBuyer,
						"one_time_buyer":     oneTimeBuyer,
						"min_items":          minItems,
						"min_spend":          minSpend,
						"single_order_min":   singleOrderMin,
						"high_aov":           highAOV,
						"low_aov":            lowAOV,
						"no_engagement_days": noEngagementDays,
						"never_opened":       neverOpened,
						"sms_subscribed":     smsSubscribed,
						"email_only":         emailOnly,
						"sms_only":           smsOnly,
						"vip":                vip,
						"was_vip":            wasVIP,
						"gift_buyer":         giftBuyer,
						"self_buyer":         selfBuyer,
						"last_purchase":      lastPurchase,
						"lapsed":             lapsed,
						"recent_buyer":       recentBuyer,
						"seasonal_buyer":     seasonalBuyer,
						"month":              months,
						"opened_recent":      openedRecent,
						"exclude_purchased":  excludePurchased,
						"exclude_in_flow":    excludeInFlow,
					},
					"planned_steps": []string{"resolve_metrics", "build_segment_definition", "POST /api/segments", "optional_tag_segment"},
				}, flags)
			}
			if name == "" {
				return usageErr(fmt.Errorf("--name is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			def, err := buildSegmentDefinition(c, segmentBuildOptions{
				Tenure:           tenure,
				Purchased:        purchased,
				NotPurchased:     notPurchased,
				BoughtTogether:   boughtTogether,
				Category:         category,
				Viewed:           viewed,
				Clicked:          clicked,
				AddedToCart:      addedToCart,
				ExactMatch:       exactMatch,
				MinOrders:        minOrders,
				MaxOrders:        maxOrders,
				RepeatBuyer:      repeatBuyer,
				OneTimeBuyer:     oneTimeBuyer,
				MinItems:         minItems,
				MinSpend:         minSpend,
				SingleOrderMin:   singleOrderMin,
				HighAOV:          highAOV,
				LowAOV:           lowAOV,
				NoEngagementDays: noEngagementDays,
				NeverOpened:      neverOpened,
				SMSSubscribed:    smsSubscribed,
				EmailOnly:        emailOnly,
				SMSOnly:          smsOnly,
				VIP:              vip,
				WasVIP:           wasVIP,
				GiftBuyer:        giftBuyer,
				SelfBuyer:        selfBuyer,
				LastPurchase:     lastPurchase,
				Lapsed:           lapsed,
				RecentBuyer:      recentBuyer,
				SeasonalBuyer:    seasonalBuyer,
				Months:           months,
				OpenedRecent:     openedRecent,
				ExcludePurchased: excludePurchased,
				ExcludeInFlow:    excludeInFlow,
			})
			if err != nil {
				return err
			}
			body := jsonAPIBody("segment", map[string]any{"name": name, "definition": def}, nil)
			resp, status, err := c.Post("/api/segments", body)
			if err != nil {
				return classifyAPIError(err)
			}
			segmentID := jsonAPIID(resp)
			result := map[string]any{
				"segment_id": segmentID,
				"name":       name,
				"status":     status,
				"definition": def,
				"response":   mustJSONAny(resp),
			}
			if tag != "" && segmentID != "" {
				tagResult, tagErr := tagSegmentByName(c, segmentID, tag)
				if tagErr != nil {
					result["tag_error"] = tagErr.Error()
				} else {
					result["tag"] = tagResult
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Segment name")
	cmd.Flags().StringVar(&tenure, "tenure", "", `Profile-created filter: >90d, <30d, or 30d-90d`)
	cmd.Flags().StringVar(&purchased, "purchased", "", "Require a Placed Order with $ProductName matching this value")
	cmd.Flags().StringVar(&notPurchased, "not-purchased", "", "Require zero Placed Order events with $ProductName matching this value")
	cmd.Flags().StringVar(&boughtTogether, "bought-together", "", "Require Placed Order events for every comma-separated product")
	cmd.Flags().StringVar(&category, "category", "", "Require a Placed Order where ProductCategories contains this value")
	cmd.Flags().StringVar(&viewed, "viewed", "", "Require a Viewed Product event with ProductName matching this value")
	cmd.Flags().StringVar(&clicked, "clicked", "", "Require a Clicked Email event with URL containing this product slug")
	cmd.Flags().StringVar(&addedToCart, "added-to-cart", "", "Require an Added to Cart event with ProductName matching this value")
	cmd.Flags().BoolVar(&exactMatch, "exact-match", false, "Use equals instead of contains for product-name metric filters")
	cmd.Flags().IntVar(&minOrders, "min-orders", 0, "Minimum total Placed Order count")
	cmd.Flags().IntVar(&maxOrders, "max-orders", 0, "Maximum total Placed Order count")
	cmd.Flags().BoolVar(&repeatBuyer, "repeat-buyer", false, "Shortcut for --min-orders 2")
	cmd.Flags().BoolVar(&oneTimeBuyer, "one-time-buyer", false, "Shortcut for --min-orders 1 --max-orders 1")
	cmd.Flags().IntVar(&minItems, "min-items", 0, "Minimum total purchased item quantity across orders")
	cmd.Flags().Float64Var(&minSpend, "min-spend", 0, "Minimum lifetime spend")
	cmd.Flags().Float64Var(&singleOrderMin, "single-order-min", 0, "Minimum value for at least one order")
	cmd.Flags().Float64Var(&highAOV, "high-aov", 0, "Minimum average order value")
	cmd.Flags().Float64Var(&lowAOV, "low-aov", 0, "Maximum average order value")
	cmd.Flags().StringVar(&noEngagementDays, "no-engagement-days", "", "Require zero opens and clicks in the last N days")
	cmd.Flags().BoolVar(&neverOpened, "never-opened", false, "Require zero Opened Email events all time")
	cmd.Flags().BoolVar(&smsSubscribed, "sms-subscribed", false, "Require SMS marketing consent")
	cmd.Flags().BoolVar(&emailOnly, "email-only", false, "Require email consent and no SMS consent")
	cmd.Flags().BoolVar(&smsOnly, "sms-only", false, "Require SMS consent and no email consent")
	cmd.Flags().BoolVar(&vip, "vip", false, "Require predicted_customer_lifetime_value greater than 500")
	cmd.Flags().BoolVar(&wasVIP, "was-vip", false, "Shortcut for historical VIP spend, equivalent to --min-spend 200")
	cmd.Flags().BoolVar(&giftBuyer, "gift-buyer", false, "Require likely gift purchase signals")
	cmd.Flags().BoolVar(&selfBuyer, "self-buyer", false, "Require shipping and billing address match signal")
	cmd.Flags().StringVar(&tag, "tag", "", "Create or reuse a tag and attach it to the segment")
	cmd.Flags().StringVar(&lastPurchase, "last-purchase", "", `Days since last Placed Order, for example ">60d"`)
	cmd.Flags().StringVar(&lapsed, "lapsed", "", `Alias for days since last purchase, for example ">90d"`)
	cmd.Flags().StringVar(&recentBuyer, "recent-buyer", "", `Require purchase recency, for example "<14d"`)
	cmd.Flags().BoolVar(&seasonalBuyer, "seasonal-buyer", false, "Restrict purchases to specific months supplied by --month")
	cmd.Flags().StringVar(&months, "month", "", "Comma-separated purchase months, for example 11,12")
	cmd.Flags().StringVar(&openedRecent, "opened-recent", "", `Require recent Opened Email activity, for example "<30d"`)
	cmd.Flags().StringVar(&excludePurchased, "exclude-purchased", "", `Require zero purchases in the lookback window, for example "<7d"`)
	cmd.Flags().StringVar(&excludeInFlow, "exclude-in-flow", "", "Exclude profiles currently active in this flow name or ID")
	return cmd
}

type segmentBuildOptions struct {
	Tenure           string
	Purchased        string
	NotPurchased     string
	BoughtTogether   string
	Category         string
	Viewed           string
	Clicked          string
	AddedToCart      string
	ExactMatch       bool
	MinOrders        int
	MaxOrders        int
	RepeatBuyer      bool
	OneTimeBuyer     bool
	MinItems         int
	MinSpend         float64
	SingleOrderMin   float64
	HighAOV          float64
	LowAOV           float64
	NoEngagementDays string
	NeverOpened      bool
	SMSSubscribed    bool
	EmailOnly        bool
	SMSOnly          bool
	VIP              bool
	WasVIP           bool
	GiftBuyer        bool
	SelfBuyer        bool
	LastPurchase     string
	Lapsed           string
	RecentBuyer      string
	SeasonalBuyer    bool
	Months           string
	OpenedRecent     string
	ExcludePurchased string
	ExcludeInFlow    string
}

func buildSegmentDefinition(c flowClient, opts segmentBuildOptions) (map[string]any, error) {
	var conditions []any
	metricIDs := map[string]string{}
	resolve := func(name string) (string, error) {
		if id := metricIDs[name]; id != "" {
			return id, nil
		}
		id, err := resolveMetricID(c, name)
		if err != nil {
			return "", err
		}
		metricIDs[name] = id
		return id, nil
	}
	if opts.RepeatBuyer && opts.MinOrders < 2 {
		opts.MinOrders = 2
	}
	if opts.OneTimeBuyer {
		if opts.MinOrders == 0 {
			opts.MinOrders = 1
		}
		if opts.MaxOrders == 0 || opts.MaxOrders > 1 {
			opts.MaxOrders = 1
		}
	}
	if opts.WasVIP && opts.MinSpend < 200 {
		opts.MinSpend = 200
	}
	if opts.Tenure != "" {
		cond, err := profileDateCondition("created", opts.Tenure)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, cond)
	}
	if opts.Purchased != "" {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, "", productMetricFilter(opts.Purchased, opts.ExactMatch)))
	}
	if opts.NotPurchased != "" {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "equals", 0, "", productMetricFilter(opts.NotPurchased, opts.ExactMatch)))
	}
	if opts.BoughtTogether != "" {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		for _, product := range splitCSV(opts.BoughtTogether) {
			conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, "", productMetricFilter(product, opts.ExactMatch)))
		}
	}
	if opts.Category != "" {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, "", []any{
			map[string]any{"type": "string", "field": "ProductCategories", "operator": "contains", "value": opts.Category},
		}))
	}
	if opts.Viewed != "" {
		id, err := resolve("Viewed Product")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, timeframeLastDays(180), productMetricFilter(opts.Viewed, opts.ExactMatch)))
	}
	if opts.Clicked != "" {
		id, err := resolve("Clicked Email")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, timeframeLastDays(180), clickedProductFilter(opts.Clicked)))
	}
	if opts.AddedToCart != "" {
		id, err := resolve("Added to Cart")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, timeframeLastDays(180), productMetricFilter(opts.AddedToCart, opts.ExactMatch)))
	}
	if opts.MinOrders > 0 {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", opts.MinOrders, "", nil))
	}
	if opts.MaxOrders > 0 {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "less-or-equal", opts.MaxOrders, "", nil))
	}
	if opts.MinItems > 0 {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricMeasurementCondition(id, "sum", "greater-or-equal", opts.MinItems, "", []any{
			map[string]any{"type": "numeric", "field": "Quantity", "operator": "greater-than", "value": 0},
		}))
	}
	if opts.MinSpend > 0 {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricMeasurementCondition(id, "sum_value", "greater-or-equal", opts.MinSpend, "", nil))
	}
	if opts.SingleOrderMin > 0 {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricMeasurementCondition(id, "max_value", "greater-or-equal", opts.SingleOrderMin, "", nil))
	}
	if opts.HighAOV > 0 {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricMeasurementCondition(id, "average_value", "greater-or-equal", opts.HighAOV, "", nil))
	}
	if opts.LowAOV > 0 {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricMeasurementCondition(id, "average_value", "less-or-equal", opts.LowAOV, "", nil))
	}
	if opts.NoEngagementDays != "" {
		days, err := strconv.Atoi(strings.TrimSuffix(opts.NoEngagementDays, "d"))
		if err != nil || days <= 0 {
			return nil, usageErr(fmt.Errorf("--no-engagement-days must be a positive day count"))
		}
		for _, metric := range []string{"Opened Email", "Clicked Email"} {
			id, err := resolve(metric)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, metricCountCondition(id, "equals", 0, timeframeLastDays(days), nil))
		}
	}
	if opts.NeverOpened {
		id, err := resolve("Opened Email")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "equals", 0, "", nil))
	}
	if opts.SMSSubscribed {
		conditions = append(conditions, profileBoolCondition("subscriptions.sms.marketing.can_receive_sms_marketing", "equals", true))
	}
	if opts.EmailOnly {
		conditions = append(conditions, profileBoolCondition("subscriptions.email.marketing.can_receive_email_marketing", "equals", true))
		conditions = append(conditions, profileBoolCondition("subscriptions.sms.marketing.can_receive_sms_marketing", "equals", false))
	}
	if opts.SMSOnly {
		conditions = append(conditions, profileBoolCondition("subscriptions.sms.marketing.can_receive_sms_marketing", "equals", true))
		conditions = append(conditions, profileBoolCondition("subscriptions.email.marketing.can_receive_email_marketing", "equals", false))
	}
	if opts.VIP {
		conditions = append(conditions, profileNumberCondition("predicted_customer_lifetime_value", "greater-than", 500))
	}
	if opts.GiftBuyer {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, "", []any{
			map[string]any{"type": "string", "field": "ShippingAddress", "operator": "not-equals-field", "value": "BillingAddress"},
		}))
	}
	if opts.SelfBuyer {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, "", []any{
			map[string]any{"type": "string", "field": "ShippingAddress", "operator": "equals-field", "value": "BillingAddress"},
		}))
	}
	if opts.LastPurchase != "" || opts.Lapsed != "" {
		expr := opts.LastPurchase
		if expr == "" {
			expr = opts.Lapsed
		}
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		days, err := parseDayComparator(expr)
		if err != nil {
			return nil, err
		}
		operator := "before"
		if strings.HasPrefix(expr, "<") {
			operator = "after"
		}
		conditions = append(conditions, map[string]any{
			"type":               "profile-metric",
			"metric_id":          id,
			"measurement":        "count",
			"measurement_filter": map[string]any{"type": "numeric", "operator": "greater-or-equal", "value": 1},
			"timeframe_filter":   map[string]any{"type": "date", "operator": operator, "date": time.Now().AddDate(0, 0, -days).Format("2006-01-02")},
			"metric_filters":     nil,
		})
	}
	if opts.RecentBuyer != "" {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		days, err := parseDayComparator(opts.RecentBuyer)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, timeframeLastDays(days), nil))
	}
	if opts.SeasonalBuyer {
		if opts.Months == "" {
			return nil, usageErr(fmt.Errorf("--month is required with --seasonal-buyer"))
		}
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, "", []any{
			map[string]any{"type": "numeric", "field": "Month", "operator": "in", "values": splitCSV(opts.Months)},
		}))
	}
	if opts.OpenedRecent != "" {
		id, err := resolve("Opened Email")
		if err != nil {
			return nil, err
		}
		days, err := parseDayComparator(opts.OpenedRecent)
		if err != nil {
			return nil, err
		}
		operator := "in-the-last"
		if strings.HasPrefix(opts.OpenedRecent, ">") {
			operator = "before"
		}
		var timeframe any = timeframeLastDays(days)
		if operator == "before" {
			timeframe = map[string]any{"type": "date", "operator": "before", "date": time.Now().AddDate(0, 0, -days).Format("2006-01-02")}
		}
		conditions = append(conditions, metricCountCondition(id, "greater-or-equal", 1, timeframe, nil))
	}
	if opts.ExcludePurchased != "" {
		id, err := resolve("Placed Order")
		if err != nil {
			return nil, err
		}
		days, err := parseDayComparator(opts.ExcludePurchased)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, metricCountCondition(id, "equals", 0, timeframeLastDays(days), nil))
	}
	if opts.ExcludeInFlow != "" {
		conditions = append(conditions, map[string]any{
			"type":     "profile-not-in-flow",
			"flow":     opts.ExcludeInFlow,
			"operator": "not-active",
		})
	}
	if len(conditions) == 0 {
		return nil, usageErr(fmt.Errorf("at least one segment condition flag is required"))
	}
	return map[string]any{
		"condition_groups": []any{
			map[string]any{"conditions": conditions},
		},
	}, nil
}

func profileDateCondition(field, expr string) (map[string]any, error) {
	now := time.Now()
	switch {
	case strings.HasPrefix(expr, ">"):
		days, err := parseDayComparator(expr)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "profile-property", "property": field, "operator": "before", "value": now.AddDate(0, 0, -days).Format("2006-01-02")}, nil
	case strings.HasPrefix(expr, "<"):
		days, err := parseDayComparator(expr)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "profile-property", "property": field, "operator": "after", "value": now.AddDate(0, 0, -days).Format("2006-01-02")}, nil
	case strings.Contains(expr, "-"):
		parts := strings.SplitN(expr, "-", 2)
		lo, err := strconv.Atoi(strings.TrimSuffix(parts[0], "d"))
		if err != nil {
			return nil, usageErr(fmt.Errorf("invalid tenure range %q", expr))
		}
		hi, err := strconv.Atoi(strings.TrimSuffix(parts[1], "d"))
		if err != nil {
			return nil, usageErr(fmt.Errorf("invalid tenure range %q", expr))
		}
		return map[string]any{
			"type":       "profile-property-between",
			"property":   field,
			"lower_date": now.AddDate(0, 0, -hi).Format("2006-01-02"),
			"upper_date": now.AddDate(0, 0, -lo).Format("2006-01-02"),
		}, nil
	default:
		return nil, usageErr(fmt.Errorf("invalid date expression %q", expr))
	}
}

func parseDayComparator(expr string) (int, error) {
	cleaned := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(expr, ">"), "<"), " ")
	cleaned = strings.TrimSuffix(cleaned, "d")
	days, err := strconv.Atoi(cleaned)
	if err != nil || days <= 0 {
		return 0, usageErr(fmt.Errorf("invalid day expression %q", expr))
	}
	return days, nil
}

func metricCountCondition(metricID, operator string, value int, timeframe any, filters []any) map[string]any {
	return metricMeasurementCondition(metricID, "count", operator, value, timeframe, filters)
}

func metricMeasurementCondition(metricID, measurement, operator string, value any, timeframe any, filters []any) map[string]any {
	cond := map[string]any{
		"type":               "profile-metric",
		"metric_id":          metricID,
		"measurement":        measurement,
		"measurement_filter": map[string]any{"type": "numeric", "operator": operator, "value": value},
		"metric_filters":     filters,
	}
	if timeframe != "" && timeframe != nil {
		cond["timeframe_filter"] = timeframe
	}
	return cond
}

func productMetricFilter(product string, exact bool) []any {
	operator := "contains"
	if exact {
		operator = "equals"
	}
	return []any{map[string]any{
		"type":     "string",
		"field":    "ProductName",
		"operator": operator,
		"value":    product,
	}}
}

func clickedProductFilter(product string) []any {
	return []any{map[string]any{
		"type":     "string",
		"field":    "URL",
		"operator": "contains",
		"value":    "/products/" + productSlug(product),
	}}
}

func productSlug(product string) string {
	s := strings.ToLower(strings.TrimSpace(product))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func timeframeLastDays(days int) map[string]any {
	return map[string]any{"type": "date", "operator": "in-the-last", "unit": "day", "quantity": days}
}

func profileNumberCondition(property, operator string, value float64) map[string]any {
	return map[string]any{"type": "profile-property", "property": property, "operator": operator, "value": value}
}

func profileBoolCondition(property, operator string, value bool) map[string]any {
	return map[string]any{"type": "profile-property", "property": property, "operator": operator, "value": value}
}

func tagSegmentByName(c flowClient, segmentID, tagName string) (map[string]any, error) {
	tagID, err := findTagIDByName(c, tagName)
	if err != nil {
		return nil, err
	}
	created := false
	if tagID == "" {
		resp, _, err := c.Post("/api/tags", jsonAPIBody("tag", map[string]any{"name": tagName}, nil))
		if err != nil {
			return nil, classifyAPIError(err)
		}
		tagID = jsonAPIID(resp)
		created = true
	}
	body := map[string]any{"data": []any{map[string]any{"type": "segment", "id": segmentID}}}
	_, _, err = c.Post("/api/tags/"+url.PathEscape(tagID)+"/relationships/segments", body)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	return map[string]any{"tag_id": tagID, "name": tagName, "created": created}, nil
}

func findTagIDByName(c flowClient, name string) (string, error) {
	cursor := ""
	for {
		params := map[string]string{"fields[tag]": "name", "page[size]": "50"}
		if cursor != "" {
			params["page[cursor]"] = cursor
		}
		resp, err := c.Get("/api/tags", params)
		if err != nil {
			return "", classifyAPIError(err)
		}
		items, next, err := parseJSONAPICollection(resp)
		if err != nil {
			return "", err
		}
		for _, item := range items {
			if stringFromMapPath(item, "attributes.name") == name {
				return fmt.Sprint(item["id"]), nil
			}
		}
		if next == "" {
			return "", nil
		}
		cursor = next
	}
}

func newSegmentsOverlapCmd(flags *rootFlags) *cobra.Command {
	var ids []string
	cmd := &cobra.Command{
		Use:     "overlap",
		Short:   "Estimate overlap between two segments",
		Example: "  klaviyo-pp-cli segments overlap --id SEG1 --id SEG2 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "ids": ids, "planned_steps": []string{"fetch_segment_profiles", "intersect_profile_ids"}}, flags)
			}
			if len(ids) != 2 {
				return usageErr(fmt.Errorf("provide exactly two --id values"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			a, aEstimated, err := fetchSegmentProfileIDs(c, ids[0], 5000)
			if err != nil {
				return err
			}
			b, bEstimated, err := fetchSegmentProfileIDs(c, ids[1], 5000)
			if err != nil {
				return err
			}
			seen := map[string]bool{}
			for _, id := range a {
				seen[id] = true
			}
			var overlap []string
			for _, id := range b {
				if seen[id] {
					overlap = append(overlap, id)
				}
			}
			denom := len(a)
			if len(b) < denom {
				denom = len(b)
			}
			pct := 0.0
			if denom > 0 {
				pct = float64(len(overlap)) / float64(denom) * 100
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"segment_a":          ids[0],
				"segment_b":          ids[1],
				"segment_a_profiles": len(a),
				"segment_b_profiles": len(b),
				"overlap_count":      len(overlap),
				"overlap_percentage": pct,
				"estimated":          aEstimated || bEstimated,
				"sample_limit":       5000,
				"summary":            fmt.Sprintf("%d shared profiles across %d/%d sampled profiles", len(overlap), len(a), len(b)),
			}, flags)
		},
	}
	cmd.Flags().StringArrayVar(&ids, "id", nil, "Segment ID; provide exactly two")
	return cmd
}

func fetchSegmentProfileIDs(c flowClient, segmentID string, limit int) ([]string, bool, error) {
	var ids []string
	cursor := ""
	estimated := false
	for {
		params := map[string]string{"fields[profile]": "id,email", "page[size]": "50"}
		if cursor != "" {
			params["page[cursor]"] = cursor
		}
		resp, err := c.Get("/api/segments/"+url.PathEscape(segmentID)+"/profiles", params)
		if err != nil {
			return nil, false, classifyAPIError(err)
		}
		items, next, err := parseJSONAPICollection(resp)
		if err != nil {
			return nil, false, err
		}
		for _, item := range items {
			id := fmt.Sprint(item["id"])
			if id != "" && id != "<nil>" {
				ids = append(ids, id)
			}
			if len(ids) >= limit {
				return ids, next != "", nil
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return ids, estimated, nil
}

func deleteSegment(c flowClient, segmentID string) error {
	if strings.TrimSpace(segmentID) == "" {
		return fmt.Errorf("segment id is empty")
	}
	_, _, err := c.Delete("/api/segments/" + url.PathEscape(segmentID))
	if err != nil {
		return classifyAPIError(err)
	}
	return nil
}

func newFlowsHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Scan live flows for common delivery problems",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_live_flows", "inspect_definitions", "check_templates", "query_trigger_metric_activity"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report, err := scanFlowHealth(c, time.Now())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	return cmd
}

func scanFlowHealth(c flowClient, now time.Time) (map[string]any, error) {
	flows, err := fetchAllJSONAPI(c, "/api/flows", map[string]string{"fields[flow]": "name,status,trigger_type", "page[size]": "50"}, 0)
	if err != nil {
		return nil, err
	}
	var issues []map[string]any
	checked := 0
	templateCache := map[string]bool{}
	metricCache := map[string]float64{}
	for _, flow := range flows {
		if strings.ToLower(stringFromMapPath(flow, "attributes.status")) != "live" {
			continue
		}
		checked++
		flowID := fmt.Sprint(flow["id"])
		flowName := stringFromMapPath(flow, "attributes.name")
		resp, err := c.Get("/api/flows/"+url.PathEscape(flowID), map[string]string{"additional-fields[flow]": "definition"})
		if err != nil {
			issues = append(issues, flowIssue(flowID, flowName, "definition_unreadable", err.Error()))
			continue
		}
		def := anyPath(mustJSONAny(resp), "data.attributes.definition")
		defMap, _ := def.(map[string]any)
		if defMap == nil {
			issues = append(issues, flowIssue(flowID, flowName, "missing_definition", "flow did not return a definition"))
			continue
		}
		actions, _ := defMap["actions"].([]any)
		if len(actions) == 0 {
			issues = append(issues, flowIssue(flowID, flowName, "live_flow_no_actions", "flow is live but has no actions"))
		}
		draftCount, manualCount, sendCount := 0, 0, 0
		for _, raw := range actions {
			action, _ := raw.(map[string]any)
			status := strings.ToLower(stringFromMapPath(action, "data.status"))
			actionType := fmt.Sprint(action["type"])
			if status == "draft" {
				draftCount++
			}
			if status == "manual" && actionType == "send-email" {
				manualCount++
			}
			if actionType == "send-email" {
				sendCount++
			}
			for _, templateID := range collectTemplateIDs(action) {
				exists, ok := templateCache[templateID]
				if !ok {
					_, tErr := c.Get("/api/templates/"+url.PathEscape(templateID), map[string]string{"fields[template]": "name"})
					exists = tErr == nil
					templateCache[templateID] = exists
				}
				if !exists {
					issues = append(issues, flowIssue(flowID, flowName, "broken_template_reference", "template "+templateID+" could not be fetched"))
				}
			}
		}
		if sendCount > 0 && draftCount == len(actions) {
			issues = append(issues, flowIssue(flowID, flowName, "live_flow_all_actions_draft", "all actions are draft"))
		}
		if manualCount > 0 {
			issues = append(issues, flowIssue(flowID, flowName, "manual_email_actions", fmt.Sprintf("%d send-email action(s) are manual", manualCount)))
		}
		for _, metricID := range collectTriggerMetricIDs(defMap) {
			count, ok := metricCache[metricID]
			if !ok {
				count = queryMetricCount(c, metricID, now.AddDate(0, 0, -30), now)
				metricCache[metricID] = count
			}
			if count == 0 {
				issues = append(issues, flowIssue(flowID, flowName, "dead_trigger_metric", "trigger metric "+metricID+" had zero events in the last 30 days"))
			}
		}
	}
	return map[string]any{
		"checked_at":    now.UTC().Format(time.RFC3339),
		"live_flows":    checked,
		"issues":        issues,
		"issue_count":   len(issues),
		"healthy":       len(issues) == 0,
		"flows_scanned": len(flows),
	}, nil
}

func flowIssue(flowID, flowName, kind, detail string) map[string]any {
	return map[string]any{"flow_id": flowID, "flow_name": flowName, "type": kind, "detail": detail}
}

func collectTemplateIDs(v any) []string {
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, v := range t {
				lk := strings.ToLower(k)
				if (lk == "template_id" || lk == "templateid" || lk == "template") && fmt.Sprint(v) != "" && fmt.Sprint(v) != "<nil>" {
					if s, ok := v.(string); ok {
						out = append(out, s)
					}
				}
				walk(v)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(v)
	return uniqueStrings(out)
}

func collectTriggerMetricIDs(def map[string]any) []string {
	var ids []string
	triggers, _ := def["triggers"].([]any)
	for _, raw := range triggers {
		trigger, _ := raw.(map[string]any)
		if strings.EqualFold(fmt.Sprint(trigger["type"]), "metric") {
			if id := fmt.Sprint(trigger["id"]); id != "" && id != "<nil>" {
				ids = append(ids, id)
			}
		}
	}
	return uniqueStrings(ids)
}

func queryMetricCount(c flowClient, metricID string, since, until time.Time) float64 {
	body := metricAggregateBody(metricID, []string{"count"}, nil, since, until)
	resp, _, err := c.Post("/api/metric-aggregates", body)
	if err != nil {
		return -1
	}
	return sumMeasurement(resp, "count")
}

func newCampaignsScheduleConflictsCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:     "schedule-conflicts",
		Short:   "Show scheduled campaigns and same-day audience conflicts",
		Example: "  klaviyo-pp-cli campaigns schedule-conflicts --days 7 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "days": days, "planned_steps": []string{"fetch_scheduled_campaigns", "group_by_send_date", "compare_audience_ids"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			campaigns, err := fetchAllJSONAPI(c, "/api/campaigns", map[string]string{"filter": `equals(status,"Scheduled")`, "fields[campaign]": "name,audiences,send_time,status", "page[size]": "50"}, 0)
			if err != nil {
				return err
			}
			result := scheduledCampaignConflicts(campaigns, time.Now(), days)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Number of days ahead to inspect")
	return cmd
}

func scheduledCampaignConflicts(campaigns []map[string]any, now time.Time, days int) map[string]any {
	until := now.AddDate(0, 0, days)
	byDate := map[string][]map[string]any{}
	for _, campaign := range campaigns {
		sendTime := stringFromMapPath(campaign, "attributes.send_time")
		if sendTime == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, sendTime)
		if err != nil {
			if d, derr := time.Parse("2006-01-02", sendTime); derr == nil {
				t = d
			} else {
				continue
			}
		}
		if t.Before(now.Add(-time.Hour)) || t.After(until) {
			continue
		}
		date := t.Format("2006-01-02")
		byDate[date] = append(byDate[date], map[string]any{
			"id":           fmt.Sprint(campaign["id"]),
			"name":         stringFromMapPath(campaign, "attributes.name"),
			"send_time":    sendTime,
			"audience_ids": campaignAudienceIDs(campaign),
		})
	}
	var conflicts []map[string]any
	for date, items := range byDate {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				overlap := intersectStrings(anyStringSlice(items[i]["audience_ids"]), anyStringSlice(items[j]["audience_ids"]))
				if len(overlap) > 0 {
					conflicts = append(conflicts, map[string]any{"date": date, "campaign_a": items[i], "campaign_b": items[j], "shared_audience_ids": overlap})
				}
			}
		}
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	calendar := make([]map[string]any, 0, len(dates))
	for _, d := range dates {
		calendar = append(calendar, map[string]any{"date": d, "campaigns": byDate[d]})
	}
	return map[string]any{"days": days, "calendar": calendar, "conflicts": conflicts, "conflict_count": len(conflicts)}
}

func campaignAudienceIDs(campaign map[string]any) []string {
	var ids []string
	if vals, ok := anyPath(campaign, "attributes.audiences.included").([]any); ok {
		for _, v := range vals {
			switch t := v.(type) {
			case string:
				ids = append(ids, t)
			case map[string]any:
				if id := fmt.Sprint(t["id"]); id != "" && id != "<nil>" {
					ids = append(ids, id)
				}
			}
		}
	}
	return uniqueStrings(ids)
}

func newProfilesPruneCmd(flags *rootFlags) *cobra.Command {
	var noEngagementDays, max int
	var suppress, bounced bool
	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Identify unengaged profiles and optionally suppress them",
		Example: "  klaviyo-pp-cli profiles prune --no-engagement-days 180 --dry-run --json\n  klaviyo-pp-cli profiles prune --no-engagement-days 180 --suppress --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "no_engagement_days": noEngagementDays, "suppress": suppress, "bounced": bounced, "max": max, "planned_steps": []string{"create_temporary_segment", "fetch_profiles", "optional_bulk_suppress"}}, flags)
			}
			if noEngagementDays <= 0 {
				return usageErr(fmt.Errorf("--no-engagement-days is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			name := fmt.Sprintf("Temporary Unengaged %dd %s", noEngagementDays, time.Now().UTC().Format("20060102T150405Z"))
			def, err := buildSegmentDefinition(c, segmentBuildOptions{NoEngagementDays: strconv.Itoa(noEngagementDays)})
			if err != nil {
				return err
			}
			if bounced {
				bouncedID, err := resolveMetricID(c, "Bounced Email")
				if err != nil {
					return err
				}
				groups, _ := def["condition_groups"].([]any)
				if len(groups) > 0 {
					if group, ok := groups[0].(map[string]any); ok {
						conds, _ := group["conditions"].([]any)
						group["conditions"] = append(conds, metricCountCondition(bouncedID, "greater-or-equal", 1, "", []any{map[string]any{"type": "string", "field": "Bounce Type", "operator": "equals", "value": "hard"}}))
					}
				}
			}
			resp, _, err := c.Post("/api/segments", jsonAPIBody("segment", map[string]any{"name": name, "definition": def}, nil))
			if err != nil {
				return classifyAPIError(err)
			}
			segmentID := jsonAPIID(resp)
			if segmentID == "" {
				return fmt.Errorf("created temporary segment response did not include an id")
			}
			profileIDs, estimated, err := fetchSegmentProfileIDs(c, segmentID, max)
			if err != nil {
				if cleanupErr := deleteSegment(c, segmentID); cleanupErr != nil {
					return fmt.Errorf("%w; additionally failed to delete temporary segment %s: %v", err, segmentID, cleanupErr)
				}
				return err
			}
			if err := deleteSegment(c, segmentID); err != nil {
				return err
			}
			result := map[string]any{"segment_id": segmentID, "segment_name": name, "temporary_segment_deleted": true, "count": len(profileIDs), "sample_profile_ids": firstStrings(profileIDs, 25), "estimated": estimated, "max": max}
			if suppress && len(profileIDs) > 0 {
				body := map[string]any{"data": map[string]any{"type": "profile-suppression-bulk-create-job", "attributes": map[string]any{"profiles": profileIDData(profileIDs)}}}
				jobResp, status, err := c.Post("/api/profile-suppression-bulk-create-jobs", body)
				if err != nil {
					return classifyAPIError(err)
				}
				result["suppression_status"] = status
				result["suppression_job"] = mustJSONAny(jobResp)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&noEngagementDays, "no-engagement-days", 0, "Profiles with zero opens and clicks in this many days")
	cmd.Flags().BoolVar(&suppress, "suppress", false, "Suppress matching profiles; never deletes profiles")
	cmd.Flags().BoolVar(&bounced, "bounced", false, "Also require a hard bounce signal")
	cmd.Flags().IntVar(&max, "max", 1000, "Maximum profiles to suppress or sample in one run")
	return cmd
}

func profileIDData(ids []string) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"type": "profile", "id": id})
	}
	return out
}

func newReportDeliverabilityCmd(flags *rootFlags) *cobra.Command {
	var last string
	cmd := &cobra.Command{
		Use:     "deliverability",
		Short:   "Bounce rates by email domain",
		Example: "  klaviyo-pp-cli report deliverability --last 30d --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "planned_steps": []string{"resolve_received_and_bounced_metrics", "query_metric_aggregates_by_domain", "calculate_bounce_rates"}}, flags)
			}
			days, err := strconv.Atoi(strings.TrimSuffix(last, "d"))
			if err != nil || days <= 0 {
				return usageErr(fmt.Errorf("--last must be a positive duration like 30d"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report, err := buildDeliverabilityReport(c, time.Now().AddDate(0, 0, -days), time.Now())
			if err != nil {
				return err
			}
			report["last"] = last
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "30d", "Lookback window, for example 30d")
	return cmd
}

func buildDeliverabilityReport(c flowClient, since, until time.Time) (map[string]any, error) {
	receivedID, err := resolveMetricID(c, "Received Email")
	if err != nil {
		return nil, err
	}
	bouncedID, err := resolveMetricID(c, "Bounced Email")
	if err != nil {
		return nil, err
	}
	received, err := aggregateByDimension(c, receivedID, "Email Domain", since, until)
	if err != nil {
		return nil, err
	}
	bounced, err := aggregateByDimension(c, bouncedID, "Email Domain", since, until)
	if err != nil {
		return nil, err
	}
	domains := map[string]bool{}
	for k := range received {
		domains[k] = true
	}
	for k := range bounced {
		domains[k] = true
	}
	var rows []map[string]any
	for domain := range domains {
		r := received[domain]
		b := bounced[domain]
		rate := 0.0
		if r > 0 {
			rate = b / r * 100
		}
		rows = append(rows, map[string]any{"domain": domain, "received": int(r), "bounced": int(b), "bounce_rate": rate, "flagged": rate > 2})
	}
	sort.Slice(rows, func(i, j int) bool {
		return anyFloat(rows[i]["bounce_rate"]) > anyFloat(rows[j]["bounce_rate"])
	})
	return map[string]any{"since": since.Format("2006-01-02"), "until": until.Format("2006-01-02"), "threshold": 2.0, "rows": rows}, nil
}

func aggregateByDimension(c flowClient, metricID, dimension string, since, until time.Time) (map[string]float64, error) {
	body := metricAggregateBody(metricID, []string{"count"}, []string{dimension}, since, until)
	resp, _, err := c.Post("/api/metric-aggregates", body)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	return metricAggregateRows(resp, "count"), nil
}

func newUniversalContentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "universal-content", Short: "Manage shared universal content blocks"}
	cmd.AddCommand(newUniversalContentSyncCmd(flags))
	return cmd
}

func newUniversalContentSyncCmd(flags *rootFlags) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Create or update universal content blocks from local HTML files",
		Example: "  klaviyo-pp-cli universal-content sync --dir ./email-blocks/ --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "dir": dir, "planned_steps": []string{"read_html_files", "fetch_template_universal_content", "create_or_update_blocks"}}, flags)
			}
			if dir == "" {
				return usageErr(fmt.Errorf("--dir is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			files, err := filepath.Glob(filepath.Join(dir, "*.html"))
			if err != nil {
				return err
			}
			existing, err := fetchUniversalContentByName(c)
			if err != nil {
				return err
			}
			var results []map[string]any
			for _, file := range files {
				b, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading %s: %w", file, err)
				}
				name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
				attrs := map[string]any{"name": name, "definition": map[string]any{"type": "html", "content": string(b)}}
				if id := existing[name]; id != "" {
					resp, status, err := c.Patch("/api/template-universal-content/"+url.PathEscape(id), jsonAPIBody("template-universal-content", attrs, nil))
					if err != nil {
						results = append(results, map[string]any{"file": file, "name": name, "id": id, "action": "error", "error": err.Error()})
					} else {
						results = append(results, map[string]any{"file": file, "name": name, "id": id, "action": "updated", "status": status, "response": mustJSONAny(resp)})
					}
					continue
				}
				resp, status, err := c.Post("/api/template-universal-content", jsonAPIBody("template-universal-content", attrs, nil))
				if err != nil {
					results = append(results, map[string]any{"file": file, "name": name, "action": "error", "error": err.Error()})
				} else {
					results = append(results, map[string]any{"file": file, "name": name, "id": jsonAPIID(resp), "action": "created", "status": status, "response": mustJSONAny(resp)})
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dir": dir, "files": len(files), "results": results}, flags)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Directory containing .html universal content blocks")
	return cmd
}

func fetchUniversalContentByName(c flowClient) (map[string]string, error) {
	items, err := fetchAllJSONAPI(c, "/api/template-universal-content", map[string]string{"fields[template-universal-content]": "name", "page[size]": "50"}, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, item := range items {
		name := stringFromMapPath(item, "attributes.name")
		if name != "" {
			out[name] = fmt.Sprint(item["id"])
		}
	}
	return out, nil
}

func updateMessageTemplatesForImageSwap(c flowClient, scope, oldURL, newURL string) ([]map[string]any, int, int, error) {
	templates, err := collectMessageTemplates(c, scope)
	if err != nil {
		return nil, 0, 0, err
	}
	seen := map[string]bool{}
	var results []map[string]any
	matched := 0
	for _, tmpl := range templates {
		templateID := fmt.Sprint(tmpl["id"])
		if templateID == "" || templateID == "<nil>" || seen[templateID] {
			continue
		}
		seen[templateID] = true
		html := stringFromMapPath(tmpl, "attributes.html")
		if html == "" {
			html = stringFromMapPath(tmpl, "attributes.definition.html")
		}
		if !strings.Contains(html, oldURL) {
			continue
		}
		matched++
		updated := strings.ReplaceAll(html, oldURL, newURL)
		patchBody := jsonAPIBody("template", map[string]any{"html": updated}, nil)
		_, _, pErr := c.Patch("/api/templates/"+url.PathEscape(templateID), patchBody)
		result := map[string]any{
			"scope":       scope,
			"template_id": templateID,
			"name":        stringFromMapPath(tmpl, "attributes.name"),
		}
		if pErr != nil {
			result["action"] = "error"
			result["error"] = pErr.Error()
		} else {
			result["action"] = "updated"
		}
		results = append(results, result)
	}
	return results, len(templates), matched, nil
}

func collectMessageTemplates(c flowClient, scope string) ([]map[string]any, error) {
	var out []map[string]any
	switch scope {
	case "flow":
		flows, err := fetchAllJSONAPI(c, "/api/flows", map[string]string{"fields[flow]": "name", "page[size]": "50"}, 0)
		if err != nil {
			return nil, err
		}
		for _, flow := range flows {
			flowID := fmt.Sprint(flow["id"])
			actions, err := fetchAllJSONAPI(c, "/api/flows/"+url.PathEscape(flowID)+"/flow-actions", map[string]string{"fields[flow-action]": "name,type", "page[size]": "50"}, 0)
			if err != nil {
				return nil, err
			}
			for _, action := range actions {
				actionID := fmt.Sprint(action["id"])
				messages, err := fetchAllJSONAPI(c, "/api/flow-actions/"+url.PathEscape(actionID)+"/flow-messages", map[string]string{"fields[flow-message]": "name", "page[size]": "50"}, 0)
				if err != nil {
					return nil, err
				}
				for _, msg := range messages {
					msgID := fmt.Sprint(msg["id"])
					tmpl, err := c.Get("/api/flow-messages/"+url.PathEscape(msgID)+"/template", map[string]string{"fields[template]": "name,html,definition"})
					if err != nil {
						continue
					}
					if m, ok := mustJSONAny(tmpl).(map[string]any); ok {
						if data, ok := m["data"].(map[string]any); ok {
							data["message_id"] = msgID
							data["parent_id"] = flowID
							out = append(out, data)
						}
					}
				}
			}
		}
	case "campaign":
		campaigns, err := fetchAllJSONAPI(c, "/api/campaigns", map[string]string{"fields[campaign]": "name,status", "page[size]": "50"}, 0)
		if err != nil {
			return nil, err
		}
		for _, campaign := range campaigns {
			campaignID := fmt.Sprint(campaign["id"])
			messages, err := fetchAllJSONAPI(c, "/api/campaigns/"+url.PathEscape(campaignID)+"/campaign-messages", map[string]string{"fields[campaign-message]": "name", "page[size]": "50"}, 0)
			if err != nil {
				return nil, err
			}
			for _, msg := range messages {
				msgID := fmt.Sprint(msg["id"])
				tmpl, err := c.Get("/api/campaign-messages/"+url.PathEscape(msgID)+"/template", map[string]string{"fields[template]": "name,html,definition"})
				if err != nil {
					continue
				}
				if m, ok := mustJSONAny(tmpl).(map[string]any); ok {
					if data, ok := m["data"].(map[string]any); ok {
						data["message_id"] = msgID
						data["parent_id"] = campaignID
						out = append(out, data)
					}
				}
			}
		}
	}
	return out, nil
}

func newSegmentsRFMCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rfm",
		Short: "Create standard RFM segments from Placed Order data",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"resolve_placed_order_metric", "query_orders_by_profile", "score_profiles", "profile_bulk_import_scores", "create_rfm_segments"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			metricID, err := resolveMetricID(c, "Placed Order")
			if err != nil {
				return err
			}
			now := time.Now()
			since := now.AddDate(-2, 0, 0)
			body := metricAggregateBody(metricID, []string{"count", "sum_value"}, []string{"Profile ID"}, since, now)
			resp, _, err := c.Post("/api/metric-aggregates", body)
			if err != nil {
				return classifyAPIError(err)
			}
			lastOrders, err := profileLastOrderTimes(c, metricID, since, now, 10000)
			if err != nil {
				return err
			}
			scores := scoreRFMProfiles(resp, lastOrders, now)
			if len(scores) > 0 {
				if _, _, err := c.Post("/api/profile-bulk-import-jobs", rfmProfileBulkImportJobBody(scores)); err != nil {
					return classifyAPIError(err)
				}
			}
			segments := []string{"Champions", "Loyal Customers", "At Risk", "About to Sleep", "Lost"}
			var created []map[string]any
			for _, label := range segments {
				def := rfmSegmentDefinition(label)
				name := "RFM - " + label
				segResp, status, err := c.Post("/api/segments", jsonAPIBody("segment", map[string]any{"name": name, "definition": def}, nil))
				if err != nil {
					created = append(created, map[string]any{"name": name, "error": err.Error()})
					continue
				}
				created = append(created, map[string]any{"name": name, "segment_id": jsonAPIID(segResp), "status": status})
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"profiles_scored": len(scores), "segments": created}, flags)
		},
	}
	return cmd
}

func buildGiftFollowupFlow(c flowClient, fromEmail, fromLabel, triggerProduct string) (map[string]any, []string, error) {
	if triggerProduct == "" {
		return nil, nil, usageErr(fmt.Errorf("--trigger-product is required for gift-followup preset"))
	}
	triggerID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving trigger metric: %w", err)
	}
	emails := []presetEmail{
		{TmpID: "tmp-2", Name: "Gift Follow-Up: Recipient Capture", Subject: "Was this a gift?", PreviewText: "A simple way to share the product with the recipient", TemplateKey: "gift-followup-1"},
		{TmpID: "tmp-4", Name: "Gift Follow-Up: Review Request", Subject: "How did they like it?", PreviewText: "We would love to hear how the gift landed", TemplateKey: "gift-followup-2"},
	}
	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}
	return map[string]any{
		"triggers": []any{map[string]any{
			"type": "metric",
			"id":   triggerID,
			"trigger_filter": map[string]any{"conditions": []any{
				map[string]any{"type": "event-property", "property": "$ProductName", "operator": "contains", "value": triggerProduct},
			}},
		}},
		"profile_filter":   map[string]any{"condition_groups": []any{}},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": 30, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, []int{72, 120}),
	}, templateIDs, nil
}

func buildReplenishmentFlow(c flowClient, fromEmail, fromLabel, product string, days int) (map[string]any, []string, error) {
	if product == "" {
		return nil, nil, usageErr(fmt.Errorf("--product is required for replenishment preset"))
	}
	if days <= 0 {
		return nil, nil, usageErr(fmt.Errorf("--days must be greater than 0"))
	}
	triggerID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving trigger metric: %w", err)
	}
	emails := []presetEmail{
		{TmpID: "tmp-2", Name: "Replenishment Reminder: " + product, Subject: "Your " + product + " is almost full", PreviewText: "Ready for a fresh start?", TemplateKey: "replenishment-1"},
	}
	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}
	return map[string]any{
		"triggers": []any{map[string]any{
			"type": "metric",
			"id":   triggerID,
			"trigger_filter": map[string]any{"conditions": []any{
				map[string]any{"type": "event-property", "property": "$ProductName", "operator": "contains", "value": product},
			}},
		}},
		"profile_filter": map[string]any{"condition_groups": []any{map[string]any{"conditions": []any{
			metricCountCondition(triggerID, "equals", 0, map[string]any{"type": "date", "operator": "flow-start"}, productMetricFilter(product, false)),
		}}}},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": days, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, []int{days * 24}),
	}, templateIDs, nil
}

func buildCrossSellFlow(c flowClient, fromEmail, fromLabel, triggerProduct, crossSell string) (map[string]any, []string, error) {
	if triggerProduct == "" {
		return nil, nil, usageErr(fmt.Errorf("--trigger-product is required for cross-sell preset"))
	}
	products := splitCSV(crossSell)
	if len(products) == 0 {
		return nil, nil, usageErr(fmt.Errorf("--cross-sell is required for cross-sell preset"))
	}
	triggerID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving trigger metric: %w", err)
	}
	emails := make([]presetEmail, 0, len(products))
	delays := make([]int, 0, len(products))
	filterConditions := []any{}
	for i, product := range products {
		emails = append(emails, presetEmail{
			TmpID:       fmt.Sprintf("tmp-%d", i*2+2),
			Name:        "Cross-Sell: " + product,
			Subject:     "A practical next step after " + triggerProduct,
			PreviewText: "You might like " + product,
			TemplateKey: fmt.Sprintf("cross-sell-%d", i+1),
		})
		if i == 0 {
			delays = append(delays, 14*24)
		} else {
			delays = append(delays, 30*24)
		}
		filterConditions = append(filterConditions, metricCountCondition(triggerID, "equals", 0, map[string]any{"type": "date", "operator": "all-time"}, productMetricFilter(product, false)))
	}
	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}
	return map[string]any{
		"triggers": []any{map[string]any{
			"type": "metric",
			"id":   triggerID,
			"trigger_filter": map[string]any{"conditions": []any{
				map[string]any{"type": "event-property", "property": "$ProductName", "operator": "contains", "value": triggerProduct},
			}},
		}},
		"profile_filter":   map[string]any{"condition_groups": []any{map[string]any{"conditions": filterConditions}}},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": 60, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, delays),
	}, templateIDs, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func profileLastOrderTimes(c flowClient, metricID string, since, until time.Time, limit int) (map[string]time.Time, error) {
	filter := fmt.Sprintf("equals(metric_id,\"%s\"),greater-or-equal(datetime,%s),less-than(datetime,%s)", metricID, since.Format(time.RFC3339), until.Format(time.RFC3339))
	items, err := fetchAllJSONAPI(c, "/api/events", map[string]string{"filter": filter, "page[size]": "200", "sort": "-datetime"}, limit)
	if err != nil {
		return nil, err
	}
	lastOrders := map[string]time.Time{}
	for _, item := range items {
		profileID := firstNonEmptyString(
			stringFromMapPath(item, "relationships.profile.data.id"),
			stringFromMapPath(item, "attributes.profile_id"),
			stringFromMapPath(item, "attributes.properties.profile_id"),
			stringFromMapPath(item, "attributes.email"),
			stringFromMapPath(item, "attributes.properties.email"),
		)
		if profileID == "" {
			continue
		}
		orderedAt := parseDate(firstNonEmptyString(
			stringFromMapPath(item, "attributes.datetime"),
			stringFromMapPath(item, "attributes.timestamp"),
		))
		if orderedAt.IsZero() {
			continue
		}
		if current, ok := lastOrders[profileID]; !ok || orderedAt.After(current) {
			lastOrders[profileID] = orderedAt
		}
	}
	return lastOrders, nil
}

func scoreRFMProfiles(raw json.RawMessage, lastOrders map[string]time.Time, now time.Time) []map[string]any {
	rows := metricAggregateRows(raw, "count")
	monetaryRows := metricAggregateRows(raw, "sum_value")
	var out []map[string]any
	for profileID, frequency := range rows {
		monetary := monetaryRows[profileID]
		out = append(out, map[string]any{
			"profile_id": profileID,
			"r_score":    recencyScore(lastOrders[profileID], now),
			"f_score":    scoreBucket(frequency),
			"m_score":    scoreBucket(monetary),
		})
	}
	return out
}

func recencyScore(lastOrder, now time.Time) int {
	if lastOrder.IsZero() || now.IsZero() || lastOrder.After(now) {
		return 1
	}
	days := now.Sub(lastOrder).Hours() / 24
	switch {
	case days <= 30:
		return 5
	case days <= 90:
		return 4
	case days <= 180:
		return 3
	case days <= 365:
		return 2
	default:
		return 1
	}
}

func scoreBucket(v float64) int {
	switch {
	case v >= 5:
		return 5
	case v >= 3:
		return 4
	case v >= 2:
		return 3
	case v >= 1:
		return 2
	default:
		return 1
	}
}

func rfmProfileImportData(scores []map[string]any) []any {
	out := make([]any, 0, len(scores))
	for _, score := range scores {
		out = append(out, map[string]any{
			"type": "profile",
			"id":   score["profile_id"],
			"attributes": map[string]any{"properties": map[string]any{
				"rfm_recency_score":   score["r_score"],
				"rfm_frequency_score": score["f_score"],
				"rfm_monetary_score":  score["m_score"],
			}},
		})
	}
	return out
}

func rfmProfileBulkImportJobBody(scores []map[string]any) map[string]any {
	return jsonAPIBody("profile-bulk-import-job", map[string]any{
		"profiles": map[string]any{"data": rfmProfileImportData(scores)},
	}, nil)
}

func rfmSegmentDefinition(label string) map[string]any {
	conditions := []any{}
	add := func(prop, op string, val int) {
		conditions = append(conditions, map[string]any{"type": "profile-property", "property": prop, "operator": op, "value": val})
	}
	switch label {
	case "Champions":
		add("rfm_recency_score", "greater-or-equal", 4)
		add("rfm_frequency_score", "greater-or-equal", 4)
		add("rfm_monetary_score", "greater-or-equal", 4)
	case "Loyal Customers":
		add("rfm_frequency_score", "greater-or-equal", 4)
	case "At Risk":
		add("rfm_frequency_score", "greater-or-equal", 4)
		add("rfm_recency_score", "less-or-equal", 2)
	case "About to Sleep":
		add("rfm_frequency_score", "less-or-equal", 2)
		add("rfm_recency_score", "less-or-equal", 2)
	case "Lost":
		add("rfm_recency_score", "less-or-equal", 1)
	}
	return map[string]any{"condition_groups": []any{map[string]any{"conditions": conditions}}}
}

func fetchAllJSONAPI(c flowClient, path string, params map[string]string, limit int) ([]map[string]any, error) {
	var items []map[string]any
	cursor := ""
	for {
		p := map[string]string{}
		for k, v := range params {
			p[k] = v
		}
		if cursor != "" {
			p["page[cursor]"] = cursor
		}
		resp, err := c.Get(path, p)
		if err != nil {
			return nil, classifyAPIError(err)
		}
		pageItems, next, err := parseJSONAPICollection(resp)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if limit > 0 && len(items) >= limit {
			return items[:limit], nil
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return items, nil
}

func metricAggregateBody(metricID string, measurements []string, by []string, since, until time.Time) map[string]any {
	attrs := map[string]any{
		"metric_id":    metricID,
		"measurements": measurements,
		"interval":     "day",
		"page_size":    500,
		"filter": []string{
			"greater-or-equal(datetime," + since.Format("2006-01-02") + "T00:00:00)",
			"less-than(datetime," + until.Format("2006-01-02") + "T23:59:59)",
		},
		"timezone": defaultMetricTimezone(),
	}
	if len(by) > 0 {
		attrs["by"] = by
	}
	return map[string]any{"data": map[string]any{"type": "metric-aggregate", "attributes": attrs}}
}

func defaultMetricTimezone() string {
	if tz := strings.TrimSpace(os.Getenv("KLAVIYO_TIMEZONE")); tz != "" {
		return tz
	}
	return "UTC"
}

func metricAggregateRows(raw json.RawMessage, measurement string) map[string]float64 {
	var parsed map[string]any
	if json.Unmarshal(raw, &parsed) != nil {
		return nil
	}
	results, _ := anyPath(parsed, "data.attributes.data").([]any)
	out := map[string]float64{}
	for _, r := range results {
		row, _ := r.(map[string]any)
		dims, _ := row["dimensions"].([]any)
		key := "(none)"
		if len(dims) > 0 {
			key = fmt.Sprint(dims[0])
		}
		meas, _ := row["measurements"].(map[string]any)
		vals, _ := meas[measurement].([]any)
		for _, v := range vals {
			out[key] += anyFloat(v)
		}
	}
	return out
}

func sumMeasurement(raw json.RawMessage, measurement string) float64 {
	total := 0.0
	for _, v := range metricAggregateRows(raw, measurement) {
		total += v
	}
	return total
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func intersectStrings(a, b []string) []string {
	seen := map[string]bool{}
	for _, s := range a {
		seen[s] = true
	}
	var out []string
	for _, s := range b {
		if seen[s] {
			out = append(out, s)
		}
	}
	return uniqueStrings(out)
}

func anyStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		var out []string
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

func firstStrings(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func newProfilesStatsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Show profile, consent, and engagement counts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "limit": limit, "planned_steps": []string{"fetch_profiles", "resolve_engagement_metrics", "query_open_windows", "summarize_consent"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report, err := buildProfilesStats(c, time.Now(), limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum profiles to inspect for profile-derived counts (0 for all)")
	return cmd
}

func buildProfilesStats(c flowClient, now time.Time, limit int) (map[string]any, error) {
	profiles, err := fetchAllJSONAPI(c, "/api/profiles", map[string]string{"fields[profile]": "email,phone_number,created,properties", "additional-fields[profile]": "subscriptions", "page[size]": "100"}, limit)
	if err != nil {
		return nil, err
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	summary := map[string]any{
		"checked_at":           now.UTC().Format(time.RFC3339),
		"total_active":         len(profiles),
		"total_suppressed":     0,
		"email_subscribed":     0,
		"sms_subscribed":       0,
		"profiles_added_month": 0,
		"profiles_lost_month":  0,
		"profiles_inspected":   len(profiles),
		"estimated":            limit > 0 && len(profiles) >= limit,
	}
	for _, profile := range profiles {
		if profileEmailSubscribed(profile) {
			summary["email_subscribed"] = anyInt(summary["email_subscribed"]) + 1
		}
		if profileSMSSubscribed(profile) {
			summary["sms_subscribed"] = anyInt(summary["sms_subscribed"]) + 1
		}
		if profileSuppressed(profile) {
			summary["total_suppressed"] = anyInt(summary["total_suppressed"]) + 1
		}
		if created := parseDate(stringFromMapPath(profile, "attributes.created")); !created.IsZero() && created.After(monthStart) {
			summary["profiles_added_month"] = anyInt(summary["profiles_added_month"]) + 1
		}
	}
	engagement := map[string]any{}
	if openedID, err := resolveMetricID(c, "Opened Email"); err == nil {
		for _, days := range []int{30, 60, 90} {
			body := metricAggregateBody(openedID, []string{"unique"}, nil, now.AddDate(0, 0, -days), now)
			resp, _, aggErr := c.Post("/api/metric-aggregates", body)
			if aggErr == nil {
				engagement[fmt.Sprintf("opened_%dd", days)] = int(sumMeasurement(resp, "unique"))
			}
		}
	}
	if v, ok := engagement["opened_90d"].(int); ok {
		engagement["never_or_not_recent"] = len(profiles) - v
	}
	summary["engagement"] = engagement
	return summary, nil
}

func newProfilesTopSpendersCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "top-spenders",
		Short:       "Rank profiles by Placed Order lifetime value",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "limit": limit, "planned_steps": []string{"resolve_placed_order_metric", "query_by_profile", "fetch_profile_emails"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := profileSpendRows(c, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"limit": limit, "rows": rows}, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum profiles to return")
	return cmd
}

func newProfilesNeverPurchasedCmd(flags *rootFlags) *cobra.Command {
	var tenure string
	var limit int
	cmd := &cobra.Command{
		Use:         "never-purchased",
		Short:       "List profiles past a tenure threshold with no Placed Order events",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "tenure": tenure, "limit": limit, "planned_steps": []string{"fetch_profiles", "query_purchasers", "subtract_sets"}}, flags)
			}
			days, err := parseDayComparator(tenure)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			purchasers, err := profileIDsWithOrders(c, time.Now().AddDate(-5, 0, 0), time.Now())
			if err != nil {
				return err
			}
			profiles, err := fetchAllJSONAPI(c, "/api/profiles", map[string]string{"fields[profile]": "email,created", "page[size]": "100"}, limit)
			if err != nil {
				return err
			}
			cutoff := time.Now().AddDate(0, 0, -days)
			var rows []map[string]any
			for _, p := range profiles {
				id := fmt.Sprint(p["id"])
				if purchasers[id] {
					continue
				}
				created := parseDate(stringFromMapPath(p, "attributes.created"))
				if !created.IsZero() && created.Before(cutoff) {
					rows = append(rows, profileSummaryRow(p))
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"tenure": tenure, "count": len(rows), "rows": rows}, flags)
		},
	}
	cmd.Flags().StringVar(&tenure, "tenure", ">60d", "Minimum profile tenure, for example >60d")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum profiles to inspect")
	return cmd
}

func newProfilesChurningCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "churning",
		Short:       "Flag profiles whose purchase cadence appears to have slowed",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "limit": limit, "planned_steps": []string{"query_orders_by_profile", "estimate_cadence", "rank_slowdowns"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := profileSpendRows(c, limit)
			if err != nil {
				return err
			}
			now := time.Now()
			metricID, err := resolveMetricID(c, "Placed Order")
			if err != nil {
				return err
			}
			lastOrders, err := profileLastOrderTimes(c, metricID, now.AddDate(-5, 0, 0), now, 10000)
			if err != nil {
				return err
			}
			flagged := churnCandidateRows(rows, lastOrders, now)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"count": len(flagged), "rows": flagged}, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum candidate profiles to inspect")
	return cmd
}

func churnCandidateRows(rows []map[string]any, lastOrders map[string]time.Time, now time.Time) []map[string]any {
	var flagged []map[string]any
	for _, row := range rows {
		profileID := fmt.Sprint(row["profile_id"])
		orderCount := anyFloat(row["order_count"])
		if orderCount < 2 {
			continue
		}
		lastOrder := lastOrders[profileID]
		if lastOrder.IsZero() || now.IsZero() || lastOrder.After(now) {
			continue
		}
		avgDays := (5 * 365.0) / orderCount
		daysSinceLastOrder := now.Sub(lastOrder).Hours() / 24
		if daysSinceLastOrder <= avgDays*1.5 {
			continue
		}
		row["estimated_avg_days_between_purchases"] = round2(avgDays)
		row["days_since_last_order"] = round2(daysSinceLastOrder)
		row["flagged"] = true
		row["reason"] = "last order is more than 1.5x the estimated purchase cadence"
		flagged = append(flagged, row)
	}
	return flagged
}

func newProfilesExportSuppressionsCmd(flags *rootFlags) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:         "export-suppressions",
		Short:       "Export suppressed profiles for compliance records",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "output": output, "planned_steps": []string{"fetch_profiles", "filter_suppressed", "write_csv"}}, flags)
			}
			if output == "" {
				return usageErr(fmt.Errorf("--output is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			profiles, err := fetchAllJSONAPI(c, "/api/profiles", map[string]string{"fields[profile]": "email,phone_number,properties", "additional-fields[profile]": "subscriptions", "page[size]": "100"}, 0)
			if err != nil {
				return err
			}
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("creating %s: %w", output, err)
			}
			defer f.Close()
			w := csv.NewWriter(f)
			if err := w.Write([]string{"id", "email", "phone_number", "suppression_reason"}); err != nil {
				return err
			}
			count := 0
			for _, p := range profiles {
				if !profileSuppressed(p) {
					continue
				}
				count++
				if err := w.Write([]string{fmt.Sprint(p["id"]), stringFromMapPath(p, "attributes.email"), stringFromMapPath(p, "attributes.phone_number"), suppressionReason(p)}); err != nil {
					return err
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"output": output, "rows": count}, flags)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "CSV output path")
	return cmd
}

func newFlowsAuditCmd(flags *rootFlags) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Audit drafts, duplicate triggers, inactive actions, and dead flow triggers",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "max": max, "planned_steps": []string{"fetch_flows_with_definitions", "inspect_action_status", "group_trigger_metrics", "query_30d_trigger_counts"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report, err := auditFlows(c, time.Now(), max)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&max, "max", 25, "Maximum flows to inspect deeply (0 for all)")
	return cmd
}

func auditFlows(c flowClient, now time.Time, max int) (map[string]any, error) {
	flows, err := fetchAllJSONAPI(c, "/api/flows", map[string]string{"fields[flow]": "name,status,created,updated,trigger_type", "page[size]": "50"}, max)
	if err != nil {
		return nil, err
	}
	var issues []map[string]any
	triggerOwners := map[string][]string{}
	metricCounts := map[string]float64{}
	for _, flow := range flows {
		id := fmt.Sprint(flow["id"])
		name := stringFromMapPath(flow, "attributes.name")
		status := strings.ToLower(stringFromMapPath(flow, "attributes.status"))
		created := parseDate(stringFromMapPath(flow, "attributes.created"))
		if status == "draft" && !created.IsZero() && created.Before(now.AddDate(0, -6, 0)) {
			issues = append(issues, flowIssue(id, name, "stale_draft", "flow was created more than six months ago and is still draft"))
		}
		resp, getErr := c.Get("/api/flows/"+url.PathEscape(id), map[string]string{"additional-fields[flow]": "definition"})
		if getErr != nil {
			issues = append(issues, flowIssue(id, name, "definition_unreadable", getErr.Error()))
			continue
		}
		def, _ := anyPath(mustJSONAny(resp), "data.attributes.definition").(map[string]any)
		if def == nil {
			issues = append(issues, flowIssue(id, name, "missing_definition", "flow definition was not returned"))
			continue
		}
		for _, metricID := range collectTriggerMetricIDs(def) {
			triggerOwners[metricID] = append(triggerOwners[metricID], id)
			count, ok := metricCounts[metricID]
			if !ok {
				count = queryMetricCount(c, metricID, now.AddDate(0, 0, -30), now)
				metricCounts[metricID] = count
			}
			if count == 0 {
				issues = append(issues, flowIssue(id, name, "dead_trigger", "trigger metric had zero events in the last 30 days"))
			}
		}
		for _, action := range mapSlice(def["actions"]) {
			if strings.EqualFold(stringFromMapPath(action, "data.status"), "draft") && status == "live" {
				issues = append(issues, flowIssue(id, name, "draft_action_in_live_flow", "a live flow contains a draft action"))
			}
			if anyFloat(anyPath(action, "data.sent_count")) == 0 && strings.EqualFold(fmt.Sprint(action["type"]), "send-email") {
				issues = append(issues, flowIssue(id, name, "action_never_sent", "a send-email action reports zero sends"))
			}
		}
	}
	for metricID, owners := range triggerOwners {
		if len(owners) > 1 {
			issues = append(issues, map[string]any{"type": "duplicate_trigger_metric", "metric_id": metricID, "flow_ids": owners, "detail": "multiple flows share the same trigger metric"})
		}
	}
	return map[string]any{"checked_at": now.UTC().Format(time.RFC3339), "flows_scanned": len(flows), "max": max, "estimated": max > 0 && len(flows) >= max, "issue_count": len(issues), "issues": issues}, nil
}

func newListsAuditCmd(flags *rootFlags) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Audit list size and stale usage signals",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "max": max, "planned_steps": []string{"fetch_lists_with_profile_counts", "fetch_flow_triggers", "flag_unused_lists"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			lists, err := fetchAllJSONAPI(c, "/api/lists", map[string]string{"fields[list]": "name,created,updated,profile_count,profiles_count,profile_count_estimate", "page[size]": "10"}, max)
			if err != nil {
				return err
			}
			var rows []map[string]any
			for _, list := range lists {
				id := fmt.Sprint(list["id"])
				subscriberCount := listSubscriberCount(list)
				flows, _ := fetchAllJSONAPI(c, "/api/lists/"+url.PathEscape(id)+"/flow-triggers", map[string]string{"fields[flow]": "name", "page[size]": "50"}, 1000)
				rows = append(rows, map[string]any{
					"id":               id,
					"name":             stringFromMapPath(list, "attributes.name"),
					"subscriber_count": subscriberCount,
					"triggered_flows":  len(flows),
					"updated":          stringFromMapPath(list, "attributes.updated"),
					"flags":            listAuditFlags(list, subscriberCount, len(flows)),
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"lists": rows, "count": len(rows), "max": max, "estimated": max > 0 && len(rows) >= max}, flags)
		},
	}
	cmd.Flags().IntVar(&max, "max", 25, "Maximum lists to inspect deeply (0 for all)")
	return cmd
}

func newTemplatesAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Find orphan and stale templates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_templates", "collect_flow_and_campaign_template_refs", "flag_orphan_stale_templates"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			templates, err := fetchAllJSONAPI(c, "/api/templates", map[string]string{"fields[template]": "name,updated,html", "page[size]": "50"}, 0)
			if err != nil {
				return err
			}
			refs := map[string]bool{}
			for _, scope := range []string{"flow", "campaign"} {
				msgTemplates, _ := collectMessageTemplates(c, scope)
				for _, tmpl := range msgTemplates {
					refs[fmt.Sprint(tmpl["id"])] = true
				}
			}
			cutoff := time.Now().AddDate(-1, 0, 0)
			var rows []map[string]any
			for _, tmpl := range templates {
				id := fmt.Sprint(tmpl["id"])
				updated := parseDate(stringFromMapPath(tmpl, "attributes.updated"))
				flags := []string{}
				if !refs[id] {
					flags = append(flags, "orphan")
				}
				if !updated.IsZero() && updated.Before(cutoff) {
					flags = append(flags, "stale_12_months")
				}
				if len(flags) > 0 {
					rows = append(rows, map[string]any{"id": id, "name": stringFromMapPath(tmpl, "attributes.name"), "updated": stringFromMapPath(tmpl, "attributes.updated"), "flags": flags})
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"issue_count": len(rows), "templates": rows}, flags)
		},
	}
	return cmd
}

func newTagsAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Find empty and near-duplicate tags",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_tags", "count_relationships", "detect_near_duplicates"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tags, err := fetchAllJSONAPI(c, "/api/tags", map[string]string{"fields[tag]": "name", "page[size]": "50"}, 0)
			if err != nil {
				return err
			}
			var rows []map[string]any
			nameBuckets := map[string][]string{}
			for _, tag := range tags {
				id := fmt.Sprint(tag["id"])
				name := stringFromMapPath(tag, "attributes.name")
				counts := map[string]int{}
				for rel, path := range map[string]string{"flows": "flows", "segments": "segments", "lists": "lists", "campaigns": "campaigns"} {
					items, _ := fetchAllJSONAPI(c, "/api/tags/"+url.PathEscape(id)+"/relationships/"+path, map[string]string{"page[size]": "100"}, 1000)
					counts[rel] = len(items)
				}
				total := counts["flows"] + counts["segments"] + counts["lists"] + counts["campaigns"]
				key := normalizeTagName(name)
				nameBuckets[key] = append(nameBuckets[key], name)
				if total == 0 {
					rows = append(rows, map[string]any{"id": id, "name": name, "flag": "empty", "counts": counts})
				}
			}
			for _, names := range nameBuckets {
				if len(names) > 1 {
					rows = append(rows, map[string]any{"flag": "near_duplicate", "names": names})
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"issue_count": len(rows), "issues": rows}, flags)
		},
	}
	return cmd
}

func newReportDashboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "dashboard",
		Short:       "Executive Klaviyo summary",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"query_30d_revenue", "rank_flows", "rank_campaigns", "profile_stats", "deliverability", "coupon_pools"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			placedID, err := resolveMetricID(c, "Placed Order")
			if err != nil {
				return err
			}
			totalRevenue := 0.0
			if resp, _, err := c.Post("/api/metric-aggregates", metricAggregateBody(placedID, []string{"sum_value"}, nil, now.AddDate(0, 0, -30), now)); err == nil {
				totalRevenue = sumMeasurement(resp, "sum_value")
			}
			flows := topAggregateRows(c, placedID, "$attributed_flow", "sum_value", now.AddDate(0, 0, -30), now, 5)
			campaigns := topAggregateRows(c, placedID, "Campaign Name", "sum_value", now.AddDate(0, 0, -30), now, 5)
			profileStats, _ := buildProfilesStats(c, now, 1000)
			deliverability, _ := buildDeliverabilityReport(c, now.AddDate(0, 0, -30), now)
			coupons, _ := checkCouponPools(c, 100, "", now)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"window_days":        30,
				"email_revenue":      totalRevenue,
				"top_flows":          flows,
				"top_campaigns":      campaigns,
				"profiles":           profileStats,
				"deliverability":     deliverability,
				"coupon_pool_status": coupons,
			}, flags)
		},
	}
	return cmd
}

func newReportOpenRatesCmd(flags *rootFlags) *cobra.Command {
	var by, last string
	var trend bool
	cmd := &cobra.Command{
		Use:         "open-rates",
		Short:       "Open and click rates by flow or campaign",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "by": by, "trend": trend, "last": last, "planned_steps": []string{"query_received_opened_clicked", "calculate_rates", "flag_large_drops"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			group := attributionDimension(by)
			rows, err := engagementRateRows(c, group, since, until)
			if err != nil {
				return err
			}
			if trend {
				if err := addEngagementTrends(c, rows, group, since, until); err != nil {
					return err
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"by": by, "last": last, "trend": trend, "rows": rows}, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "flow", "Group by flow or campaign")
	cmd.Flags().BoolVar(&trend, "trend", false, "Include trend flags")
	cmd.Flags().StringVar(&last, "last", "90d", "Lookback window")
	return cmd
}

func newReportMetricRankCmd(flags *rootFlags, use, short, metricName, defaultBy string) *cobra.Command {
	var by, last string
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "by": by, "last": last, "metric": metricName, "planned_steps": []string{"resolve_metric", "query_metric_aggregates", "rank_groups"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			metricID, err := resolveMetricID(c, metricName)
			if err != nil {
				return err
			}
			rows := topAggregateRows(c, metricID, attributionDimension(by), "count", since, until, 100)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"metric": metricName, "by": by, "last": last, "rows": rows}, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", defaultBy, "Group by flow or campaign")
	cmd.Flags().StringVar(&last, "last", "30d", "Lookback window")
	return cmd
}

func newReportListGrowthCmd(flags *rootFlags) *cobra.Command {
	var last string
	cmd := &cobra.Command{
		Use:         "list-growth",
		Short:       "Net list growth over a lookback window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "planned_steps": []string{"query_subscribe_unsubscribe_metrics", "calculate_window_net_growth"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			rows := metricTotalsByName(c, []string{"Subscribed to List", "Unsubscribed Email"}, since, until)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"last": last, "rows": rows, "note": "full-window net growth totals"}, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "90d", "Lookback window")
	return cmd
}

func newReportDomainReputationCmd(flags *rootFlags) *cobra.Command {
	var last string
	cmd := &cobra.Command{
		Use:         "domain-reputation",
		Short:       "Per-domain deliverability deep dive",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "planned_steps": []string{"query_received_bounced_opened_spam_by_domain", "flag_reputation_risks"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			rows := domainReputationRows(c, since, until)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"last": last, "rows": rows}, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "30d", "Lookback window")
	return cmd
}

func newReportFlowFunnelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "flow-funnel",
		Short:       "Account-level flow conversion funnel by core email metrics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"query_entered_received_opened_clicked_converted", "calculate_dropoffs"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows := metricTotalsByName(c, []string{"Received Email", "Opened Email", "Clicked Email", "Placed Order"}, time.Now().AddDate(0, 0, -90), time.Now())
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"window_days": 90, "funnel": rows, "note": "account-level aggregate metrics"}, flags)
		},
	}
	return cmd
}

func newReportFlowComparisonCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "flow-comparison",
		Short:       "Compare flow revenue and engagement",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"query_flow_revenue", "query_flow_engagement", "sort_by_revenue"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			placedID, err := resolveMetricID(c, "Placed Order")
			if err != nil {
				return err
			}
			rows := topAggregateRows(c, placedID, "$attributed_flow", "sum_value", time.Now().AddDate(0, 0, -90), time.Now(), 100)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"window_days": 90, "rows": rows}, flags)
		},
	}
	return cmd
}

func newReportEmailPerformanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "email-performance",
		Short:       "Account-level email performance summary",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"query_delivery_engagement_conversion_metrics"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows := metricTotalsByName(c, []string{"Received Email", "Bounced Email", "Opened Email", "Clicked Email", "Unsubscribed Email", "Marked Email as Spam", "Placed Order"}, time.Now().AddDate(0, 0, -90), time.Now())
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"window_days": 90, "metrics": rows}, flags)
		},
	}
	return cmd
}

func newReportFormsCmd(flags *rootFlags) *cobra.Command {
	var last string
	cmd := &cobra.Command{
		Use:         "forms",
		Short:       "Signup form performance",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "planned_steps": []string{"fetch_forms"}}, flags)
			}
			c, _, _, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			forms, err := fetchAllJSONAPI(c, "/api/forms", map[string]string{"fields[form]": "name,status,created,updated", "page[size]": "50"}, 0)
			if err != nil {
				return err
			}
			note := "forms list is not filtered by submission activity; Klaviyo forms API does not expose submission-date filtering"
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"last": last, "forms": forms, "count": len(forms), "note": note}, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "30d", "Requested lookback label; forms are not filtered by submission date")
	return cmd
}

func newReportSignupSourcesCmd(flags *rootFlags) *cobra.Command {
	var last string
	cmd := &cobra.Command{
		Use:         "signup-sources",
		Short:       "Profile signup source summary",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "planned_steps": []string{"fetch_recent_profiles", "group_source_properties"}}, flags)
			}
			c, since, _, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			profiles, err := fetchAllJSONAPI(c, "/api/profiles", map[string]string{"fields[profile]": "created,properties", "page[size]": "100"}, 5000)
			if err != nil {
				return err
			}
			counts := map[string]int{}
			for _, p := range profiles {
				created := parseDate(stringFromMapPath(p, "attributes.created"))
				if !created.IsZero() && created.Before(since) {
					continue
				}
				source := firstNonEmptyString(stringFromMapPath(p, "attributes.properties.$source"), stringFromMapPath(p, "attributes.properties.source"), "(unknown)")
				counts[source]++
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"last": last, "sources": counts}, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "30d", "Lookback window")
	return cmd
}

func newReportProductsCmd(flags *rootFlags) *cobra.Command {
	var last string
	cmd := &cobra.Command{
		Use:         "products",
		Short:       "Revenue by product from email attribution",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "planned_steps": []string{"resolve_placed_order_metric", "query_product_revenue"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			metricID, err := resolveMetricID(c, "Placed Order")
			if err != nil {
				return err
			}
			rows := topAggregateRows(c, metricID, "ProductName", "sum_value", since, until, 100)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"last": last, "rows": rows}, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "90d", "Lookback window")
	return cmd
}

func newReportProductAffinityCmd(flags *rootFlags) *cobra.Command {
	var product string
	cmd := &cobra.Command{
		Use:         "product-affinity",
		Short:       "Co-purchase signal for a product",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "product": product, "planned_steps": []string{"query_placed_order_by_product", "rank_affinity"}}, flags)
			}
			if product == "" {
				return usageErr(fmt.Errorf("--product is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			metricID, err := resolveMetricID(c, "Placed Order")
			if err != nil {
				return err
			}
			rows, anchorOrders, err := productAffinityRows(c, metricID, product, time.Now().AddDate(-1, 0, 0), time.Now(), 5000)
			if err != nil {
				return err
			}
			result := map[string]any{
				"product":       product,
				"anchor_orders": anchorOrders,
				"rows":          rows,
				"sample_limit":  5000,
				"note":          "Ranks products found in the same Placed Order events as the requested anchor product.",
			}
			if anchorOrders == 0 {
				result["message"] = "No Placed Order events in the sample contained the requested anchor product."
			} else if len(rows) == 0 {
				result["message"] = "Anchor product orders did not include additional product names in the sampled event properties."
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "Anchor product name")
	return cmd
}

func productAffinityRows(c flowClient, metricID, product string, since, until time.Time, limit int) ([]map[string]any, int, error) {
	filter := fmt.Sprintf("equals(metric_id,\"%s\"),greater-or-equal(datetime,%s),less-than(datetime,%s)", metricID, since.Format(time.RFC3339), until.Format(time.RFC3339))
	items, err := fetchAllJSONAPI(c, "/api/events", map[string]string{"filter": filter, "page[size]": "200", "sort": "datetime"}, limit)
	if err != nil {
		return nil, 0, err
	}
	counts := map[string]int{}
	display := map[string]string{}
	anchorOrders := 0
	for _, item := range items {
		names := orderEventProductNames(item)
		if !productNamesContain(names, product) {
			continue
		}
		anchorOrders++
		seenInOrder := map[string]bool{}
		for _, name := range names {
			if productNameMatches(name, product) {
				continue
			}
			key := normalizeProductAffinityName(name)
			if key == "" || seenInOrder[key] {
				continue
			}
			seenInOrder[key] = true
			counts[key]++
			if display[key] == "" {
				display[key] = name
			}
		}
	}
	rows := make([]map[string]any, 0, len(counts))
	for key, count := range counts {
		row := map[string]any{"name": display[key], "orders": count}
		if anchorOrders > 0 {
			row["affinity_rate"] = round3(float64(count) / float64(anchorOrders))
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if anyInt(rows[i]["orders"]) == anyInt(rows[j]["orders"]) {
			return fmt.Sprint(rows[i]["name"]) < fmt.Sprint(rows[j]["name"])
		}
		return anyInt(rows[i]["orders"]) > anyInt(rows[j]["orders"])
	})
	return rows, anchorOrders, nil
}

func orderEventProductNames(item map[string]any) []string {
	var names []string
	for _, path := range []string{
		"attributes.properties.ProductName",
		"attributes.properties.$ProductName",
		"attributes.properties.Product Name",
		"attributes.properties.product_name",
		"attributes.event_properties.ProductName",
		"attributes.event_properties.$ProductName",
	} {
		collectProductNameValue(anyPath(item, path), &names)
	}
	for _, path := range []string{
		"attributes.properties.ItemNames",
		"attributes.properties.Items",
		"attributes.properties.LineItems",
		"attributes.properties.Products",
		"attributes.properties.products",
		"attributes.properties.line_items",
		"attributes.properties.$extra.Items",
		"attributes.event_properties.ItemNames",
		"attributes.event_properties.Items",
	} {
		collectProductNameValue(anyPath(item, path), &names)
	}
	return uniqueProductNames(names)
}

func collectProductNameValue(value any, names *[]string) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		if name := strings.TrimSpace(v); name != "" {
			*names = append(*names, name)
		}
	case []string:
		for _, item := range v {
			collectProductNameValue(item, names)
		}
	case []any:
		for _, item := range v {
			collectProductNameValue(item, names)
		}
	case map[string]any:
		for _, key := range []string{"ProductName", "$ProductName", "Product Name", "product_name", "name", "Name", "title", "Title"} {
			collectProductNameValue(v[key], names)
		}
		for _, key := range []string{"ItemNames", "Items", "LineItems", "Products", "products", "line_items"} {
			collectProductNameValue(v[key], names)
		}
	}
}

func productNamesContain(names []string, product string) bool {
	for _, name := range names {
		if productNameMatches(name, product) {
			return true
		}
	}
	return false
}

func productNameMatches(name, product string) bool {
	name = normalizeProductAffinityName(name)
	product = normalizeProductAffinityName(product)
	return name != "" && product != "" && (name == product || strings.Contains(name, product) || strings.Contains(product, name))
}

func uniqueProductNames(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, name := range in {
		key := normalizeProductAffinityName(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, strings.TrimSpace(name))
	}
	return out
}

func normalizeProductAffinityName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func newReportConsentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "consent",
		Short:       "Consent status breakdown",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_profiles", "count_email_sms_consent"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			stats, err := buildProfilesStats(c, time.Now(), 1000)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"email_subscribed": stats["email_subscribed"], "sms_subscribed": stats["sms_subscribed"], "total_active": stats["total_active"], "total_suppressed": stats["total_suppressed"]}, flags)
		},
	}
	return cmd
}

func profileSpendRows(c flowClient, limit int) ([]map[string]any, error) {
	metricID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, err
	}
	resp, _, err := c.Post("/api/metric-aggregates", metricAggregateBody(metricID, []string{"count", "sum_value"}, []string{"Profile ID"}, time.Now().AddDate(-5, 0, 0), time.Now()))
	if err != nil {
		return nil, classifyAPIError(err)
	}
	var rows []map[string]any
	for _, r := range metricRows(resp) {
		profileID := firstNonEmptyString(firstStrings(r.Dimensions, 1)...)
		if profileID == "" {
			continue
		}
		row := map[string]any{"profile_id": profileID, "total_spend": r.Measurements["sum_value"], "order_count": int(r.Measurements["count"])}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return anyFloat(rows[i]["total_spend"]) > anyFloat(rows[j]["total_spend"]) })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	for i := range rows {
		profileID := fmt.Sprint(rows[i]["profile_id"])
		if p, err := c.Get("/api/profiles/"+url.PathEscape(profileID), map[string]string{"fields[profile]": "email"}); err == nil {
			rows[i]["email"] = firstString(p, "data.attributes.email")
		}
	}
	return rows, nil
}

func profileIDsWithOrders(c flowClient, since, until time.Time) (map[string]bool, error) {
	metricID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, err
	}
	resp, _, err := c.Post("/api/metric-aggregates", metricAggregateBody(metricID, []string{"count"}, []string{"Profile ID"}, since, until))
	if err != nil {
		return nil, classifyAPIError(err)
	}
	out := map[string]bool{}
	for _, r := range metricRows(resp) {
		if len(r.Dimensions) > 0 && r.Measurements["count"] > 0 {
			out[r.Dimensions[0]] = true
		}
	}
	return out, nil
}

type metricRow struct {
	Dimensions   []string
	Measurements map[string]float64
}

func metricRows(raw json.RawMessage) []metricRow {
	var parsed map[string]any
	if json.Unmarshal(raw, &parsed) != nil {
		return nil
	}
	results, _ := anyPath(parsed, "data.attributes.data").([]any)
	var rows []metricRow
	for _, item := range results {
		row, _ := item.(map[string]any)
		dims := anyStringSlice(row["dimensions"])
		meas, _ := row["measurements"].(map[string]any)
		out := metricRow{Dimensions: dims, Measurements: map[string]float64{}}
		for name, rawVals := range meas {
			for _, v := range anySlice(rawVals) {
				out.Measurements[name] += anyFloat(v)
			}
		}
		rows = append(rows, out)
	}
	return rows
}

func topAggregateRows(c flowClient, metricID, dimension, measurement string, since, until time.Time, limit int) []map[string]any {
	resp, _, err := c.Post("/api/metric-aggregates", metricAggregateBody(metricID, []string{measurement, "count"}, []string{dimension}, since, until))
	if err != nil {
		return []map[string]any{{"error": err.Error(), "dimension": dimension}}
	}
	var rows []map[string]any
	for _, r := range metricRows(resp) {
		name := "(none)"
		if len(r.Dimensions) > 0 && r.Dimensions[0] != "" {
			name = r.Dimensions[0]
		}
		rows = append(rows, map[string]any{"name": name, measurement: r.Measurements[measurement], "count": int(r.Measurements["count"])})
	}
	sort.Slice(rows, func(i, j int) bool { return anyFloat(rows[i][measurement]) > anyFloat(rows[j][measurement]) })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func engagementRateRows(c flowClient, dimension string, since, until time.Time) ([]map[string]any, error) {
	receivedID, err := resolveMetricID(c, "Received Email")
	if err != nil {
		return nil, err
	}
	openedID, err := resolveMetricID(c, "Opened Email")
	if err != nil {
		return nil, err
	}
	clickedID, err := resolveMetricID(c, "Clicked Email")
	if err != nil {
		return nil, err
	}
	received := topAggregateRows(c, receivedID, dimension, "count", since, until, 0)
	opened := keyedMetricValues(topAggregateRows(c, openedID, dimension, "count", since, until, 0), "count")
	clicked := keyedMetricValues(topAggregateRows(c, clickedID, dimension, "count", since, until, 0), "count")
	for _, row := range received {
		name := fmt.Sprint(row["name"])
		delivered := anyFloat(row["count"])
		openCount := opened[name]
		clickCount := clicked[name]
		row["opens"] = int(openCount)
		row["clicks"] = int(clickCount)
		if delivered > 0 {
			row["open_rate"] = openCount / delivered * 100
			row["click_rate"] = clickCount / delivered * 100
		}
	}
	return received, nil
}

func domainReputationRows(c flowClient, since, until time.Time) []map[string]any {
	metrics := map[string]string{"received": "Received Email", "bounced": "Bounced Email", "opened": "Opened Email", "spam": "Marked Email as Spam"}
	values := map[string]map[string]float64{}
	for key, name := range metrics {
		id, err := resolveMetricID(c, name)
		if err != nil {
			continue
		}
		values[key] = keyedMetricValues(topAggregateRows(c, id, "Email Domain", "count", since, until, 0), "count")
	}
	domains := map[string]bool{}
	for _, m := range values {
		for d := range m {
			domains[d] = true
		}
	}
	var rows []map[string]any
	for domain := range domains {
		received := values["received"][domain]
		bounced := values["bounced"][domain]
		spam := values["spam"][domain]
		row := map[string]any{"domain": domain, "received": int(received), "bounced": int(bounced), "opened": int(values["opened"][domain]), "spam_complaints": int(spam)}
		if received > 0 {
			row["bounce_rate"] = bounced / received * 100
			row["complaint_rate"] = spam / received * 100
			row["flagged"] = anyFloat(row["bounce_rate"]) > 1 || anyFloat(row["complaint_rate"]) > 0.05
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return anyFloat(rows[i]["bounce_rate"]) > anyFloat(rows[j]["bounce_rate"]) })
	return rows
}

func metricTotalsByName(c flowClient, metricNames []string, since, until time.Time) []map[string]any {
	var rows []map[string]any
	for _, name := range metricNames {
		id, err := resolveMetricID(c, name)
		if err != nil {
			rows = append(rows, map[string]any{"metric": name, "error": err.Error()})
			continue
		}
		resp, _, err := c.Post("/api/metric-aggregates", metricAggregateBody(id, []string{"count", "unique", "sum_value"}, nil, since, until))
		if err != nil {
			rows = append(rows, map[string]any{"metric": name, "error": err.Error()})
			continue
		}
		rows = append(rows, map[string]any{"metric": name, "count": int(sumMeasurement(resp, "count")), "unique": int(sumMeasurement(resp, "unique")), "sum_value": sumMeasurement(resp, "sum_value")})
	}
	return rows
}

func addEngagementTrends(c flowClient, rows []map[string]any, dimension string, since, until time.Time) error {
	midpoint := since.Add(until.Sub(since) / 2)
	previous, err := engagementRateRows(c, dimension, since, midpoint)
	if err != nil {
		return err
	}
	current, err := engagementRateRows(c, dimension, midpoint, until)
	if err != nil {
		return err
	}
	annotateEngagementTrends(rows, previous, current)
	return nil
}

func annotateEngagementTrends(rows, previous, current []map[string]any) {
	previousByName := rowsByName(previous)
	currentByName := rowsByName(current)
	for _, row := range rows {
		name := fmt.Sprint(row["name"])
		prev := previousByName[name]
		cur := currentByName[name]
		openDelta := anyFloat(cur["open_rate"]) - anyFloat(prev["open_rate"])
		clickDelta := anyFloat(cur["click_rate"]) - anyFloat(prev["click_rate"])
		row["previous_open_rate"] = round3(anyFloat(prev["open_rate"]))
		row["current_open_rate"] = round3(anyFloat(cur["open_rate"]))
		row["open_rate_delta"] = round3(openDelta)
		row["click_rate_delta"] = round3(clickDelta)
		switch {
		case openDelta <= -5 || clickDelta <= -2:
			row["trend_flag"] = "declining"
		case openDelta >= 5 || clickDelta >= 2:
			row["trend_flag"] = "improving"
		default:
			row["trend_flag"] = "flat"
		}
	}
}

func rowsByName(rows []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range rows {
		out[fmt.Sprint(row["name"])] = row
	}
	return out
}

func clientAndWindow(flags *rootFlags, last string) (flowClient, time.Time, time.Time, error) {
	days, err := strconv.Atoi(strings.TrimSuffix(last, "d"))
	if err != nil || days <= 0 {
		return nil, time.Time{}, time.Time{}, usageErr(fmt.Errorf("--last must be a positive duration like 30d"))
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	until := time.Now()
	return c, until.AddDate(0, 0, -days), until, nil
}

func attributionDimension(by string) string {
	switch strings.ToLower(by) {
	case "campaign":
		return "Campaign Name"
	case "channel":
		return "$attributed_channel"
	default:
		return "$attributed_flow"
	}
}

func keyedMetricValues(rows []map[string]any, field string) map[string]float64 {
	out := map[string]float64{}
	for _, row := range rows {
		out[fmt.Sprint(row["name"])] = anyFloat(row[field])
	}
	return out
}

func profileEmailSubscribed(p map[string]any) bool {
	return boolish(anyPath(p, "attributes.subscriptions.email.marketing.can_receive_email_marketing")) || strings.EqualFold(stringFromMapPath(p, "attributes.subscriptions.email.marketing.consent"), "SUBSCRIBED")
}

func profileSMSSubscribed(p map[string]any) bool {
	return boolish(anyPath(p, "attributes.subscriptions.sms.marketing.can_receive_sms_marketing")) || strings.EqualFold(stringFromMapPath(p, "attributes.subscriptions.sms.marketing.consent"), "SUBSCRIBED")
}

func profileSuppressed(p map[string]any) bool {
	return boolish(anyPath(p, "attributes.suppressed")) || strings.Contains(strings.ToLower(suppressionReason(p)), "suppress")
}

func suppressionReason(p map[string]any) string {
	return firstNonEmptyString(stringFromMapPath(p, "attributes.subscriptions.email.marketing.suppression.reason"), stringFromMapPath(p, "attributes.properties.suppression_reason"), stringFromMapPath(p, "attributes.suppression.reason"))
}

func profileSummaryRow(p map[string]any) map[string]any {
	return map[string]any{"id": fmt.Sprint(p["id"]), "email": stringFromMapPath(p, "attributes.email"), "created": stringFromMapPath(p, "attributes.created")}
}

func listAuditFlags(list map[string]any, subscribers, flows int) []string {
	flags := []string{}
	if subscribers == 0 {
		flags = append(flags, "empty")
	}
	if flows == 0 {
		flags = append(flags, "not_flow_trigger")
	}
	if updated := parseDate(stringFromMapPath(list, "attributes.updated")); !updated.IsZero() && updated.Before(time.Now().AddDate(0, -3, 0)) {
		flags = append(flags, "not_updated_90d")
	}
	return flags
}

func listSubscriberCount(list map[string]any) int {
	return anyInt(firstNonEmptyString(
		stringFromMapPath(list, "attributes.profile_count"),
		stringFromMapPath(list, "attributes.profiles_count"),
		stringFromMapPath(list, "attributes.profile_count_estimate"),
	))
}

func normalizeTagName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(s)
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" && v != "<nil>" {
			return v
		}
	}
	return ""
}

func boolish(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || strings.EqualFold(t, "subscribed") || strings.EqualFold(t, "yes")
	default:
		return anyFloat(v) != 0
	}
}

func mapSlice(v any) []map[string]any {
	raw, _ := v.([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func anySlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case []float64:
		out := make([]any, 0, len(t))
		for _, x := range t {
			out = append(out, x)
		}
		return out
	default:
		return nil
	}
}
