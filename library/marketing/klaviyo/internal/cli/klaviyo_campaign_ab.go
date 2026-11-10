// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

const campaignABOpensNote = "Apple Mail Privacy Protection inflates opens; clicks or orders pick a truer winner."

type campaignABVariation struct {
	VariationID string `json:"variation_id"`
	Label       string `json:"label"`
	Subject     string `json:"subject"`
	PreviewText string `json:"preview_text"`
	Created     bool   `json:"created"`
}

type campaignABCreateResult struct {
	MessageID  string                `json:"message_id"`
	CampaignID string                `json:"campaign_id"`
	ABTest     map[string]any        `json:"ab_test"`
	Variations []campaignABVariation `json:"variations"`
	DryRun     bool                  `json:"dry_run,omitempty"`
}

func newCampaignsABTestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "ab-test", Short: "Create subject-line tests and compare variation results"}
	cmd.AddCommand(newCampaignsABTestCreateCmd(flags))
	cmd.AddCommand(newCampaignsABTestResultsCmd(flags))
	return cmd
}

func resolveCampaignABMessage(c templateReader, id string) (campaignModelMessage, error) {
	model, err := resolveCampaign(c, id)
	if err != nil {
		return campaignModelMessage{}, err
	}
	if len(model.Messages) != 1 {
		ids := make([]string, 0, len(model.Messages))
		for _, m := range model.Messages {
			ids = append(ids, m.ID)
		}
		return campaignModelMessage{}, usageErr(fmt.Errorf("campaign has %d messages; choose a message ID with --campaign-id: %s", len(ids), strings.Join(ids, ", ")))
	}
	m := model.Messages[0]
	// Keep create and results labels in creation order, regardless of variation names.
	sort.Slice(m.Variations, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, stringFromMapPath(m.Variations[i].Data, "attributes.created"))
		b, _ := time.Parse(time.RFC3339, stringFromMapPath(m.Variations[j].Data, "attributes.created"))
		if a.Equal(b) {
			return m.Variations[i].ID < m.Variations[j].ID
		}
		return a.Before(b)
	})
	for _, v := range m.Variations {
		if v.Channel == "email" {
			return m, nil
		}
	}
	return m, usageErr(fmt.Errorf("message %s has no email variation", m.ID))
}

func campaignABMetric(metric string) (string, error) {
	switch metric {
	case "clicks":
		return "clicks_unique", nil
	case "opens":
		return "opens_unique", nil
	case "orders":
		return "placed_orders_unique", nil
	default:
		return "", usageErr(fmt.Errorf("metric must be clicks, opens or orders"))
	}
}

func newCampaignsABTestCreateCmd(flags *rootFlags) *cobra.Command {
	var campaignID, metric string
	var subjects, previews []string
	var testPercent, waitHours int
	cmd := &cobra.Command{
		Use: "create", Short: "Add subject variations to one unscheduled draft email message",
		Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(campaignID) == "" {
				return usageErr(fmt.Errorf("--campaign-id is required (campaign or message ID)"))
			}
			winnerMetric, err := campaignABMetric(metric)
			if err != nil {
				return err
			}
			if testPercent < 1 || testPercent > 100 || waitHours < 1 || waitHours > 264 {
				return usageErr(fmt.Errorf("--test-percent must be 1 to 100; --wait-hours must be 1 to 264"))
			}
			if len(subjects) < 1 || len(subjects) > 6 {
				return usageErr(fmt.Errorf("provide 1 to 6 --subject values"))
			}
			if cmd.Flags().Changed("preview-text") && len(previews) != len(subjects) {
				return usageErr(fmt.Errorf("--preview-text count must equal --subject count"))
			}
			seen := map[string]bool{}
			for _, subject := range subjects {
				key := strings.TrimSpace(subject)
				if key == "" || strings.ContainsAny(subject, "\r\n") || seen[key] {
					return usageErr(fmt.Errorf("subjects must be non-empty, distinct and contain no line breaks"))
				}
				seen[key] = true
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
			if m.Status != "draft" || m.SendTime != "" {
				return usageErr(fmt.Errorf("message %s must be draft with no send time (status=%q, send_time=%q)", m.ID, m.Status, m.SendTime))
			}
			if anyPath(m.Data, "attributes.definition.ab_test") != nil || len(m.Variations) > 1 {
				return usageErr(fmt.Errorf("message %s already has an A/B test or multiple variations", m.ID))
			}
			a := m.Variations[0]
			if seen[strings.TrimSpace(a.Subject)] {
				return usageErr(fmt.Errorf("new subjects must differ from variation A's subject"))
			}
			if a.TemplateID == "" {
				return usageErr(fmt.Errorf("variation A has no template_id"))
			}
			settings := map[string]any{"winner_metric": winnerMetric, "winner_percentage": 100 - testPercent, "winner_wait_hours": waitHours}
			result := campaignABCreateResult{
				MessageID: m.ID, CampaignID: m.CampaignID, ABTest: settings, DryRun: dryRunOK(flags),
				Variations: []campaignABVariation{{VariationID: a.ID, Label: "A", Subject: a.Subject, PreviewText: a.PreviewText}},
			}
			for i, subject := range subjects {
				preview := a.PreviewText
				if len(previews) != 0 {
					preview = previews[i]
				}
				result.Variations = append(result.Variations, campaignABVariation{Label: string(rune('B' + i)), Subject: subject, PreviewText: preview})
			}
			if metric == "opens" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", campaignABOpensNote)
			}
			if result.DryRun {
				return printCampaignABCreate(cmd, flags, result)
			}
			created := []string{}
			for i := 1; i < len(result.Variations); i++ {
				v := &result.Variations[i]
				details := map[string]any{"channel": "email", "subject": v.Subject, "preview_text": v.PreviewText}
				for _, key := range []string{"template_id", "from_email", "from_label", "reply_to_email", "cc_email", "bcc_email"} {
					if value, exists := anyPath(a.Data, "attributes.definition.details").(map[string]any)[key]; exists {
						details[key] = value
					}
				}
				body := jsonAPIBody("campaign-variation", map[string]any{"definition": map[string]any{
					"name": stringFromMapPath(a.Data, "attributes.definition.name") + " - " + v.Label, "details": details,
				}}, map[string]any{"campaign-message": map[string]any{"data": map[string]any{"type": "campaign-message", "id": m.ID}}})
				raw, code, err := c.PostCampaignVariation(cmd.Context(), body)
				id := jsonAPIID(raw)
				if id != "" && id != a.ID {
					created = append(created, id)
				}
				if err != nil {
					if code == 0 || code >= 500 {
						err = fmt.Errorf("variation %s create outcome unknown; an unconfirmed variation may remain; request was not retried: %w", v.Label, err)
					}
					return rollbackCampaignAB(cmd.Context(), c, created, err)
				}
				if code != http.StatusCreated || id == "" || id == a.ID {
					return rollbackCampaignAB(cmd.Context(), c, created, fmt.Errorf("variation %s create unverified: expected HTTP 201 and a new ID, got HTTP %d ID %q; an unconfirmed variation may remain", v.Label, code, id))
				}
				v.VariationID, v.Created = id, true
			}
			body := map[string]any{"data": map[string]any{"type": "campaign-message", "id": m.ID,
				"attributes": map[string]any{"definition": map[string]any{"ab_test": settings}}}}
			if _, code, err := c.Patch(cmd.Context(), "/api/campaign-messages/"+url.PathEscape(m.ID), body); err != nil || code != http.StatusOK {
				if err == nil {
					err = fmt.Errorf("expected HTTP 200, got HTTP %d", code)
				}
				return rollbackCampaignAB(cmd.Context(), c, created, fmt.Errorf("A/B settings PATCH failed; settings may need inspection: %w", err))
			}
			verified, err := resolveCampaignABMessage(c, m.ID)
			if err == nil {
				err = verifyCampaignABCreate(verified, result)
			}
			if err != nil {
				return fmt.Errorf("A/B test read-back unverified for message %s (created variations: %s): %w", m.ID, strings.Join(created, ", "), err)
			}
			byID := map[string]campaignABVariation{}
			for _, v := range result.Variations {
				byID[v.VariationID] = v
			}
			for i, v := range verified.Variations {
				result.Variations[i] = byID[v.ID]
				result.Variations[i].Label = string(rune('A' + i))
			}
			return printCampaignABCreate(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Campaign container or message ID")
	cmd.Flags().StringArrayVar(&subjects, "subject", nil, "New subject (repeat 1 to 6 times)")
	cmd.Flags().StringArrayVar(&previews, "preview-text", nil, "Replacement preview text (repeat once per subject)")
	cmd.Flags().StringVar(&metric, "winner-metric", "clicks", "Winner metric: clicks, opens or orders")
	cmd.Flags().IntVar(&testPercent, "test-percent", 20, "Audience percentage split evenly across test variations (1 to 100)")
	cmd.Flags().IntVar(&waitHours, "wait-hours", 4, "Hours before choosing the winner (1 to 264)")
	return cmd
}

func rollbackCampaignAB(ctx context.Context, c flowClient, ids []string, cause error) error {
	// Cleanup must still run when the create's request context was cancelled.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	deleted, failed := []string{}, []string{}
	for _, id := range uniqueStrings(ids) {
		_, _, err := c.Delete(ctx, "/api/campaign-variations/"+url.PathEscape(id))
		if err != nil && !campaignNotFound(err) {
			failed = append(failed, fmt.Sprintf("%s (%v)", id, err))
		} else {
			deleted = append(deleted, id)
		}
	}
	return fmt.Errorf("A/B test failed: %w; rolled back variations: [%s]; could not roll back: [%s]", classifyAPIErrorOnly(cause), strings.Join(deleted, ", "), strings.Join(failed, ", "))
}

func verifyCampaignABCreate(m campaignModelMessage, want campaignABCreateResult) error {
	if err := verifyCampaignDraft(&campaignModel{Messages: []campaignModelMessage{m}}); err != nil {
		return err
	}
	settings, ok := anyPath(m.Data, "attributes.definition.ab_test").(map[string]any)
	percentage, hasPercentage := settings["winner_percentage"]
	wait, hasWait := settings["winner_wait_hours"]
	if !ok || !hasPercentage || !hasWait || len(settings) != len(want.ABTest) || settings["winner_metric"] != want.ABTest["winner_metric"] ||
		anyFloat(percentage) != anyFloat(want.ABTest["winner_percentage"]) ||
		anyFloat(wait) != anyFloat(want.ABTest["winner_wait_hours"]) {
		return fmt.Errorf("A/B settings differ from requested settings")
	}
	if len(m.Variations) != len(want.Variations) {
		return fmt.Errorf("expected %d variations, got %d", len(want.Variations), len(m.Variations))
	}
	byID := map[string]campaignModelVariation{}
	for _, v := range m.Variations {
		byID[v.ID] = v
	}
	for _, expected := range want.Variations {
		v, ok := byID[expected.VariationID]
		if !ok || v.Channel != "email" || v.Subject != expected.Subject || v.PreviewText != expected.PreviewText {
			return fmt.Errorf("variation %s (%s) does not match the planned email subject and preview text", expected.Label, expected.VariationID)
		}
	}
	return nil
}

func printCampaignABCreate(cmd *cobra.Command, flags *rootFlags, result campaignABCreateResult) error {
	if flags.asJSON || flags.agent || flags.csv || flags.compact || flags.selectFields != "" {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if flags.quiet {
		return nil
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Message %s; A/B settings: %s, winner %v%%, wait %vh\n", result.MessageID, result.ABTest["winner_metric"], result.ABTest["winner_percentage"], result.ABTest["winner_wait_hours"])
	if result.DryRun {
		out.WriteString("Dry run: planned variations and settings; no writes.\n")
	}
	table := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "Label\tVariation ID\tSubject\tPreview text\tCreated")
	for _, v := range result.Variations {
		fmt.Fprintf(table, "%s\t%s\t%q\t%q\t%t\n", v.Label, v.VariationID, v.Subject, v.PreviewText, v.Created)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(&out, "Next: campaigns test-send --campaign-id %s to preview every variation.\n", result.MessageID)
	_, err := fmt.Fprint(cmd.OutOrStdout(), terminalText(cmd.OutOrStdout(), out.String()))
	return err
}

func campaignABWindow(sendTime string) (platform.ResolvedWindow, error) {
	location, err := time.LoadLocation(defaultMetricTimezone())
	if err != nil {
		return platform.ResolvedWindow{}, err
	}
	sent, err := time.Parse(time.RFC3339, sendTime)
	if err != nil {
		return platform.ResolvedWindow{}, fmt.Errorf("sent message has no valid send time: %w", err)
	}
	sent = sent.In(location)
	start := time.Date(sent.Year(), sent.Month(), sent.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -1)
	end := klaviyoWindowNow().In(location)
	window, err := platform.ResolveWindow(platform.WindowRequest{Start: &start, End: &end, Timezone: location.String()}, platform.WindowPolicy{
		MaximumDays: 364, ProviderMaximumName: "klaviyo_reporting_364_days", EndExclusive: true,
	})
	if err != nil {
		return window, err
	}
	bounded, clamped := clampKlaviyoAnalyticsWindow(window.Start, window.End)
	if window.Clamped || clamped {
		window.Start = time.Date(bounded.Year(), bounded.Month(), bounded.Day(), 0, 0, 0, 0, location)
		if window.Start.Before(bounded) {
			window.Start = window.Start.AddDate(0, 0, 1)
		}
		window.Clamped, window.ClampReason = true, "requested_window_exceeds_provider_maximum"
	}
	return window, nil
}
