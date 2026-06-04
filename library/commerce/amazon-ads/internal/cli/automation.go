package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/store"
	"github.com/spf13/cobra"
)

func newAutoNegateCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var threshold float64
	var minClicks int
	var maxChanges int
	var apply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "auto-negate",
		Short: "Plan negative exact keywords for high-spend zero-conversion search terms",
		Annotations: map[string]string{
			"mcp:destructive": "true",
			"mcp:open-world":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadSearchTermReport(reportPath)
			if err != nil {
				return err
			}
			plans := adsanalytics.AutoNegate(rows, threshold, minClicks)
			out := map[string]any{
				"report":     reportPath,
				"threshold":  threshold,
				"min_clicks": minClicks,
				"dry_run":    true,
				"plans":      plans,
				"count":      len(plans),
			}
			if apply {
				applyOut, err := applyAutomationPlans(cmd, flags, "auto-negate", reportPath, plans, automationApplyOptions{
					MaxChanges: maxChanges,
				})
				if err != nil {
					return err
				}
				for k, v := range applyOut {
					out[k] = v
				}
			}
			if err := attachAutomationAudit(cmd, out, "auto-negate", reportPath, automationMode(apply, flags.dryRun), plans, dbPath); err != nil {
				return err
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a Search Term Report CSV or JSON export")
	cmd.Flags().Float64Var(&threshold, "threshold", 15, "Minimum zero-conversion spend before planning a negative keyword")
	cmd.Flags().IntVar(&minClicks, "min-clicks", 20, "Minimum clicks before planning a negative keyword")
	cmd.Flags().IntVar(&maxChanges, "max-changes", 25, "Maximum changes allowed with --apply (0 disables)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply changes to Amazon Ads instead of printing a dry-run plan")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path for the automation audit record (defaults to the per-user store)")
	return cmd
}

func newAutoPromoteCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var minConversions int
	var maxACOS float64
	var bid float64
	var maxBid float64
	var maxChanges int
	var apply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "auto-promote",
		Short: "Plan exact-match keyword promotion for converting search terms",
		Annotations: map[string]string{
			"mcp:destructive": "true",
			"mcp:open-world":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadSearchTermReport(reportPath)
			if err != nil {
				return err
			}
			plans := adsanalytics.AutoPromote(rows, minConversions, maxACOS)
			out := map[string]any{
				"report":          reportPath,
				"min_conversions": minConversions,
				"max_acos":        maxACOS,
				"dry_run":         true,
				"plans":           plans,
				"count":           len(plans),
			}
			if apply {
				applyOut, err := applyAutomationPlans(cmd, flags, "auto-promote", reportPath, plans, automationApplyOptions{
					Bid:        bid,
					MaxBid:     maxBid,
					MaxChanges: maxChanges,
				})
				if err != nil {
					return err
				}
				for k, v := range applyOut {
					out[k] = v
				}
			}
			if err := attachAutomationAudit(cmd, out, "auto-promote", reportPath, automationMode(apply, flags.dryRun), plans, dbPath); err != nil {
				return err
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a Search Term Report CSV or JSON export")
	cmd.Flags().IntVar(&minConversions, "min-conversions", 3, "Minimum conversions before planning exact-match promotion")
	cmd.Flags().Float64Var(&maxACOS, "max-acos", 30, "Maximum ACOS percentage before promotion is blocked")
	cmd.Flags().Float64Var(&bid, "bid", 0, "Keyword bid to use with --apply when creating exact-match keywords")
	cmd.Flags().Float64Var(&maxBid, "max-bid", 5, "Maximum keyword bid allowed with --apply (0 disables)")
	cmd.Flags().IntVar(&maxChanges, "max-changes", 25, "Maximum changes allowed with --apply (0 disables)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply changes to Amazon Ads instead of printing a dry-run plan")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path for the automation audit record (defaults to the per-user store)")
	return cmd
}

func newBudgetRebalanceCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var totalBudget float64
	var maxDailyBudget float64
	var maxChanges int
	var apply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "budget-rebalance",
		Short: "Plan daily budget redistribution across campaigns by performance",
		Annotations: map[string]string{
			"mcp:destructive": "true",
			"mcp:open-world":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			if totalBudget <= 0 {
				return usageErr(fmt.Errorf("--total-budget must be greater than zero"))
			}
			rows, err := adsanalytics.LoadPerformanceReport(reportPath)
			if err != nil {
				return err
			}
			campaigns := adsanalytics.CampaignComparison(rows)
			plans := adsanalytics.BudgetRebalance(campaigns, totalBudget)
			out := map[string]any{
				"report":       reportPath,
				"total_budget": totalBudget,
				"dry_run":      true,
				"plans":        plans,
				"count":        len(plans),
			}
			if apply {
				applyOut, err := applyAutomationPlans(cmd, flags, "budget-rebalance", reportPath, plans, automationApplyOptions{
					MaxDailyBudget: maxDailyBudget,
					MaxChanges:     maxChanges,
				})
				if err != nil {
					return err
				}
				for k, v := range applyOut {
					out[k] = v
				}
			}
			if err := attachAutomationAudit(cmd, out, "budget-rebalance", reportPath, automationMode(apply, flags.dryRun), plans, dbPath); err != nil {
				return err
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a campaign performance CSV or JSON export")
	cmd.Flags().Float64Var(&totalBudget, "total-budget", 0, "Total daily budget to redistribute")
	cmd.Flags().Float64Var(&maxDailyBudget, "max-daily-budget", 0, "Maximum per-campaign daily budget allowed with --apply (0 disables)")
	cmd.Flags().IntVar(&maxChanges, "max-changes", 25, "Maximum changes allowed with --apply (0 disables)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply changes to Amazon Ads instead of printing a dry-run plan")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path for the automation audit record (defaults to the per-user store)")
	return cmd
}

func newBidRulesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bid-rules",
		Short: "Conditional keyword bid rules — use the `apply` subcommand to plan or apply",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBidRulesApplyCmd(flags))
	return cmd
}

func newBidRulesApplyCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var rulesPath string
	var maxBid float64
	var maxChanges int
	var apply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply conditional bid rules as a dry-run plan",
		Annotations: map[string]string{
			"mcp:destructive": "true",
			"mcp:open-world":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			if rulesPath == "" {
				return usageErr(fmt.Errorf("--file is required"))
			}
			rows, err := adsanalytics.LoadKeywordPerformanceReport(reportPath)
			if err != nil {
				return err
			}
			rules, err := adsanalytics.LoadBidRules(rulesPath)
			if err != nil {
				return err
			}
			plans := adsanalytics.ApplyBidRules(rows, rules)
			out := map[string]any{
				"report":     reportPath,
				"file":       rulesPath,
				"dry_run":    true,
				"plans":      plans,
				"count":      len(plans),
				"rule_count": len(rules),
			}
			if apply {
				applyOut, err := applyAutomationPlans(cmd, flags, "bid-rules apply", reportPath, plans, automationApplyOptions{
					MaxBid:     maxBid,
					MaxChanges: maxChanges,
				})
				if err != nil {
					return err
				}
				for k, v := range applyOut {
					out[k] = v
				}
			}
			if err := attachAutomationAudit(cmd, out, "bid-rules apply", reportPath, automationMode(apply, flags.dryRun), plans, dbPath); err != nil {
				return err
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a keyword performance CSV or JSON export")
	cmd.Flags().StringVar(&rulesPath, "file", "", "Path to bid rules JSON")
	cmd.Flags().Float64Var(&maxBid, "max-bid", 10, "Maximum keyword bid allowed with --apply (0 disables)")
	cmd.Flags().IntVar(&maxChanges, "max-changes", 25, "Maximum changes allowed with --apply (0 disables)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply changes to Amazon Ads instead of printing a dry-run plan")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path for the automation audit record (defaults to the per-user store)")
	return cmd
}

func newAutomationAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "automation-audit",
		Short: "List recent automation dry-run/apply audit records",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("amazon-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			audits, err := db.ListAutomationAudits(cmd.Context(), limit)
			if err != nil {
				return err
			}
			return printCommandJSON(cmd, flags, map[string]any{
				"db":     dbPath,
				"audits": audits,
				"count":  len(audits),
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum audit records to return")
	return cmd
}

func automationMode(apply, dryRun bool) string {
	if apply && !dryRun && cliutil.IsVerifyEnv() && !cliutil.IsVerifyLiveHTTPEnv() {
		return "verify_short_circuit"
	}
	if apply && !dryRun {
		return "apply"
	}
	return "dry_run"
}

func attachAutomationAudit(cmd *cobra.Command, out map[string]any, command, reportPath, mode string, plans any, dbPath string) error {
	payloadBody := map[string]any{
		"command": command,
		"mode":    mode,
		"report":  reportPath,
		"plans":   plans,
	}
	if out != nil {
		payloadBody["result"] = out
	}
	payload, err := json.Marshal(payloadBody)
	if err != nil {
		return fmt.Errorf("marshaling automation audit: %w", err)
	}
	planCount := 0
	switch v := plans.(type) {
	case []adsanalytics.AutoNegatePlan:
		planCount = len(v)
	case []adsanalytics.AutoPromotePlan:
		planCount = len(v)
	case []adsanalytics.BudgetRebalancePlan:
		planCount = len(v)
	case []adsanalytics.BidRulePlan:
		planCount = len(v)
	case []adsanalytics.WeeklyReviewAction:
		planCount = len(v)
	}
	if dbPath == "" {
		dbPath = defaultDBPath("amazon-ads-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return fmt.Errorf("opening automation audit store: %w", err)
	}
	defer db.Close()
	audit, err := db.AppendAutomationAudit(cmd.Context(), command, mode, reportPath, planCount, payload)
	if err != nil {
		return err
	}
	out["audit"] = map[string]any{
		"id":         audit.ID,
		"created_at": audit.CreatedAt,
	}
	return nil
}

type automationApplyOptions struct {
	Bid            float64
	MaxBid         float64
	MaxDailyBudget float64
	MaxChanges     int
}

func applyAutomationPlans(cmd *cobra.Command, flags *rootFlags, command, reportPath string, plans any, opts automationApplyOptions) (map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"dry_run": c.DryRun,
		"applied": false,
	}
	switch v := plans.(type) {
	case []adsanalytics.AutoNegatePlan:
		body, err := autoNegateApplyBody(v)
		if err != nil {
			return nil, err
		}
		if err := enforceMaxChanges(len(body), opts.MaxChanges); err != nil {
			return nil, err
		}
		return applyAutomationMutation(cmd, flags, c, "POST", "/v2/sp/negativeKeywords", body, out)
	case []adsanalytics.AutoPromotePlan:
		body, err := autoPromoteApplyBody(v, opts.Bid, opts.MaxBid)
		if err != nil {
			return nil, err
		}
		if err := enforceMaxChanges(len(body), opts.MaxChanges); err != nil {
			return nil, err
		}
		return applyAutomationMutation(cmd, flags, c, "POST", "/v2/sp/keywords", body, out)
	case []adsanalytics.BudgetRebalancePlan:
		body, err := budgetRebalanceApplyBody(v, opts.MaxDailyBudget)
		if err != nil {
			return nil, err
		}
		if err := enforceMaxChanges(len(body), opts.MaxChanges); err != nil {
			return nil, err
		}
		return applyAutomationMutation(cmd, flags, c, "PUT", "/v2/sp/campaigns", body, out)
	case []adsanalytics.BidRulePlan:
		body, err := bidRulesApplyBody(v, opts.MaxBid)
		if err != nil {
			return nil, err
		}
		if err := enforceMaxChanges(len(body), opts.MaxChanges); err != nil {
			return nil, err
		}
		return applyAutomationMutation(cmd, flags, c, "PUT", "/v2/sp/keywords", body, out)
	default:
		return nil, fmt.Errorf("unsupported automation apply command %q for %s", command, reportPath)
	}
}

func enforceMaxChanges(count, maxChanges int) error {
	if maxChanges > 0 && count > maxChanges {
		return usageErr(fmt.Errorf("--apply would change %d items, above --max-changes %d", count, maxChanges))
	}
	return nil
}

func applyAutomationMutation(cmd *cobra.Command, flags *rootFlags, c *client.Client, method, path string, body any, out map[string]any) (map[string]any, error) {
	sentCount := automationBodyLen(body)
	if sentCount == 0 {
		// No plans to apply — record a noop instead of claiming success.
		// success:true here was ambiguous (caller can't tell "applied the
		// thing" from "nothing to apply"); explicit noop:true keeps the
		// "nothing happened" path distinct from the "request succeeded"
		// path while preserving applied:false and skipped:true.
		out["applied"] = false
		out["skipped"] = true
		out["noop"] = true
		out["reason"] = "no matching plans"
		out["path"] = path
		out["status"] = 0
		out["sent_count"] = 0
		return out, nil
	}
	var data json.RawMessage
	var statusCode int
	var err error
	switch method {
	case "POST":
		data, statusCode, err = c.Post(cmd.Context(), path, body)
	case "PUT":
		data, statusCode, err = c.Put(cmd.Context(), path, body)
	default:
		return nil, fmt.Errorf("unsupported automation method %s", method)
	}
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	out["path"] = path
	out["status"] = statusCode
	out["sent_count"] = sentCount
	if isVerifySyntheticMutation(data) {
		out["applied"] = false
		out["success"] = false
		out["dry_run"] = true
		out["verify_short_circuit"] = true
		out["reason"] = "verify_short_circuit"
	} else if c.DryRun {
		out["applied"] = false
		out["success"] = false
		out["dry_run"] = true
	} else {
		out["applied"] = statusCode >= 200 && statusCode < 300
		out["success"] = statusCode >= 200 && statusCode < 300
	}
	if len(data) > 0 {
		var parsed any
		if err := json.Unmarshal(data, &parsed); err == nil {
			out["response"] = parsed
		}
	}
	return out, nil
}

func isVerifySyntheticMutation(data json.RawMessage) bool {
	if len(data) == 0 {
		return false
	}
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		return false
	}
	v, _ := envelope["__pp_verify_synthetic__"].(bool)
	return v
}

func automationBodyLen(body any) int {
	switch v := body.(type) {
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func autoNegateApplyBody(plans []adsanalytics.AutoNegatePlan) ([]map[string]any, error) {
	body := make([]map[string]any, 0, len(plans))
	seen := map[string]struct{}{}
	for _, plan := range plans {
		if plan.CampaignID == "" || plan.AdGroupID == "" {
			return nil, usageErr(fmt.Errorf("--apply requires campaign_id and ad_group_id in the search term report for %q", plan.SearchTerm))
		}
		key := automationDedupeKey(plan.CampaignID, plan.AdGroupID, strings.ToLower(strings.TrimSpace(plan.SearchTerm)), "negativeExact")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		body = append(body, map[string]any{
			"campaignId":  plan.CampaignID,
			"adGroupId":   plan.AdGroupID,
			"keywordText": plan.SearchTerm,
			"matchType":   "negativeExact",
			"state":       "enabled",
		})
	}
	return body, nil
}

func autoPromoteApplyBody(plans []adsanalytics.AutoPromotePlan, bid, maxBid float64) ([]map[string]any, error) {
	if len(plans) > 0 && bid <= 0 {
		return nil, usageErr(fmt.Errorf("--apply requires --bid greater than zero for exact keyword creation"))
	}
	if maxBid > 0 && bid > maxBid {
		return nil, usageErr(fmt.Errorf("--bid %.2f exceeds --max-bid %.2f", bid, maxBid))
	}
	body := make([]map[string]any, 0, len(plans))
	seen := map[string]struct{}{}
	for _, plan := range plans {
		if plan.CampaignID == "" || plan.AdGroupID == "" {
			return nil, usageErr(fmt.Errorf("--apply requires campaign_id and ad_group_id in the search term report for %q", plan.SearchTerm))
		}
		key := automationDedupeKey(plan.CampaignID, plan.AdGroupID, strings.ToLower(strings.TrimSpace(plan.SearchTerm)), "exact")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		body = append(body, map[string]any{
			"campaignId":  plan.CampaignID,
			"adGroupId":   plan.AdGroupID,
			"keywordText": plan.SearchTerm,
			"matchType":   "exact",
			"state":       "enabled",
			"bid":         bid,
		})
	}
	return body, nil
}

func budgetRebalanceApplyBody(plans []adsanalytics.BudgetRebalancePlan, maxDailyBudget float64) ([]map[string]any, error) {
	body := make([]map[string]any, 0, len(plans))
	seen := map[string]struct{}{}
	for _, plan := range plans {
		if plan.CampaignID == "" {
			return nil, usageErr(fmt.Errorf("--apply requires campaign_id in the performance report for campaign %q", plan.Campaign))
		}
		if _, ok := seen[plan.CampaignID]; ok {
			continue
		}
		seen[plan.CampaignID] = struct{}{}
		if maxDailyBudget > 0 && plan.Recommended > maxDailyBudget {
			return nil, usageErr(fmt.Errorf("recommended budget %.2f for campaign %q exceeds --max-daily-budget %.2f", plan.Recommended, plan.Campaign, maxDailyBudget))
		}
		body = append(body, map[string]any{
			"campaignId":  plan.CampaignID,
			"dailyBudget": plan.Recommended,
		})
	}
	return body, nil
}

func bidRulesApplyBody(plans []adsanalytics.BidRulePlan, maxBid float64) ([]map[string]any, error) {
	body := make([]map[string]any, 0, len(plans))
	seen := map[string]struct{}{}
	for _, plan := range plans {
		if plan.KeywordID == "" {
			return nil, usageErr(fmt.Errorf("--apply requires keyword_id in the keyword performance report for %q", plan.Keyword))
		}
		if _, ok := seen[plan.KeywordID]; ok {
			continue
		}
		seen[plan.KeywordID] = struct{}{}
		if maxBid > 0 && plan.RecommendedBid > maxBid {
			return nil, usageErr(fmt.Errorf("recommended bid %.2f for keyword %q exceeds --max-bid %.2f", plan.RecommendedBid, plan.Keyword, maxBid))
		}
		body = append(body, map[string]any{
			"keywordId": plan.KeywordID,
			"bid":       plan.RecommendedBid,
		})
	}
	return body, nil
}

func automationDedupeKey(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		clean = append(clean, strings.TrimSpace(part))
	}
	return strings.Join(clean, "\x00")
}
