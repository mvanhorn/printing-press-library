// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"math"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

type campaignABVariationResult struct {
	VariationID         string  `json:"variation_id"`
	Label               string  `json:"label"`
	Subject             string  `json:"subject"`
	Recipients          float64 `json:"recipients"`
	OpensUnique         float64 `json:"opens_unique"`
	OpenRate            float64 `json:"open_rate"`
	ClicksUnique        float64 `json:"clicks_unique"`
	ClickRate           float64 `json:"click_rate"`
	Orders              float64 `json:"orders"`
	Revenue             float64 `json:"revenue"`
	RevenuePerRecipient float64 `json:"revenue_per_recipient"`
}

type campaignABComparison struct {
	LeaderID    string  `json:"leader_id"`
	VariationID string  `json:"variation_id"`
	PValue      float64 `json:"p_value"`
}

type campaignABInferredWinner struct {
	VariationID string `json:"variation_id"`
	Label       string `json:"label"`
	Basis       string `json:"basis"`
}

type campaignABResults struct {
	MessageID             string                      `json:"message_id"`
	ABTest                any                         `json:"ab_test"`
	Timeframe             *platform.ResolvedWindow    `json:"timeframe"`
	Metric                string                      `json:"metric"`
	Variations            []campaignABVariationResult `json:"variations"`
	Verdict               string                      `json:"verdict"`
	Leader                string                      `json:"leader"`
	PValues               []campaignABComparison      `json:"p_values"`
	Notes                 []string                    `json:"notes"`
	KlaviyoWinnerInferred *campaignABInferredWinner   `json:"klaviyo_winner_inferred,omitempty"`
}

func newCampaignsABTestResultsCmd(flags *rootFlags) *cobra.Command {
	var campaignID, metric string
	cmd := &cobra.Command{
		Use: "results", Short: "Compare per-variation email rates with two-proportion z-tests",
		Args: cobra.NoArgs, SilenceUsage: true, Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(campaignID) == "" {
				return usageErr(fmt.Errorf("--campaign-id is required (campaign or message ID)"))
			}
			if cmd.Flags().Changed("metric") {
				if _, err := campaignABMetric(metric); err != nil {
					return err
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.DryRun, c.NoCache = false, true
			m, err := resolveCampaignABMessage(c, campaignID)
			if err != nil {
				return err
			}
			settings := anyPath(m.Data, "attributes.definition.ab_test")
			if settings == nil {
				return usageErr(fmt.Errorf("message %s is not an A/B test (no ab_test settings)", m.ID))
			}
			if metric == "" {
				switch stringFromMapPath(m.Data, "attributes.definition.ab_test.winner_metric") {
				case "opens_unique":
					metric = "opens"
				case "placed_orders_unique":
					metric = "orders"
				default:
					metric = "clicks"
				}
			}
			result := campaignABResults{MessageID: m.ID, ABTest: settings, Metric: metric, Verdict: "too_early",
				Variations: []campaignABVariationResult{}, PValues: []campaignABComparison{}, Notes: []string{}}
			for _, v := range m.Variations {
				if v.Channel == "email" {
					result.Variations = append(result.Variations, campaignABVariationResult{VariationID: v.ID, Label: string(rune('A' + len(result.Variations))), Subject: v.Subject})
				}
			}
			if len(result.Variations) < 2 {
				return usageErr(fmt.Errorf("message %s is not an A/B test (only one email variation)", m.ID))
			}
			if metric == "opens" {
				result.Notes = append(result.Notes, campaignABOpensNote)
			}
			if m.Status != "sent" && m.Status != "sending" {
				result.Notes = append(result.Notes, "Message has not been sent; results are too early.")
				return printCampaignABResults(cmd, flags, result)
			}
			window, err := campaignABWindow(m.SendTime)
			if err != nil {
				return err
			}
			result.Timeframe = &window
			if flags.platformSession != nil {
				flags.platformSession.SetResolvedWindow(window)
			}
			if window.Clamped {
				result.Notes = append(result.Notes, "Reporting timeframe was capped to the provider limit; older events are excluded.")
			}
			body, _, err := buildKlaviyoReportBody(c, klaviyoReportRequest{
				Type:       "campaign-values-report",
				Statistics: []string{"recipients", "opens_unique", "open_rate", "clicks_unique", "click_rate", "conversion_uniques", "conversion_value", "conversion_rate"},
				Timeframe:  map[string]any{"start": window.Start.Format("2006-01-02T15:04:05Z07:00"), "end": window.End.Format("2006-01-02T15:04:05Z07:00")},
			}, dryRunOK(flags))
			if err != nil {
				return err
			}
			attrs := anyPath(body, "data.attributes").(map[string]any)
			attrs["group_by"] = []string{"campaign_id", "campaign_message_id", "variation", "variation_name"}
			attrs["filter"] = fmt.Sprintf("equals(campaign_message_id,%q)", m.ID)
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "message_id": m.ID, "ab_test": settings, "timeframe": window, "report_body": body}, flags)
			}
			raw, _, err := postKlaviyoReport(c, "/api/campaign-values-reports/", body)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			rows, err := klaviyoReportRows(raw)
			if err != nil {
				return err
			}
			if err := populateCampaignABResults(&result, rows, m); err != nil {
				return err
			}
			return printCampaignABResults(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Campaign container or message ID")
	cmd.Flags().StringVar(&metric, "metric", "", "Compare clicks, opens or orders (default: test winner metric, else clicks)")
	return cmd
}

func populateCampaignABResults(result *campaignABResults, rows []map[string]any, message campaignModelMessage) error {
	byID := map[string]map[string]any{}
	for _, row := range rows {
		id := stringFromMapPath(row, "groupings.variation")
		if id == "" || byID[id] != nil {
			return fmt.Errorf("report returned a missing or duplicate variation ID %q", id)
		}
		if messageID := stringFromMapPath(row, "groupings.campaign_message_id"); messageID != "" && messageID != result.MessageID {
			return fmt.Errorf("report returned another message's results: %s", messageID)
		}
		byID[id] = row
	}
	leader, mostRecipients := 0, 0
	for i := range result.Variations {
		v := &result.Variations[i]
		row := byID[v.VariationID]
		v.Recipients = anyFloat(anyPath(row, "statistics.recipients"))
		v.OpensUnique = anyFloat(anyPath(row, "statistics.opens_unique"))
		v.OpenRate = anyFloat(anyPath(row, "statistics.open_rate"))
		v.ClicksUnique = anyFloat(anyPath(row, "statistics.clicks_unique"))
		v.ClickRate = anyFloat(anyPath(row, "statistics.click_rate"))
		v.Orders = anyFloat(anyPath(row, "statistics.conversion_uniques"))
		v.Revenue = anyFloat(anyPath(row, "statistics.conversion_value"))
		v.RevenuePerRecipient = rate(v.Revenue, v.Recipients)
		events := v.metricEvents(result.Metric)
		if v.Recipients < 0 || events < 0 || events > v.Recipients || math.Trunc(events) != events || math.Trunc(v.Recipients) != v.Recipients {
			return fmt.Errorf("variation %s has invalid metric uniques or recipient counts for a two-proportion z-test", v.VariationID)
		}
		if events < 100 {
			result.Notes = append(result.Notes, fmt.Sprintf("Small sample: variation %s has fewer than 100 %s events (%.0f).", v.Label, result.Metric, events))
		}
		if rate(events, v.Recipients) > rate(result.Variations[leader].metricEvents(result.Metric), result.Variations[leader].Recipients) {
			leader = i
		}
		if v.Recipients > result.Variations[mostRecipients].Recipients {
			mostRecipients = i
		}
	}
	result.Verdict = "no_clear_winner"
	if len(result.Variations) == 0 {
		return nil
	}
	best := result.Variations[leader]
	result.Leader = best.VariationID
	clear := len(result.Variations) > 1
	for i, v := range result.Variations {
		if i == leader {
			continue
		}
		p := campaignABZTest(best.metricEvents(result.Metric), best.Recipients, v.metricEvents(result.Metric), v.Recipients)
		result.PValues = append(result.PValues, campaignABComparison{LeaderID: best.VariationID, VariationID: v.VariationID, PValue: p})
		clear = clear && p < 0.05
	}
	if clear {
		result.Verdict = "clear_winner"
	}
	settings, _ := result.ABTest.(map[string]any)
	if anyFloat(settings["winner_percentage"]) <= 0 {
		result.Notes = append(result.Notes, "No Klaviyo rollout winner: winner_percentage is zero.")
		return nil
	}
	sent, err := time.Parse(time.RFC3339, message.SendTime)
	wait := time.Duration(anyFloat(settings["winner_wait_hours"]) * float64(time.Hour))
	if message.Status != "sent" || err != nil || klaviyoWindowNow().Before(sent.Add(wait)) {
		result.Notes = append(result.Notes, "A/B test is still running; the Klaviyo rollout winner is not yet inferred.")
		return nil
	}
	v := result.Variations[mostRecipients]
	for i, other := range result.Variations {
		if i != mostRecipients && other.Recipients >= v.Recipients {
			result.Notes = append(result.Notes, "Recipient counts are tied; the Klaviyo rollout winner cannot be inferred.")
			return nil
		}
	}
	result.KlaviyoWinnerInferred = &campaignABInferredWinner{VariationID: v.VariationID, Label: v.Label, Basis: "inferred from recipient counts"}
	return nil
}

func (v campaignABVariationResult) metricEvents(metric string) float64 {
	switch metric {
	case "opens":
		return v.OpensUnique
	case "orders":
		return v.Orders
	default:
		return v.ClicksUnique
	}
}

// Two-sided pooled two-proportion z-test; zero samples or zero variance give no evidence.
func campaignABZTest(x1, n1, x2, n2 float64) float64 {
	if n1 <= 0 || n2 <= 0 {
		return 1
	}
	pooled := (x1 + x2) / (n1 + n2)
	variance := pooled * (1 - pooled) * (1/n1 + 1/n2)
	if variance <= 0 {
		return 1
	}
	z := (x1/n1 - x2/n2) / math.Sqrt(variance)
	return math.Erfc(math.Abs(z) / math.Sqrt2)
}

func printCampaignABResults(cmd *cobra.Command, flags *rootFlags, result campaignABResults) error {
	if flags.asJSON || flags.agent || flags.csv || flags.compact || flags.selectFields != "" {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if flags.quiet {
		return nil
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Message %s: %s (%s); leader: %s\n", result.MessageID, result.Verdict, result.Metric, result.Leader)
	table := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "Label\tSubject\tRecipients\tUnique opens\tOpen rate\tUnique clicks\tClick rate\tOrders\tRevenue\tRevenue/recipient")
	for _, v := range result.Variations {
		fmt.Fprintf(table, "%s\t%q\t%.0f\t%.0f\t%.4f\t%.0f\t%.4f\t%.0f\t%.2f\t%.4f\n", v.Label, v.Subject, v.Recipients, v.OpensUnique, v.OpenRate, v.ClicksUnique, v.ClickRate, v.Orders, v.Revenue, v.RevenuePerRecipient)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	for _, comparison := range result.PValues {
		fmt.Fprintf(&out, "%s vs %s: p=%.4f\n", comparison.LeaderID, comparison.VariationID, comparison.PValue)
	}
	if v := result.KlaviyoWinnerInferred; v != nil {
		fmt.Fprintf(&out, "Klaviyo winner: %s (%s), %s\n", v.Label, v.VariationID, v.Basis)
	}
	for _, note := range result.Notes {
		fmt.Fprintln(&out, "Note:", note)
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), terminalText(cmd.OutOrStdout(), out.String()))
	return err
}
