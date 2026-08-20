package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/client"
	"github.com/spf13/cobra"
)

func newWeeklyReviewCmd(flags *rootFlags) *cobra.Command {
	var campaignReport string
	var searchTermReport string
	var keywordReport string
	var campaignKind string
	var searchTermKind string
	var keywordKind string
	var allowPartial bool
	var targetACOS float64
	var grossMargin float64
	var cogsPath string
	var negateThreshold float64
	var minClicks int
	var totalBudget float64
	var maxBid float64
	var maxDailyBudget float64
	var maxChanges int
	var maxBidChangePct float64
	var maxBudgetChangePct float64
	var maxTotalBudgetIncrease float64
	var currency string
	var apply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "weekly-review",
		Short: "Plan a Sponsored Products weekly optimization review from reports",
		Annotations: map[string]string{
			"mcp:destructive": "true",
			"mcp:open-world":  "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if campaignReport == "" && searchTermReport == "" && keywordReport == "" {
				return usageErr(fmt.Errorf("at least one of --campaign-report, --search-term-report, or --keyword-report is required"))
			}
			targetSource := weeklyTargetSource(targetACOS, grossMargin, cogsPath)
			if targetACOS <= 0 {
				if grossMargin > 0 {
					targetACOS = grossMargin
				} else if cogsPath != "" {
					costs, err := adsanalytics.LoadCOGS(cogsPath)
					if err != nil {
						return err
					}
					targetACOS = averageBreakEvenACOS(costs)
				}
			}
			if targetACOS <= 0 {
				return usageErr(fmt.Errorf("--target-acos or margin/COGS config is required"))
			}

			var campaignRows []adsanalytics.PerformanceRow
			var campaignSchema adsanalytics.NormalizedSchemaReport
			if campaignReport != "" {
				rows, report, err := loadSchemaPerformanceReport(cmd, campaignReport, reportLoadOptions{ReportKind: campaignKind, AllowPartial: allowPartial, Command: "weekly-review"})
				if err != nil {
					return err
				}
				campaignRows, campaignSchema = rows, report
			}
			var searchRows []adsanalytics.SearchTermPerformance
			var searchSchema adsanalytics.NormalizedSchemaReport
			if searchTermReport != "" {
				rows, report, err := loadSchemaSearchTermReport(cmd, searchTermReport, reportLoadOptions{ReportKind: searchTermKind, AllowPartial: allowPartial, Command: "weekly-review"})
				if err != nil {
					return err
				}
				searchRows, searchSchema = rows, report
			}
			var keywordRows []adsanalytics.KeywordPerformance
			var keywordSchema adsanalytics.NormalizedSchemaReport
			if keywordReport != "" {
				rows, report, err := loadSchemaKeywordReport(cmd, keywordReport, reportLoadOptions{ReportKind: keywordKind, AllowPartial: allowPartial, Command: "weekly-review"})
				if err != nil {
					return err
				}
				keywordRows, keywordSchema = rows, report
			}

			plan := adsanalytics.WeeklyReview(campaignRows, searchRows, keywordRows, adsanalytics.WeeklyReviewOptions{
				TargetACOSPercent:      targetACOS,
				NegateSpendThreshold:   negateThreshold,
				NegateMinClicks:        minClicks,
				MaxBid:                 maxBid,
				MaxDailyBudget:         maxDailyBudget,
				MaxBidChangePercent:    maxBidChangePct,
				MaxBudgetChangePercent: maxBudgetChangePct,
				MaxTotalBudgetIncrease: maxTotalBudgetIncrease,
				TotalBudget:            totalBudget,
				Currency:               strings.ToUpper(strings.TrimSpace(currency)),
			})
			out := map[string]any{
				"dry_run":            true,
				"target_acos":        plan.TargetACOS,
				"target_acos_pct":    targetACOS,
				"target_acos_source": targetSource,
				"campaign_report":    campaignReport,
				"search_term_report": searchTermReport,
				"keyword_report":     keywordReport,
				"plan":               plan,
				"count":              len(plan.Actions),
			}
			reportKinds := map[string]string{}
			if campaignSchema.Kind != "" {
				reportKinds["campaign"] = campaignSchema.Kind
			}
			if searchSchema.Kind != "" {
				reportKinds["search_term"] = searchSchema.Kind
			}
			if keywordSchema.Kind != "" {
				reportKinds["keyword"] = keywordSchema.Kind
			}
			if len(reportKinds) > 0 {
				out["report_kinds"] = reportKinds
			}
			if apply {
				applyOut, err := applyWeeklyReviewActions(cmd, flags, plan.Actions, weeklyApplyOptions{
					MaxChanges: maxChanges,
				})
				if applyOut == nil && err != nil {
					return err
				}
				for k, v := range applyOut {
					out[k] = v
				}
				if err != nil {
					if auditErr := attachAutomationAudit(cmd, out, "weekly-review", strings.Join([]string{campaignReport, searchTermReport, keywordReport}, ","), automationMode(apply, flags.dryRun), plan.Actions, dbPath); auditErr != nil {
						return auditErr
					}
					if printErr := printCommandJSON(cmd, flags, out); printErr != nil {
						return printErr
					}
					return err
				}
			}
			if err := attachAutomationAudit(cmd, out, "weekly-review", strings.Join([]string{campaignReport, searchTermReport, keywordReport}, ","), automationMode(apply, flags.dryRun), plan.Actions, dbPath); err != nil {
				return err
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&campaignReport, "campaign-report", "", "Sponsored Products campaign CSV/TSV/JSON/GZIP report")
	cmd.Flags().StringVar(&searchTermReport, "search-term-report", "", "Sponsored Products search term CSV/TSV/JSON/GZIP report")
	cmd.Flags().StringVar(&keywordReport, "keyword-report", "", "Sponsored Products keyword CSV/TSV/JSON/GZIP report")
	cmd.Flags().StringVar(&campaignKind, "campaign-report-kind", "", "Campaign report schema kind (default sp-campaign-daily)")
	cmd.Flags().StringVar(&searchTermKind, "search-term-report-kind", "", "Search term report schema kind (default sp-search-term)")
	cmd.Flags().StringVar(&keywordKind, "keyword-report-kind", "", "Keyword report schema kind (default sp-keyword)")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Allow missing schema columns with a warning")
	cmd.Flags().Float64Var(&targetACOS, "target-acos", 0, "Required target ACOS percentage unless margin/COGS config is supplied")
	cmd.Flags().Float64Var(&grossMargin, "gross-margin-pct", 0, "Gross margin percentage to use as break-even ACOS when --target-acos is omitted")
	cmd.Flags().StringVar(&cogsPath, "cogs-file", "", "COGS TOML file used to derive average break-even ACOS when --target-acos is omitted")
	cmd.Flags().Float64Var(&negateThreshold, "negate-threshold", 10, "Spend threshold for zero-order negative keyword candidates")
	cmd.Flags().IntVar(&minClicks, "min-clicks", 20, "Minimum clicks before planning a negative keyword")
	cmd.Flags().Float64Var(&totalBudget, "total-budget", 0, "Total daily budget to redistribute across campaign actions")
	cmd.Flags().Float64Var(&maxBid, "max-bid", 10, "Maximum keyword bid allowed with --apply (0 disables)")
	cmd.Flags().Float64Var(&maxDailyBudget, "max-daily-budget", 0, "Maximum per-campaign daily budget allowed with --apply (0 disables)")
	cmd.Flags().IntVar(&maxChanges, "max-changes", 25, "Maximum changes allowed with --apply (0 disables)")
	cmd.Flags().Float64Var(&maxBidChangePct, "max-bid-change-pct", 25, "Maximum bid increase or decrease percentage per action (0 disables)")
	cmd.Flags().Float64Var(&maxBudgetChangePct, "max-budget-change-pct", 25, "Maximum budget increase or decrease percentage per action (0 disables)")
	cmd.Flags().Float64Var(&maxTotalBudgetIncrease, "max-total-budget-increase", 0, "Maximum aggregate daily budget increase across actions (0 disables)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code for planned budget, bid, audit, and rollback metadata")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply changes to Amazon Ads instead of printing a dry-run plan")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path for the automation audit record (defaults to the per-user store)")
	return cmd
}

type weeklyApplyOptions struct {
	MaxChanges int
}

func applyWeeklyReviewActions(cmd *cobra.Command, flags *rootFlags, actions []adsanalytics.WeeklyReviewAction, opts weeklyApplyOptions) (map[string]any, error) {
	if err := enforceMaxChanges(len(actions), opts.MaxChanges); err != nil {
		return nil, err
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"dry_run": c.DryRun,
		"applied": false,
	}
	verified := actions
	var skipped []adsanalytics.WeeklyReviewSkip
	if !c.DryRun {
		verified, skipped = verifyWeeklyReviewCurrentState(cmd, c, actions)
	}
	if len(skipped) > 0 {
		out["skipped"] = skipped
	}
	batches := weeklyReviewMutationBatches(verified)
	out["sent_count"] = len(verified)
	if len(verified) == 0 {
		out["noop"] = true
		out["reason"] = "no verified actions"
		return out, nil
	}
	responses := []map[string]any{}
	for _, batch := range batches {
		mutated, err := applyAutomationMutation(cmd, flags, c, batch.Method, batch.Path, batch.Body, map[string]any{})
		if err != nil {
			out["responses"] = responses
			out["partial_failure"] = len(responses) > 0
			out["failed_batch"] = map[string]any{
				"method": batch.Method,
				"path":   batch.Path,
				"count":  len(batch.Body),
			}
			out["error"] = err.Error()
			return out, err
		}
		responses = append(responses, mutated)
		if success, _ := mutated["success"].(bool); success {
			out["applied"] = true
			out["success"] = true
		}
	}
	out["responses"] = responses
	return out, nil
}

type weeklyMutationBatch struct {
	Method string
	Path   string
	Body   []map[string]any
}

func weeklyReviewMutationBatches(actions []adsanalytics.WeeklyReviewAction) []weeklyMutationBatch {
	keywords := []map[string]any{}
	campaigns := []map[string]any{}
	negativeKeywords := []map[string]any{}
	seenKeywords := map[string]bool{}
	seenCampaigns := map[string]bool{}
	seenNegativeKeywords := map[string]bool{}
	for _, action := range actions {
		switch action.Type {
		case "lower_bid", "raise_bid":
			if action.Entity.KeywordID == "" || seenKeywords[action.Entity.KeywordID] {
				continue
			}
			seenKeywords[action.Entity.KeywordID] = true
			keywords = append(keywords, map[string]any{
				"keywordId": action.Entity.KeywordID,
				"bid":       action.ProposedBid,
			})
		case "adjust_budget":
			if action.Entity.CampaignID == "" || seenCampaigns[action.Entity.CampaignID] {
				continue
			}
			seenCampaigns[action.Entity.CampaignID] = true
			campaigns = append(campaigns, map[string]any{
				"campaignId":  action.Entity.CampaignID,
				"dailyBudget": action.ProposedBudget,
			})
		case "create_negative_keyword":
			key := strings.Join([]string{action.Entity.CampaignID, action.Entity.AdGroupID, action.Entity.Text, action.Entity.MatchType}, "\x00")
			if action.Entity.CampaignID == "" || action.Entity.AdGroupID == "" || action.Entity.Text == "" || seenNegativeKeywords[key] {
				continue
			}
			seenNegativeKeywords[key] = true
			negativeKeywords = append(negativeKeywords, map[string]any{
				"campaignId":  action.Entity.CampaignID,
				"adGroupId":   action.Entity.AdGroupID,
				"keywordText": action.Entity.Text,
				"matchType":   action.Entity.MatchType,
				"state":       "enabled",
			})
		}
	}
	var batches []weeklyMutationBatch
	if len(keywords) > 0 {
		batches = append(batches, weeklyMutationBatch{Method: "PUT", Path: "/v2/sp/keywords", Body: keywords})
	}
	if len(campaigns) > 0 {
		batches = append(batches, weeklyMutationBatch{Method: "PUT", Path: "/v2/sp/campaigns", Body: campaigns})
	}
	if len(negativeKeywords) > 0 {
		batches = append(batches, weeklyMutationBatch{Method: "POST", Path: "/v2/sp/negativeKeywords", Body: negativeKeywords})
	}
	return batches
}

func verifyWeeklyReviewCurrentState(cmd *cobra.Command, c *client.Client, actions []adsanalytics.WeeklyReviewAction) ([]adsanalytics.WeeklyReviewAction, []adsanalytics.WeeklyReviewSkip) {
	verified := make([]adsanalytics.WeeklyReviewAction, 0, len(actions))
	var skipped []adsanalytics.WeeklyReviewSkip
	for _, action := range actions {
		ok, reason := verifyWeeklyAction(cmd, c, action)
		if ok {
			verified = append(verified, action)
		} else {
			skipped = append(skipped, adsanalytics.WeeklyReviewSkip{Type: action.Type, Entity: action.Entity, Reason: reason})
		}
	}
	return verified, skipped
}

func verifyWeeklyAction(cmd *cobra.Command, c *client.Client, action adsanalytics.WeeklyReviewAction) (bool, string) {
	switch action.Type {
	case "lower_bid", "raise_bid":
		if action.Entity.KeywordID == "" {
			return false, "missing keywordId"
		}
		raw, err := c.Get(cmd.Context(), "/v2/sp/keywords/"+action.Entity.KeywordID, nil)
		if err != nil {
			return false, "could not fetch current keyword state: " + err.Error()
		}
		if !jsonNumberMatches(raw, "bid", action.CurrentBid) {
			return false, "current bid no longer matches report"
		}
	case "adjust_budget":
		if action.Entity.CampaignID == "" {
			return false, "missing campaignId"
		}
		raw, err := c.Get(cmd.Context(), "/v2/sp/campaigns/"+action.Entity.CampaignID, nil)
		if err != nil {
			return false, "could not fetch current campaign state: " + err.Error()
		}
		if !jsonNumberMatches(raw, "dailyBudget", action.CurrentBudget) {
			return false, "current daily budget no longer matches report"
		}
	case "create_negative_keyword":
		if action.Entity.CampaignID == "" || action.Entity.AdGroupID == "" {
			return false, "missing campaignId or adGroupId"
		}
		raw, err := c.Get(cmd.Context(), "/v2/sp/adGroups/"+action.Entity.AdGroupID, nil)
		if err != nil {
			return false, "could not fetch current ad group state: " + err.Error()
		}
		if !jsonStringMatches(raw, "campaignId", action.Entity.CampaignID) {
			return false, "current ad group campaignId no longer matches report"
		}
	}
	return true, ""
}

func jsonNumberMatches(raw json.RawMessage, key string, expected float64) bool {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	number, ok := jsonValueNumber(value)
	return ok && math.Abs(number-expected) < 0.005
}

func jsonValueNumber(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case string:
		return parseFloatLoose(n)
	default:
		return 0, false
	}

}

func jsonStringMatches(raw json.RawMessage, key, expected string) bool {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	stringValue, ok := value.(string)
	if !ok {
		stringValue = fmt.Sprint(value)
	}
	return stringValue == expected
}

func parseFloatLoose(raw string) (float64, bool) {
	var value float64
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%f", &value)
	return value, err == nil
}

func averageBreakEvenACOS(costs map[string]adsanalytics.ProductCost) float64 {
	total := 0.0
	count := 0
	for _, cost := range costs {
		if cost.SellingPrice <= 0 || cost.COGS <= 0 {
			continue
		}
		total += ((cost.SellingPrice - cost.COGS) / cost.SellingPrice) * 100
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func weeklyTargetSource(targetACOS, grossMargin float64, cogsPath string) string {
	if targetACOS > 0 {
		return "target_acos"
	}
	if grossMargin > 0 {
		return "gross_margin_pct"
	}
	if cogsPath != "" {
		return "cogs_file_average_break_even_acos"
	}
	return "target_acos"
}
